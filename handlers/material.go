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

// CreateMaterialHandler creates a new material
func CreateMaterialHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can create materials"})
		return
	}

	var req models.Material
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

	// Check if the teacher is authorized to create materials for this classroom
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to create materials for this classroom"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO material (course_id, title, type, file_path, uploaded_at, description, archive_delete_flag)
		VALUES (?, ?, ?, ?, ?, ?, TRUE)`,
		req.CourseID, req.Title, req.Type, req.FilePath, time.Now(), req.Description)
	if err != nil {
		log.Printf("Error inserting material: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	materialID, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{
		"material_id": materialID,
		"course_id":   req.CourseID,
		"title":       req.Title,
		"type":        req.Type,
		"file_path":   req.FilePath,
		"description": req.Description,
	})
}

// UpdateMaterialHandler updates a material
func UpdateMaterialHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can update materials"})
		return
	}

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
		return
	}

	var req models.Material
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

	// Check if the teacher is authorized to update this material
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM material m
			JOIN classroom c ON m.course_id = c.course_id
			WHERE m.material_id = ? AND c.teacher_id = ? AND m.archive_delete_flag = TRUE
			AND c.archive_delete_flag = TRUE
		)`, materialID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking material authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to update this material"})
		return
	}

	_, err = db.Exec(`
		UPDATE material 
		SET title = ?, type = ?, file_path = ?, description = ?
		WHERE material_id = ? AND archive_delete_flag = TRUE`,
		req.Title, req.Type, req.FilePath, req.Description, materialID)
	if err != nil {
		log.Printf("Error updating material: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"material_id": materialID,
		"title":       req.Title,
		"type":        req.Type,
		"file_path":   req.FilePath,
		"description": req.Description,
	})
}

// DeleteMaterialHandler deletes a material
func DeleteMaterialHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can delete materials"})
		return
	}

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
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

	// Check if the teacher is authorized to delete this material
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM material m
			JOIN classroom c ON m.course_id = c.course_id
			WHERE m.material_id = ? AND c.teacher_id = ? AND m.archive_delete_flag = TRUE
			AND c.archive_delete_flag = TRUE
		)`, materialID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking material authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to delete this material"})
		return
	}

	_, err = db.Exec(`
		UPDATE material 
		SET archive_delete_flag = FALSE 
		WHERE material_id = ? AND archive_delete_flag = TRUE`, materialID)
	if err != nil {
		log.Printf("Error deleting material: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Material deleted"})
}

// GetMaterialsByClassroomHandler lists materials for a classroom
func GetMaterialsByClassroomHandler(c *gin.Context) {
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
		SELECT material_id, course_id, title, type, file_path, uploaded_at, description
		FROM material 
		WHERE course_id = ? AND archive_delete_flag = TRUE`, courseID)
	if err != nil {
		log.Printf("Error querying materials: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var materials []models.Material
	for rows.Next() {
		var m models.Material
		if err := rows.Scan(&m.MaterialID, &m.CourseID, &m.Title, &m.Type, &m.FilePath, &m.UploadedAt, &m.Description); err != nil {
			log.Printf("Error scanning material: %v", err)
			continue
		}
		materials = append(materials, m)
	}

	c.JSON(http.StatusOK, materials)
}