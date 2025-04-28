package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"edusync/models"
	"edusync/utils"
)

// RegisterHandler creates a new user and associated teacher/student record
func RegisterHandler(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if !utils.ValidateEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	// Check for existing email
	var existingEmail string
	err := db.QueryRow("SELECT email FROM user WHERE email = ? AND archive_delete_flag = TRUE", req.Email).Scan(&existingEmail)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	} else if err != sql.ErrNoRows {
		log.Printf("Error checking existing email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO user (name, email, password, role, contact_number, profile_picture, org, archive_delete_flag)
		VALUES (?, ?, ?, ?, ?, ?, ?, TRUE)`,
		req.Name, req.Email, passwordHash, req.Role, req.ContactNumber, req.ProfilePicture, req.Org)
	if err != nil {
		log.Printf("Error inserting user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Error retrieving user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user ID"})
		return
	}

	if req.Role == "teacher" {
		_, err = tx.Exec(`
			INSERT INTO teacher (user_id, dept, archive_delete_flag)
			VALUES (?, ?, TRUE)`, userID, req.Dept)
	} else {
		_, err = tx.Exec(`
			INSERT INTO student (user_id, grade_level, enrollment_year, archive_delete_flag)
			VALUES (?, ?, ?, TRUE)`, userID, req.GradeLevel, req.EnrollmentYear)
	}
	if err != nil {
		log.Printf("Error inserting %s: %v", req.Role, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create " + req.Role + " profile"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"name":    req.Name,
		"email":   req.Email,
		"role":    req.Role,
	})
}

// GetProfileHandler returns the authenticated user's profile
func GetProfileHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	db := c.MustGet("db").(*sql.DB)
	var user models.User
	err := db.QueryRow(`
		SELECT user_id, name, email, role, contact_number, profile_picture, org
		FROM user 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(
		&user.UserID, &user.Name, &user.Email, &user.Role, &user.ContactNumber, &user.ProfilePicture, &user.Org,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	} else if err != nil {
		log.Printf("Error querying user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	var profile interface{}
	if role == "teacher" {
		var teacher models.Teacher
		err = db.QueryRow(`
			SELECT teacher_id, user_id, dept
			FROM teacher 
			WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(
			&teacher.TeacherID, &teacher.UserID, &teacher.Dept,
		)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher profile not found"})
			return
		} else if err != nil {
			log.Printf("Error querying teacher: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		profile = gin.H{
			"user":    user,
			"teacher": teacher,
		}
	} else {
		var student models.Student
		err = db.QueryRow(`
			SELECT student_id, user_id, grade_level, enrollment_year
			FROM student 
			WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(
			&student.StudentID, &student.UserID, &student.GradeLevel, &student.EnrollmentYear,
		)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Student profile not found"})
			return
		} else if err != nil {
			log.Printf("Error querying student: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		profile = gin.H{
			"user":    user,
			"student": student,
		}
	}

	c.JSON(http.StatusOK, profile)
}

// CheckAuthHandler verifies authentication
func CheckAuthHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Role not found in context"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
}