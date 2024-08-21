package redis

import (
	"context"
	"encoding/json"
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

func StoreSummarizes(ctx context.Context, key string, value map[string]interface{}, stringified string, duration time.Duration) error {
	var err error
	var result string
	var temp []byte
	if stringified == "" {
		temp, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to store summarizes in Redis: %v", err)
		}
		result = string(temp)
	} else {
		result = stringified
	}
	err = historyClient.Set(ctx, key, result, duration).Err()
	if err != nil {
		return fmt.Errorf("failed to store key %s in Redis: %v", key, err)
	}
	return nil
}

func GetSummarizes(ctx context.Context, key string) (string, bool, error) {
	value, err := historyClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, fmt.Errorf("key %s does not exist", key)
	} else if err != nil {
		return "", false, fmt.Errorf("failed to get key %s from Redis: %v", key, err)
	}
	var data map[string]interface{}
	err = json.Unmarshal([]byte(value), &data)
	if err != nil {
		return value, false, nil
	}
	return data["value"].(string), data["addIt"].(bool), nil
}
