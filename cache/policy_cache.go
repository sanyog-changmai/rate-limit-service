package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"rate-limit-service/constants"
	customerrors "rate-limit-service/errors"
	"rate-limit-service/model"
	"rate-limit-service/repository"
	"rate-limit-service/store"
	"strconv"
	"time"
)

var cacheTTL time.Duration

func init() {
	ttl, err := strconv.Atoi(os.Getenv("CACHE_TTL_SECONDS"))
	if err != nil || ttl <= 0 {
		cacheTTL = time.Duration(constants.DefaultCacheTTLSeconds) * time.Second
		return
	}
	cacheTTL = time.Duration(ttl) * time.Second
}

func GetPolicyByKey(apiKey string) (*model.APIKey, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", constants.APIKeyCachePrefix, apiKey)

	cached, err := store.RedisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var policy model.APIKey
		if err := json.Unmarshal([]byte(cached), &policy); err == nil {
			return &policy, nil
		}
	}

	policy, err := repository.GetPolicyByKey(apiKey)
	if err != nil {
		return nil, err
	}

	if policy == nil {
		policy, err = repository.GetDefaultPolicy()
		if err != nil {
			return nil, err
		}
	}

	if policy == nil {
		return nil, customerrors.ErrNoPolicyFound
	}

	data, _ := json.Marshal(policy)
	store.RedisClient.Set(ctx, cacheKey, data, cacheTTL)

	return policy, nil
}

func InvalidatePolicyCache(apiKey string) error {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("%s%s", constants.APIKeyCachePrefix, apiKey)
	return store.RedisClient.Del(ctx, cacheKey).Err()
}

func InvalidateAlgorithmCache() error {
	ctx := context.Background()
	return store.RedisClient.Del(ctx, constants.AlgorithmCacheKey).Err()
}

func GetActiveAlgorithm() (*model.Algorithm, error) {
	ctx := context.Background()
	cacheKey := constants.AlgorithmCacheKey

	cached, err := store.RedisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var algorithm model.Algorithm
		if err := json.Unmarshal([]byte(cached), &algorithm); err == nil {
			return &algorithm, nil
		}
	}

	algorithm, err := repository.GetActiveAlgorithm()
	if err != nil {
		return nil, err
	}

	if algorithm == nil {
		return nil, customerrors.ErrNoActiveAlgorithm
	}

	data, _ := json.Marshal(algorithm)
	store.RedisClient.Set(ctx, cacheKey, data, cacheTTL)

	return algorithm, nil
}
