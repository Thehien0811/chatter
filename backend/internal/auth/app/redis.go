package app

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thehien0811/chatter/config"
)

func NewRedisClient(cfg *config.AppConfig) *redis.Client {
	if cfg == nil {
		cfg = &config.AppConfig{}
		cfg.Redis.Host = "localhost"
		cfg.Redis.Port = 6379
		cfg.Redis.Password = ""
		cfg.Redis.DB = 0
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return client
	}

	return client
}
