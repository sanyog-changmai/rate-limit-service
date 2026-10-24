package router

import (
	"rate-limit-service/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerHealthCheckRoutes(router *gin.Engine) {
	health := router.Group("/health")
	{
		health.GET("", handler.HealthCheck)
	}
}
