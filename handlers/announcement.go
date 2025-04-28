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

// CreateAnnouncementHandler creates a new announcement
func CreateAnnouncementHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can create announcements"})
		return
	}

	var req models.Announcement
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

	// Check if the teacher is authorized to create announcements for this classroom
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to create announcements for this classroom"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO announcement (course_id, title, content, created_at, is_pinned, archive_delete_flag)
		VALUES (?, ?, ?, ?, ?, TRUE)`,
		req.CourseID, req.Title, req.Content, time.Now(), req.IsPinned)
	if err != nil {
		log.Printf("Error inserting announcement: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	announcementID, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{
		"announcement_id": announcementID,
		"course_id":       req.CourseID,
		"title":           req.Title,
		"content":         req.Content,
		"is_pinned":       req.IsPinned,
	})
}

// UpdateAnnouncementHandler updates an announcement
func UpdateAnnouncementHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can update announcements"})
		return
	}

	announcementID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
		return
	}

	var req models.Announcement
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

	// Check if the teacher is authorized to update this announcement
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM announcement a
			JOIN classroom c ON a.course_id = c.course_id
			WHERE a.announcement_id = ? AND c.teacher_id = ? AND a.archive_delete_flag = TRUE
			AND c.archive_delete_flag = TRUE
		)`, announcementID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking announcement authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to update this announcement"})
		return
	}

	_, err = db.Exec(`
		UPDATE announcement 
		SET title = ?, content = ?, is_pinned = ?
		WHERE announcement_id = ? AND archive_delete_flag = TRUE`,
		req.Title, req.Content, req.IsPinned, announcementID)
	if err != nil {
		log.Printf("Error updating announcement: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"announcement_id": announcementID,
		"title":           req.Title,
		"content":         req.Content,
		"is_pinned":       req.IsPinned,
	})
}

// DeleteAnnouncementHandler deletes an announcement
func DeleteAnnouncementHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can delete announcements"})
		return
	}

	announcementID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
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

	// Check if the teacher is authorized to delete this announcement
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM announcement a
			JOIN classroom c ON a.course_id = c.course_id
			WHERE a.announcement_id = ? AND c.teacher_id = ? AND a.archive_delete_flag = TRUE
			AND c.archive_delete_flag = TRUE
		)`, announcementID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking announcement authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to delete this announcement"})
		return
	}

	_, err = db.Exec(`
		UPDATE announcement 
		SET archive_delete_flag = FALSE 
		WHERE announcement_id = ? AND archive_delete_flag = TRUE`, announcementID)
	if err != nil {
		log.Printf("Error deleting announcement: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Announcement deleted"})
}

// GetAnnouncementsByClassroomHandler lists announcements for a classroom
func GetAnnouncementsByClassroomHandler(c *gin.Context) {
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
		SELECT announcement_id, course_id, title, content, created_at, is_pinned
		FROM announcement 
		WHERE course_id = ? AND archive_delete_flag = TRUE`, courseID)
	if err != nil {
		log.Printf("Error querying announcements: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var announcements []models.Announcement
	for rows.Next() {
		var a models.Announcement
		if err := rows.Scan(&a.AnnouncementID, &a.CourseID, &a.Title, &a.Content, &a.CreatedAt, &a.IsPinned); err != nil {
			log.Printf("Error scanning announcement: %v", err)
			continue
		}
		announcements = append(announcements, a)
	}

	c.JSON(http.StatusOK, announcements)
}