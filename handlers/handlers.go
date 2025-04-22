package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"edusync/auth"
	"edusync/db"
	"edusync/models"
	"edusync/utils"
)

// RegisterHandler uses root connection for all registrations
func RegisterHandler(c *gin.Context) {
	// Parse request body
	var req models.RegisterRequest
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

	// Validate password strength
	if err := utils.ValidatePassword(req.User.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Always use rootDB for registration
	// Check if user with email already exists
	var count int
	err := db.RootDB.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", req.User.Email).Scan(&count)
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
	tx, err := db.RootDB.Begin()
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

// LoginHandler handles user login
func LoginHandler(c *gin.Context) {
	// Parse request body
	var req models.LoginRequest
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
	var user models.User
	var hashedPassword string
	err := db.RootDB.QueryRow(
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

	// Generate JWT token
	tokenString, err := auth.GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		log.Printf("Error signing token: %v", err)
		return
	}

	// Return token and user info
	user.Password = "" // Don't send password back
	response := models.LoginResponse{
		Token: tokenString,
		User:  user,
	}

	c.JSON(http.StatusOK, response)
}

// GetProfileHandler is an example of a protected endpoint
func GetProfileHandler(c *gin.Context) {
	// Get user ID and role from context (set by authMiddleware)
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Use the appropriate DB connection based on role
	database := db.GetDBForRole(role)

	// Get user info
	var user models.User
	err := database.QueryRow(
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
		var student models.Student
		err := database.QueryRow(
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
		var teacher models.Teacher
		err := database.QueryRow(
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

// CheckAuthHandler is a simple endpoint to check if the user is authenticated
func CheckAuthHandler(c *gin.Context) {
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
