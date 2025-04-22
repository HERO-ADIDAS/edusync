package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Material represents educational content
type Material struct {
	ID          int       `json:"id" db:"material_id"`
	CourseID    int       `json:"course_id" db:"course_id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	Type        string    `json:"type" db:"type"`
	FilePath    string    `json:"file_path" db:"file_path"`
	UploadedAt  time.Time `json:"uploaded_at" db:"uploaded_at"`
}

// CreateMaterialRequest represents the request body for creating a material
type CreateMaterialRequest struct {
	CourseID    int    `json:"course_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type" binding:"required"`
	FilePath    string `json:"file_path" binding:"required"`
}

// UpdateMaterialRequest represents the request body for updating a material
type UpdateMaterialRequest struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
}

// CreateMaterial handles the creation of new educational content
func CreateMaterial(c *gin.Context, db *sql.DB) {
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Ensure user is a teacher
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can create materials"})
		return
	}

	var req CreateMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Verify the teacher is associated with the classroom
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM classroom c JOIN teacher t ON c.teacher_id = t.teacher_id WHERE c.course_id = ? AND t.user_id = ?",
		req.CourseID, userID,
	).Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error verifying classroom ownership: %v", err)
		return
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to add materials to this classroom"})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// Insert new material
	result, err := tx.Exec(
		`INSERT INTO material (course_id, title, description, type, file_path, uploaded_at) 
		VALUES (?, ?, ?, ?, ?, NOW())`,
		req.CourseID, req.Title, req.Description, req.Type, req.FilePath,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating material"})
		log.Printf("Error creating material: %v", err)
		return
	}

	materialID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving material ID"})
		log.Printf("Error getting last insert ID: %v", err)
		return
	}

	/*
	// Commented out due to missing material_stats table
	_, err = tx.Exec(
		"INSERT INTO material_stats (material_id, views, downloads) VALUES (?, 0, 0)",
		materialID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error initializing material stats"})
		log.Printf("Error creating material stats: %v", err)
		return
	}
	*/

	/*
	// Commented out due to missing activity_log table
	_, err = tx.Exec(
		`INSERT INTO activity_log 
		(user_id, activity_type, resource_id, resource_type, description) 
		VALUES (?, 'create', ?, 'material', ?)`,
		userID, materialID, "Created new material: "+req.Title,
	)
	if err != nil {
		log.Printf("Warning: Failed to log activity: %v", err)
		// Continue despite error in logging
	}
	*/

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error committing transaction: %v", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Material created successfully",
		"material_id": materialID,
	})
}

// UpdateMaterial handles updating existing educational content
func UpdateMaterial(c *gin.Context, db *sql.DB) {
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Ensure user is a teacher
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can update materials"})
		return
	}

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
		return
	}

	var req UpdateMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Verify the teacher owns this material
	var count int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM material m
		JOIN classroom c ON m.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE m.material_id = ? AND t.user_id = ?`,
		materialID, userID,
	).Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error verifying material ownership: %v", err)
		return
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to update this material"})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// Update material - using dynamic query building to handle optional fields
	query := "UPDATE material SET uploaded_at = NOW()"
	params := []interface{}{}

	if req.Title != "" {
		query += ", title = ?"
		params = append(params, req.Title)
	}
	if req.Description != "" {
		query += ", description = ?"
		params = append(params, req.Description)
	}
	if req.Type != "" {
		query += ", type = ?"
		params = append(params, req.Type)
	}
	if req.FilePath != "" {
		query += ", file_path = ?"
		params = append(params, req.FilePath)
	}

	query += " WHERE material_id = ?"
	params = append(params, materialID)

	_, err = tx.Exec(query, params...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating material"})
		log.Printf("Error updating material: %v", err)
		return
	}

	/*
	// Commented out due to missing activity_log table
	_, err = tx.Exec(
		`INSERT INTO activity_log 
		(user_id, activity_type, resource_id, resource_type, description) 
		VALUES (?, 'update', ?, 'material', ?)`,
		userID, materialID, "Updated material",
	)
	if err != nil {
		log.Printf("Warning: Failed to log activity: %v", err)
		// Continue despite error in logging
	}
	*/

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error committing transaction: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Material updated successfully",
	})
}

// DeleteMaterial handles removal of educational content
func DeleteMaterial(c *gin.Context, db *sql.DB) {
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Ensure user is a teacher
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can delete materials"})
		return
	}

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
		return
	}

	// Verify the teacher owns this material
	var count int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM material m
		JOIN classroom c ON m.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE m.material_id = ? AND t.user_id = ?`,
		materialID, userID,
	).Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error verifying material ownership: %v", err)
		return
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to delete this material"})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback()

	/*
	// Commented out due to missing material_access_log table
	_, err = tx.Exec("DELETE FROM material_access_log WHERE material_id = ?", materialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error removing material access logs"})
		log.Printf("Error deleting material access logs: %v", err)
		return
	}
	*/

	/*
	// Commented out due to missing material_stats table
	_, err = tx.Exec("DELETE FROM material_stats WHERE material_id = ?", materialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error removing material stats"})
		log.Printf("Error deleting material stats: %v", err)
		return
	}
	*/

	// Delete the material itself
	_, err = tx.Exec("DELETE FROM material WHERE material_id = ?", materialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting material"})
		log.Printf("Error deleting material: %v", err)
		return
	}

	/*
	// Commented out due to missing activity_log table
	_, err = tx.Exec(
		`INSERT INTO activity_log 
		(user_id, activity_type, resource_id, resource_type, description) 
		VALUES (?, 'delete', ?, 'material', ?)`,
		userID, materialID, "Deleted material",
	)
	if err != nil {
		log.Printf("Warning: Failed to log activity: %v", err)
		// Continue despite error in logging
	}
	*/

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error committing transaction: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Material deleted successfully",
	})
}

// GetMaterialsByClassroom retrieves all materials for a specific classroom
func GetMaterialsByClassroom(c *gin.Context, db *sql.DB) {
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	courseID, err := strconv.Atoi(c.Param("classroom_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid classroom ID"})
		return
	}

	// Check if the user is authorized to view this classroom's materials
	var count int
	var query string
	var queryParams []interface{}

	if role == "teacher" {
		// For teachers, check if they own the classroom
		query = `SELECT COUNT(*) FROM classroom c
				 JOIN teacher t ON c.teacher_id = t.teacher_id
				 WHERE c.course_id = ? AND t.user_id = ?`
		queryParams = []interface{}{courseID, userID}
	} else if role == "student" {
		// For students, check if they are enrolled in the classroom
		query = `SELECT COUNT(*) FROM enrollment e
				 WHERE e.course_id = ? AND e.student_id = (SELECT student_id FROM student WHERE user_id = ?)`
		queryParams = []interface{}{courseID, userID}
	} else if role == "admin" || role == "dev" {
		// Admins and devs have access to all classrooms
		count = 1
	}

	if query != "" {
		err = db.QueryRow(query, queryParams...).Scan(&count)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking classroom access: %v", err)
			return
		}
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this classroom"})
		return
	}

	// Query to get all materials for the classroom
	rows, err := db.Query(
		`SELECT material_id, course_id, title, description, type, file_path, uploaded_at
		 FROM material
		 WHERE course_id = ?
		 ORDER BY uploaded_at DESC`,
		courseID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving materials"})
		log.Printf("Error querying materials: %v", err)
		return
	}
	defer rows.Close()

	materials := []Material{}
	for rows.Next() {
		var material Material
		err := rows.Scan(
			&material.ID, &material.CourseID, &material.Title, &material.Description,
			&material.Type, &material.FilePath, &material.UploadedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing materials"})
			log.Printf("Error scanning material row: %v", err)
			return
		}
		materials = append(materials, material)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving materials"})
		log.Printf("Error iterating material rows: %v", err)
		return
	}

	c.JSON(http.StatusOK, materials)
}

// GetMaterialDetails retrieves complete details for a specific material
func GetMaterialDetails(c *gin.Context, db *sql.DB) {
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	materialID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid material ID"})
		return
	}

	// Get the material details
	var material Material
	err = db.QueryRow(
		`SELECT material_id, course_id, title, description, type, file_path, uploaded_at
		 FROM material
		 WHERE material_id = ?`,
		materialID,
	).Scan(
		&material.ID, &material.CourseID, &material.Title, &material.Description,
		&material.Type, &material.FilePath, &material.UploadedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Material not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving material"})
			log.Printf("Error querying material details: %v", err)
		}
		return
	}

	// Check if the user has access to this material
	var hasAccess bool
	if role == "admin" || role == "dev" {
		hasAccess = true
	} else {
		var count int
		var query string
		var queryParams []interface{}

		if role == "teacher" {
			// Teachers can access if they own the classroom
			query = `SELECT COUNT(*) FROM classroom c
					 JOIN teacher t ON c.teacher_id = t.teacher_id
					 WHERE c.course_id = ? AND t.user_id = ?`
			queryParams = []interface{}{material.CourseID, userID}
		} else if role == "student" {
			// Students can access if they're enrolled
			query = `SELECT COUNT(*) FROM enrollment e
					 WHERE e.course_id = ? AND e.student_id = (SELECT student_id FROM student WHERE user_id = ?)`
			queryParams = []interface{}{material.CourseID, userID}
		}

		err = db.QueryRow(query, queryParams...).Scan(&count)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking material access: %v", err)
			return
		}
		hasAccess = count > 0
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this material"})
		return
	}

	/*
	// Commented out due to missing material_access_log table
	if role == "student" {
		// Log this view asynchronously
		go func() {
			_, err := db.Exec(
				"INSERT INTO material_access_log (material_id, student_id, access_type) VALUES (?, (SELECT student_id FROM student WHERE user_id = ?), 'view')",
				materialID, userID,
			)
			if err != nil {
				log.Printf("Error logging material view: %v", err)
			}

			// Update view count
			_, err = db.Exec(
				"UPDATE material_stats SET views = views + 1 WHERE material_id = ?",
				materialID,
			)
			if err != nil {
				log.Printf("Error updating material view count: %v", err)
			}
		}()
	}
	*/

	c.JSON(http.StatusOK, material)
}

// Other handlers (GetMaterialStats, OrganizeMaterials, SearchMaterials, LogMaterialDownload, BulkUpdateMaterials)
// are commented out as they rely on undefined tables (material_stats, material_access_log, classroom_enrollment, activity_log)

/*
func GetMaterialStats(c *gin.Context, db *sql.DB) {
	// ... (relies on material_stats and material_access_log)
}

func OrganizeMaterials(c *gin.Context, db *sql.DB) {
	// ... (relies on activity_log)
}

func SearchMaterials(c *gin.Context, db *sql.DB) {
	// ... (relies on classroom_enrollment)
}

func LogMaterialDownload(c *gin.Context, db *sql.DB) {
	// ... (relies on material_access_log and material_stats)
}

func BulkUpdateMaterials(c *gin.Context, db *sql.DB) {
	// ... (relies on activity_log)
}
*/