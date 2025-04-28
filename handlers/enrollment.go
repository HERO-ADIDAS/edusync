package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"edusync/models"
)

// EnrollStudentHandler enrolls a student in a classroom
func EnrollStudentHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can enroll in classrooms"})
		return
	}

	var req models.Enrollment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var studentID int
	err := db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
	if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
		return
	}

	// Check if the student is already enrolled
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM enrollment 
			WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
		)`, studentID, req.CourseID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking enrollment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Student already enrolled in this classroom"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO enrollment (student_id, course_id, enrollment_date, status, archive_delete_flag)
		VALUES (?, ?, ?, 'active', TRUE)`,
		studentID, req.CourseID, time.Now())
	if err != nil {
		log.Printf("Error inserting enrollment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	enrollmentID, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{
		"enrollment_id": enrollmentID,
		"course_id":     req.CourseID,
		"status":        "active",
	})
}

// GetStudentEnrollmentsHandler lists all enrollments for a student
func GetStudentEnrollmentsHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can view their enrollments"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var studentID int
	err := db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
	if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
		return
	}

	rows, err := db.Query(`
		SELECT enrollment_id, student_id, course_id, enrollment_date, status
		FROM enrollment 
		WHERE student_id = ? AND archive_delete_flag = TRUE`, studentID)
	if err != nil {
		log.Printf("Error querying enrollments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var enrollments []models.Enrollment
	for rows.Next() {
		var e models.Enrollment
		if err := rows.Scan(&e.EnrollmentID, &e.StudentID, &e.CourseID, &e.EnrollmentDate, &e.Status); err != nil {
			log.Printf("Error scanning enrollment: %v", err)
			continue
		}
		enrollments = append(enrollments, e)
	}

	c.JSON(http.StatusOK, enrollments)
}