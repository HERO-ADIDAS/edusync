package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"edusync/models"
)

// UpdateStudentProfileHandler updates a student's profile
func UpdateStudentProfileHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can update their profiles"})
		return
	}

	var req models.Student
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var studentID int
	err := db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	_, err = db.Exec(`
		UPDATE student 
		SET grade_level = ?, enrollment_year = ?
		WHERE student_id = ? AND archive_delete_flag = TRUE`,
		req.GradeLevel, req.EnrollmentYear, studentID)
	if err != nil {
		log.Printf("Error updating student profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update student profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"student_id":      studentID,
		"grade_level":     req.GradeLevel,
		"enrollment_year": req.EnrollmentYear,
	})
}

// GetStudentDashboardHandler retrieves the student's dashboard data
func GetStudentDashboardHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can view their dashboard"})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var studentID int
	err := db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Get enrolled courses with teacher name and subject area
	rows, err := db.Query(`
		SELECT c.course_id, c.title, c.description, c.subject_area, u.name AS teacher_name
		FROM enrollment e
		JOIN classroom c ON e.course_id = c.course_id
		LEFT JOIN teacher t ON c.teacher_id = t.teacher_id
		LEFT JOIN user u ON t.user_id = u.user_id
		WHERE e.student_id = ? AND e.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE`, studentID)
	if err != nil {
		log.Printf("Error querying enrollments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var courses []map[string]interface{}
	var courseIDs []int
	for rows.Next() {
		var courseID int
		var title, description, subjectArea, teacherName sql.NullString
		if err := rows.Scan(&courseID, &title, &description, &subjectArea, &teacherName); err != nil {
			log.Printf("Error scanning course: %v", err)
			continue
		}
		course := map[string]interface{}{
			"course_id":    courseID,
			"title":        title.String,
			"description":  description.String,
			"subject_area": subjectArea.String,
			"teacher_name": teacherName.String,
		}
		courses = append(courses, course)
		courseIDs = append(courseIDs, courseID)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating enrollments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Calculate upcoming assignments for each course
	if len(courseIDs) > 0 {
		for _, course := range courses {
			var upcomingAssignments int
			err = db.QueryRow(`
				SELECT COUNT(*) 
				FROM assignment 
				WHERE course_id = ? AND due_date > NOW() AND archive_delete_flag = TRUE`, course["course_id"]).Scan(&upcomingAssignments)
			if err != nil {
				log.Printf("Error counting upcoming assignments for course %v: %v", course["course_id"], err)
				continue
			}
			course["upcoming_assignments"] = upcomingAssignments
		}
	}

	// Get recent submissions
	rows, err = db.Query(`
		SELECT s.submission_id, s.assignment_id, s.submitted_at, s.status
		FROM submission s
		WHERE s.student_id = ? AND s.archive_delete_flag = TRUE
		ORDER BY s.submitted_at DESC
		LIMIT 5`, studentID)
	if err != nil {
		log.Printf("Error querying submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var submissions []map[string]interface{}
	for rows.Next() {
		var submissionID, assignmentID int
		var submittedAt time.Time
		var status string
		if err := rows.Scan(&submissionID, &assignmentID, &submittedAt, &status); err != nil {
			log.Printf("Error scanning submission: %v", err)
			continue
		}
		submissions = append(submissions, map[string]interface{}{
			"submission_id": submissionID,
			"assignment_id": assignmentID,
			"submitted_at":  submittedAt,
			"status":        status,
		})
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Define placeholders and args for the IN clause once, to be reused
	var placeholders []string
	var args []interface{}
	if len(courseIDs) > 0 {
		for i := range courseIDs {
			placeholders = append(placeholders, "?")
			args = append(args, courseIDs[i])
		}
	}

	// Get pinned announcements for the student's enrolled courses
	var pinnedAnnouncements []map[string]interface{}
	if len(courseIDs) > 0 {
		query := `
			SELECT a.announcement_id, a.course_id, a.title, a.content, a.created_at, a.is_pinned
		 FROM announcement a
		 WHERE a.course_id IN (` + strings.Join(placeholders, ",") + `)
		 AND a.is_pinned = TRUE AND a.archive_delete_flag = TRUE`

		rows, err = db.Query(query, args...)
		if err != nil {
			log.Printf("Error querying pinned announcements: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var a models.Announcement
			if err := rows.Scan(&a.AnnouncementID, &a.CourseID, &a.Title, &a.Content, &a.CreatedAt, &a.IsPinned); err != nil {
				log.Printf("Error scanning pinned announcement: %v", err)
				continue
			}
			pinnedAnnouncements = append(pinnedAnnouncements, map[string]interface{}{
				"announcement_id": a.AnnouncementID,
				"course_id":       a.CourseID,
				"title":           a.Title,
				"content":         a.Content,
				"created_at":      a.CreatedAt,
				"is_pinned":       a.IsPinned,
			})
		}

		if err = rows.Err(); err != nil {
			log.Printf("Error iterating pinned announcements: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	}

	// Get recent announcements (last 5) for the student's enrolled courses
	var recentAnnouncements []map[string]interface{}
	if len(courseIDs) > 0 {
		query := `
			SELECT a.announcement_id, a.course_id, a.title, a.content, a.created_at, a.is_pinned
		 FROM announcement a
		 WHERE a.course_id IN (` + strings.Join(placeholders, ",") + `)
		 AND a.archive_delete_flag = TRUE
		 ORDER BY a.created_at DESC
		 LIMIT 5`

		rows, err = db.Query(query, args...)
		if err != nil {
			log.Printf("Error querying recent announcements: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var a models.Announcement
			if err := rows.Scan(&a.AnnouncementID, &a.CourseID, &a.Title, &a.Content, &a.CreatedAt, &a.IsPinned); err != nil {
				log.Printf("Error scanning recent announcement: %v", err)
				continue
			}
			recentAnnouncements = append(recentAnnouncements, map[string]interface{}{
				"announcement_id": a.AnnouncementID,
				"course_id":       a.CourseID,
				"title":           a.Title,
				"content":         a.Content,
				"created_at":      a.CreatedAt,
				"is_pinned":       a.IsPinned,
			})
		}

		if err = rows.Err(); err != nil {
			log.Printf("Error iterating recent announcements: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	}

	// Get assignments due soon (within the next 24 hours)
	var dueSoonAssignments []map[string]interface{}
	if len(courseIDs) > 0 {
		now := time.Now()
		dueThreshold := now.Add(24 * time.Hour)
		query := `
			SELECT a.assignment_id, a.course_id, a.title, a.description, a.due_date, a.max_points
		 FROM assignment a
		 WHERE a.course_id IN (` + strings.Join(placeholders, ",") + `)
		 AND a.due_date BETWEEN ? AND ?
		 AND a.archive_delete_flag = TRUE`
		
		// Create a new args slice for this query, starting with courseIDs
		dueArgs := make([]interface{}, len(args), len(args)+2)
		copy(dueArgs, args)
		dueArgs = append(dueArgs, now, dueThreshold)

		rows, err = db.Query(query, dueArgs...)
		if err != nil {
			log.Printf("Error querying due soon assignments: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var a models.Assignment
			if err := rows.Scan(&a.AssignmentID, &a.CourseID, &a.Title, &a.Description, &a.DueDate, &a.MaxPoints); err != nil {
				log.Printf("Error scanning due soon assignment: %v", err)
				continue
			}
			dueSoonAssignments = append(dueSoonAssignments, map[string]interface{}{
				"assignment_id": a.AssignmentID,
				"course_id":     a.CourseID,
				"title":         a.Title,
				"description":   a.Description,
				"due_date":      a.DueDate,
				"max_points":    a.MaxPoints,
			})
		}

		if err = rows.Err(); err != nil {
			log.Printf("Error iterating due soon assignments: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"student_id":           studentID,
		"courses":              courses,
		"submissions":          submissions,
		"pinned_announcements": pinnedAnnouncements,
		"recent_announcements": recentAnnouncements,
		"due_soon_assignments": dueSoonAssignments,
	})
}