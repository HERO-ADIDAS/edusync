package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Assignment represents the assignment data structure
type Assignment struct {
	AssignmentID int       `json:"assignment_id" db:"assignment_id"`
	CourseID     int       `json:"course_id" db:"course_id"`
	Title        string    `json:"title" db:"title"`
	Description  string    `json:"description" db:"description"`
	DueDate      time.Time `json:"due_date" db:"due_date"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	MaxPoints    int       `json:"max_points" db:"max_points"`
}

// AssignmentDetailsResponse includes assignment details with statistics
type AssignmentDetailsResponse struct {
	Assignment        Assignment `json:"assignment"`
	ClassSize         int        `json:"class_size"`
	TotalSubmissions  int        `json:"total_submissions"`
	GradedSubmissions int        `json:"graded_submissions"`
	LateSubmissions   int        `json:"late_submissions"`
	AverageGrade      float64    `json:"average_grade"`
	CompletionPercent float64    `json:"completion_percent"`
	GradedPercent     float64    `json:"graded_percent"`
}

// CreateAssignmentHandler handles the creation of a new assignment
func CreateAssignmentHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int) // user_id from token
	db := c.MustGet("db").(*sql.DB)

	type AssignmentRequest struct {
		CourseID    int    `json:"course_id" binding:"required"`
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		DueDate     string `json:"due_date" binding:"required"` // Accept string
		MaxPoints   int    `json:"max_points" binding:"required"`
	}

	var req AssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment data"})
		return
	}

	// Parse due_date
	dueTime, err := time.Parse(time.RFC3339, req.DueDate)
	if err != nil {
		dueTime, err = time.Parse("2006-01-02T15:04", req.DueDate) // Match datetime-local format
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid due date format"})
			return
		}
	}

	log.Printf("Authenticated userID: %d, Requested CourseID: %d", teacherID, req.CourseID)
	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM classroom c
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE c.course_id = ? AND t.user_id = ?`,
		req.CourseID, teacherID).Scan(&count)
	if err != nil {
		log.Printf("Error verifying classroom ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if count == 0 {
		log.Printf("Permission denied: UserID %d does not own CourseID %d", teacherID, req.CourseID)
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to create assignments in this classroom"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	now := time.Now()
	result, err := tx.Exec(
		"INSERT INTO assignment (course_id, title, description, due_date, max_points, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		req.CourseID, req.Title, req.Description, dueTime, req.MaxPoints, now,
	)
	if err != nil {
		log.Printf("Error creating assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating assignment"})
		return
	}

	assignmentID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Error getting assignment ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Assignment created successfully",
		"id":      assignmentID,
	})
}

// UpdateAssignmentHandler updates an existing assignment
func UpdateAssignmentHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int) // user_id from token
	db := c.MustGet("db").(*sql.DB)

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	type UpdateAssignmentRequest struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
		MaxPoints   int    `json:"max_points"`
	}

	var req UpdateAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment data"})
		return
	}

	log.Printf("Authenticated userID: %d, Updating AssignmentID: %d", teacherID, assignmentID)
	var classroomID int
	err = db.QueryRow(`
		SELECT c.course_id FROM assignment a
		JOIN classroom c ON a.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE a.assignment_id = ? AND t.user_id = ?`,
		assignmentID, teacherID).Scan(&classroomID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Permission denied: UserID %d does not own AssignmentID %d", teacherID, assignmentID)
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this assignment"})
		} else {
			log.Printf("Error verifying assignment ownership: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	// Parse due_date if provided
	var dueTime time.Time
	if req.DueDate != "" {
		dueTime, err = time.Parse(time.RFC3339, req.DueDate)
		if err != nil {
			dueTime, err = time.Parse("2006-01-02T15:04", req.DueDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid due date format"})
				return
			}
		}
	}

	// Build dynamic update query
	query := "UPDATE assignment SET"
	args := []interface{}{}
	first := true
	if req.Title != "" {
		if !first {
			query += ","
		}
		query += " title = ?"
		args = append(args, req.Title)
		first = false
	}
	if req.Description != "" {
		if !first {
			query += ","
		}
		query += " description = ?"
		args = append(args, req.Description)
		first = false
	}
	if !dueTime.IsZero() {
		if !first {
			query += ","
		}
		query += " due_date = ?"
		args = append(args, dueTime)
		first = false
	}
	if req.MaxPoints > 0 {
		if !first {
			query += ","
		}
		query += " max_points = ?"
		args = append(args, req.MaxPoints)
		first = false
	}
	if first {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No update fields provided"})
		return
	}
	query += " WHERE assignment_id = ?"
	args = append(args, assignmentID)

	_, err = tx.Exec(query, args...)
	if err != nil {
		log.Printf("Error updating assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating assignment"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Assignment updated successfully"})
}

// DeleteAssignmentHandler deletes an assignment
func DeleteAssignmentHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int) // user_id from token
	db := c.MustGet("db").(*sql.DB)

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	log.Printf("Authenticated userID: %d, Deleting AssignmentID: %d", teacherID, assignmentID)
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE a.assignment_id = ? AND t.user_id = ?
		)`, assignmentID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error verifying assignment ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		log.Printf("Permission denied: UserID %d does not own AssignmentID %d", teacherID, assignmentID)
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to delete this assignment"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE submission SET status = 'deleted' WHERE assignment_id = ?", assignmentID)
	if err != nil {
		log.Printf("Error archiving submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing submissions"})
		return
	}

	_, err = tx.Exec("DELETE FROM assignment WHERE assignment_id = ?", assignmentID)
	if err != nil {
		log.Printf("Error deleting assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting assignment"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Assignment deleted successfully"})
}

// GetAssignmentsByClassroomHandler retrieves all assignments for a classroom
func GetAssignmentsByClassroomHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int) // user_id from token
	db := c.MustGet("db").(*sql.DB)

	classroomID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid classroom ID"})
		return
	}

	log.Printf("Authenticated userID: %d, Requested ClassroomID: %d", teacherID, classroomID)
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM classroom c
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE c.course_id = ? AND t.user_id = ?
		)`, classroomID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error verifying classroom ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		log.Printf("Permission denied: UserID %d does not own ClassroomID %d", teacherID, classroomID)
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to access this classroom"})
		return
	}

	rows, err := db.Query(
		`SELECT a.assignment_id, a.course_id, a.title, a.description, a.due_date, a.created_at, a.max_points,
		COUNT(DISTINCT s.submission_id) as submission_count,
		COUNT(DISTINCT CASE WHEN s.score IS NOT NULL THEN s.submission_id END) as graded_count
		FROM assignment a
		LEFT JOIN submission s ON a.assignment_id = s.assignment_id AND s.status != 'deleted'
		WHERE a.course_id = ?
		GROUP BY a.assignment_id
		ORDER BY a.due_date DESC`,
		classroomID,
	)
	if err != nil {
		log.Printf("Error querying assignments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving assignments"})
		return
	}
	defer rows.Close()

	assignments := []map[string]interface{}{}
	for rows.Next() {
		var assignment Assignment
		var submissionCount, gradedCount int
		err := rows.Scan(
			&assignment.AssignmentID, &assignment.CourseID, &assignment.Title,
			&assignment.Description, &assignment.DueDate, &assignment.CreatedAt,
			&assignment.MaxPoints,
			&submissionCount, &gradedCount,
		)
		if err != nil {
			log.Printf("Error scanning assignment row: %v", err)
			continue
		}

		classSize := getClassSize(db, classroomID)
		assignmentMap := map[string]interface{}{
			"assignment_id":      assignment.AssignmentID,
			"course_id":          assignment.CourseID,
			"title":              assignment.Title,
			"description":        assignment.Description,
			"due_date":           assignment.DueDate,
			"created_at":         assignment.CreatedAt,
			"max_points":         assignment.MaxPoints,
			"submission_count":   submissionCount,
			"graded_count":       gradedCount,
			"completion_percent": calculatePercentage(submissionCount, classSize),
			"graded_percent":     calculatePercentage(gradedCount, submissionCount),
		}
		assignments = append(assignments, assignmentMap)
	}

	c.JSON(http.StatusOK, assignments)
}

// GetAssignmentDetailsHandler retrieves detailed information about an assignment
func GetAssignmentDetailsHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int) // user_id from token
	db := c.MustGet("db").(*sql.DB)

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	log.Printf("Authenticated userID: %d, Requested AssignmentID: %d", teacherID, assignmentID)
	var assignment Assignment
	var totalSubmissions, gradedSubmissions, lateSubmissions int
	var averageGrade float64
	err = db.QueryRow(
		`SELECT a.assignment_id, a.course_id, a.title, a.description, a.due_date, a.created_at, a.max_points,
		COUNT(DISTINCT s.submission_id) as total_submissions,
		COUNT(DISTINCT CASE WHEN s.score IS NOT NULL THEN s.submission_id END) as graded_submissions,
		COUNT(DISTINCT CASE WHEN s.submitted_at > a.due_date THEN s.submission_id END) as late_submissions,
		COALESCE(AVG(s.score), 0) as average_grade
		FROM assignment a
		JOIN classroom c ON a.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		LEFT JOIN submission s ON a.assignment_id = s.assignment_id AND s.status != 'deleted'
		WHERE a.assignment_id = ? AND t.user_id = ?
		GROUP BY a.assignment_id`,
		assignmentID, teacherID).Scan(
		&assignment.AssignmentID, &assignment.CourseID, &assignment.Title,
		&assignment.Description, &assignment.DueDate, &assignment.CreatedAt,
		&assignment.MaxPoints,
		&totalSubmissions, &gradedSubmissions, &lateSubmissions, &averageGrade,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Permission denied or not found: UserID %d does not own AssignmentID %d", teacherID, assignmentID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found or you don't have permission to view it"})
		} else {
			log.Printf("Error retrieving assignment details: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	classSize := getClassSize(db, assignment.CourseID)
	response := AssignmentDetailsResponse{
		Assignment:        assignment,
		ClassSize:         classSize,
		TotalSubmissions:  totalSubmissions,
		GradedSubmissions: gradedSubmissions,
		LateSubmissions:   lateSubmissions,
		AverageGrade:      averageGrade,
		CompletionPercent: calculatePercentage(totalSubmissions, classSize),
		GradedPercent:     calculatePercentage(gradedSubmissions, totalSubmissions),
	}

	c.JSON(http.StatusOK, response)
}

// Helper functions
func calculatePercentage(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func getClassSize(db *sql.DB, classroomID int) int {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM enrollment WHERE course_id = ? AND status = 'enrolled'",
		classroomID).Scan(&count)
	if err != nil {
		log.Printf("Error getting class size: %v", err)
		return 0
	}
	return count
}
