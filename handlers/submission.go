package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// SubmissionResponse represents the response structure for submission operations
type SubmissionResponse struct {
	SubmissionID int           `json:"submission_id"`
	AssignmentID int           `json:"assignment_id"`
	StudentID    int           `json:"student_id"`
	StudentName  string        `json:"student_name"`
	Link         string        `json:"link"`
	SubmittedAt  time.Time     `json:"submitted_at"`
	Score        sql.NullInt64 `json:"score"`
	Feedback     sql.NullString `json:"feedback"`
	Status       string        `json:"status"`
	IsLate       bool          `json:"is_late,omitempty"`
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

// CreateSubmissionHandler creates a new submission
func CreateSubmissionHandler(c *gin.Context) {
	// Get user ID and role from context
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Check if user has student role
	if role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only students can submit assignments"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Parse request body
	var submissionReq struct {
		AssignmentID int    `json:"assignment_id" binding:"required"`
		Link         string `json:"link" binding:"required,url"`
	}
	if err := c.ShouldBindJSON(&submissionReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get student ID
	var studentID int
	err := db.QueryRow("SELECT student_id FROM student WHERE user_id = ?", userID).Scan(&studentID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving student ID: %v", err)
		}
		return
	}

	// Insert new submission
	result, err := db.Exec(
		`INSERT INTO submission (assignment_id, student_id, link, submitted_at, status)
		VALUES (?, ?, ?, NOW(), 'submitted')`,
		submissionReq.AssignmentID, studentID, submissionReq.Link,
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

	c.JSON(http.StatusCreated, gin.H{"submission_id": submissionID, "message": "Submission created successfully"})
}

// GetAssignmentSubmissionsHandler retrieves all submissions for a specific assignment
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

	log.Printf("Logged in teacher ID: %d", teacherID)
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
		JOIN student st ON s.student_id = st.student_id
		JOIN user u ON st.user_id = u.user_id
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
			log.Printf("Found submission: %+v", submission)
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
			log.Printf("Found submission (no due date): %+v", submission)
			submissions = append(submissions, submission)
		}
	}

	if len(submissions) == 0 {
		log.Printf("No submissions found for assignment %d with teacher %d", assignmentID, teacherID)
	}

	c.JSON(http.StatusOK, submissions)
}

// GradeSubmissionHandler grades a specific submission
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Verify teacher ownership (via assignment)
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM submission s
			JOIN assignment a ON s.assignment_id = a.assignment_id
			JOIN classroom c ON a.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE s.submission_id = ? AND t.user_id = ?
		)`, submissionID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking submission ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found or access denied"})
		return
	}

	// Update submission with grade
	_, err = db.Exec(
		`UPDATE submission SET score = ?, feedback = ?, status = 'graded'
		WHERE submission_id = ?`,
		gradeReq.Score, gradeReq.Feedback, submissionID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error grading submission"})
		log.Printf("Error updating submission: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Submission graded successfully"})
}

// BulkGradeSubmissionsHandler grades multiple submissions for an assignment
func BulkGradeSubmissionsHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can bulk grade submissions"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	var bulkGradeReq BulkGradeRequest
	if err := c.ShouldBindJSON(&bulkGradeReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Verify teacher ownership
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found or access denied"})
		return
	}

	// Bulk update submissions
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	stmt, err := tx.Prepare(
		`UPDATE submission SET score = ?, feedback = ?, status = 'graded'
		WHERE submission_id = ? AND assignment_id = ? AND status != 'deleted'`,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare statement"})
		return
	}
	defer stmt.Close()

	for _, submissionID := range bulkGradeReq.SubmissionIDs {
		_, err := stmt.Exec(bulkGradeReq.Score, bulkGradeReq.Feedback, submissionID, assignmentID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error grading submission"})
			log.Printf("Error updating submission %d: %v", submissionID, err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bulk grading completed successfully"})
}

// GetSubmissionHandler retrieves details of a specific submission
func GetSubmissionHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can view submission details"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	submissionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	log.Printf("Logged in teacher ID: %d", teacherID)
	log.Printf("Checking submission %d for teacher %d", submissionID, teacherID)

	// Verify teacher ownership (via assignment)
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM submission s
			JOIN assignment a ON s.assignment_id = a.assignment_id
			JOIN classroom c ON a.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE s.submission_id = ? AND t.user_id = ?
		)`, submissionID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking submission ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		log.Printf("Submission %d not found or not owned by teacher %d", submissionID, teacherID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found or access denied"})
		return
	}

	var submission SubmissionResponse
	err = db.QueryRow(
		`SELECT s.submission_id, s.assignment_id, s.student_id, u.name, s.link,
		s.submitted_at, s.score, s.feedback, s.status
		FROM submission s
		JOIN student st ON s.student_id = st.student_id
		JOIN user u ON st.user_id = u.user_id
		WHERE s.submission_id = ? AND s.status != 'deleted'`,
		submissionID,
	).Scan(
		&submission.SubmissionID, &submission.AssignmentID, &submission.StudentID,
		&submission.StudentName, &submission.Link, &submission.SubmittedAt,
		&submission.Score, &submission.Feedback, &submission.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Submission %d not found", submissionID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found"})
		} else {
			log.Printf("Error querying submission: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving submission"})
		}
		return
	}

	var dueDate time.Time
	err = db.QueryRow("SELECT due_date FROM assignment WHERE assignment_id = ?", submission.AssignmentID).Scan(&dueDate)
	if err == nil {
		submission.IsLate = submission.SubmittedAt.After(dueDate)
	}

	log.Printf("Found submission: %+v", submission)
	c.JSON(http.StatusOK, submission)
}