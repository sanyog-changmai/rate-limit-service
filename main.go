package main

import (
	"log"
	"net/http"
	"os"
	"rate-limit-service/config"
	"rate-limit-service/internal/router"
	"rate-limit-service/migration"
	"rate-limit-service/store"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.ForceConsoleColor()
	config.LoadConfig()
	store.ConnectDB()
	store.ConnectRedis()
	store.LoadScripts()
	migration.MigrateDB()
}

func main() {
	router := router.SetupRouter()
	port := os.Getenv("PORT")

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("Server is up and running on port: %v", port)
	err := server.ListenAndServe()

	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
