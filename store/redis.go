package store

import (
	"context"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var TokenBucketSHA string

func ConnectRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_URL"),
	})

	_, err := RedisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis, error: %v", err)
	}

	log.Println("Connected to Redis successfully.")
}

func LoadScripts() {
	script, err := os.ReadFile("limiter/scripts/token_bucket.lua")
	if err != nil {
		log.Fatalf("Failed to read token bucket script: %v", err)
	}

	sha, err := RedisClient.ScriptLoad(context.Background(), string(script)).Result()
	if err != nil {
		log.Fatalf("Failed to load token bucket script: %v", err)
	}

	TokenBucketSHA = sha
	log.Println("Lua scripts loaded successfully.")
}
