package handler

import (
	"context"
	"net/http"
	"rate-limit-service/internal/dto"
	"rate-limit-service/pkg/response"
	"rate-limit-service/store"
	"time"

	"github.com/gin-gonic/gin"
)

func HealthCheck(ctx *gin.Context) {
	healthResponse := dto.HealthResponse{
		Timestamp:   time.Now().Format(time.RFC1123),
		DBStatus:    "up",
		RedisStatus: "up",
	}

	db, err := store.DB.DB()
	if err != nil || db.Ping() != nil {
		healthResponse.DBStatus = "down"
	}

	if err := store.RedisClient.Ping(context.Background()).Err(); err != nil {
		healthResponse.RedisStatus = "down"
	}

	if healthResponse.DBStatus == "down" || healthResponse.RedisStatus == "down" {
		healthResponse.Message = "One or more services are down."
		response.Error(ctx, http.StatusInternalServerError, healthResponse.Message, "health check failed")
		return
	}

	healthResponse.Message = "All services are up and running."
	response.Success(ctx, http.StatusOK, "Server is up and running.", healthResponse)
}
