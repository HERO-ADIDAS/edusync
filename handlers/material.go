package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// MaterialResponse represents the response structure for material operations
type MaterialResponse struct {
	MaterialID  int       `json:"material_id"`
	CourseID    int       `json:"course_id"`
	Title       string    `json:"title"`
	Type        string    `json:"type"`
	FilePath    string    `json:"file_path"` // Stores URL link
	UploadedAt  time.Time `json:"uploaded_at"`
	Description string    `json:"description"`
}

// CreateMaterialRequest represents the request body for creating a material
type CreateMaterialRequest struct {
	CourseID    int    `json:"course_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Type        string `json:"type" binding:"required"`
	FilePath    string `json:"file_path" binding:"required,url"` // Enforces URL validation
	Description string `json:"description"`
}

// UpdateMaterialRequest represents the request body for updating a material
type UpdateMaterialRequest struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	FilePath    string `json:"file_path" binding:"omitempty,url"` // Optional, enforces URL if provided
	Description string `json:"description"`
}

// CreateMaterialHandler creates a new material (link only)
func CreateMaterialHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can create materials"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	var materialReq CreateMaterialRequest
	if err := c.ShouldBindJSON(&materialReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Verify teacher ownership of the course
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM classroom c
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE c.course_id = ? AND t.user_id = ?
		)`, materialReq.CourseID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking course ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Course not found or access denied"})
		return
	}

	// Insert new material
	result, err := db.Exec(
		`INSERT INTO material (course_id, title, type, file_path, description, uploaded_at)
		VALUES (?, ?, ?, ?, ?, NOW())`,
		materialReq.CourseID, materialReq.Title, materialReq.Type, materialReq.FilePath, materialReq.Description,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating material"})
		log.Printf("Error inserting material: %v", err)
		return
	}

	materialID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving material ID"})
		log.Printf("Error getting last insert ID: %v", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"material_id": materialID, "message": "Material created successfully"})
}

// GetMaterialsByCourseHandler retrieves all materials for a specific course
func GetMaterialsByCourseHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can view materials"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	log.Printf("Logged in teacher ID: %d", teacherID)
	log.Printf("Checking materials for course %d by teacher %d", courseID, teacherID)

	// Verify teacher ownership of the course
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM classroom c
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE c.course_id = ? AND t.user_id = ?
		)`, courseID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking course ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		log.Printf("Course %d not found or not owned by teacher %d", courseID, teacherID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Course not found or access denied"})
		return
	}

	rows, err := db.Query(
		`SELECT material_id, course_id, title, type, file_path, uploaded_at, description
		FROM material
		WHERE course_id = ?
		ORDER BY uploaded_at DESC`,
		courseID,
	)
	if err != nil {
		log.Printf("Error querying materials: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving materials"})
		return
	}
	defer rows.Close()

	var materials []MaterialResponse
	for rows.Next() {
		var material MaterialResponse
		err := rows.Scan(
			&material.MaterialID, &material.CourseID, &material.Title,
			&material.Type, &material.FilePath, &material.UploadedAt, &material.Description,
		)
		if err != nil {
			log.Printf("Error scanning material row: %v", err)
			continue
		}
		log.Printf("Found material: %+v", material)
		materials = append(materials, material)
	}

	if len(materials) == 0 {
		log.Printf("No materials found for course %d with teacher %d", courseID, teacherID)
	}

	c.JSON(http.StatusOK, materials)
}

// GetMaterialHandler retrieves details of a specific material
func GetMaterialHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can view material details"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
		return
	}

	log.Printf("Logged in teacher ID: %d", teacherID)
	log.Printf("Checking material %d for teacher %d", materialID, teacherID)

	// Verify teacher ownership via course
	var material MaterialResponse
	err = db.QueryRow(
		`SELECT m.material_id, m.course_id, m.title, m.type, m.file_path, m.uploaded_at, m.description
		FROM material m
		JOIN classroom c ON m.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE m.material_id = ? AND t.user_id = ?`,
		materialID, teacherID,
	).Scan(
		&material.MaterialID, &material.CourseID, &material.Title,
		&material.Type, &material.FilePath, &material.UploadedAt, &material.Description,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Material %d not found or not owned by teacher %d", materialID, teacherID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Material not found or access denied"})
		} else {
			log.Printf("Error querying material: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving material"})
		}
		return
	}

	log.Printf("Found material: %+v", material)
	c.JSON(http.StatusOK, material)
}

// UpdateMaterialHandler updates details of a specific material
func UpdateMaterialHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can update materials"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
		return
	}

	var updateReq UpdateMaterialRequest
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Verify teacher ownership
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM material m
			JOIN classroom c ON m.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE m.material_id = ? AND t.user_id = ?
		)`, materialID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking material ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Material not found or access denied"})
		return
	}

	// Update material
	setClause := "SET "
	values := []interface{}{}
	if updateReq.Title != "" {
		setClause += "title = ?, "
		values = append(values, updateReq.Title)
	}
	if updateReq.Type != "" {
		setClause += "type = ?, "
		values = append(values, updateReq.Type)
	}
	if updateReq.FilePath != "" {
		if _, err := url.ParseRequestURI(updateReq.FilePath); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File path must be a valid URL"})
			return
		}
		setClause += "file_path = ?, "
		values = append(values, updateReq.FilePath)
	}
	if updateReq.Description != "" {
		setClause += "description = ?, "
		values = append(values, updateReq.Description)
	}
	if len(values) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}
	setClause = setClause[:len(setClause)-2] // Remove trailing comma and space
	values = append(values, materialID)

	_, err = db.Exec(
		"UPDATE material "+setClause+" WHERE material_id = ?", values...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating material"})
		log.Printf("Error updating material: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Material updated successfully"})
}

// DeleteMaterialHandler deletes a specific material
func DeleteMaterialHandler(c *gin.Context) {
	teacherID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Only teachers can delete materials"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
		return
	}

	// Verify teacher ownership
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM material m
			JOIN classroom c ON m.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE m.material_id = ? AND t.user_id = ?
		)`, materialID, teacherID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking material ownership: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Material not found or access denied"})
		return
	}

	// Delete material
	_, err = db.Exec("DELETE FROM material WHERE material_id = ?", materialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting material"})
		log.Printf("Error deleting material: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Material deleted successfully"})
}
