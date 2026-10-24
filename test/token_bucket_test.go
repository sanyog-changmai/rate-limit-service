package integration

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"rate-limit-service/limiter"
	"rate-limit-service/model"
	"rate-limit-service/store"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func TestMain(m *testing.M) {
	// .env is at project root, tests run from test/
	_ = godotenv.Load("../.env")

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	store.RedisClient = redis.NewClient(&redis.Options{Addr: redisURL})

	ctx := context.Background()
	if _, err := store.RedisClient.Ping(ctx).Result(); err != nil {
		fmt.Printf("Redis not available at %s: %v\n", redisURL, err)
		os.Exit(1)
	}

	script, err := os.ReadFile("../limiter/scripts/token_bucket.lua")
	if err != nil {
		fmt.Printf("Failed to read token_bucket.lua: %v\n", err)
		os.Exit(1)
	}

	sha, err := store.RedisClient.ScriptLoad(ctx, string(script)).Result()
	if err != nil {
		fmt.Printf("Failed to load Lua script into Redis: %v\n", err)
		os.Exit(1)
	}
	store.TokenBucketSHA = sha

	os.Exit(m.Run())
}

// TestConcurrentBurst fires 200 goroutines simultaneously against a bucket
// with burst=100. Exactly 100 should be allowed and 100 denied.
// The Lua script's atomicity is what makes this deterministic — run with -race.
func TestConcurrentBurst(t *testing.T) {
	key := fmt.Sprintf("test:concurrent:%d", time.Now().UnixNano())
	ctx := context.Background()

	policy := model.APIKey{
		Rate:  0.1, // very slow refill so no token is added during the test
		Burst: 100,
	}

	l := limiter.NewLimiter("token_bucket")

	var allowed, denied int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := l.Allow(ctx, key, policy)
			if err != nil {
				return
			}
			if result.Allowed {
				atomic.AddInt64(&allowed, 1)
			} else {
				atomic.AddInt64(&denied, 1)
			}
		}()
	}

	close(start)
	wg.Wait()

	t.Cleanup(func() { store.RedisClient.Del(ctx, key) })

	if allowed != 100 {
		t.Errorf("expected exactly 100 allowed, got %d", allowed)
	}
	if denied != 100 {
		t.Errorf("expected exactly 100 denied, got %d", denied)
	}
}

// TestTokenRefill drains a bucket, confirms denial, waits for refill, then
// confirms the refilled tokens are granted.
func TestTokenRefill(t *testing.T) {
	key := fmt.Sprintf("test:refill:%d", time.Now().UnixNano())
	ctx := context.Background()

	policy := model.APIKey{
		Rate:  1.0, // 1 token per second
		Burst: 5,
	}

	l := limiter.NewLimiter("token_bucket")

	t.Cleanup(func() { store.RedisClient.Del(ctx, key) })

	// drain the bucket
	for i := 0; i < 5; i++ {
		result, err := l.Allow(ctx, key, policy)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("request %d should be allowed, bucket not yet empty", i+1)
		}
	}

	// bucket is empty — next request must be denied
	result, err := l.Allow(ctx, key, policy)
	if err != nil {
		t.Fatalf("unexpected error on empty bucket check: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial when bucket is empty, got allowed")
	}

	// wait 3 seconds — at rate=1 token/sec, 3 tokens should refill
	time.Sleep(3 * time.Second)

	for i := 0; i < 3; i++ {
		result, err := l.Allow(ctx, key, policy)
		if err != nil {
			t.Fatalf("post-refill request %d: unexpected error: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("post-refill request %d should be allowed after 3s refill", i+1)
		}
	}

	// bucket should now be empty again
	result, err = l.Allow(ctx, key, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial after draining refilled tokens")
	}
}
