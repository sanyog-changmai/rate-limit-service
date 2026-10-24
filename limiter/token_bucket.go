package limiter

import (
	"context"
	"rate-limit-service/model"
	"rate-limit-service/store"
	"strings"
)

type TokenBucketLimiter struct{}

func (t *TokenBucketLimiter) Allow(ctx context.Context, key string, policy model.APIKey) (Result, error) {
	timeResult, err := store.RedisClient.Time(ctx).Result()
	if err != nil {
		return Result{}, err
	}
	nowMicros := timeResult.UnixMicro()

	res, err := evalTokenBucket(ctx, key, policy.Rate, float64(policy.Burst), nowMicros)
	if err != nil {
		return Result{}, err
	}

	values := res.([]interface{})
	allowed := values[0].(int64) == 1
	remaining := int(values[1].(int64))
	limit := int(values[2].(int64))
	retryAfter := values[3].(int64)

	resetAt := timeResult.Unix() + retryAfter

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		Limit:      limit,
		ResetAt:    resetAt,
		RetryAfter: float64(retryAfter),
	}, nil
}

func evalTokenBucket(ctx context.Context, key string, rate float64, burst float64, nowMicros int64) (interface{}, error) {
	res, err := store.RedisClient.EvalSha(ctx, store.TokenBucketSHA,
		[]string{key}, rate, burst, nowMicros,
	).Result()

	if err != nil && strings.Contains(err.Error(), "NOSCRIPT") {
		store.LoadScripts()
		res, err = store.RedisClient.EvalSha(ctx, store.TokenBucketSHA,
			[]string{key}, rate, burst, nowMicros,
		).Result()
	}

	return res, err
}
