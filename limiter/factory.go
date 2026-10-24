package limiter

func NewLimiter(algorithm string) Limiter {
	switch algorithm {
	case "token_bucket":
		return &TokenBucketLimiter{}
	default:
		return &TokenBucketLimiter{}
	}
}
