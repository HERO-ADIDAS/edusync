package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"


	"github.com/gin-gonic/gin"
)

// ClassroomResponse represents the response structure for classroom operations
type ClassroomResponse struct {
	CourseID     int       `json:"course_id"`
	TeacherID    int       `json:"teacher_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	SubjectArea  string    `json:"subject_area"`
}

// GetTeacherClassroomsHandler retrieves all classrooms for a teacher
func GetTeacherClassroomsHandler(c *gin.Context) {
	// Get user ID and role from context (set by authMiddleware)
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Check if user has teacher role
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Get teacher ID
	var teacherID int
	err := db.QueryRow("SELECT teacher_id FROM teacher WHERE user_id = ?", userID).Scan(&teacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving teacher ID: %v", err)
		}
		return
	}

	// Query classrooms
	rows, err := db.Query(
		`SELECT course_id, teacher_id, title, description, start_date, end_date, subject_area 
		FROM classroom 
		WHERE teacher_id = ? 
		ORDER BY start_date DESC`,
		teacherID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving classrooms"})
		log.Printf("Error querying classrooms: %v", err)
		return
	}
	defer rows.Close()

	var classrooms []ClassroomResponse
	for rows.Next() {
		var classroom ClassroomResponse
		var startDateStr, endDateStr string
		err := rows.Scan(
			&classroom.CourseID, &classroom.TeacherID, &classroom.Title,
			&classroom.Description, &startDateStr, &endDateStr, &classroom.SubjectArea,
		)
		if err != nil {
			log.Printf("Error scanning classroom row: %v", err)
			continue
		}
		classroom.StartDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			log.Printf("Error parsing start date: %v", err)
			classroom.StartDate = time.Now()
		}
		classroom.EndDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			log.Printf("Error parsing end date: %v", err)
			classroom.EndDate = time.Now()
		}
		classrooms = append(classrooms, classroom)
	}

	c.JSON(http.StatusOK, gin.H{"classrooms": classrooms})
}

// CreateClassroomHandler creates a new classroom
func CreateClassroomHandler(c *gin.Context) {
	// Get user ID and role from context
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Parse request body
	var classroomReq struct {
		Title        string `json:"title" binding:"required"`
		Description  string `json:"description"`
		StartDate    string `json:"start_date" binding:"required"`
		EndDate      string `json:"end_date" binding:"required"`
		SubjectArea  string `json:"subject_area" binding:"required"`
	}
	if err := c.ShouldBindJSON(&classroomReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get teacher ID
	var teacherID int
	err := db.QueryRow("SELECT teacher_id FROM teacher WHERE user_id = ?", userID).Scan(&teacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving teacher ID: %v", err)
		}
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", classroomReq.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
		return
	}
	endDate, err := time.Parse("2006-01-02", classroomReq.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
		return
	}

	// Insert new classroom
	result, err := db.Exec(
		`INSERT INTO classroom (teacher_id, title, description, start_date, end_date, subject_area) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		teacherID, classroomReq.Title, classroomReq.Description, startDate, endDate, classroomReq.SubjectArea,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating classroom"})
		log.Printf("Error inserting classroom: %v", err)
		return
	}

	courseID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving course ID"})
		log.Printf("Error getting last insert ID: %v", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"course_id": courseID, "message": "Classroom created successfully"})
}

// GetClassroomDetailsHandler retrieves details of a specific classroom
func GetClassroomDetailsHandler(c *gin.Context) {
	// Get user ID and role from context
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Get course ID from URL parameter
	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	// Get teacher ID
	var teacherID int
	err = db.QueryRow("SELECT teacher_id FROM teacher WHERE user_id = ?", userID).Scan(&teacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving teacher ID: %v", err)
		}
		return
	}

	// Query classroom details
	var classroom ClassroomResponse
	var startDateStr, endDateStr string
	err = db.QueryRow(
		`SELECT course_id, teacher_id, title, description, start_date, end_date, subject_area 
		FROM classroom 
		WHERE course_id = ? AND teacher_id = ?`,
		courseID, teacherID,
	).Scan(&classroom.CourseID, &classroom.TeacherID, &classroom.Title,
		&classroom.Description, &startDateStr, &endDateStr, &classroom.SubjectArea)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Classroom not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving classroom: %v", err)
		}
		return
	}

	classroom.StartDate, err = time.Parse("2006-01-02", startDateStr)
	if err != nil {
		log.Printf("Error parsing start date: %v", err)
		classroom.StartDate = time.Now()
	}
	classroom.EndDate, err = time.Parse("2006-01-02", endDateStr)
	if err != nil {
		log.Printf("Error parsing end date: %v", err)
		classroom.EndDate = time.Now()
	}

	c.JSON(http.StatusOK, classroom)
}

// UpdateClassroomHandler updates an existing classroom
func UpdateClassroomHandler(c *gin.Context) {
	// Get user ID and role from context
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Get course ID from URL parameter
	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	// Parse request body
	var classroomReq struct {
		Title        string `json:"title" binding:"required"`
		Description  string `json:"description"`
		StartDate    string `json:"start_date" binding:"required"`
		EndDate      string `json:"end_date" binding:"required"`
		SubjectArea  string `json:"subject_area" binding:"required"`
	}
	if err := c.ShouldBindJSON(&classroomReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get teacher ID
	var teacherID int
	err = db.QueryRow("SELECT teacher_id FROM teacher WHERE user_id = ?", userID).Scan(&teacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving teacher ID: %v", err)
		}
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", classroomReq.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
		return
	}
	endDate, err := time.Parse("2006-01-02", classroomReq.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
		return
	}

	// Update classroom
	_, err = db.Exec(
		`UPDATE classroom 
		SET title = ?, description = ?, start_date = ?, end_date = ?, subject_area = ?
		WHERE course_id = ? AND teacher_id = ?`,
		classroomReq.Title, classroomReq.Description, startDate, endDate, classroomReq.SubjectArea,
		courseID, teacherID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating classroom"})
		log.Printf("Error updating classroom: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Classroom updated successfully"})
}

// DeleteClassroomHandler deletes a classroom
func DeleteClassroomHandler(c *gin.Context) {
	// Get user ID and role from context
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Get course ID from URL parameter
	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	// Get teacher ID
	var teacherID int
	err = db.QueryRow("SELECT teacher_id FROM teacher WHERE user_id = ?", userID).Scan(&teacherID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving teacher ID: %v", err)
		}
		return
	}

	// Delete classroom
	result, err := db.Exec(
		`DELETE FROM classroom WHERE course_id = ? AND teacher_id = ?`,
		courseID, teacherID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting classroom"})
		log.Printf("Error deleting classroom: %v", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking deletion"})
		log.Printf("Error checking rows affected: %v", err)
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Classroom not found or not authorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Classroom deleted successfully"})
}