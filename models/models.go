package models

import "time"

// LoginRequest for authentication
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest for user registration
type RegisterRequest struct {
	Name           string  `json:"name" binding:"required"`
	Email          string  `json:"email" binding:"required,email"`
	Password       string  `json:"password" binding:"required"`
	Role           string  `json:"role" binding:"required,oneof=teacher student"`
	ContactNumber  *string `json:"contact_number"`
	ProfilePicture *string `json:"profile_picture"`
	Org            *string `json:"org"`
	Dept           *string `json:"dept"`           // For teacher
	GradeLevel     *string `json:"grade_level"`    // For student
	EnrollmentYear *int    `json:"enrollment_year"` // For student
}

// User model
type User struct {
	UserID         int       `json:"user_id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Password       string    `json:"password"`
	CreatedAt      time.Time `json:"created_at"`
	ContactNumber  *string   `json:"contact_number"`
	ProfilePicture *string   `json:"profile_picture"`
	Role           string    `json:"role"`
	Org            *string   `json:"org"`
}

// Teacher model
type Teacher struct {
	TeacherID int     `json:"teacher_id"`
	UserID    int     `json:"user_id"`
	Dept      *string `json:"dept"`
}

// Student model
type Student struct {
	StudentID      int     `json:"student_id"`
	UserID         int     `json:"user_id"`
	GradeLevel     *string `json:"grade_level"`
	EnrollmentYear *int    `json:"enrollment_year"`
}

// Classroom model
type Classroom struct {
	CourseID     int       `json:"course_id"`
	TeacherID    int       `json:"teacher_id"`
	Title        string    `json:"title"`
	Description  *string   `json:"description"`
	StartDate    *time.Time `json:"start_date"`
	EndDate      *time.Time `json:"end_date"`
	SubjectArea  *string   `json:"subject_area"`
}

// Enrollment model
type Enrollment struct {
	EnrollmentID   int       `json:"enrollment_id"`
	StudentID      int       `json:"student_id"`
	CourseID       int       `json:"course_id"`
	EnrollmentDate time.Time `json:"enrollment_date"`
	Status         string    `json:"status"`
}

// UpdateStudentProfileRequest for profile updates
type UpdateStudentProfileRequest struct {
	GradeLevel     *string `json:"grade_level"`
	EnrollmentYear *int    `json:"enrollment_year"`
}

// Material model
type Material struct {
	MaterialID  int       `json:"material_id"`
	CourseID    int       `json:"course_id"`
	Title       string    `json:"title"`
	Type        *string   `json:"type"`
	FilePath    *string   `json:"file_path"`
	UploadedAt  time.Time `json:"uploaded_at"`
	Description *string   `json:"description"`
}

// Announcement model
type Announcement struct {
	AnnouncementID int       `json:"announcement_id"`
	CourseID       int       `json:"course_id"`
	Title          string    `json:"title"`
	Content        *string   `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
	IsPinned       bool      `json:"is_pinned"`
}

// Assignment model
type Assignment struct {
	AssignmentID int       `json:"assignment_id"`
	CourseID     int       `json:"course_id"`
	Title        string    `json:"title"`
	Description  *string   `json:"description"`
	DueDate      time.Time `json:"due_date"`
	MaxPoints    int       `json:"max_points"`
	CreatedAt    time.Time `json:"created_at"`
}

// Submission model
type Submission struct {
	SubmissionID int       `json:"submission_id"`
	AssignmentID int       `json:"assignment_id"`
	StudentID    int       `json:"student_id"`
	Content      *string   `json:"content"`
	SubmittedAt  time.Time `json:"submitted_at"`
	Score        *int      `json:"score"`
	Feedback     *string   `json:"feedback"`
	Status       string    `json:"status"`
}