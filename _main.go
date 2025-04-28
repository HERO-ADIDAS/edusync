package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"edusync/auth"
	"edusync/db"
	"edusync/handlers"
	"edusync/utils"
)

func main() {
	// Initialize environment variables with defaults
	utils.SetDefaultEnvVars()

	// Initialize all database connections
	if err := db.InitDatabaseConnections(); err != nil {
		log.Fatalf("Failed to initialize database connections: %v", err)
	}
	defer db.CloseConnections()

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Custom middleware to set database connection based on role
	router.Use(func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			role = "root" // Default to root if role not set (e.g., before auth)
		}
		dbConn := db.GetDBForRole(role.(string))
		if dbConn == nil {
			log.Printf("No valid database connection for role: %v", role)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			c.Abort()
			return
		}
		c.Set("db", dbConn)
		c.Next()
	})

	// API routes
	api := router.Group("/api")
	{
		api.POST("/register", handlers.RegisterHandler)
		api.POST("/login", handlers.LoginHandler)
	}

	// Protected API routes
	protected := router.Group("/api")
	protected.Use(auth.AuthMiddleware())
	{
		protected.GET("/profile", handlers.GetProfileHandler)
		protected.GET("/checkauth", handlers.CheckAuthHandler)

		// Teacher-specific routes
		protected.GET("/teacher/profile", handlers.GetTeacherProfileHandler)
		protected.PUT("/teacher/profile", handlers.UpdateTeacherProfileHandler)
		protected.GET("/teacher/dashboard", handlers.GetTeacherDashboardHandler)
		protected.GET("/teacher/stats", handlers.GetTeacherStatsHandler)

		// Classroom-related routes
		protected.GET("/classrooms", handlers.GetTeacherClassroomsHandler)
		protected.POST("/classrooms", handlers.CreateClassroomHandler)
		protected.GET("/classrooms/:id", handlers.GetClassroomDetailsHandler)
		protected.PUT("/classrooms/:id", handlers.UpdateClassroomHandler)
		protected.DELETE("/classrooms/:id", handlers.DeleteClassroomHandler)

		// Assignment-related routes
		protected.POST("/assignments", handlers.CreateAssignmentHandler)
		protected.PUT("/assignments/:id", handlers.UpdateAssignmentHandler)
		protected.DELETE("/assignments/:id", handlers.DeleteAssignmentHandler)
		protected.GET("/classrooms/:id/assignments", handlers.GetAssignmentsByClassroomHandler)
		protected.GET("/assignments/:id", handlers.GetAssignmentDetailsHandler)

		// Submission-related routes
		protected.POST("/submissions", handlers.CreateSubmissionHandler)
		protected.GET("/submissions/:id", handlers.GetSubmissionHandler)
		protected.GET("/assignments/:id/submissions", handlers.GetAssignmentSubmissionsHandler)
		protected.POST("/submissions/:id/grade", handlers.GradeSubmissionHandler)
		protected.POST("/assignments/:id/bulk-grade", handlers.BulkGradeSubmissionsHandler)

		protected.POST("/materials", handlers.CreateMaterialHandler)
		protected.GET("/classrooms/:id/materials", handlers.GetMaterialsByCourseHandler)
		protected.GET("/materials/:id", handlers.GetMaterialHandler)
		protected.PUT("/materials/:id", handlers.UpdateMaterialHandler)
		protected.DELETE("/materials/:id", handlers.DeleteMaterialHandler)
	}

	// Catch-all for 404 errors to log unhandled routes
	router.NoRoute(func(c *gin.Context) {
		log.Printf("404 Not Found: %s %s", c.Request.Method, c.Request.URL.Path)
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server running on port %s\n", port)
	log.Fatal(router.Run(":" + port))
}