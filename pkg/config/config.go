package config

import (
    "fmt"
    "os"
)

type Config struct {
    Port      string
    RedisAddr string
    PodID     string
    TotalPods int
}

func Load() *Config {
    return &Config{
        Port:      getEnv("PORT", "8080"),
        RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"),
        PodID:     getEnv("POD_ID", "pod-0"),
        TotalPods: getEnvInt("TOTAL_PODS", 3),
    }
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