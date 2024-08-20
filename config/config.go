package config

import (
	"github.com/joho/godotenv"
	"os"
	"sync"
)

type AppConfig struct {
	IsBusy               bool
	isBanned             bool
	Queue                []string
	RedisHistoryAddress  string
	RedisHistoryPassword string
	RedisCookiesAddress  string
	RedisCookiesPassword string
	DefaultLogin         string
	DefaultPassword      string
	OpenAiKey            string
	GeminiKey            string
	Port                 int
	mu                   sync.Mutex
}

type queueLength struct {
	length int
	mu     sync.Mutex
}

var Config AppConfig
var QueueLength queueLength

func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		// panic("Error loading .env file")
	}

	Config = AppConfig{
		IsBusy:               false,
		Queue:                []string{},
		RedisHistoryAddress:  os.Getenv("REDIS_HISTORY_ADDRESS"),
		RedisHistoryPassword: os.Getenv("REDIS_HISTORY_PASSWORD"),
		RedisCookiesAddress:  os.Getenv("REDIS_COOKIES_ADDRESS"),
		RedisCookiesPassword: os.Getenv("REDIS_COOKIES_PASSWORD"),
		DefaultLogin:         os.Getenv("DEFAULT_LOGIN"),
		DefaultPassword:      os.Getenv("DEFAULT_PASSWORD"),
		OpenAiKey:            os.Getenv("OPENAI_KEY"),
		GeminiKey:            os.Getenv("GEMINI_KEY"),
		Port:                 5000, // Default port, update as needed
	}
	QueueLength = queueLength{
		length: 0,
		mu:     sync.Mutex{},
	}
}

func GetQueueLength() int {
	QueueLength.mu.Lock()
	defer QueueLength.mu.Unlock()
	return QueueLength.length
}

func SetBusy() {
	Config.mu.Lock()
	defer Config.mu.Unlock()
	Config.IsBusy = true
}

func UnBusy() {
	Config.mu.Lock()
	defer Config.mu.Unlock()
	Config.IsBusy = false
}

func IsBusy() bool {
	Config.mu.Lock()
	defer Config.mu.Unlock()
	return Config.IsBusy
}

func Enqueue(item string, length int) {
	Config.mu.Lock()
	defer Config.mu.Unlock()
	QueueLength.mu.Lock()
	defer QueueLength.mu.Unlock()
	QueueLength.length += length
	Config.Queue = append(Config.Queue, item)
}

func Dequeue() string {
	Config.mu.Lock()
	defer Config.mu.Unlock()

	if len(Config.Queue) == 0 {
		return ""
	}

	item := Config.Queue[0]
	Config.Queue = Config.Queue[1:]
	return item
}

func RemoveFromQueue(item string, length int) {
	Config.mu.Lock()
	defer Config.mu.Unlock()
	QueueLength.mu.Lock()
	defer QueueLength.mu.Unlock()
	QueueLength.length -= length

	for i, v := range Config.Queue {
		if v == item {
			Config.Queue = append(Config.Queue[:i], Config.Queue[i+1:]...)
		}
	}
}

func NextInQueue() string {
	Config.mu.Lock()
	defer Config.mu.Unlock()

	if len(Config.Queue) == 0 {
		return ""
	}

	return Config.Queue[0]
}
