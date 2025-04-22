package models

import (
	"github.com/golang-jwt/jwt/v4"
	"time"
)

// User represents the user data structure
type User struct {
	ID          int       `json:"id,omitempty" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Email       string    `json:"email" db:"email"`
	Password    string    `json:"password,omitempty" db:"password"`
	Role        string    `json:"role" db:"role"`
	ContactNum  string    `json:"contact_number,omitempty" db:"contact_number"`
	ProfilePic  string    `json:"profile_picture,omitempty" db:"profile_picture"`
	Org         string    `json:"org,omitempty" db:"org"`
	CreatedAt   time.Time `json:"created_at,omitempty" db:"created_at"`
}

// Student specific data
type Student struct {
	StudentID      int    `json:"student_id,omitempty" db:"student_id"`
	UserID         int    `json:"user_id,omitempty" db:"user_id"`
	GradeLevel     string `json:"grade_level" db:"grade_level"`
	EnrollmentYear int    `json:"enrollment_year" db:"enrollment_year"`
}

// Teacher specific data
type Teacher struct {
	TeacherID int    `json:"teacher_id,omitempty" db:"teacher_id"`
	UserID    int    `json:"user_id,omitempty" db:"user_id"`
	Dept      string `json:"dept" db:"dept"`
	Name      string `json:"name,omitempty" db:"name"` // Added to match teacher.go queries
}

// Classroom represents a classroom entity
type Classroom struct {
	CourseID     int       `json:"course_id" db:"course_id"`
	TeacherID    int       `json:"teacher_id" db:"teacher_id"`
	Title        string    `json:"title" db:"title"`
	Description  string    `json:"description" db:"description"`
	StartDate    time.Time `json:"start_date" db:"start_date"`
	EndDate      time.Time `json:"end_date" db:"end_date"`
	SubjectArea  string    `json:"subject_area" db:"subject_area"`
}

// Enrollment represents an enrollment record
type Enrollment struct {
	EnrollmentID   int       `json:"enrollment_id,omitempty" db:"enrollment_id"`
	StudentID      int       `json:"student_id,omitempty" db:"student_id"`
	CourseID       int       `json:"course_id,omitempty" db:"course_id"`
	EnrollmentDate time.Time `json:"enrollment_date,omitempty" db:"enrollment_date"`
	Status         string    `json:"status" db:"status"`
}

// Assignment represents an assignment entity
type Assignment struct {
	AssignmentID int       `json:"assignment_id" db:"assignment_id"`
	CourseID     int       `json:"course_id" db:"course_id"`
	Title        string    `json:"title" db:"title"`
	Description  string    `json:"description" db:"description"`
	DueDate      time.Time `json:"due_date" db:"due_date"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	MaxPoints    int       `json:"max_points" db:"max_points"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	AllowLate    bool      `json:"allow_late" db:"allow_late"`
	Materials    []int     `json:"materials,omitempty" db:"materials"` // Assumed join table reference
}

// Submission represents a submission entity
type Submission struct {
	SubmissionID int       `json:"submission_id" db:"submission_id"`
	AssignmentID int       `json:"assignment_id" db:"assignment_id"`
	StudentID    int       `json:"student_id" db:"student_id"`
	Link         string    `json:"link" db:"content"` // Changed from content to link, maps to content column
	SubmittedAt  time.Time `json:"submitted_at" db:"submitted_at"`
	Score        int       `json:"score" db:"score"`
	Feedback     string    `json:"feedback" db:"feedback"`
	Status       string    `json:"status" db:"status"`
	IsLate       bool      `json:"is_late" db:"is_late"`
	GradedAt     *time.Time `json:"graded_at,omitempty" db:"graded_at"` // Optional graded timestamp
}

// RegisterRequest combines user data with role-specific data
type RegisterRequest struct {
	User    User     `json:"user"`
	Student *Student `json:"student,omitempty"`
	Teacher *Teacher `json:"teacher,omitempty"`
}

// LoginRequest contains login credentials
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse contains JWT token and user info
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Claims for JWT token
type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}