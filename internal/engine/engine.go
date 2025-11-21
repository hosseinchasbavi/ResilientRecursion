package engine

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/yourusername/sequence-calc/internal/cache"
)

type ComputeEngine struct {
    l1Cache       *cache.L1Cache
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
        l1Cache:       cache.NewL1Cache(75),
        redisClient:   rdb,
        checkpointMod: 1000,
        podID:         podID,
        totalPods:     totalPods,
    }
}

func (e *ComputeEngine) Compute(ctx context.Context, r float64, n int) (float64, error) {
    rHash := HashFloat64(r)

    if val, ok := e.l1Cache.Get(rHash, n); ok {
        return val, nil
    }

    if !e.isLocalR(rHash) {
        log.Printf("Warning: Computing non-local r=%.6f", r)
    }

    checkpoint, startN := e.findNearestCheckpoint(ctx, rHash, n)
    
    var x float64
    var computeFrom int

    if checkpoint != nil {
        x = *checkpoint
        computeFrom = startN
    } else {
        x = 0.5
        computeFrom = 0
    }

    for i := computeFrom; i < n; i++ {
        x = r * x * (1 - x)
        e.l1Cache.Set(rHash, i+1, x)

        if (i+1)%e.checkpointMod == 0 {
            e.storeCheckpoint(ctx, rHash, i+1, x)
        }
    }

    return x, nil
}

func (e *ComputeEngine) isLocalR(rHash uint64) bool {
    return GetPodForR(rHash, e.totalPods) == ParsePodID(e.podID)
}

func (e *ComputeEngine) findNearestCheckpoint(ctx context.Context, rHash uint64, n int) (*float64, int) {
    key := fmt.Sprintf("cp:%d", rHash)
    
    result, err := e.redisClient.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
        Min:    "0",
        Max:    fmt.Sprintf("%d", n),
        Offset: 0,
        Count:  1,
    }).Result()

    if err != nil || len(result) == 0 {
        return nil, 0
    }

    checkpointN := int(result[0].Score)
    var x float64
    fmt.Sscanf(result[0].Member.(string), "%f", &x)
    
    return &x, checkpointN
}

func (e *ComputeEngine) storeCheckpoint(ctx context.Context, rHash uint64, n int, x float64) {
    key := fmt.Sprintf("cp:%d", rHash)
    member := fmt.Sprintf("%.15e", x)
    
    pipe := e.redisClient.Pipeline()
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(n), Member: member})
    pipe.Expire(ctx, key, time.Hour)
    pipe.Exec(ctx)
}

func (e *ComputeEngine) PreheatCache(ctx context.Context) {
    log.Println("Preheating cache...")
    iter := e.redisClient.Scan(ctx, 0, "cp:*", 50).Iterator()
    loaded := 0
    
    for iter.Next(ctx) {
        key := iter.Val()
        var rHash uint64
        fmt.Sscanf(key, "cp:%d", &rHash)
        
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

func (e *ComputeEngine) FlushToRedis(ctx context.Context) {
    log.Println("Flushing cache...")
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
        pipe.Exec(ctx)
        log.Printf("Flushed %d checkpoints", count)
    }
}

func (e *ComputeEngine) Close() {
    e.redisClient.Close()
}