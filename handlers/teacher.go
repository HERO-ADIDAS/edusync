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

// TeacherRequest is a temporary struct to handle incoming JSON
type TeacherRequest struct {
	Dept string `json:"dept" binding:"required"`
}

// CreateTeacherHandler creates a new teacher profile
func CreateTeacherHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can create teacher profiles"})
		return
	}

	var req TeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	teacher := models.Teacher{
		Dept: &req.Dept,
	}

	db := c.MustGet("db").(*sql.DB)

	// Check if teacher profile already exists for this user
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM teacher 
			WHERE user_id = ? AND archive_delete_flag = TRUE
		)`, userID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking teacher existence for user_id %v: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Teacher profile already exists for this user"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO teacher (user_id, dept, archive_delete_flag)
		VALUES (?, ?, TRUE)`,
		userID, teacher.Dept)
	if err != nil {
		log.Printf("Error inserting teacher for user_id %v: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	teacherID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Error retrieving last insert ID for teacher: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve teacher ID"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher_id": teacherID,
		"user_id":    userID,
		"dept":       teacher.Dept,
	})
}

// UpdateTeacherHandler updates a teacher's profile
func UpdateTeacherHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can update their profiles"})
		return
	}

	var req TeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	teacher := models.Teacher{
		Dept: &req.Dept,
	}

	db := c.MustGet("db").(*sql.DB)
	var teacherID int
	err := db.QueryRow(`
		SELECT teacher_id FROM teacher 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
	if err == sql.ErrNoRows {
		log.Printf("Teacher profile not found for user_id %v", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher profile not found"})
		return
	} else if err != nil {
		log.Printf("Error querying teacher for user_id %v: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	_, err = db.Exec(`
		UPDATE teacher 
		SET dept = ?
		WHERE teacher_id = ? AND archive_delete_flag = TRUE`,
		teacher.Dept, teacherID)
	if err != nil {
		log.Printf("Error updating teacher for teacher_id %d: %v", teacherID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher_id": teacherID,
		"message":    "Teacher profile updated",
		"dept":       teacher.Dept,
	})
}

// DeleteTeacherHandler deletes a teacher profile
func DeleteTeacherHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can delete their profiles"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var teacherID int
	err := db.QueryRow(`
		SELECT teacher_id FROM teacher 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
	if err == sql.ErrNoRows {
		log.Printf("Teacher profile not found for user_id %v", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher profile not found"})
		return
	} else if err != nil {
		log.Printf("Error querying teacher for user_id %v: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	_, err = db.Exec(`
		UPDATE teacher 
		SET archive_delete_flag = FALSE 
		WHERE teacher_id = ? AND user_id = ? AND archive_delete_flag = TRUE`,
		teacherID, userID)
	if err != nil {
		log.Printf("Error deleting teacher for teacher_id %d: %v", teacherID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Teacher profile deleted"})
}

// GetTeacherProfileHandler retrieves a teacher's profile
func GetTeacherProfileHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can view their profiles"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var teacher models.Teacher
	err := db.QueryRow(`
		SELECT teacher_id, user_id, dept
		FROM teacher 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).
		Scan(&teacher.TeacherID, &teacher.UserID, &teacher.Dept)
	if err == sql.ErrNoRows {
		log.Printf("Teacher profile not found for user_id %v", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher profile not found"})
		return
	} else if err != nil {
		log.Printf("Error querying teacher for user_id %v: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	response := gin.H{
		"teacher_id": teacher.TeacherID,
		"user_id":    teacher.UserID,
		"dept":       teacher.Dept,
	}

	log.Printf("Successfully retrieved teacher profile for user_id %v: %+v", userID, response)
	c.JSON(http.StatusOK, response)
}

// GetTeacherByIDHandler retrieves a teacher's profile by teacher_id (for admin or authorized users)
func GetTeacherByIDHandler(c *gin.Context) {
	teacherID, err := strconv.Atoi(c.Param("teacher_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid teacher ID"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var teacher models.Teacher
	err = db.QueryRow(`
		SELECT teacher_id, user_id, dept
		FROM teacher 
		WHERE teacher_id = ? AND archive_delete_flag = TRUE`, teacherID).
		Scan(&teacher.TeacherID, &teacher.UserID, &teacher.Dept)
	if err == sql.ErrNoRows {
		log.Printf("Teacher not found for teacher_id %d", teacherID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
		return
	} else if err != nil {
		log.Printf("Error querying teacher for teacher_id %d: %v", teacherID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	response := gin.H{
		"teacher_id": teacher.TeacherID,
		"user_id":    teacher.UserID,
		"dept":       teacher.Dept,
	}

	log.Printf("Successfully retrieved teacher profile for teacher_id %d: %+v", teacherID, response)
	c.JSON(http.StatusOK, response)
}

// GetTeacherDashboardHandler provides a teacher's dashboard data
func GetTeacherDashboardHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can access their dashboard"})
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

	rows, err := db.Query(`
		SELECT course_id, title, description
		FROM classroom 
		WHERE teacher_id = ? AND archive_delete_flag = TRUE`, teacherID)
	if err != nil {
		log.Printf("Error querying classrooms: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var courses []gin.H
	for rows.Next() {
		var courseID int
		var title string
		var description *string
		if err := rows.Scan(&courseID, &title, &description); err != nil {
			log.Printf("Error scanning classroom: %v", err)
			continue
		}
		courses = append(courses, gin.H{
			"course_id":   courseID,
			"title":       title,
			"description": description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher_id": teacherID,
		"courses":    courses,
	})
}

// GetTeacherUpcomingAssignmentsHandler retrieves upcoming assignments for a teacher
func GetTeacherUpcomingAssignmentsHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can view upcoming assignments"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var teacherID int
	err := db.QueryRow(`
		SELECT teacher_id FROM teacher 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
	if err == sql.ErrNoRows {
		log.Printf("Teacher profile not found for user_id %v", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Teacher profile not found"})
		return
	} else if err != nil {
		log.Printf("Error querying teacher for user_id %v: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Query assignments with due dates in the future for the teacher's classrooms
	rows, err := db.Query(`
		SELECT a.assignment_id, a.course_id, a.title, a.description, a.due_date, a.max_points
		FROM assignment a
		JOIN classroom c ON a.course_id = c.course_id
		WHERE c.teacher_id = ? 
		AND a.due_date > ? 
		AND a.archive_delete_flag = TRUE 
		AND c.archive_delete_flag = TRUE
		ORDER BY a.due_date ASC`, teacherID, time.Now())
	if err != nil {
		log.Printf("Error querying upcoming assignments for teacher_id %d: %v", teacherID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var assignments []gin.H
	for rows.Next() {
		var assignmentID, courseID, maxPoints int
		var title string
		var description *string
		var dueDate time.Time
		if err := rows.Scan(&assignmentID, &courseID, &title, &description, &dueDate, &maxPoints); err != nil {
			log.Printf("Error scanning assignment: %v", err)
			continue
		}
		assignments = append(assignments, gin.H{
			"assignment_id": assignmentID,
			"course_id":     courseID,
			"title":         title,
			"description":   description,
			"due_date":      dueDate,
			"max_points":    maxPoints,
		})
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating over assignments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher_id":   teacherID,
		"assignments":  assignments,
	})
}

// ListTeachersHandler lists all teachers (for admin or authorized users)
func ListTeachersHandler(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can list all teachers"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	rows, err := db.Query(`
		SELECT teacher_id, user_id, dept
		FROM teacher 
		WHERE archive_delete_flag = TRUE`)
	if err != nil {
		log.Printf("Error querying teachers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var teachers []gin.H
	for rows.Next() {
		var teacher models.Teacher
		if err := rows.Scan(&teacher.TeacherID, &teacher.UserID, &teacher.Dept); err != nil {
			log.Printf("Error scanning teacher: %v", err)
			continue
		}

		teacherResponse := gin.H{
			"teacher_id": teacher.TeacherID,
			"user_id":    teacher.UserID,
			"dept":       teacher.Dept,
		}
		teachers = append(teachers, teacherResponse)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating over teachers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"teachers": teachers})
}