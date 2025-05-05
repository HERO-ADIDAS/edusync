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

// AssignmentRequest is a temporary struct to handle incoming JSON with string dates
type AssignmentRequest struct {
	CourseID    int     `json:"course_id" binding:"required"`
	Title       string  `json:"title" binding:"required"`
	Description *string `json:"description"`
	DueDate     string  `json:"due_date" binding:"required"`
	MaxPoints   int     `json:"max_points" binding:"required"`
}

// CreateAssignmentHandler creates a new assignment
func CreateAssignmentHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can create assignments"})
		return
	}

	var req AssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Parse due_date in ISO 8601 format (e.g., "2025-05-10T14:30:00Z")
	dueDate, err := time.Parse(time.RFC3339, req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid due_date format, expected YYYY-MM-DDThh:mm:ssZ (e.g., 2025-05-10T14:30:00Z)"})
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

	// Check if the teacher is authorized to create an assignment for this course
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to create assignment for this course"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO assignment (course_id, title, description, due_date, max_points, archive_delete_flag)
		VALUES (?, ?, ?, ?, ?, TRUE)`,
		req.CourseID, req.Title, req.Description, dueDate, req.MaxPoints)
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
	})
}

// UpdateAssignmentHandler updates an existing assignment
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

	var req AssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Parse due_date in ISO 8601 format (e.g., "2025-05-10T14:30:00Z")
	dueDate, err := time.Parse(time.RFC3339, req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid due_date format, expected YYYY-MM-DDThh:mm:ssZ (e.g., 2025-05-10T14:30:00Z)"})
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
			SELECT 1 
			FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			WHERE a.assignment_id = ? AND c.teacher_id = ? AND a.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE
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
		SET course_id = ?, title = ?, description = ?, due_date = ?, max_points = ?
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`,
		req.CourseID, req.Title, req.Description, dueDate, req.MaxPoints, assignmentID)
	if err != nil {
		log.Printf("Error updating assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"assignment_id": assignmentID,
		"course_id":     req.CourseID,
		"title":         req.Title,
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
			SELECT 1 
			FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			WHERE a.assignment_id = ? AND c.teacher_id = ? AND a.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE
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
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`,
		assignmentID)
	if err != nil {
		log.Printf("Error deleting assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Assignment deleted"})
}

// GetAssignmentsByClassroomHandler lists all assignments for a classroom
func GetAssignmentsByClassroomHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	// Check if the classroom exists
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM classroom 
			WHERE course_id = ? AND archive_delete_flag = TRUE
		)`, courseID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking if classroom exists: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Classroom not found"})
		return
	}

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
		var teacherAuthorized bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM classroom 
				WHERE course_id = ? AND teacher_id = ? AND archive_delete_flag = TRUE
			)`, courseID, teacherID).Scan(&teacherAuthorized)
		if err != nil {
			log.Printf("Error checking teacher authorization: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !teacherAuthorized {
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
		var studentEnrolled bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM enrollment 
				WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
			)`, studentID, courseID).Scan(&studentEnrolled)
		if err != nil {
			log.Printf("Error checking enrollment: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !studentEnrolled {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not enrolled in this classroom"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized role"})
		return
	}

	rows, err := db.Query(`
		SELECT assignment_id, course_id, title, description, due_date, max_points
		FROM assignment 
		WHERE course_id = ? AND archive_delete_flag = TRUE`, courseID)
	if err != nil {
		log.Printf("Error querying assignments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var assignments []map[string]interface{}
	for rows.Next() {
		var assignment models.Assignment
		if err := rows.Scan(&assignment.AssignmentID, &assignment.CourseID, &assignment.Title, &assignment.Description, &assignment.DueDate, &assignment.MaxPoints); err != nil {
			log.Printf("Error scanning assignment: %v", err)
			continue
		}
		assignments = append(assignments, map[string]interface{}{
			"assignment_id": assignment.AssignmentID,
			"course_id":     assignment.CourseID,
			"title":         assignment.Title,
			"description":   assignment.Description,
			"due_date":      assignment.DueDate.Format(time.RFC3339), // Ensure ISO 8601 format in response
			"max_points":    assignment.MaxPoints,
		})
	}

	c.JSON(http.StatusOK, assignments)
}

// GetUpcomingAssignmentsHandler lists all upcoming assignments for the teacher's classrooms due within 3 days
func GetUpcomingAssignmentsHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can access upcoming assignments"})
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

	// Use the application's current time in UTC to avoid database time zone issues
	now := time.Now().UTC()
	threeDaysLater := now.Add(3 * 24 * time.Hour)

	rows, err := db.Query(`
		SELECT a.assignment_id, a.course_id, a.title, a.description, a.due_date, a.max_points
		FROM assignment a
		JOIN classroom c ON a.course_id = c.course_id
		WHERE c.teacher_id = ? 
		AND a.due_date >= ? 
		AND a.due_date <= ?
		AND a.archive_delete_flag = TRUE 
		AND c.archive_delete_flag = TRUE
		ORDER BY a.due_date ASC`, teacherID, now, threeDaysLater)
	if err != nil {
		log.Printf("Error querying upcoming assignments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var assignments []map[string]interface{}
	for rows.Next() {
		var assignment models.Assignment
		if err := rows.Scan(&assignment.AssignmentID, &assignment.CourseID, &assignment.Title, &assignment.Description, &assignment.DueDate, &assignment.MaxPoints); err != nil {
			log.Printf("Error scanning assignment: %v", err)
			continue
		}
		assignments = append(assignments, map[string]interface{}{
			"assignment_id": assignment.AssignmentID,
			"course_id":     assignment.CourseID,
			"title":         assignment.Title,
			"description":   assignment.Description,
			"due_date":      assignment.DueDate.Format(time.RFC3339),
			"max_points":    assignment.MaxPoints,
		})
	}

	c.JSON(http.StatusOK, assignments)
}

// GetAssignmentStatsHandler retrieves statistics for an assignment
func GetAssignmentStatsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can view assignment statistics"})
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

	// Check if the teacher is authorized to view this assignment
	exists = false
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			WHERE a.assignment_id = ? AND c.teacher_id = ? AND a.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE
		)`, assignmentID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking assignment authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to view this assignment"})
		return
	}

	// Get assignment statistics
	var totalSubmissions, totalGraded int
	var averageGrade sql.NullFloat64
	err = db.QueryRow(`
		SELECT 
			COUNT(*) as total_submissions,
			COUNT(grade) as total_graded,
			AVG(grade) as average_grade
		FROM submission 
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`,
		assignmentID).Scan(&totalSubmissions, &totalGraded, &averageGrade)
	if err != nil {
		log.Printf("Error querying assignment statistics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Get grade distribution
	rows, err := db.Query(`
		SELECT grade, COUNT(*) as count
		FROM submission 
		WHERE assignment_id = ? AND grade IS NOT NULL AND archive_delete_flag = TRUE
		GROUP BY grade`,
		assignmentID)
	if err != nil {
		log.Printf("Error querying grade distribution: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	gradeDistribution := make(map[string]int)
	for rows.Next() {
		var grade int
		var count int
		if err := rows.Scan(&grade, &count); err != nil {
			log.Printf("Error scanning grade distribution: %v", err)
			continue
		}
		gradeDistribution[strconv.Itoa(grade)] = count
	}

	c.JSON(http.StatusOK, gin.H{
		"assignment_id":      assignmentID,
		"total_submissions":  totalSubmissions,
		"total_graded":       totalGraded,
		"average_grade":      averageGrade.Float64,
		"grade_distribution": gradeDistribution,
	})
}
