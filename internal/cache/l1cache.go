package cache

import "sync"

type L1Cache struct {
    entries map[uint64]map[int]float64
    keys    []uint64
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