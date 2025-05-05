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

// CreateSubmissionHandler creates a new submission
func CreateSubmissionHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can submit assignments"})
		return
	}

	var req models.Submission
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch student: " + err.Error()})
		return
	}

	// Check if the assignment exists and fetch due date
	var courseID int
	var dueDate time.Time
	err = db.QueryRow(`
		SELECT course_id, due_date FROM assignment 
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`, req.AssignmentID).Scan(&courseID, &dueDate)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found"})
		return
	} else if err != nil {
		log.Printf("Error querying assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch assignment: " + err.Error()})
		return
	}

	// Check due date
	if time.Now().After(dueDate) {
		// Check if a submission exists
		var existingSubmissionID int
		err = db.QueryRow(`
			SELECT submission_id FROM submission 
			WHERE assignment_id = ? AND student_id = ? AND archive_delete_flag = TRUE`, req.AssignmentID, studentID).
			Scan(&existingSubmissionID)
		if err == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Due date is over. You submitted on time, but you can no longer update your submission"})
			return
		} else if err != sql.ErrNoRows {
			log.Printf("Error checking existing submission: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing submission: " + err.Error()})
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Due date is over. You cannot submit this assignment"})
		return
	}

	// Check if the student is enrolled in the course
	var enrolled bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM enrollment 
			WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
		)`, studentID, courseID).Scan(&enrolled)
	if err != nil {
		log.Printf("Error checking enrollment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check enrollment: " + err.Error()})
		return
	}
	if !enrolled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Student not enrolled in the course"})
		return
	}

	// Check for existing submission
	var existingSubmissionID int
	err = db.QueryRow(`
		SELECT submission_id FROM submission 
		WHERE assignment_id = ? AND student_id = ? AND archive_delete_flag = TRUE`, req.AssignmentID, studentID).
		Scan(&existingSubmissionID)
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already submitted this assignment"})
		return
	} else if err != sql.ErrNoRows {
		log.Printf("Error checking existing submission: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing submission: " + err.Error()})
		return
	}

	// Create submission
	result, err := db.Exec(`
		INSERT INTO submission (assignment_id, student_id, content, submitted_at, status, archive_delete_flag)
		VALUES (?, ?, ?, NOW(), 'submitted', TRUE)`,
		req.AssignmentID, studentID, req.Content)
	if err != nil {
		log.Printf("Error inserting submission: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create submission: " + err.Error()})
		return
	}

	submissionID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Error retrieving submission ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve submission ID: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Submission created successfully",
		"submission_id": submissionID,
		"assignment_id": req.AssignmentID,
		"student_id":    studentID,
		"status":        "submitted",
	})
}

// UpdateSubmissionHandler updates a submission
func UpdateSubmissionHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can update submissions"})
		return
	}

	submissionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	var req models.Submission
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	db := c.MustGet("db").(*sql.DB)
	var studentID int
	err = db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch student: " + err.Error()})
		return
	}

	// Check if the submission exists and belongs to the student, and fetch assignment_id
	var assignmentID int
	err = db.QueryRow(`
		SELECT assignment_id FROM submission 
		WHERE submission_id = ? AND student_id = ? AND archive_delete_flag = TRUE`, submissionID, studentID).
		Scan(&assignmentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found or unauthorized"})
		return
	} else if err != nil {
		log.Printf("Error querying submission: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch submission: " + err.Error()})
		return
	}

	// Fetch due date
	var dueDate time.Time
	err = db.QueryRow(`
		SELECT due_date FROM assignment 
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`, assignmentID).Scan(&dueDate)
	if err != nil {
		log.Printf("Error querying assignment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch assignment: " + err.Error()})
		return
	}

	// Check due date
	if time.Now().After(dueDate) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Due date is over. You can no longer update your submission"})
		return
	}

	// Update submission
	_, err = db.Exec(`
		UPDATE submission 
		SET content = ?, submitted_at = NOW(), status = 'submitted'
		WHERE submission_id = ? AND student_id = ? AND archive_delete_flag = TRUE`,
		req.Content, submissionID, studentID)
	if err != nil {
		log.Printf("Error updating submission: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update submission: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Submission updated successfully",
		"submission_id": submissionID,
		"content":       req.Content,
		"status":        "submitted",
	})
}

// GradeSubmissionHandler grades a submission
func GradeSubmissionHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can grade submissions"})
		return
	}

	userIDInt, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	submissionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	// Check if the submission exists
	db := c.MustGet("db").(*sql.DB)
	var submissionExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM submission 
			WHERE submission_id = ? AND archive_delete_flag = TRUE
		)`, submissionID).Scan(&submissionExists)
	if err != nil {
		log.Printf("Error checking submission existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check submission: " + err.Error()})
		return
	}
	if !submissionExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found"})
		return
	}

	// Check if the teacher is authorized to grade this submission
	var teacherID int
	err = db.QueryRow(`
		SELECT t.teacher_id
		FROM submission s
		JOIN assignment a ON s.assignment_id = a.assignment_id
		JOIN classroom c ON a.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE s.submission_id = ? AND s.archive_delete_flag = TRUE
		AND a.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE
		AND t.archive_delete_flag = TRUE AND t.user_id = ?`, submissionID, userIDInt).Scan(&teacherID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to grade this submission"})
		return
	} else if err != nil {
		log.Printf("Error querying teacher authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check authorization: " + err.Error()})
		return
	}

	var req struct {
		Score    int    `json:"score"`
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	_, err = db.Exec(`
		UPDATE submission 
		SET score = ?, feedback = ?, status = 'graded'
		WHERE submission_id = ? AND archive_delete_flag = TRUE`,
		req.Score, req.Feedback, submissionID)
	if err != nil {
		log.Printf("Error grading submission: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to grade submission: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"submission_id": submissionID,
		"score":         req.Score,
		"feedback":      req.Feedback,
		"status":        "graded",
	})
}

// GetSubmissionsByAssignmentHandler lists submissions for an assignment
func GetSubmissionsByAssignmentHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Role not found in context"})
		return
	}

	assignmentID, err := strconv.Atoi(c.Param("assignment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	// Check if the assignment exists
	db := c.MustGet("db").(*sql.DB)
	var assignmentExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM assignment 
			WHERE assignment_id = ? AND archive_delete_flag = TRUE
		)`, assignmentID).Scan(&assignmentExists)
	if err != nil {
		log.Printf("Error checking assignment existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check assignment: " + err.Error()})
		return
	}
	if !assignmentExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found"})
		return
	}

	var query string
	var args []interface{}

	if role == "teacher" {
		userIDInt, ok := userID.(int)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
			return
		}

		// Check if the teacher is authorized to view submissions for this assignment
		var teacherID int
		err = db.QueryRow(`
			SELECT t.teacher_id
			FROM assignment a
			JOIN classroom c ON a.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE a.assignment_id = ? AND a.archive_delete_flag = TRUE
			AND c.archive_delete_flag = TRUE AND t.archive_delete_flag = TRUE
			AND t.user_id = ?`, assignmentID, userIDInt).Scan(&teacherID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to view submissions for this assignment"})
			return
		} else if err != nil {
			log.Printf("Error checking teacher authorization: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check authorization: " + err.Error()})
			return
		}

		query = `
			SELECT s.submission_id, s.assignment_id, s.student_id, s.content, s.submitted_at, s.score, s.feedback, s.status
			FROM submission s
			JOIN student st ON s.student_id = st.student_id
			JOIN enrollment e ON st.student_id = e.student_id
			JOIN classroom c ON e.course_id = c.course_id
			JOIN teacher t ON c.teacher_id = t.teacher_id
			WHERE s.assignment_id = ? AND s.archive_delete_flag = TRUE AND st.archive_delete_flag = TRUE
			AND e.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE AND t.archive_delete_flag = TRUE
			AND t.user_id = ?`
		args = []interface{}{assignmentID, userIDInt}
	} else if role == "student" {
		userIDInt, ok := userID.(int)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
			return
		}

		var studentID int
		err := db.QueryRow(`
			SELECT student_id FROM student 
			WHERE user_id = ? AND archive_delete_flag = TRUE`, userIDInt).Scan(&studentID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
			return
		} else if err != nil {
			log.Printf("Error querying student: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch student: " + err.Error()})
			return
		}

		// Check if the student is enrolled in the course
		var courseID int
		err = db.QueryRow(`
			SELECT course_id FROM assignment 
			WHERE assignment_id = ? AND archive_delete_flag = TRUE`, assignmentID).Scan(&courseID)
		if err != nil {
			log.Printf("Error querying course ID: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch course: " + err.Error()})
			return
		}

		var enrolled bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM enrollment 
				WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
			)`, studentID, courseID).Scan(&enrolled)
		if err != nil {
			log.Printf("Error checking enrollment: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check enrollment: " + err.Error()})
			return
		}
		if !enrolled {
			c.JSON(http.StatusForbidden, gin.H{"error": "Student not enrolled in the course"})
			return
		}

		query = `
			SELECT submission_id, assignment_id, student_id, content, submitted_at, score, feedback, status
			FROM submission 
			WHERE assignment_id = ? AND student_id = ? AND archive_delete_flag = TRUE`
		args = []interface{}{assignmentID, studentID}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized role"})
		return
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch submissions: " + err.Error()})
		return
	}
	defer rows.Close()

	var submissions []models.Submission
	for rows.Next() {
		var s models.Submission
		var score sql.NullInt64
		var feedback sql.NullString
		if err := rows.Scan(&s.SubmissionID, &s.AssignmentID, &s.StudentID, &s.Content, &s.SubmittedAt, &score, &feedback, &s.Status); err != nil {
			log.Printf("Error scanning submission: %v", err)
			continue
		}
		if score.Valid {
			scoreValue := int(score.Int64)
			s.Score = &scoreValue
		}
		if feedback.Valid {
			feedbackValue := feedback.String
			s.Feedback = &feedbackValue
		}
		submissions = append(submissions, s)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to iterate submissions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, submissions)
}

// GetAssignmentStatisticsHandler retrieves statistics for an assignment (average grade, submission rate)
func GetAssignmentStatisticsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can view assignment statistics"})
		return
	}

	userIDInt, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	assignmentID, err := strconv.Atoi(c.Param("assignment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assignment ID"})
		return
	}

	// Check if the assignment exists
	db := c.MustGet("db").(*sql.DB)
	var assignmentExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM assignment 
			WHERE assignment_id = ? AND archive_delete_flag = TRUE
		)`, assignmentID).Scan(&assignmentExists)
	if err != nil {
		log.Printf("Error checking assignment existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check assignment: " + err.Error()})
		return
	}
	if !assignmentExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found"})
		return
	}

	// Check if the teacher is authorized to view statistics for this assignment
	var teacherID int
	err = db.QueryRow(`
		SELECT t.teacher_id
		FROM assignment a
		JOIN classroom c ON a.course_id = c.course_id
		JOIN teacher t ON c.teacher_id = t.teacher_id
		WHERE a.assignment_id = ? AND a.archive_delete_flag = TRUE
		AND c.archive_delete_flag = TRUE AND t.archive_delete_flag = TRUE
		AND t.user_id = ?`, assignmentID, userIDInt).Scan(&teacherID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to view statistics for this assignment"})
		return
	} else if err != nil {
		log.Printf("Error checking teacher authorization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check authorization: " + err.Error()})
		return
	}

	// Get the course ID for the assignment
	var courseID int
	err = db.QueryRow(`
		SELECT course_id FROM assignment 
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`, assignmentID).Scan(&courseID)
	if err != nil {
		log.Printf("Error querying course ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch course: " + err.Error()})
		return
	}

	// Get the total number of enrolled students
	var totalStudents int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM enrollment 
		WHERE course_id = ? AND archive_delete_flag = TRUE`, courseID).Scan(&totalStudents)
	if err != nil {
		log.Printf("Error counting enrolled students: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count students: " + err.Error()})
		return
	}

	// Get the number of submissions
	var submissionCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM submission 
		WHERE assignment_id = ? AND archive_delete_flag = TRUE`, assignmentID).Scan(&submissionCount)
	if err != nil {
		log.Printf("Error counting submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count submissions: " + err.Error()})
		return
	}

	// Calculate submission rate
	var submissionRate float64
	if totalStudents > 0 {
		submissionRate = (float64(submissionCount) / float64(totalStudents)) * 100
	} else {
		submissionRate = 0
	}

	// Get the average grade (only for submissions that are graded, i.e., have a non-null score)
	var averageGrade sql.NullFloat64
	err = db.QueryRow(`
		SELECT AVG(score)
		FROM submission 
		WHERE assignment_id = ? AND status = 'graded' AND score IS NOT NULL AND archive_delete_flag = TRUE`, assignmentID).Scan(&averageGrade)
	if err != nil {
		log.Printf("Error calculating average grade: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate average grade: " + err.Error()})
		return
	}

	// Handle the case where there are no graded submissions
	avgGrade := 0.0
	if averageGrade.Valid {
		avgGrade = averageGrade.Float64
	}

	c.JSON(http.StatusOK, gin.H{
		"assignment_id":    assignmentID,
		"average_grade":    avgGrade,
		"submission_rate":  submissionRate,
		"total_students":   totalStudents,
		"submission_count": submissionCount,
	})
}

// GetSubmissionHandler retrieves a specific submission by ID for a student
func GetSubmissionHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can view their submissions"})
		return
	}

	userIDInt, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	submissionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	// Fetch the student_id for the user
	var studentID int
	err = db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userIDInt).Scan(&studentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch student: " + err.Error()})
		return
	}

	// Fetch the submission
	var submission models.Submission
	var score sql.NullInt64
	var feedback sql.NullString
	err = db.QueryRow(`
		SELECT submission_id, assignment_id, student_id, content, submitted_at, score, feedback, status
		FROM submission 
		WHERE submission_id = ? AND student_id = ? AND archive_delete_flag = TRUE`,
		submissionID, studentID).Scan(
		&submission.SubmissionID,
		&submission.AssignmentID,
		&submission.StudentID,
		&submission.Content,
		&submission.SubmittedAt,
		&score,
		&feedback,
		&submission.Status,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Submission not found or unauthorized"})
		return
	} else if err != nil {
		log.Printf("Error querying submission: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch submission: " + err.Error()})
		return
	}

	// Handle nullable fields
	if score.Valid {
		scoreValue := int(score.Int64)
		submission.Score = &scoreValue
	}
	if feedback.Valid {
		feedbackValue := feedback.String
		submission.Feedback = &feedbackValue
	}

	c.JSON(http.StatusOK, submission)
}

// GetStudentSubmissionsHandler retrieves all submissions for a student
func GetStudentSubmissionsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only students can view their submissions"})
		return
	}

	userIDInt, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	db := c.MustGet("db").(*sql.DB)

	// Fetch the student_id for the user
	var studentID int
	err := db.QueryRow(`
		SELECT student_id FROM student 
		WHERE user_id = ? AND archive_delete_flag = TRUE`, userIDInt).Scan(&studentID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch student: " + err.Error()})
		return
	}

	// Fetch all submissions for the student
	rows, err := db.Query(`
		SELECT submission_id, assignment_id, student_id, content, submitted_at, score, feedback, status
		FROM submission 
		WHERE student_id = ? AND archive_delete_flag = TRUE`, studentID)
	if err != nil {
		log.Printf("Error querying submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch submissions: " + err.Error()})
		return
	}
	defer rows.Close()

	var submissions []models.Submission
	for rows.Next() {
		var s models.Submission
		var score sql.NullInt64
		var feedback sql.NullString
		if err := rows.Scan(&s.SubmissionID, &s.AssignmentID, &s.StudentID, &s.Content, &s.SubmittedAt, &score, &feedback, &s.Status); err != nil {
			log.Printf("Error scanning submission: %v", err)
			continue
		}
		if score.Valid {
			scoreValue := int(score.Int64)
			s.Score = &scoreValue
		}
		if feedback.Valid {
			feedbackValue := feedback.String
			s.Feedback = &feedbackValue
		}
		submissions = append(submissions, s)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating submissions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to iterate submissions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, submissions)
}