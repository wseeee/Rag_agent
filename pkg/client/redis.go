package client

import (
	cfg "SuperBizAgent/internal/config"
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	redisOnce   sync.Once
)

// NewRedisClient 根据配置创建新的 Redis 客户端实例
func NewRedisClient() *redis.Client {
	addr := cfg.C.Redis.Addr
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.C.Redis.Password,
		DB:       cfg.C.Redis.DB,
	})
}

// GetRedisClient 获取全局单例 Redis 客户端连接池
func GetRedisClient() *redis.Client {
	redisOnce.Do(func() {
		redisClient = NewRedisClient()
	})
	return redisClient
}
