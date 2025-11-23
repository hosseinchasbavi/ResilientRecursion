// File: cmd/server/main.go
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// Request represents incoming calculation request
type Request struct {
	R float64 `json:"r"`
	N int     `json:"n"`
}

type Response struct {
	R      float64 `json:"r"`
	N      int     `json:"n"`
	Result float64 `json:"result"`
}

// L1Cache implements ring buffer LRU cache
type L1Cache struct {
	entries map[uint64]map[int]float64 // r_hash -> {n -> value}
	keys    []uint64                   // ring buffer
	size    int
	head    int
	mu      sync.RWMutex
}

func NewL1Cache(size int) *L1Cache {
	return &L1Cache{
		entries: make(map[uint64]map[int]float64),
		keys:    make([]uint64, size),
		size:    size,
	}
}

func (c *L1Cache) Get(rHash uint64, n int) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if series, ok := c.entries[rHash]; ok {
		if val, exists := series[n]; exists {
			return val, true
		}
	}
	return 0, false
}

func (c *L1Cache) Set(rHash uint64, n int, val float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[rHash]; !ok {
		// Evict oldest entry if full
		if len(c.entries) >= c.size {
			oldKey := c.keys[c.head]
			delete(c.entries, oldKey)
		}
		c.entries[rHash] = make(map[int]float64)
		c.keys[c.head] = rHash
		c.head = (c.head + 1) % c.size
	}
	c.entries[rHash][n] = val
}

func (c *L1Cache) GetSeries(rHash uint64) map[int]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entries[rHash]
}

func (c *L1Cache) GetAllEntries() map[uint64]map[int]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snapshot := make(map[uint64]map[int]float64)
	for k, v := range c.entries {
		snapshot[k] = make(map[int]float64)
		for n, val := range v {
			snapshot[k][n] = val
		}
	}
	return snapshot
}

// ComputeEngine handles sequence calculations
type ComputeEngine struct {
	l1Cache       *L1Cache
	redisClient   *redis.Client
	checkpointMod int
	podID         string
	totalPods     int
}

func NewComputeEngine(redisAddr, podID string, totalPods int) *ComputeEngine {
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolSize:     10,
	})

	return &ComputeEngine{
		l1Cache:       NewL1Cache(75),
		redisClient:   rdb,
		checkpointMod: 1000,
		podID:         podID,
		totalPods:     totalPods,
	}
}

// hashFloat64 converts float64 to deterministic hash
func hashFloat64(r float64) uint64 {
	bits := math.Float64bits(r)
	return bits
}

// getPodForR determines which pod owns this r value
func (e *ComputeEngine) getPodForR(rHash uint64) int {
	h := fnv.New32a()
	binary.Write(h, binary.LittleEndian, rHash)
	return int(h.Sum32() % uint32(e.totalPods))
}

// isLocalR checks if this pod owns the r value
func (e *ComputeEngine) isLocalR(rHash uint64) bool {
	return e.getPodForR(rHash) == parsePodID(e.podID)
}

func parsePodID(podID string) int {
	// Extract numeric suffix from pod name (e.g., "pod-2" -> 2)
	var id int
	fmt.Sscanf(podID, "pod-%d", &id)
	return id
}

// Compute calculates x_n for given r and n
func (e *ComputeEngine) Compute(ctx context.Context, r float64, n int) (float64, error) {
	rHash := hashFloat64(r)

	// Check L1 cache
	if val, ok := e.l1Cache.Get(rHash, n); ok {
		return val, nil
	}

	// Check if we should forward to another pod
	if !e.isLocalR(rHash) {
		// In production, this would use gRPC to forward to correct pod
		// For now, compute locally but mark as non-optimal
		log.Printf("Warning: Computing non-local r=%.6f (should be on pod %d)", r, e.getPodForR(rHash))
	}

	// Try Redis checkpoint lookup
	checkpoint, startN := e.findNearestCheckpoint(ctx, rHash, n)

	var x float64
	var computeFrom int

	if checkpoint != nil {
		x = *checkpoint
		computeFrom = startN
	} else {
		x = 0.5 // x_0
		computeFrom = 0
	}

	// Sequential computation
	for i := computeFrom; i < n; i++ {
		x = r * x * (1 - x)
		e.l1Cache.Set(rHash, i+1, x)

		// Store checkpoint
		if (i+1)%e.checkpointMod == 0 {
			e.storeCheckpoint(ctx, rHash, i+1, x)
		}
	}

	return x, nil
}

func (e *ComputeEngine) findNearestCheckpoint(ctx context.Context, rHash uint64, n int) (*float64, int) {
	key := fmt.Sprintf("cp:%d", rHash)

	// Find largest checkpoint <= n
	result, err := e.redisClient.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%d", n),
		Offset: 0,
		Count:  1,
	}).Result()

	if err != nil || len(result) == 0 {
		return nil, 0
	}

	val := result[0].Score
	checkpointN := int(val)

	// Parse stored value
	var x float64
	fmt.Sscanf(result[0].Member.(string), "%f", &x)

	return &x, checkpointN
}

func (e *ComputeEngine) storeCheckpoint(ctx context.Context, rHash uint64, n int, x float64) {
	key := fmt.Sprintf("cp:%d", rHash)
	member := fmt.Sprintf("%.15e", x) // Use scientific notation for precision

	// Store with expiration of 1 hour
	pipe := e.redisClient.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(n), Member: member})
	pipe.Expire(ctx, key, time.Hour)
	_, err := pipe.Exec(ctx)

	if err != nil {
		log.Printf("Redis checkpoint error: %v", err)
	}
}

// PreheatCache loads recent checkpoints on startup
func (e *ComputeEngine) PreheatCache(ctx context.Context) {
	log.Println("Preheating cache from Redis...")

	// Scan for checkpoint keys
	iter := e.redisClient.Scan(ctx, 0, "cp:*", 50).Iterator()
	loaded := 0

	for iter.Next(ctx) {
		key := iter.Val()
		var rHash uint64
		fmt.Sscanf(key, "cp:%d", &rHash)

		// Get latest checkpoint
		result, err := e.redisClient.ZRevRangeWithScores(ctx, key, 0, 0).Result()
		if err != nil || len(result) == 0 {
			continue
		}

		n := int(result[0].Score)
		var x float64
		fmt.Sscanf(result[0].Member.(string), "%f", &x)

		e.l1Cache.Set(rHash, n, x)
		loaded++

		if loaded >= 50 {
			break
		}
	}

	log.Printf("Preheated %d entries", loaded)
}

// FlushToRedis saves L1 cache to Redis on shutdown
func (e *ComputeEngine) FlushToRedis(ctx context.Context) {
	log.Println("Flushing L1 cache to Redis...")

	entries := e.l1Cache.GetAllEntries()
	pipe := e.redisClient.Pipeline()
	count := 0

	for rHash, series := range entries {
		for n, x := range series {
			if n%e.checkpointMod == 0 {
				key := fmt.Sprintf("cp:%d", rHash)
				member := fmt.Sprintf("%.15e", x)
				pipe.ZAdd(ctx, key, redis.Z{Score: float64(n), Member: member})
				pipe.Expire(ctx, key, time.Hour)
				count++
			}
		}
	}

	if count > 0 {
		_, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("Flush error: %v", err)
		} else {
			log.Printf("Flushed %d checkpoints", count)
		}
	}
}

// Server handles HTTP requests
type Server struct {
	engine *ComputeEngine
	server *http.Server
}

func NewServer(port string, engine *ComputeEngine) *Server {
	s := &Server{engine: engine}

	mux := http.NewServeMux()
	mux.HandleFunc("/calculate", s.handleCalculate)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s
}

func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requests []Request
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Group by r and sort n values
	grouped := make(map[float64][]int)
	for _, req := range requests {
		grouped[req.R] = append(grouped[req.R], req.N)
	}

	for r := range grouped {
		sort.Ints(grouped[r])
	}

	// Compute results
	ctx := r.Context()
	responses := make([]Response, 0, len(requests))

	for r, nValues := range grouped {
		for _, n := range nValues {
			result, err := s.engine.Compute(ctx, r, n)
			if err != nil {
				log.Printf("Compute error: %v", err)
				continue
			}
			responses = append(responses, Response{R: r, N: n, Result: result})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) Start() error {
	log.Printf("Starting server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func main() {
	// Configuration from environment
	port := getEnv("PORT", "2586")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	podID := getEnv("POD_ID", "pod-0")
	totalPods := getEnvInt("TOTAL_PODS", 3)

	// Initialize engine
	engine := NewComputeEngine(redisAddr, podID, totalPods)

	// Preheat cache
	ctx := context.Background()
	engine.PreheatCache(ctx)

	// Start server
	server := NewServer(port, engine)

	// Graceful shutdown
	go func() {
		if err := server.Start(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")

	// Flush cache before shutdown
	engine.FlushToRedis(ctx)

	// Shutdown server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	// Close Redis connection
	engine.redisClient.Close()

	log.Println("Shutdown complete")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		fmt.Sscanf(value, "%d", &i)
		return i
	}
	return fallback
}
