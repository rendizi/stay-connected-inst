package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/rendizi/stay-connected-inst/config"
	"github.com/rendizi/stay-connected-inst/pkg/logger"
	"log"
	"time"
)

var historyClient *redis.Client
var cookiesClient *redis.Client

func init() {
	historyClient = redis.NewClient(&redis.Options{
		Addr:     config.Config.RedisHistoryAddress,
		Password: config.Config.RedisHistoryPassword,
		DB:       0,
	})
	_, err := historyClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to connect to Redis: %v", err))
		log.Fatal()
	}

	cookiesClient = redis.NewClient(&redis.Options{
		Addr:     config.Config.RedisCookiesAddress,
		Password: config.Config.RedisCookiesPassword,
		DB:       0,
	})
	_, err = cookiesClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to connect to Redis: %v", err))
		log.Fatal()
	}
}

func GetCookies(ctx context.Context, username string) (string, error) {
	cookies, err := cookiesClient.Get(ctx, username).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("user %s does not exist", username)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get cookies from Redis: %v", err)
	}
	return cookies, nil
}

func StoreCookies(ctx context.Context, username string, cookies string) error {
	err := cookiesClient.Set(ctx, username, cookies, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store cookies in Redis: %v", err)
	}
	return nil
}

func StoreSummarizes(ctx context.Context, key string, value string, duration time.Duration) error {
	err := historyClient.Set(ctx, key, value, duration).Err()
	if err != nil {
		return fmt.Errorf("failed to store key %s in Redis: %v", key, err)
	}
	return nil
}

func GetSummarizes(ctx context.Context, key string) (string, error) {
	value, err := historyClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key %s does not exist", key)
	} else if err != nil {
		return "", fmt.Errorf("failed to get key %s from Redis: %v", key, err)
	}
	return value, nil
}
