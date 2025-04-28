package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"edusync/config"
	"edusync/db"
	"edusync/middleware"
	"edusync/routes"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	config.ConfigInstance = cfg

	if err := db.InitDatabaseConnection(); err != nil {
		log.Fatalf("Failed to initialize database connection: %v", err)
	}
	defer db.CloseConnection()

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	middleware.ApplyMiddleware(router)

	router.Use(func(c *gin.Context) {
		c.Set("db", db.DB)
		c.Next()
	})

	routes.SetupRoutes(router)

	port := cfg.Port
	fmt.Printf("Server running on port %s\n", port)
	log.Fatal(router.Run(":" + port))
}