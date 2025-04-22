package handlers

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// SubmissionResponse represents the response structure for submission operations
type SubmissionResponse struct {
	SubmissionID int       `json:"submission_id"`
	AssignmentID int       `json:"assignment_id"`
	StudentID    int       `json:"student_id"`
	StudentName  string    `json:"student_name"`
	Link         string    `json:"link"` // Changed back to Link for redirect
	SubmittedAt  time.Time `json:"submitted_at"`
	Score        int       `json:"score"`
	Feedback     string    `json:"feedback"`
	Status       string    `json:"status"`
	IsLate       bool      `json:"is_late,omitempty"` // Optional field, computed in memory
}

// GradeRequest represents the request body for grading a submission
type GradeRequest struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

// BulkGradeRequest represents the request for grading multiple submissions
type BulkGradeRequest struct {
	SubmissionIDs []int  `json:"submission_ids"`
	Score         int    `json:"score"`
	Feedback      string `json:"feedback"`
}

// CreateSubmissionHandler creates a new submission with a redirect link
func CreateSubmissionHandler(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only students can create submissions"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	// Map userID to student_id, limiting to the first match to avoid duplicates
	var studentID int
	err := db.QueryRow("SELECT student_id FROM student WHERE user_id = ? LIMIT 1", userID).Scan(&studentID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: User is not registered as a student"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error mapping user to student: %v", err)
		}
		return
	}

	var submissionReq struct {
		AssignmentID int    `json:"assignment_id" binding:"required"`
		Link         string `json:"link" binding:"required,url"`
	}
	if err := c.ShouldBindJSON(&submissionReq); err != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // Reset body for logging
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		log.Printf("Invalid request body: %v, Raw body: %s", err, string(bodyBytes)) // Enhanced logging
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: link must be a valid URL and assignment_id is required"})
		return
	}

	var dueDate time.Time
	err = db.QueryRow("SELECT due_date FROM assignment WHERE assignment_id = ?", submissionReq.AssignmentID).Scan(&dueDate)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error verifying assignment: %v", err)
		}
		return
	}

	now := time.Now()
	isLate := now.After(dueDate)

	result, err := db.Exec(
		`INSERT INTO submission (assignment_id, student_id, link, submitted_at, status) 
		VALUES (?, ?, ?, ?, 'submitted')`,
		submissionReq.AssignmentID, studentID, submissionReq.Link, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating submission"})
		log.Printf("Error inserting submission: %v", err)
		return
	}

	submissionID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving submission ID"})
		log.Printf("Error getting last insert ID: %v", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"submission_id": submissionID,
		"message":       "Submission created successfully",
		"is_late":       isLate,
	})
}

// GetSubmissionHandler retrieves a student's submission
func GetSubmissionHandler(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	db := c.MustGet("db").(*sql.DB)

	submissionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	var submission SubmissionResponse
	query := `
		SELECT s.submission_id, s.assignment_id, s.student_id, u.name, s.link, 
		s.submitted_at, s.score, s.feedback, s.status
		FROM submission s
		JOIN user u ON s.student_id = u.user_id
		WHERE s.submission_id = ?`
	args := []interface{}{submissionID}

	if role == "student" {
		query += " AND s.student_id = ?"
		args = append(args, userID)
	} else if role == "teacher" {
		// Verify teacher owns the assignment
		var teacherID int
		err = db.QueryRow(`
			SELECT t.user_id
			FROM submission s
			JOIN assignment a ON s.assignment_id = a.assignment_id
			JOIN classroom c ON a.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE s.submission_id = ?`,
			submissionID).Scan(&teacherID)
		if err != nil || teacherID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	err = db.QueryRow(query, args...).Scan(
		&submission.SubmissionID, &submission.AssignmentID, &submission.StudentID,
		&submission.StudentName, &submission.Link, &submission.SubmittedAt,
		&submission.Score, &submission.Feedback, &submission.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found or not authorized"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving submission: %v", err)
		}
		return
	}

	// Compute is_late dynamically
	var dueDate time.Time
	err = db.QueryRow("SELECT due_date FROM assignment WHERE assignment_id = ?", submission.AssignmentID).Scan(&dueDate)
	if err == nil {
		submission.IsLate = submission.SubmittedAt.After(dueDate)
	}

	c.JSON(http.StatusOK, submission)
}

// GetAssignmentSubmissionsHandler retrieves all submissions for an assignment
func GetAssignmentSubmissionsHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can view submissions"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	log.Printf("Checking assignment %d for teacher %d", assignmentID, teacherID)
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE a.assignment_id = ? AND t.user_id = ?
		)`, assignmentID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking assignment ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		log.Printf("Assignment %d not found or not owned by teacher %d", assignmentID, teacherID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found or access denied"})
		return
	}

	rows, err := db.Query(
		`SELECT s.submission_id, s.assignment_id, s.student_id, u.name, s.link,
		s.submitted_at, s.score, s.feedback, s.status
		FROM submission s
		JOIN user u ON s.student_id = u.user_id
		WHERE s.assignment_id = ? AND s.status != 'deleted'
		ORDER BY s.submitted_at DESC`,
		assignmentID,
	)
	if err != nil {
		log.Printf("Error querying submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving submissions"})
		return
	}
	defer rows.Close()

	var submissions []SubmissionResponse
	var dueDate time.Time
	err = db.QueryRow("SELECT due_date FROM assignment WHERE assignment_id = ?", assignmentID).Scan(&dueDate)
	if err == nil {
		for rows.Next() {
			var submission SubmissionResponse
			err := rows.Scan(
				&submission.SubmissionID, &submission.AssignmentID, &submission.StudentID,
				&submission.StudentName, &submission.Link, &submission.SubmittedAt,
				&submission.Score, &submission.Feedback, &submission.Status,
			)
			if err != nil {
				log.Printf("Error scanning submission row: %v", err)
				continue
			}
			submission.IsLate = submission.SubmittedAt.After(dueDate)
			submissions = append(submissions, submission)
		}
	} else {
		for rows.Next() {
			var submission SubmissionResponse
			err := rows.Scan(
				&submission.SubmissionID, &submission.AssignmentID, &submission.StudentID,
				&submission.StudentName, &submission.Link, &submission.SubmittedAt,
				&submission.Score, &submission.Feedback, &submission.Status,
			)
			if err != nil {
				log.Printf("Error scanning submission row: %v", err)
				continue
			}
			submissions = append(submissions, submission)
		}
	}

	c.JSON(http.StatusOK, submissions)
}

// GradeSubmissionHandler grades a submission
func GradeSubmissionHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can grade submissions"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	submissionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	var gradeReq GradeRequest
	if err := c.ShouldBindJSON(&gradeReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid grade data"})
		return
	}

	// Verify teacher owns the assignment and get max points
	var assignmentID int
	var maxPoints int
	err = db.QueryRow(
		`SELECT a.assignment_id, a.max_points
		FROM submission s
		JOIN assignment a ON s.assignment_id = a.assignment_id
		JOIN classroom c ON a.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE s.submission_id = ? AND t.user_id = ?`,
		submissionID, teacherID).Scan(&assignmentID, &maxPoints)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to grade this submission"})
		} else {
			log.Printf("Error verifying submission permission: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	if gradeReq.Score < 0 || gradeReq.Score > maxPoints {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Score must be between 0 and max points", "max_points": maxPoints})
		return
	}

	_, err = db.Exec(
		`UPDATE submission SET score = ?, feedback = ?, status = 'graded' WHERE submission_id = ?`,
		gradeReq.Score, gradeReq.Feedback, submissionID,
	)
	if err != nil {
		log.Printf("Error recording grade: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving grade"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Submission graded successfully",
		"score":    gradeReq.Score,
		"feedback": gradeReq.Feedback,
	})
}

// BulkGradeSubmissionsHandler grades multiple submissions at once
func BulkGradeSubmissionsHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can grade submissions"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	var bulkReq BulkGradeRequest
	if err := c.ShouldBindJSON(&bulkReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if len(bulkReq.SubmissionIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No submission IDs provided"})
		return
	}

	// Verify teacher owns the assignment and get max points
	var maxPoints int
	err = db.QueryRow(
		`SELECT a.max_points
		FROM assignment a
		JOIN classroom c ON a.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE a.assignment_id = ? AND t.user_id = ?`,
		assignmentID, teacherID).Scan(&maxPoints)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to grade submissions for this assignment"})
		} else {
			log.Printf("Error verifying assignment ownership: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	if bulkReq.Score < 0 || bulkReq.Score > maxPoints {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Score must be between 0 and max points", "max_points": maxPoints})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	successCount := 0
	failedIDs := []int{}
	for _, submissionID := range bulkReq.SubmissionIDs {
		result, err := tx.Exec(
			`UPDATE submission SET score = ?, feedback = ?, status = 'graded' 
			WHERE submission_id = ? AND assignment_id = ? AND status != 'deleted'`,
			bulkReq.Score, bulkReq.Feedback, submissionID, assignmentID,
		)
		if err != nil {
			log.Printf("Error grading submission %d: %v", submissionID, err)
			failedIDs = append(failedIDs, submissionID)
			continue
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			successCount++
		} else {
			failedIDs = append(failedIDs, submissionID)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Bulk grading completed",
		"success_count": successCount,
		"failed_ids":    failedIDs,
	})
}