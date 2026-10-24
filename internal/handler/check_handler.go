package handler

import (
	"fmt"
	"net/http"
	"rate-limit-service/cache"
	"rate-limit-service/internal/dto"
	"rate-limit-service/limiter"
	"rate-limit-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func Check(ctx *gin.Context) {
	var req dto.CheckRequestDto
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}

	policy, err := cache.GetPolicyByKey(req.APIKey)
	if err != nil {
		response.Error(ctx, http.StatusInternalServerError, "Failed to fetch policy", err.Error())
		return
	}

	algorithm, err := cache.GetActiveAlgorithm()
	if err != nil {
		response.Error(ctx, http.StatusInternalServerError, "Failed to fetch algorithm", err.Error())
		return
	}

	key := fmt.Sprintf("tb:%s:%s:%s", req.APIKey, req.Service, req.Route)

	result, err := limiter.NewLimiter(algorithm.Name).Allow(ctx.Request.Context(), key, *policy)
	if err != nil {
		response.Error(ctx, http.StatusInternalServerError, "Rate limit check failed", err.Error())
		return
	}

	response.Success(ctx, http.StatusOK, "check successful", dto.CheckResponseDto{
		Allowed:    result.Allowed,
		Remaining:  result.Remaining,
		Limit:      result.Limit,
		ResetAt:    result.ResetAt,
		RetryAfter: result.RetryAfter,
	})
}
