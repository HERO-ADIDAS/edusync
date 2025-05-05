package routes

import (
	"database/sql"

	"github.com/gin-gonic/gin"

	"edusync/auth"
	"edusync/handlers"
)

// SetupRoutes configures the API routes
func SetupRoutes(r *gin.Engine) {
	r.Use(func(c *gin.Context) {
		c.Set("db", c.MustGet("db").(*sql.DB))
		c.Next()
	})

	// Public routes
	r.POST("/api/register", handlers.RegisterHandler)
	r.POST("/api/login", auth.LoginHandler)

	// Protected routes
	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())

	// User routes
	protected.GET("/profile", handlers.GetProfileHandler)
	protected.GET("/auth/check", handlers.CheckAuthHandler)

	// Teacher routes
	protected.POST("/classrooms", handlers.CreateClassroomHandler)
	protected.PUT("/classrooms/:id", handlers.UpdateClassroomHandler)
	protected.DELETE("/classrooms/:id", handlers.DeleteClassroomHandler)
	protected.GET("/teacher/classrooms", handlers.GetTeacherClassroomsHandler)
	protected.GET("/classrooms/:id", handlers.GetClassroomDetailsHandler)
	protected.POST("/announcements", handlers.CreateAnnouncementHandler)
	protected.PUT("/announcements/:id", handlers.UpdateAnnouncementHandler)
	protected.DELETE("/announcements/:id", handlers.DeleteAnnouncementHandler)
	protected.GET("/classrooms/:id/announcements", handlers.GetAnnouncementsByClassroomHandler)
	protected.POST("/assignments", handlers.CreateAssignmentHandler)
	protected.PUT("/assignments/:id", handlers.UpdateAssignmentHandler)
	protected.DELETE("/assignments/:id", handlers.DeleteAssignmentHandler)
	protected.GET("/classrooms/:id/assignments", handlers.GetAssignmentsByClassroomHandler)
	protected.POST("/materials", handlers.CreateMaterialHandler)
	protected.PUT("/materials/:id", handlers.UpdateMaterialHandler)
	protected.DELETE("/materials/:id", handlers.DeleteMaterialHandler)
	protected.GET("/classrooms/:id/materials", handlers.GetMaterialsByClassroomHandler)
	protected.POST("/submissions/:id/grade", handlers.GradeSubmissionHandler)
	protected.GET("/assignments/:assignment_id/submissions", handlers.GetSubmissionsByAssignmentHandler)
	protected.PUT("/teacher/profile", handlers.UpdateTeacherHandler)
	protected.GET("/teacher/profile", handlers.GetTeacherProfileHandler)
	protected.GET("/teacher/dashboard", handlers.GetTeacherDashboardHandler)
	protected.GET("/classrooms/:id/students", handlers.GetEnrolledStudentsHandler)
	protected.DELETE("/classrooms/:id/students/:student_id", handlers.RemoveStudentFromClassroomHandler)
	protected.GET("/classrooms/:id/students/:student_id", handlers.GetStudentProfileHandler)
	protected.GET("/assignments/:assignment_id/stats", handlers.GetAssignmentStatsHandler)
	protected.GET("/teacher/assignments/upcoming", handlers.GetUpcomingAssignmentsHandler)
	protected.GET("/assignments/:assignment_id/statistics", handlers.GetAssignmentStatisticsHandler)

	// Student routes
	protected.POST("/submissions", handlers.CreateSubmissionHandler)
	protected.PUT("/submissions/:id", handlers.UpdateSubmissionHandler)
	protected.POST("/enroll", handlers.EnrollStudentHandler)
	protected.GET("/student/enrollments", handlers.GetStudentEnrollmentsHandler)
	protected.PUT("/student/profile", handlers.UpdateStudentProfileHandler)
	protected.GET("/student/dashboard", handlers.GetStudentDashboardHandler)
}