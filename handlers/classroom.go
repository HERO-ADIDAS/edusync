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

// ClassroomRequest is a temporary struct to handle incoming JSON with string dates
type ClassroomRequest struct {
	Title       string  `json:"title" binding:"required"`
	Description *string `json:"description"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
	SubjectArea *string `json:"subject_area"`
}

// parseDate converts a date string (YYYY-MM-DD) to time.Time
func parseDate(dateStr *string) (*time.Time, error) {
	if dateStr == nil {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", *dateStr)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// CreateClassroomHandler creates a new classroom
func CreateClassroomHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can create classrooms"})
		return
	}

	var req ClassroomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Parse start_date and end_date
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format, expected YYYY-MM-DD"})
		return
	}
	endDate, err := parseDate(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format, expected YYYY-MM-DD"})
		return
	}

	// Map to models.Classroom
	classroom := models.Classroom{
		Title:       req.Title,
		Description: req.Description,
		StartDate:   startDate,
		EndDate:     endDate,
		SubjectArea: req.SubjectArea,
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

	// Log the current time for debugging
	log.Printf("Creating classroom at %v", time.Now())

	result, err := db.Exec(`
		INSERT INTO classroom (teacher_id, title, description, start_date, end_date, subject_area, archive_delete_flag)
		VALUES (?, ?, ?, ?, ?, ?, TRUE)`,
		teacherID, classroom.Title, classroom.Description, classroom.StartDate, classroom.EndDate, classroom.SubjectArea)
	if err != nil {
		log.Printf("Error inserting classroom: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	courseID, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{
		"course_id": courseID,
		"title":     classroom.Title,
	})
}

// UpdateClassroomHandler updates a classroom
func UpdateClassroomHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can update classrooms"})
		return
	}

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	var req ClassroomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Parse start_date and end_date
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format, expected YYYY-MM-DD"})
		return
	}
	endDate, err := parseDate(req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format, expected YYYY-MM-DD"})
		return
	}

	// Map to models.Classroom
	classroom := models.Classroom{
		Title:       req.Title,
		Description: req.Description,
		StartDate:   startDate,
		EndDate:     endDate,
		SubjectArea: req.SubjectArea,
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

	// Check if the teacher is authorized to update this classroom
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to update this classroom"})
		return
	}

	_, err = db.Exec(`
		UPDATE classroom 
		SET title = ?, description = ?, start_date = ?, end_date = ?, subject_area = ?
		WHERE course_id = ? AND teacher_id = ? AND archive_delete_flag = TRUE`,
		classroom.Title, classroom.Description, classroom.StartDate, classroom.EndDate, classroom.SubjectArea, courseID, teacherID)
	if err != nil {
		log.Printf("Error updating classroom: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"course_id": courseID,
		"title":     classroom.Title,
	})
}

// DeleteClassroomHandler deletes a classroom
func DeleteClassroomHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can delete classrooms"})
		return
	}

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
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

	// Check if the teacher is authorized to delete this classroom
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to delete this classroom"})
		return
	}

	_, err = db.Exec(`
		UPDATE classroom 
		SET archive_delete_flag = FALSE 
		WHERE course_id = ? AND teacher_id = ? AND archive_delete_flag = TRUE`,
		courseID, teacherID)
	if err != nil {
		log.Printf("Error deleting classroom: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Classroom deleted"})
}

// GetTeacherClassroomsHandler lists all classrooms for a teacher
func GetTeacherClassroomsHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	if role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can view their classrooms"})
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
		SELECT course_id, teacher_id, title, description, start_date, end_date, subject_area
		FROM classroom 
		WHERE teacher_id = ? AND archive_delete_flag = TRUE`, teacherID)
	if err != nil {
		log.Printf("Error querying classrooms: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var classrooms []models.Classroom
	for rows.Next() {
		var c models.Classroom
		if err := rows.Scan(&c.CourseID, &c.TeacherID, &c.Title, &c.Description, &c.StartDate, &c.EndDate, &c.SubjectArea); err != nil {
			log.Printf("Error scanning classroom: %v", err)
			continue
		}
		classrooms = append(classrooms, c)
	}

	c.JSON(http.StatusOK, classrooms)
}

// GetClassroomDetailsHandler retrieves details of a specific classroom
func GetClassroomDetailsHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("Invalid course ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}

	log.Printf("Fetching classroom details for course_id: %d, user_id: %v, role: %s", courseID, userID, role)

	db := c.MustGet("db").(*sql.DB)
	var classroom models.Classroom

	// Check if the classroom exists
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM classroom 
			WHERE course_id = ? AND archive_delete_flag = TRUE
		)`, courseID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking if classroom exists: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !exists {
		log.Printf("Classroom with course_id %d does not exist", courseID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Classroom not found"})
		return
	}

	if role == "teacher" {
		var teacherID int
		err = db.QueryRow(`
			SELECT teacher_id FROM teacher 
			WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
		if err != nil {
			log.Printf("Error querying teacher for user_id %v: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Teacher not found"})
			return
		}

		// Check if the teacher is authorized to view this classroom
		var teacherAuthorized bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM classroom 
				WHERE course_id = ? AND teacher_id = ? AND archive_delete_flag = TRUE
			)`, courseID, teacherID).Scan(&teacherAuthorized)
		if err != nil {
			log.Printf("Error checking teacher authorization for course_id %d, teacher_id %d: %v", courseID, teacherID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !teacherAuthorized {
			log.Printf("Teacher with teacher_id %d is not authorized to view classroom with course_id %d", teacherID, courseID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to view this classroom"})
			return
		}

		err = db.QueryRow(`
			SELECT course_id, teacher_id, title, description, start_date, end_date, subject_area
			FROM classroom 
			WHERE course_id = ? AND teacher_id = ? AND archive_delete_flag = TRUE`,
			courseID, teacherID).Scan(
			&classroom.CourseID, &classroom.TeacherID, &classroom.Title, &classroom.Description,
			&classroom.StartDate, &classroom.EndDate, &classroom.SubjectArea)
		if err != nil {
			log.Printf("Error querying classroom for course_id %d, teacher_id %d: %v", courseID, teacherID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	} else if role == "student" {
		var studentID int
		err = db.QueryRow(`
			SELECT student_id FROM student 
			WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
		if err != nil {
			log.Printf("Error querying student for user_id %v: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
			return
		}

		// Check if the student is enrolled in this classroom
		var studentEnrolled bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM enrollment 
				WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
			)`, studentID, courseID).Scan(&studentEnrolled)
		if err != nil {
			log.Printf("Error checking enrollment for student_id %d, course_id %d: %v", studentID, courseID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !studentEnrolled {
			log.Printf("Student with student_id %d is not enrolled in classroom with course_id %d", studentID, courseID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Not enrolled in this classroom"})
			return
		}

		err = db.QueryRow(`
			SELECT course_id, teacher_id, title, description, start_date, end_date, subject_area
			FROM classroom 
			WHERE course_id = ? AND archive_delete_flag = TRUE`, courseID).Scan(
			&classroom.CourseID, &classroom.TeacherID, &classroom.Title, &classroom.Description,
			&classroom.StartDate, &classroom.EndDate, &classroom.SubjectArea)
		if err != nil {
			log.Printf("Error querying classroom for course_id %d (student role): %v", courseID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	} else {
		log.Printf("Unauthorized role: %s", role)
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized role"})
		return
	}

	log.Printf("Successfully retrieved classroom: %+v", classroom)
	c.JSON(http.StatusOK, classroom)
}

// GetEnrolledStudentsHandler lists all students enrolled in a classroom
func GetEnrolledStudentsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can view enrolled students"})
		return
	}

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
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

	// Check if the teacher is authorized to view this classroom
	exists = false
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

	rows, err := db.Query(`
		SELECT e.enrollment_id, e.student_id, u.name, s.grade_level, s.enrollment_year
		FROM enrollment e
		JOIN student s ON e.student_id = s.student_id
		JOIN user u ON s.user_id = u.user_id
		WHERE e.course_id = ? AND e.archive_delete_flag = TRUE AND s.archive_delete_flag = TRUE AND u.archive_delete_flag = TRUE`, courseID)
	if err != nil {
		log.Printf("Error querying enrolled students: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var students []map[string]interface{}
	for rows.Next() {
		var enrollmentID, studentID int
		var name string
		var gradeLevel sql.NullString
		var enrollmentYear sql.NullInt64
		if err := rows.Scan(&enrollmentID, &studentID, &name, &gradeLevel, &enrollmentYear); err != nil {
			log.Printf("Error scanning student: %v", err)
			continue
		}
		students = append(students, map[string]interface{}{
			"enrollment_id":   enrollmentID,
			"student_id":      studentID,
			"name":            name,
			"grade_level":     gradeLevel.String,
			"enrollment_year": enrollmentYear.Int64,
		})
	}

	c.JSON(http.StatusOK, students)
}

// RemoveStudentFromClassroomHandler removes a student from a classroom
func RemoveStudentFromClassroomHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can remove students from classrooms"})
		return
	}

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}
	studentID, err := strconv.Atoi(c.Param("student_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
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

	// Check if the teacher is authorized to manage this classroom
	exists = false
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
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to manage this classroom"})
		return
	}

	// Check if the student is enrolled
	var enrollmentExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM enrollment 
			WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
		)`, studentID, courseID).Scan(&enrollmentExists)
	if err != nil {
		log.Printf("Error checking enrollment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !enrollmentExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not enrolled in this classroom"})
		return
	}

	_, err = db.Exec(`
		UPDATE enrollment 
		SET archive_delete_flag = FALSE 
		WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE`,
		studentID, courseID)
	if err != nil {
		log.Printf("Error removing student from classroom: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Student removed from classroom"})
}

// GetStudentProfileHandler retrieves the profile of a specific student in a classroom
func GetStudentProfileHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	role, exists := c.Get("role")
	if !exists || role != "teacher" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only teachers can view student profiles"})
		return
	}

	courseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid course ID"})
		return
	}
	studentID, err := strconv.Atoi(c.Param("student_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid student ID"})
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

	// Check if the teacher is authorized to view this classroom
	exists = false
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

	// Check if the student is enrolled in the classroom
	var enrollmentExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM enrollment 
			WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
		)`, studentID, courseID).Scan(&enrollmentExists)
	if err != nil {
		log.Printf("Error checking enrollment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if !enrollmentExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not enrolled in this classroom"})
		return
	}

	// Fetch student profile
	var name string
	var gradeLevel sql.NullString
	var enrollmentYear sql.NullInt64
	err = db.QueryRow(`
		SELECT u.name, s.grade_level, s.enrollment_year
		FROM student s
		JOIN user u ON s.user_id = u.user_id
		WHERE s.student_id = ? AND s.archive_delete_flag = TRUE AND u.archive_delete_flag = TRUE`,
		studentID).Scan(&name, &gradeLevel, &enrollmentYear)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
		return
	} else if err != nil {
		log.Printf("Error querying student profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"student_id":      studentID,
		"name":            name,
		"grade_level":     gradeLevel.String,
		"enrollment_year": enrollmentYear.Int64,
	})
}