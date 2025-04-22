package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// Global database connections
var (
	rootDB    *sql.DB // For registration and admin operations
	studentDB *sql.DB // For student role
	teacherDB *sql.DB // For teacher role
)

// DatabaseConfig holds connection information
type DatabaseConfig struct {
	User     string
	Password string
	Host     string
	DBName   string
}

// User represents the user data structure
type User struct {
	ID         int    `json:"id,omitempty"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Password   string `json:"password,omitempty"`
	Role       string `json:"role"`
	ContactNum string `json:"contact_number,omitempty"`
	ProfilePic string `json:"profile_picture,omitempty"`
	Org        string `json:"org,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// Student specific data
type Student struct {
	StudentID      int    `json:"student_id,omitempty"`
	UserID         int    `json:"user_id,omitempty"`
	GradeLevel     string `json:"grade_level"`
	EnrollmentYear int    `json:"enrollment_year"`
}

// Teacher specific data
type Teacher struct {
	TeacherID int    `json:"teacher_id,omitempty"`
	UserID    int    `json:"user_id,omitempty"`
	Dept      string `json:"dept"`
}

// RegisterRequest combines user data with role-specific data
type RegisterRequest struct {
	User    User     `json:"user"`
	Student *Student `json:"student,omitempty"`
	Teacher *Teacher `json:"teacher,omitempty"`
}

// LoginRequest contains login credentials
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse contains JWT token and user info
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Claims for JWT token
type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func main() {
	// Set default values if environment variables are not set
	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost:3306")
	}
	if os.Getenv("DB_NAME") == "" {
		os.Setenv("DB_NAME", "edusync_db")
	}
	if os.Getenv("DB_ROOT_PASSWORD") == "" {
		os.Setenv("DB_ROOT_PASSWORD", "adidas") // Default for development
	}
	if os.Getenv("DB_STUDENT_PASSWORD") == "" {
		os.Setenv("DB_STUDENT_PASSWORD", "student") // Default for development
	}
	if os.Getenv("DB_TEACHER_PASSWORD") == "" {
		os.Setenv("DB_TEACHER_PASSWORD", "teacher") // Default for development
	}
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "3f8a3d6ea42995bcb8003a4d85a62c93cfae6f3bc8fbdf71817a0bda7c054cb3")
	}

	// Initialize all database connections
	if err := initDatabaseConnections(); err != nil {
		log.Fatalf("Failed to initialize database connections: %v", err)
	}
	defer func() {
		rootDB.Close()
		studentDB.Close()
		teacherDB.Close()
	}()

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

	// API routes
	api := router.Group("/api")
	{
		api.POST("/register", registerHandler)
		api.POST("/login", loginHandler)
	}

	// Protected API routes
	protected := router.Group("/api")
	protected.Use(authMiddleware())
	{
		protected.GET("/profile", getProfileHandler)
		protected.GET("/checkauth", checkAuthHandler)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server running on port %s\n", port)
	log.Fatal(router.Run(":" + port))
}

// initDatabaseConnections initializes all required database connections
func initDatabaseConnections() error {
	// Root connection for registration
	rootConfig := DatabaseConfig{
		User:     "root",
		Password: os.Getenv("DB_ROOT_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		DBName:   os.Getenv("DB_NAME"),
	}

	// Student connection
	studentConfig := DatabaseConfig{
		User:     "student",
		Password: os.Getenv("DB_STUDENT_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		DBName:   os.Getenv("DB_NAME"),
	}

	// Teacher connection
	teacherConfig := DatabaseConfig{
		User:     "TEACHER",
		Password: os.Getenv("DB_TEACHER_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		DBName:   os.Getenv("DB_NAME"),
	}

	// Initialize connections
	var err error
	rootDB, err = createDBConnection(rootConfig)
	if err != nil {
		return fmt.Errorf("failed to connect as root: %v", err)
	}
	log.Println("Root database connection established")

	studentDB, err = createDBConnection(studentConfig)
	if err != nil {
		log.Printf("Warning: failed to connect as student: %v", err)
		// Don't return error as this might be a new setup without student user yet
	} else {
		log.Println("Student database connection established")
	}

	teacherDB, err = createDBConnection(teacherConfig)
	if err != nil {
		log.Printf("Warning: failed to connect as teacher: %v", err)
		// Don't return error as this might be a new setup without teacher user yet
	} else {
		log.Println("Teacher database connection established")
	}

	return nil
}

// createDBConnection creates a database connection with the given config
func createDBConnection(config DatabaseConfig) (*sql.DB, error) {
	dbConfig := mysql.Config{
		User:                 config.User,
		Passwd:               config.Password,
		Net:                  "tcp",
		Addr:                 config.Host,
		DBName:               config.DBName,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	db, err := sql.Open("mysql", dbConfig.FormatDSN())
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 3)

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// getDBForRole returns the appropriate database connection for the given role
func getDBForRole(role string) *sql.DB {
	switch role {
	case "student":
		if studentDB != nil {
			return studentDB
		}
		// Fallback to root if student connection isn't available
		return rootDB
	case "teacher":
		if teacherDB != nil {
			return teacherDB
		}
		// Fallback to root if teacher connection isn't available
		return rootDB
	default:
		// Admin and dev roles use the root connection
		return rootDB
	}
}

// registerHandler uses root connection for all registrations
func registerHandler(c *gin.Context) {
	// Parse request body
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate input
	if req.User.Email == "" || req.User.Password == "" || req.User.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name, email and password are required"})
		return
	}

	// Validate role
	if req.User.Role != "student" && req.User.Role != "teacher" && req.User.Role != "admin" && req.User.Role != "dev" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
		return
	}

	// Always use rootDB for registration
	// Check if user with email already exists
	var count int
	err := rootDB.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", req.User.Email).Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error checking email existence: %v", err)
		return
	}

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.User.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing your request"})
		log.Printf("Error hashing password: %v", err)
		return
	}

	// Begin transaction
	tx, err := rootDB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback() // Will be ignored if tx.Commit() is called

	// Insert into user table
	result, err := tx.Exec(
		"INSERT INTO user (name, email, password, role, contact_number, profile_picture, org) VALUES (?, ?, ?, ?, ?, ?, ?)",
		req.User.Name, req.User.Email, hashedPassword, req.User.Role, req.User.ContactNum, req.User.ProfilePic, req.User.Org,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error registering user"})
		log.Printf("Error inserting user: %v", err)
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving user ID"})
		log.Printf("Error getting last insert ID: %v", err)
		return
	}

	// Insert role-specific data
	switch req.User.Role {
	case "student":
		if req.Student == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Student data is required for student role"})
			return
		}

		_, err := tx.Exec(
			"INSERT INTO student (user_id, grade_level, enrollment_year) VALUES (?, ?, ?)",
			userID, req.Student.GradeLevel, req.Student.EnrollmentYear,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error registering student"})
			log.Printf("Error inserting student data: %v", err)
			return
		}

	case "teacher":
		if req.Teacher == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Teacher data is required for teacher role"})
			return
		}

		_, err := tx.Exec(
			"INSERT INTO teacher (user_id, dept) VALUES (?, ?)",
			userID, req.Teacher.Dept,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error registering teacher"})
			log.Printf("Error inserting teacher data: %v", err)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error committing transaction: %v", err)
		return
	}

	// Return success response
	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
}

// loginHandler handles user login
func loginHandler(c *gin.Context) {
	// Parse request body
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	// Use root DB to get user initially (to determine role)
	var user User
	var hashedPassword string
	err := rootDB.QueryRow(
		"SELECT user_id, name, email, password, role, contact_number, profile_picture, org, created_at FROM user WHERE email = ?",
		req.Email,
	).Scan(&user.ID, &user.Name, &user.Email, &hashedPassword, &user.Role, &user.ContactNum, &user.ProfilePic, &user.Org, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving user: %v", err)
		}
		return
	}

	// Compare passwords
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Create JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		log.Printf("Error signing token: %v", err)
		return
	}

	// Return token and user info
	user.Password = "" // Don't send password back
	response := LoginResponse{
		Token: tokenString,
		User:  user,
	}

	c.JSON(http.StatusOK, response)
}

// authMiddleware validates JWT tokens
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix if present
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		// Parse and validate token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Add claims to request context
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// getProfileHandler is an example of a protected endpoint
func getProfileHandler(c *gin.Context) {
	// Get user ID and role from context (set by authMiddleware)
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Use the appropriate DB connection based on role
	db := getDBForRole(role)

	// Get user info
	var user User
	err := db.QueryRow(
		"SELECT user_id, name, email, role, contact_number, profile_picture, org, created_at FROM user WHERE user_id = ?",
		userID,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.ContactNum, &user.ProfilePic, &user.Org, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving user profile: %v", err)
		}
		return
	}

	// Get role-specific data
	switch role {
	case "student":
		var student Student
		err := db.QueryRow(
			"SELECT student_id, grade_level, enrollment_year FROM student WHERE user_id = ?",
			userID,
		).Scan(&student.StudentID, &student.GradeLevel, &student.EnrollmentYear)

		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving student data"})
			log.Printf("Error retrieving student data: %v", err)
			return
		}

		// Return user with student data
		c.JSON(http.StatusOK, gin.H{
			"user":    user,
			"student": student,
		})

	case "teacher":
		var teacher Teacher
		err := db.QueryRow(
			"SELECT teacher_id, dept FROM teacher WHERE user_id = ?",
			userID,
		).Scan(&teacher.TeacherID, &teacher.Dept)

		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving teacher data"})
			log.Printf("Error retrieving teacher data: %v", err)
			return
		}

		// Return user with teacher data
		c.JSON(http.StatusOK, gin.H{
			"user":    user,
			"teacher": teacher,
		})

	default:
		// For admin and dev roles, just return user data
		c.JSON(http.StatusOK, user)
	}
}

// checkAuthHandler is a simple endpoint to check if the user is authenticated
func checkAuthHandler(c *gin.Context) {
	// This endpoint just returns info about the authenticated user
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user_id":       userID,
		"role":          role,
		"db_connection": role, // Indicates which DB connection would be used
	})
}