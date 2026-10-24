package router

import (
	"rate-limit-service/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerCheckRoutes(router *gin.Engine) {
	router.POST("/check", handler.Check)
}
