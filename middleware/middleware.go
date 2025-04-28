package middleware

import (
	"github.com/gin-gonic/gin"
	"log"
)

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("Request: %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
}

func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				c.JSON(500, gin.H{"error": "Internal server error"})
				c.Abort()
			}
		}()
		c.Next()
	}
}

func ApplyMiddleware(r *gin.Engine) {
	r.Use(LoggerMiddleware())
	r.Use(RecoveryMiddleware())
}