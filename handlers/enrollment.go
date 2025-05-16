package handlers

import (
    "database/sql"
    "log"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/gin-gonic/gin"

    "edusync/models"
)

// EnrollStudentHandler enrolls a student in a classroom
func EnrollStudentHandler(c *gin.Context) {
    userID, _ := c.Get("userID")
    role, _ := c.Get("role")
    if role != "student" {
        c.JSON(http.StatusForbidden, gin.H{"error": "Only students can enroll in classrooms"})
        return
    }

    // Define a custom struct for the request body since models.Enrollment might not include teacher_name
    type EnrollmentRequest struct {
        CourseID    int    `json:"course_id"`
        TeacherName string `json:"teacher_name"`
    }
    var req EnrollmentRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
        return
    }

    // Validate required fields
    if req.CourseID <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Valid course ID is required"})
        return
    }
    if req.TeacherName == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Teacher name is required"})
        return
    }

    db := c.MustGet("db").(*sql.DB)
    var studentID int
    err := db.QueryRow(`
        SELECT student_id FROM student 
        WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
    if err != nil {
        log.Printf("Error querying student: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
        return
    }

    // Check if the course exists and fetch its details
    var courseTitle, actualTeacherName sql.NullString
    err = db.QueryRow(`
        SELECT c.title, u.name
        FROM classroom c
        LEFT JOIN teacher t ON c.teacher_id = t.teacher_id
        LEFT JOIN user u ON t.user_id = u.user_id
        WHERE c.course_id = ? AND c.archive_delete_flag = TRUE`, req.CourseID).Scan(&courseTitle, &actualTeacherName)
    if err == sql.ErrNoRows {
        c.JSON(http.StatusNotFound, gin.H{"error": "No classroom exists"})
        return
    } else if err != nil {
        log.Printf("Error querying classroom: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // Compare the provided teacher name with the actual teacher name (case-insensitive)
    providedTeacherName := strings.TrimSpace(req.TeacherName)
    dbTeacherName := strings.TrimSpace(actualTeacherName.String)
    if !strings.EqualFold(providedTeacherName, dbTeacherName) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Teacher does not match the course"})
        return
    }

    // Check if the student is already enrolled
    var exists bool
    err = db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM enrollment 
            WHERE student_id = ? AND course_id = ? AND archive_delete_flag = TRUE
        )`, studentID, req.CourseID).Scan(&exists)
    if err != nil {
        log.Printf("Error checking enrollment: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    if exists {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Student already enrolled in this classroom"})
        return
    }

    // Enroll the student
    result, err := db.Exec(`
        INSERT INTO enrollment (student_id, course_id, enrollment_date, status, archive_delete_flag)
        VALUES (?, ?, ?, 'active', TRUE)`,
        studentID, req.CourseID, time.Now())
    if err != nil {
        log.Printf("Error inserting enrollment: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    enrollmentID, _ := result.LastInsertId()
    c.JSON(http.StatusOK, gin.H{
        "enrollment_id": enrollmentID,
        "course_id":     req.CourseID,
        "course_title":  courseTitle.String,
        "teacher_name":  actualTeacherName.String,
        "status":        "active",
        "message":       "Successfully enrolled in the course",
    })
}

// GetStudentEnrollmentsHandler lists all enrollments for a student
func GetStudentEnrollmentsHandler(c *gin.Context) {
    userID, _ := c.Get("userID")
    role, _ := c.Get("role")
    if role != "student" {
        c.JSON(http.StatusForbidden, gin.H{"error": "Only students can view their enrollments"})
        return
    }

    db := c.MustGet("db").(*sql.DB)
    var studentID int
    err := db.QueryRow(`
        SELECT student_id FROM student 
        WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
    if err != nil {
        log.Printf("Error querying student: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
        return
    }

    // Fetch enrollments with course title and teacher name
    rows, err := db.Query(`
        SELECT e.enrollment_id, e.student_id, e.course_id, e.enrollment_date, e.status, c.title, u.name
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

    var enrollments []map[string]interface{}
    for rows.Next() {
        var e models.Enrollment
        var title, teacherName sql.NullString
        if err := rows.Scan(&e.EnrollmentID, &e.StudentID, &e.CourseID, &e.EnrollmentDate, &e.Status, &title, &teacherName); err != nil {
            log.Printf("Error scanning enrollment: %v", err)
            continue
        }
        enrollments = append(enrollments, map[string]interface{}{
            "enrollment_id":   e.EnrollmentID,
            "student_id":      e.StudentID,
            "course_id":       e.CourseID,
            "enrollment_date": e.EnrollmentDate,
            "status":          e.Status,
            "title":           title.String,
            "teacher_name":    teacherName.String,
        })
    }

    c.JSON(http.StatusOK, enrollments)
}

// UnenrollStudentHandler allows a student to unenroll from a course
func UnenrollStudentHandler(c *gin.Context) {
    userID, _ := c.Get("userID")
    role, _ := c.Get("role")
    if role != "student" {
        c.JSON(http.StatusForbidden, gin.H{"error": "Only students can unenroll from classrooms"})
        return
    }

    enrollmentID, err := strconv.Atoi(c.Param("enrollment_id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid enrollment ID"})
        return
    }

    db := c.MustGet("db").(*sql.DB)
    var studentID int
    err = db.QueryRow(`
        SELECT student_id FROM student 
        WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
    if err != nil {
        log.Printf("Error querying student: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
        return
    }

    // Check if the enrollment exists and belongs to the student
    var exists bool
    err = db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 FROM enrollment 
            WHERE enrollment_id = ? AND student_id = ? AND archive_delete_flag = TRUE
        )`, enrollmentID, studentID).Scan(&exists)
    if err != nil {
        log.Printf("Error checking enrollment: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    if !exists {
        c.JSON(http.StatusNotFound, gin.H{"error": "Enrollment not found or unauthorized"})
        return
    }

    // Soft delete the enrollment
    _, err = db.Exec(`
        UPDATE enrollment 
        SET archive_delete_flag = FALSE 
        WHERE enrollment_id = ? AND student_id = ? AND archive_delete_flag = TRUE`,
        enrollmentID, studentID)
    if err != nil {
        log.Printf("Error deleting enrollment: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Successfully unenrolled from the course"})
}

// GetUserStatsHandler retrieves total students and assignments for a user (teacher or student)
func GetUserStatsHandler(c *gin.Context) {
    userID, _ := c.Get("userID")
    role, _ := c.Get("role")

    db := c.MustGet("db").(*sql.DB)
    var totalStudents, totalAssignments int
    var err error

    if role == "teacher" {
        // Fetch teacher ID
        var teacherID int
        err = db.QueryRow(`
            SELECT teacher_id FROM teacher 
            WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&teacherID)
        if err != nil {
            log.Printf("Error querying teacher for user_id %v: %v", userID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Teacher not found"})
            return
        }

        // Count total enrolled students across all teacher's classrooms
        err = db.QueryRow(`
            SELECT COALESCE(COUNT(DISTINCT e.student_id), 0)
            FROM enrollment e
            JOIN classroom c ON e.course_id = c.course_id
            WHERE c.teacher_id = ? AND e.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE`, teacherID).Scan(&totalStudents)
        if err != nil {
            log.Printf("Error counting total students for teacher_id %d: %v", teacherID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
            return
        }

        // Count total assignments across all teacher's classrooms
        err = db.QueryRow(`
            SELECT COALESCE(COUNT(*), 0)
            FROM assignment a
            JOIN classroom c ON a.course_id = c.course_id
            WHERE c.teacher_id = ? AND a.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE`, teacherID).Scan(&totalAssignments)
        if err != nil {
            log.Printf("Error counting total assignments for teacher_id %d: %v", teacherID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
            return
        }
    } else if role == "student" {
        // Fetch student ID
        var studentID int
        err = db.QueryRow(`
            SELECT student_id FROM student 
            WHERE user_id = ? AND archive_delete_flag = TRUE`, userID).Scan(&studentID)
        if err != nil {
            log.Printf("Error querying student for user_id %v: %v", userID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Student not found"})
            return
        }

        // Count total students enrolled across all the student's classrooms
        err = db.QueryRow(`
            SELECT COALESCE(COUNT(DISTINCT e2.student_id), 0)
            FROM enrollment e
            JOIN enrollment e2 ON e.course_id = e2.course_id
            JOIN classroom c ON e.course_id = c.course_id
            WHERE e.student_id = ? AND e.archive_delete_flag = TRUE AND e2.archive_delete_flag = TRUE AND c.archive_delete_flag = TRUE`, studentID).Scan(&totalStudents)
        if err != nil {
            log.Printf("Error counting total students for student_id %d: %v", studentID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
            return
        }

        // Count total assignments across all the student's classrooms
        err = db.QueryRow(`
            SELECT COALESCE(COUNT(*), 0)
            FROM assignment a
            JOIN enrollment e ON a.course_id = e.course_id
            WHERE e.student_id = ? AND a.archive_delete_flag = TRUE AND e.archive_delete_flag = TRUE`, studentID).Scan(&totalAssignments)
        if err != nil {
            log.Printf("Error counting total assignments for student_id %d: %v", studentID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
            return
        }
    } else {
        c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized role"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "total_students":    totalStudents,
        "total_assignments": totalAssignments,
    })
}