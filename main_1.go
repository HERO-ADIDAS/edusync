package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"edusync/auth"
	"edusync/db"
	"edusync/handlers" // Your custom handlers package

	gorillaHandlers "github.com/gorilla/handlers" // Alias for CORS package
	"github.com/gorilla/mux"
)

func main() {
	// Set default environment variables
	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost:3306")
	}
	if os.Getenv("DB_NAME") == "" {
		os.Setenv("DB_NAME", "edusync_db")
	}
	if os.Getenv("DB_ROOT_PASSWORD") == "" {
		os.Setenv("DB_ROOT_PASSWORD", "adidas")
	}
	if os.Getenv("DB_STUDENT_PASSWORD") == "" {
		os.Setenv("DB_STUDENT_PASSWORD", "student")
	}
	if os.Getenv("DB_TEACHER_PASSWORD") == "" {
		os.Setenv("DB_TEACHER_PASSWORD", "teacher")
	}
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "3f8a3d6ea42995bcb8003a4d85a62c93cfae6f3bc8fbdf71817a0bda7c054cb3")
	}

	// Initialize database connections
	if err := db.InitDatabaseConnections(); err != nil {
		log.Fatalf("Failed to initialize database connections: %v", err)
	}
	defer db.CloseConnections()

	// Initialize auth with JWT secret
	auth.InitAuth(os.Getenv("JWT_SECRET"))

	// Initialize handlers
	handlers.InitHandlers()

	// Setup router
	router := mux.NewRouter()

	// Public routes
	router.HandleFunc("/api/register", handlers.RegisterHandler).Methods("POST")
	router.HandleFunc("/api/login", handlers.LoginHandler).Methods("POST")

	// Protected routes
	protectedRouter := router.PathPrefix("/api").Subrouter()
	protectedRouter.Use(auth.AuthMiddleware)
	protectedRouter.HandleFunc("/profile", handlers.GetProfileHandler).Methods("GET")
	protectedRouter.HandleFunc("/checkauth", handlers.CheckAuthHandler).Methods("GET")

	// CORS middleware using the aliased package
	corsHandler := gorillaHandlers.CORS(
		gorillaHandlers.AllowedOrigins([]string{"*"}), // Restrict this in production
		gorillaHandlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		gorillaHandlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)(router)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, corsHandler))
}
