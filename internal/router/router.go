package router

import "github.com/gin-gonic/gin"

func SetupRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"message": "pong",
		})
	})
	registerHealthCheckRoutes(router)
	registerCheckRoutes(router)
	// registerUserRoutes(router)
	// registerJournalRoutes(router)
	return router
}
