package main

import (
	"github.com/go-redis/redis"
	"time"
)

type RunCache interface {
	Get(key string) (string, error)
	Set(key string, value string) (bool, error)
}

type RedisCache struct {
	rc     *redis.Client
	config RedisConfig
}

func NewRedisCache(redisConfig RedisConfig) *RedisCache {
	rc := redis.NewClient(&redis.Options{
		Addr:     redisConfig.Addr,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})
	return &RedisCache{rc: rc,
		config: redisConfig}
}

func (c *RedisCache) Set(key string, value string) (bool, error) {
	return c.rc.SetNX(key, value, time.Minute*c.config.TTL).Result()
}

func (c *RedisCache) Get(key string) (string, error) {
	return c.rc.Get(key).Result()
}
