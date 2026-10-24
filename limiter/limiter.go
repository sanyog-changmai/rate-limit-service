package limiter

import (
	"context"
	"rate-limit-service/model"
)

type Result struct {
	Allowed    bool
	Remaining  int
	Limit      int
	ResetAt    int64
	RetryAfter float64
}

type Limiter interface {
	Allow(ctx context.Context, key string, policy model.APIKey) (Result, error)
}
