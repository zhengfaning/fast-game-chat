package repository

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/go-redis/redis/v8"
)

type RedisClient struct {
    Client *redis.Client
}

func NewRedisClient(addr, password string) (*RedisClient, error) {
    rdb := redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       0,
        DialTimeout: 5 * time.Second,
    })

    if err := rdb.Ping(context.Background()).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to redis: %w", err)
    }

    log.Println("Connected to Redis")
    return &RedisClient{Client: rdb}, nil
}

func (r *RedisClient) Close() {
    r.Client.Close()
}
