package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"edusync/models"
	

	"github.com/gin-gonic/gin"
)

// TeacherStats represents statistics for teacher dashboard
type TeacherStats struct {
	TotalClassrooms    int       `json:"total_classrooms"`
	TotalStudents      int       `json:"total_students"`
	TotalMaterials     int       `json:"total_materials"`
	TotalAssignments   int       `json:"total_assignments"`
	TotalAnnouncements int       `json:"total_announcements"`
	LastActive         time.Time `json:"last_active"`
}

// RecentActivity represents recent activity for teacher dashboard
type RecentActivity struct {
	ActivityID   int       `json:"activity_id"`
	ActivityType string    `json:"activity_type"` // "material", "assignment", "announcement", etc.
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	ClassroomID  int       `json:"classroom_id"`
	ClassTitle   string    `json:"class_title"`
	CreatedAt    time.Time `json:"created_at"`
}

// UpcomingDeadline represents upcoming assignment deadlines
type UpcomingDeadline struct {
	AssignmentID int       `json:"assignment_id"`
	Title        string    `json:"title"`
	ClassroomID  int       `json:"classroom_id"`
	ClassTitle   string    `json:"class_title"`
	DueDate      time.Time `json:"due_date"`
}

// ClassSchedule represents a teacher's class schedule
type ClassSchedule struct {
	ClassroomID int       `json:"classroom_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
}

// TeacherDashboard represents teacher dashboard data
type TeacherDashboard struct {
	Teacher           models.Teacher       `json:"teacher"`
	Stats             TeacherStats         `json:"stats"`
	RecentActivities  []RecentActivity     `json:"recent_activities"`
	UpcomingDeadlines []UpcomingDeadline   `json:"upcoming_deadlines"`
	ClassSchedules    []ClassSchedule      `json:"class_schedules"`
}

// GetTeacherProfileHandler retrieves teacher profile data
func GetTeacherProfileHandler(c *gin.Context) {
	// Get user ID from context (set by authMiddleware)
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Check if user has teacher role
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Get user info
	var user models.User
	err := db.QueryRow(
		"SELECT user_id, name, email, role, contact_number, profile_picture, org, created_at FROM user WHERE user_id = ?",
		userID,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.ContactNum, &user.ProfilePic, &user.Org, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving user: %v", err)
		}
		return
	}

	// Get teacher data
	var teacher models.Teacher
	err = db.QueryRow(
		"SELECT teacher_id, user_id, dept FROM teacher WHERE user_id = ?",
		userID,
	).Scan(&teacher.TeacherID, &teacher.UserID, &teacher.Dept)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher data not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving teacher data: %v", err)
		}
		return
	}

	// Get teaching history (classrooms taught)
	rows, err := db.Query(
		`SELECT course_id, title, description, start_date, end_date, subject_area 
		FROM classroom 
		WHERE teacher_id = (SELECT teacher_id FROM teacher WHERE user_id = ?)
		ORDER BY start_date DESC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving teaching history"})
		log.Printf("Error retrieving teaching history: %v", err)
		return
	}
	defer rows.Close()

	var classrooms []models.Classroom
	for rows.Next() {
		var classroom models.Classroom
		err := rows.Scan(
			&classroom.CourseID, &classroom.Title, &classroom.Description,
			&classroom.StartDate, &classroom.EndDate, &classroom.SubjectArea,
		)
		if err != nil {
			log.Printf("Error scanning classroom row: %v", err)
			continue
		}
		classrooms = append(classrooms, classroom)
	}

	// Prepare response
	teacher.Name = user.Name // Populate Name from User
	teacher.UserID = userID
	response := gin.H{
		"user":       user,
		"teacher":    teacher,
		"classrooms": classrooms,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateTeacherProfileHandler updates teacher profile information
func UpdateTeacherProfileHandler(c *gin.Context) {
	// Get user ID from context (set by authMiddleware)
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Check if user has teacher role
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Parse request body
	var updateRequest struct {
		Name       string `json:"name"`
		ContactNum string `json:"contact_number"`
		ProfilePic string `json:"profile_picture"`
		Org        string `json:"org"`
		Dept       string `json:"dept"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback() // Will be ignored if tx.Commit() is called

	// Update user table
	_, err = tx.Exec(
		"UPDATE user SET name = ?, contact_number = ?, profile_picture = ?, org = ? WHERE user_id = ?",
		updateRequest.Name, updateRequest.ContactNum, updateRequest.ProfilePic, updateRequest.Org, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating user"})
		log.Printf("Error updating user: %v", err)
		return
	}

	// Update teacher table
	_, err = tx.Exec(
		"UPDATE teacher SET dept = ? WHERE user_id = ?",
		updateRequest.Dept, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating teacher profile"})
		log.Printf("Error updating teacher data: %v", err)
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		log.Printf("Error committing transaction: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Teacher profile updated successfully"})
}

// GetTeacherDashboardHandler retrieves dashboard data for teacher
func GetTeacherDashboardHandler(c *gin.Context) {
	// Get user ID from context
	userID := c.MustGet("userID").(int)
	role := c.MustGet("role").(string)

	// Check if user has teacher role
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get database connection
	db := c.MustGet("db").(*sql.DB)

	// Get teacher data
	var teacher models.Teacher
	err := db.QueryRow(
		`SELECT t.teacher_id, t.user_id, t.dept, u.name
		FROM teacher t
		JOIN user u ON t.user_id = u.user_id
		WHERE t.user_id = ?`,
		userID,
	).Scan(&teacher.TeacherID, &teacher.UserID, &teacher.Dept, &teacher.Name)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			log.Printf("Error retrieving teacher: %v", err)
		}
		return
	}

	// Get teacher stats
	var stats TeacherStats
	var lastActiveStr string
	err = db.QueryRow(
		`SELECT 
			(SELECT COUNT(*) FROM classroom WHERE teacher_id = ?) AS total_classrooms,
			(SELECT COUNT(*) FROM enrollment e 
				JOIN classroom c ON e.course_id = c.course_id 
				WHERE c.teacher_id = ?) AS total_students,
			(SELECT COUNT(*) FROM material m 
				JOIN classroom c ON m.course_id = c.course_id 
				WHERE c.teacher_id = ?) AS total_materials,
			(SELECT COUNT(*) FROM assignment a 
				JOIN classroom c ON a.course_id = c.course_id 
				WHERE c.teacher_id = ?) AS total_assignments,
			(SELECT COUNT(*) FROM announcement a 
				JOIN classroom c ON a.course_id = c.course_id 
				WHERE c.teacher_id = ?) AS total_announcements,
			(SELECT IFNULL(MAX(m.uploaded_at), NOW()) FROM material m 
				JOIN classroom c ON m.course_id = c.course_id 
				WHERE c.teacher_id = ?) AS last_active`,
		teacher.TeacherID, teacher.TeacherID, teacher.TeacherID, teacher.TeacherID, teacher.TeacherID, teacher.TeacherID,
	).Scan(
		&stats.TotalClassrooms, &stats.TotalStudents, &stats.TotalMaterials,
		&stats.TotalAssignments, &stats.TotalAnnouncements, &lastActiveStr,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving teacher statistics"})
		log.Printf("Error retrieving teacher stats: %v", err)
		return
	}

	// Parse last active time
	stats.LastActive, err = time.Parse("2006-01-02 15:04:05", lastActiveStr)
	if err != nil {
		log.Printf("Error parsing last active time: %v", err)
		stats.LastActive = time.Now() // Use current time as fallback
	}

	// Get recent activities
	activities, err := getRecentActivities(db, teacher.TeacherID)
	if err != nil {
		log.Printf("Error retrieving recent activities: %v", err)
		activities = []RecentActivity{}
	}

	// Get upcoming deadlines
	deadlines, err := getUpcomingDeadlines(db, teacher.TeacherID)
	if err != nil {
		log.Printf("Error retrieving upcoming deadlines: %v", err)
		deadlines = []UpcomingDeadline{}
	}

	// Get class schedules
	schedules, err := getClassSchedules(db, teacher.TeacherID)
	if err != nil {
		log.Printf("Error retrieving class schedules: %v", err)
		schedules = []ClassSchedule{}
	}

	// Prepare dashboard response
	dashboard := TeacherDashboard{
		Teacher:           teacher,
		Stats:             stats,
		RecentActivities:  activities,
		UpcomingDeadlines: deadlines,
		ClassSchedules:    schedules,
	}

	c.JSON(http.StatusOK, dashboard)
}

// getRecentActivities retrieves recent activities for a teacher
func getRecentActivities(db *sql.DB, teacherID int) ([]RecentActivity, error) {
	query := `
	(SELECT 
		m.material_id AS id, 
		'material' AS type, 
		m.title, 
		m.description, 
		m.course_id, 
		c.title AS class_title, 
		m.uploaded_at AS created_at
	FROM material m
	JOIN classroom c ON m.course_id = c.course_id
	WHERE c.teacher_id = ?)
	
	UNION
	
	(SELECT 
		a.assignment_id AS id, 
		'assignment' AS type, 
		a.title, 
		a.description, 
		a.course_id, 
		c.title AS class_title, 
		a.created_at
	FROM assignment a
	JOIN classroom c ON a.course_id = c.course_id
	WHERE c.teacher_id = ?)
	
	UNION
	
	(SELECT 
		a.announcement_id AS id, 
		'announcement' AS type, 
		a.title, 
		a.content AS description, 
		a.course_id, 
		c.title AS class_title, 
		a.created_at
	FROM announcement a
	JOIN classroom c ON a.course_id = c.course_id
	WHERE c.teacher_id = ?)
	
	ORDER BY created_at DESC
	LIMIT 10
	`

	rows, err := db.Query(query, teacherID, teacherID, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []RecentActivity
	for rows.Next() {
		var activity RecentActivity
		var createdAtStr string

		err := rows.Scan(
			&activity.ActivityID,
			&activity.ActivityType,
			&activity.Title,
			&activity.Description,
			&activity.ClassroomID,
			&activity.ClassTitle,
			&createdAtStr,
		)
		if err != nil {
			log.Printf("Error scanning activity row: %v", err)
			continue
		}

		activity.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			log.Printf("Error parsing created at time: %v", err)
			activity.CreatedAt = time.Now()
		}

		activities = append(activities, activity)
	}

	return activities, nil
}

// getUpcomingDeadlines retrieves upcoming assignment deadlines for a teacher
func getUpcomingDeadlines(db *sql.DB, teacherID int) ([]UpcomingDeadline, error) {
	query := `
	SELECT 
		a.assignment_id, 
		a.title, 
		a.course_id, 
		c.title AS class_title, 
		a.due_date
	FROM assignment a
	JOIN classroom c ON a.course_id = c.course_id
	WHERE c.teacher_id = ? AND a.due_date >= CURRENT_DATE()
	ORDER BY a.due_date ASC
	LIMIT 5
	`

	rows, err := db.Query(query, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deadlines []UpcomingDeadline
	for rows.Next() {
		var deadline UpcomingDeadline
		var dueDateStr string

		err := rows.Scan(
			&deadline.AssignmentID,
			&deadline.Title,
			&deadline.ClassroomID,
			&deadline.ClassTitle,
			&dueDateStr,
		)
		if err != nil {
			log.Printf("Error scanning deadline row: %v", err)
			continue
		}

		deadline.DueDate, err = time.Parse("2006-01-02", dueDateStr)
		if err != nil {
			log.Printf("Error parsing due date: %v", err)
			deadline.DueDate = time.Now().AddDate(0, 0, 7)
		}

		deadlines = append(deadlines, deadline)
	}

	return deadlines, nil
}

// getClassSchedules retrieves class schedules for a teacher
func getClassSchedules(db *sql.DB, teacherID int) ([]ClassSchedule, error) {
	query := `
	SELECT 
		course_id, 
		title, 
		description, 
		start_date, 
		end_date
	FROM classroom
	WHERE teacher_id = ? AND end_date >= CURRENT_DATE()
	ORDER BY start_date ASC
	`

	rows, err := db.Query(query, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []ClassSchedule
	for rows.Next() {
		var schedule ClassSchedule
		var startDateStr, endDateStr string

		err := rows.Scan(
			&schedule.ClassroomID,
			&schedule.Title,
			&schedule.Description,
			&startDateStr,
			&endDateStr,
		)
		if err != nil {
			log.Printf("Error scanning schedule row: %v", err)
			continue
		}

		schedule.StartDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			log.Printf("Error parsing start date: %v", err)
		}

		schedule.EndDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			log.Printf("Error parsing end date: %v", err)
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// GetTeacherStatsHandler retrieves detailed statistics for a teacher
func GetTeacherStatsHandler(c *gin.Context) {
	// Get user ID from context
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

	// Get detailed statistics
	// Class performance metrics
	var classStats []struct {
		ClassroomID    int     `json:"classroom_id"`
		ClassTitle     string  `json:"class_title"`
		StudentCount   int     `json:"student_count"`
		AssignmentAvg  float64 `json:"assignment_avg"`
		CompletionRate float64 `json:"completion_rate"`
	}

	classQuery := `
	SELECT 
		c.course_id,
		c.title,
		COUNT(DISTINCT e.student_id) AS student_count,
		IFNULL(AVG(s.score), 0) AS assignment_avg,
		IFNULL(
			(COUNT(s.submission_id) / 
			(COUNT(DISTINCT e.student_id) * GREATEST(COUNT(DISTINCT a.assignment_id), 1))) * 100, 
			0
		) AS completion_rate
	FROM classroom c
	LEFT JOIN enrollment e ON c.course_id = e.course_id
	LEFT JOIN assignment a ON c.course_id = a.course_id
	LEFT JOIN submission s ON a.assignment_id = s.assignment_id AND e.student_id = s.student_id
	WHERE c.teacher_id = ?
	GROUP BY c.course_id
	ORDER BY c.start_date DESC
	`

	classRows, err := db.Query(classQuery, teacherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving class statistics"})
		log.Printf("Error retrieving class statistics: %v", err)
		return
	}
	defer classRows.Close()

	for classRows.Next() {
		var stat struct {
			ClassroomID    int     `json:"classroom_id"`
			ClassTitle     string  `json:"class_title"`
			StudentCount   int     `json:"student_count"`
			AssignmentAvg  float64 `json:"assignment_avg"`
			CompletionRate float64 `json:"completion_rate"`
		}

		err := classRows.Scan(
			&stat.ClassroomID,
			&stat.ClassTitle,
			&stat.StudentCount,
			&stat.AssignmentAvg,
			&stat.CompletionRate,
		)
		if err != nil {
			log.Printf("Error scanning class stats row: %v", err)
			continue
		}

		classStats = append(classStats, stat)
	}

	// Assignment engagement metrics
	var assignmentStats []struct {
		AssignmentID       int       `json:"assignment_id"`
		Title              string    `json:"title"`
		ClassroomID        int       `json:"classroom_id"`
		ClassTitle         string    `json:"class_title"`
		SubmissionCount    int       `json:"submission_count"`
		AvgScore           float64   `json:"avg_score"`
		DueDate            time.Time `json:"due_date"`
		EarlySubmissions   int       `json:"early_submissions"`
		OnTimeSubmissions  int       `json:"on_time_submissions"`
		LateSubmissions    int       `json:"late_submissions"`
		MissingSubmissions int       `json:"missing_submissions"`
	}

	assignmentQuery := `
	SELECT 
		a.assignment_id,
		a.title,
		a.course_id,
		c.title AS class_title,
		COUNT(s.submission_id) AS submission_count,
		IFNULL(AVG(s.score), 0) AS avg_score,
		a.due_date,
		SUM(CASE WHEN s.submitted_at < DATE_SUB(a.due_date, INTERVAL 1 DAY) THEN 1 ELSE 0 END) AS early_submissions,
		SUM(CASE WHEN s.submitted_at <= a.due_date AND s.submitted_at >= DATE_SUB(a.due_date, INTERVAL 1 DAY) THEN 1 ELSE 0 END) AS on_time_submissions,
		SUM(CASE WHEN s.submitted_at > a.due_date THEN 1 ELSE 0 END) AS late_submissions,
		(SELECT COUNT(DISTINCT e.student_id) FROM enrollment e WHERE e.course_id = a.course_id) - COUNT(s.submission_id) AS missing_submissions
	FROM assignment a
	JOIN classroom c ON a.course_id = c.course_id
	LEFT JOIN submission s ON a.assignment_id = s.assignment_id
	WHERE c.teacher_id = ?
	GROUP BY a.assignment_id
	ORDER BY a.due_date DESC
	LIMIT 10
	`

	assignmentRows, err := db.Query(assignmentQuery, teacherID)
	if err != nil {
		log.Printf("Error retrieving assignment statistics: %v", err)
	} else {
		defer assignmentRows.Close()

		for assignmentRows.Next() {
			var stat struct {
				AssignmentID       int       `json:"assignment_id"`
				Title              string    `json:"title"`
				ClassroomID        int       `json:"classroom_id"`
				ClassTitle         string    `json:"class_title"`
				SubmissionCount    int       `json:"submission_count"`
				AvgScore           float64   `json:"avg_score"`
				DueDate            time.Time `json:"due_date"`
				EarlySubmissions   int       `json:"early_submissions"`
				OnTimeSubmissions  int       `json:"on_time_submissions"`
				LateSubmissions    int       `json:"late_submissions"`
				MissingSubmissions int       `json:"missing_submissions"`
			}
			var dueDateStr string

			err := assignmentRows.Scan(
				&stat.AssignmentID,
				&stat.Title,
				&stat.ClassroomID,
				&stat.ClassTitle,
				&stat.SubmissionCount,
				&stat.AvgScore,
				&dueDateStr,
				&stat.EarlySubmissions,
				&stat.OnTimeSubmissions,
				&stat.LateSubmissions,
				&stat.MissingSubmissions,
			)
			if err != nil {
				log.Printf("Error scanning assignment stats row: %v", err)
				continue
			}

			stat.DueDate, err = time.Parse("2006-01-02", dueDateStr)
			if err != nil {
				log.Printf("Error parsing due date: %v", err)
			}

			assignmentStats = append(assignmentStats, stat)
		}
	}

	// Prepare response
	response := gin.H{
		"teacher_id":        teacherID,
		"class_stats":       classStats,
		"assignment_stats":  assignmentStats,
		"total_classrooms":  len(classStats),
		"total_assignments": len(assignmentStats),
	}

	c.JSON(http.StatusOK, response)
}