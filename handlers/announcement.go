//announcement 

package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Announcement represents the announcement data structure
type Announcement struct {
	ID          int       `json:"id"`
	ClassroomID int       `json:"classroom_id"`
	TeacherID   int       `json:"teacher_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	IsPinned    bool      `json:"is_pinned"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ViewCount   int       `json:"view_count,omitempty"`
}

// CreateAnnouncementRequest contains data for creating a new announcement
type CreateAnnouncementRequest struct {
	ClassroomID int    `json:"classroom_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Content     string `json:"content" binding:"required"`
	IsPinned    bool   `json:"is_pinned"`
}

// CreateAnnouncement handles creating a new announcement for a classroom
func CreateAnnouncement(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get teacher ID from context (set by authMiddleware)
		teacherID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Parse request body
		var req CreateAnnouncementRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Verify that the teacher has access to this classroom
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM classroom WHERE classroom_id = ? AND teacher_id = ?",
			req.ClassroomID, teacherID,
		).Scan(&count)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking classroom access: %v", err)
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this classroom"})
			return
		}

		// Insert the announcement
		result, err := db.Exec(
			"INSERT INTO announcement (classroom_id, teacher_id, title, content, is_pinned, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NOW(), NOW())",
			req.ClassroomID, teacherID, req.Title, req.Content, req.IsPinned,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating announcement"})
			log.Printf("Error inserting announcement: %v", err)
			return
		}

		// Get the inserted announcement ID
		announcementID, err := result.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving announcement ID"})
			log.Printf("Error getting last insert ID: %v", err)
			return
		}

		// Return success response
		c.JSON(http.StatusCreated, gin.H{
			"message":        "Announcement created successfully",
			"announcement_id": announcementID,
		})
	}
}

// UpdateAnnouncementRequest contains data for updating an announcement
type UpdateAnnouncementRequest struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	IsPinned bool   `json:"is_pinned"`
}

// UpdateAnnouncement handles updating an existing announcement
func UpdateAnnouncement(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get teacher ID from context
		teacherID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Get announcement ID from URL
		announcementID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
			return
		}

		// Parse request body
		var req UpdateAnnouncementRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Verify that the teacher owns this announcement
		var count int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM announcement WHERE announcement_id = ? AND teacher_id = ?",
			announcementID, teacherID,
		).Scan(&count)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking announcement ownership: %v", err)
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this announcement"})
			return
		}

		// Update the announcement
		_, err = db.Exec(
			"UPDATE announcement SET title = ?, content = ?, is_pinned = ?, updated_at = NOW() WHERE announcement_id = ?",
			req.Title, req.Content, req.IsPinned, announcementID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating announcement"})
			log.Printf("Error updating announcement: %v", err)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{"message": "Announcement updated successfully"})
	}
}

// DeleteAnnouncement handles deleting an announcement
func DeleteAnnouncement(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get teacher ID from context
		teacherID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Get announcement ID from URL
		announcementID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
			return
		}

		// Verify that the teacher owns this announcement
		var count int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM announcement WHERE announcement_id = ? AND teacher_id = ?",
			announcementID, teacherID,
		).Scan(&count)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking announcement ownership: %v", err)
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this announcement"})
			return
		}

		// Delete the announcement
		_, err = db.Exec("DELETE FROM announcement WHERE announcement_id = ?", announcementID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting announcement"})
			log.Printf("Error deleting announcement: %v", err)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{"message": "Announcement deleted successfully"})
	}
}

// GetAnnouncementsByClassroom handles listing all announcements for a classroom
func GetAnnouncementsByClassroom(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get teacher ID from context
		teacherID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Get classroom ID from URL
		classroomID, err := strconv.Atoi(c.Param("classroom_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid classroom ID"})
			return
		}

		// Verify that the teacher has access to this classroom
		var count int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM classroom WHERE classroom_id = ? AND teacher_id = ?",
			classroomID, teacherID,
		).Scan(&count)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking classroom access: %v", err)
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this classroom"})
			return
		}

		// Get announcements for the classroom
		rows, err := db.Query(
			`SELECT announcement_id, classroom_id, teacher_id, title, content, 
			is_pinned, created_at, updated_at, 
			(SELECT COUNT(*) FROM announcement_view WHERE announcement_id = a.announcement_id) as view_count
			FROM announcement a
			WHERE classroom_id = ?
			ORDER BY is_pinned DESC, created_at DESC`,
			classroomID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving announcements"})
			log.Printf("Error querying announcements: %v", err)
			return
		}
		defer rows.Close()

		// Parse results
		var announcements []Announcement
		for rows.Next() {
			var announcement Announcement
			err := rows.Scan(
				&announcement.ID,
				&announcement.ClassroomID,
				&announcement.TeacherID,
				&announcement.Title,
				&announcement.Content,
				&announcement.IsPinned,
				&announcement.CreatedAt,
				&announcement.UpdatedAt,
				&announcement.ViewCount,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing announcement data"})
				log.Printf("Error scanning announcement row: %v", err)
				return
			}
			announcements = append(announcements, announcement)
		}

		// Return the announcements
		c.JSON(http.StatusOK, gin.H{"announcements": announcements})
	}
}

// PinAnnouncement handles marking an announcement as pinned
func PinAnnouncement(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get teacher ID from context
		teacherID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Get announcement ID from URL
		announcementID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
			return
		}

		// Verify that the teacher owns this announcement
		var count int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM announcement WHERE announcement_id = ? AND teacher_id = ?",
			announcementID, teacherID,
		).Scan(&count)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking announcement ownership: %v", err)
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this announcement"})
			return
		}

		// Update the announcement to be pinned
		_, err = db.Exec(
			"UPDATE announcement SET is_pinned = true, updated_at = NOW() WHERE announcement_id = ?",
			announcementID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error pinning announcement"})
			log.Printf("Error pinning announcement: %v", err)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{"message": "Announcement pinned successfully"})
	}
}

// UnpinAnnouncement handles removing pinned status from an announcement
func UnpinAnnouncement(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get teacher ID from context
		teacherID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Get announcement ID from URL
		announcementID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
			return
		}

		// Verify that the teacher owns this announcement
		var count int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM announcement WHERE announcement_id = ? AND teacher_id = ?",
			announcementID, teacherID,
		).Scan(&count)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking announcement ownership: %v", err)
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this announcement"})
			return
		}

		// Update the announcement to remove pinned status
		_, err = db.Exec(
			"UPDATE announcement SET is_pinned = false, updated_at = NOW() WHERE announcement_id = ?",
			announcementID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unpinning announcement"})
			log.Printf("Error unpinning announcement: %v", err)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{"message": "Announcement unpinned successfully"})
	}
}

// GetAnnouncementStats handles retrieving view statistics for an announcement
func GetAnnouncementStats(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get teacher ID from context
		teacherID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Get announcement ID from URL
		announcementID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
			return
		}

		// Verify that the teacher owns this announcement
		var count int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM announcement WHERE announcement_id = ? AND teacher_id = ?",
			announcementID, teacherID,
		).Scan(&count)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error checking announcement ownership: %v", err)
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this announcement"})
			return
		}

		// Get announcement views
		var totalViews int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM announcement_view WHERE announcement_id = ?",
			announcementID,
		).Scan(&totalViews)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving view count"})
			log.Printf("Error counting views: %v", err)
			return
		}

		// Get unique viewers
		var uniqueViewers int
		err = db.QueryRow(
			"SELECT COUNT(DISTINCT student_id) FROM announcement_view WHERE announcement_id = ?",
			announcementID,
		).Scan(&uniqueViewers)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving unique viewers"})
			log.Printf("Error counting unique viewers: %v", err)
			return
		}

		// Get recent views (last 7 days)
		var recentViews int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM announcement_view WHERE announcement_id = ? AND viewed_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)",
			announcementID,
		).Scan(&recentViews)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving recent views"})
			log.Printf("Error counting recent views: %v", err)
			return
		}

		// Get daily view counts for the last 30 days
		rows, err := db.Query(
			`SELECT DATE(viewed_at) as view_date, COUNT(*) as view_count 
			FROM announcement_view 
			WHERE announcement_id = ? AND viewed_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
			GROUP BY DATE(viewed_at) 
			ORDER BY view_date DESC`,
			announcementID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving daily view counts"})
			log.Printf("Error querying daily views: %v", err)
			return
		}
		defer rows.Close()

		// Parse daily view results
		type DailyViews struct {
			Date  string `json:"date"`
			Count int    `json:"count"`
		}
		var dailyViews []DailyViews

		for rows.Next() {
			var dv DailyViews
			var viewDate time.Time
			err := rows.Scan(&viewDate, &dv.Count)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing view data"})
				log.Printf("Error scanning view row: %v", err)
				return
			}
			dv.Date = viewDate.Format("2006-01-02")
			dailyViews = append(dailyViews, dv)
		}

		// Return the statistics
		c.JSON(http.StatusOK, gin.H{
			"total_views":     totalViews,
			"unique_viewers":  uniqueViewers,
			"recent_views":    recentViews,
			"daily_view_data": dailyViews,
		})
	}
}