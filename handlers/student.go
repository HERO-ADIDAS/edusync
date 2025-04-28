package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"edusync/models"
)

// UpdateStudentProfileHandler updates a student's profile
func UpdateStudentProfileHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can update their profiles"})
		return
	}

	var req models.Student
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var studentID int
	err := db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	_, err = db.Exec(`
		UPDATE student 
		SET grade_level = ?, enrollment_year = ?
		WHERE student_id = ? AND archive_delete_flag = TRUE`,
		req.GradeLevel, req.EnrollmentYear, studentID)
	if err != nil {
		log.Printf("Error updating student profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update student profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"student_id":      studentID,
		"grade_level":     req.GradeLevel,
		"enrollment_year": req.EnrollmentYear,
	})
}

// GetStudentDashboardHandler retrieves the student's dashboard data
func GetStudentDashboardHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can view their dashboard"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var studentID int
	err := db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Get enrolled courses
	rows, err := db.Query(`
		SELECT c.course_id, c.title, c.description
		FROM enrollment e
		JOIN classroom c ON e.course_id = c.course_id
		WHERE e.student_id = ? AND e.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE`, studentID)
	if err != nil {
		log.Printf("Error querying enrollments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var courses []map[string]interface{}
	for rows.Next() {
		var courseID int
		var title, description string
		if err := rows.Scan(&courseID, &title, &description); err != nil {
			log.Printf("Error scanning course: %v", err)
			continue
		}
		courses = append(courses, map[string]interface{}{
			"course_id":   courseID,
			"title":       title,
			"description": description,
		})
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating enrollments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Get recent submissions
	rows, err = db.Query(`
		SELECT s.submission_id, s.assignment_id, s.submitted_at, s.status
		FROM submission s
		WHERE s.student_id = ? AND s.archive_delete_flag = TRUE
		ORDER BY s.submitted_at DESC
		LIMIT 5`, studentID)
	if err != nil {
		log.Printf("Error querying submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var submissions []map[string]interface{}
	for rows.Next() {
		var submissionID, assignmentID int
		var submittedAt time.Time
		var status string
		if err := rows.Scan(&submissionID, &assignmentID, &submittedAt, &status); err != nil {
			log.Printf("Error scanning submission: %v", err)
			continue
		}
		submissions = append(submissions, map[string]interface{}{
			"submission_id": submissionID,
			"assignment_id": assignmentID,
			"submitted_at":  submittedAt,
			"status":        status,
		})
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"student_id":  studentID,
		"courses":     courses,
		"submissions": submissions,
	})
}