package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"edusync/models"
)

// UpdateTeacherProfileHandler updates a teacher's profile
func UpdateTeacherProfileHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can update their profiles"})
		return
	}

	var req models.Teacher
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

	_, err = db.Exec(`
		UPDATE teacher 
		SET dept = ?
		WHERE teacher_id = ? AND archive_delete_flag = TRUE`,
		req.Dept, teacherID)
	if err != nil {
		log.Printf("Error updating teacher: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher_id": teacherID,
		"dept":       req.Dept,
	})
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