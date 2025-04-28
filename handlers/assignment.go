package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"edusync/models"
)

// CreateAssignmentHandler creates a new assignment
func CreateAssignmentHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can create assignments"})
		return
	}

	var req models.Assignment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var teacherID int
	err := db.QueryRow(`
		SELECT teacher_id FROM teacher 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
	if err != nil {
		log.Printf("Error querying teacher: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Teacher not found"})
		return
	}

	// Check if the teacher is authorized to create assignments for this classroom
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM classroom 
			WHERE course_id = ? AND teacher_id = ? AND archive_delete_flag = TRUE
		)`, req.CourseID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking classroom authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to create assignments for this classroom"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO assignment (course_id, title, description, due_date, max_points, created_at, archive_delete_flag)
		VALUES (?, ?, ?, ?, ?, ?, TRUE)`,
		req.CourseID, req.Title, req.Description, req.DueDate, req.MaxPoints, time.Now())
	if err != nil {
		log.Printf("Error inserting assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	assignmentID, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{
		"assignment_id": assignmentID,
		"course_id":     req.CourseID,
		"title":         req.Title,
		"description":   req.Description,
		"due_date":      req.DueDate,
		"max_points":    req.MaxPoints,
	})
}

// UpdateAssignmentHandler updates an assignment
func UpdateAssignmentHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can update assignments"})
		return
	}

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	var req models.Assignment
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var teacherID int
	err = db.QueryRow(`
		SELECT teacher_id FROM teacher 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
	if err != nil {
		log.Printf("Error querying teacher: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Teacher not found"})
		return
	}

	// Check if the teacher is authorized to update this assignment
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			WHERE a.assignment_id = ? AND c.teacher_id = ? AND a.archive_delete_flag = TRUE
			AND c.archive_delete_flag = TRUE
		)`, assignmentID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking assignment authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to update this assignment"})
		return
	}

	_, err = db.Exec(`
		UPDATE assignment 
		SET title = ?, description = ?, due_date = ?, max_points = ?
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`,
		req.Title, req.Description, req.DueDate, req.MaxPoints, assignmentID)
	if err != nil {
		log.Printf("Error updating assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"assignment_id": assignmentID,
		"title":         req.Title,
		"description":   req.Description,
		"due_date":      req.DueDate,
		"max_points":    req.MaxPoints,
	})
}

// DeleteAssignmentHandler deletes an assignment
func DeleteAssignmentHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can delete assignments"})
		return
	}

	assignmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var teacherID int
	err = db.QueryRow(`
		SELECT teacher_id FROM teacher 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
	if err != nil {
		log.Printf("Error querying teacher: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Teacher not found"})
		return
	}

	// Check if the teacher is authorized to delete this assignment
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			WHERE a.assignment_id = ? AND c.teacher_id = ? AND a.archive_delete_flag = TRUE
			AND c.archive_delete_flag = TRUE
		)`, assignmentID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking assignment authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to delete this assignment"})
		return
	}

	_, err = db.Exec(`
		UPDATE assignment 
		SET archive_delete_flag = FALSE 
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`, assignmentID)
	if err != nil {
		log.Printf("Error deleting assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Assignment deleted"})
}

// GetAssignmentsByClassroomHandler lists assignments for a classroom
func GetAssignmentsByClassroomHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	if role == "teacher" {
		var teacherID int
		err = db.QueryRow(`
			SELECT teacher_id FROM teacher 
			WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
		if err != nil {
			log.Printf("Error querying teacher: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Teacher not found"})
			return
		}

		// Check if the teacher is authorized to view this classroom
		var exists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM classroom 
				WHERE course_id = ? AND teacher_id = ? AND archive_delete_flag = TRUE
			)`, courseID, teacherID).Scan(&exists)
		if err != nil {
			log.Printf("Error checking classroom authorization: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to view this classroom"})
			return
		}
	} else if role == "student" {
		var studentID int
		err = db.QueryRow(`
			SELECT student_id FROM student 
			WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
		if err != nil {
			log.Printf("Error querying student: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
			return
		}

		// Check if the student is enrolled in this classroom
		var exists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM enrollment 
				WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
			)`, studentID, courseID).Scan(&exists)
		if err != nil {
			log.Printf("Error checking enrollment: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not enrolled in this classroom"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized role"})
		return
	}

	rows, err := db.Query(`
		SELECT assignment_id, course_id, title, description, due_date, max_points, created_at
		FROM assignment 
		WHERE course_id = ? AND archive_delete_flag = TRUE`, courseID)
	if err != nil {
		log.Printf("Error querying assignments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var assignments []models.Assignment
	for rows.Next() {
		var a models.Assignment
		if err := rows.Scan(&a.AssignmentID, &a.CourseID, &a.Title, &a.Description, &a.DueDate, &a.MaxPoints, &a.CreatedAt); err != nil {
			log.Printf("Error scanning assignment: %v", err)
			continue
		}
		assignments = append(assignments, a)
	}

	c.JSON(http.StatusOK, assignments)
}