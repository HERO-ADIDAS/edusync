package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Global database connections
var (
	RootDB    *sql.DB // For registration and admin operations
	StudentDB *sql.DB // For student role
	TeacherDB *sql.DB // For teacher role
)

// DatabaseConfig holds connection information
type DatabaseConfig struct {
	User     string
	Password string
	Host     string
	DBName   string
}

// InitDatabaseConnections initializes all required database connections
func InitDatabaseConnections() error {
	// Root connection for registration
	rootConfig := DatabaseConfig{
		User:     "root",
		Password: os.Getenv("DB_ROOT_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		DBName:   os.Getenv("DB_NAME"),
	}

	// Student connection
	studentConfig := DatabaseConfig{
		User:     "student",
		Password: os.Getenv("DB_STUDENT_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		DBName:   os.Getenv("DB_NAME"),
	}

	// Teacher connection
	teacherConfig := DatabaseConfig{
		User:     "TEACHER",
		Password: os.Getenv("DB_TEACHER_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		DBName:   os.Getenv("DB_NAME"),
	}

	// Initialize connections
	var err error
	RootDB, err = createDBConnection(rootConfig)
	if err != nil {
		return fmt.Errorf("failed to connect as root: %v", err)
	}
	log.Println("Root database connection established")

	StudentDB, err = createDBConnection(studentConfig)
	if err != nil {
		log.Printf("Warning: failed to connect as student: %v", err)
		// Don't return error as this might be a new setup without student user yet
	} else {
		log.Println("Student database connection established")
	}

	TeacherDB, err = createDBConnection(teacherConfig)
	if err != nil {
		log.Printf("Warning: failed to connect as teacher: %v", err)
		// Don't return error as this might be a new setup without teacher user yet
	} else {
		log.Println("Teacher database connection established")
	}

	return nil
}

// createDBConnection creates a database connection with the given config
func createDBConnection(config DatabaseConfig) (*sql.DB, error) {
	dbConfig := mysql.Config{
		User:                 config.User,
		Passwd:               config.Password,
		Net:                  "tcp",
		Addr:                 config.Host,
		DBName:               config.DBName,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	db, err := sql.Open("mysql", dbConfig.FormatDSN())
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 3)

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// GetDBForRole returns the appropriate database connection for the given role
// GetDBForRole returns the appropriate database connection for the given role
func GetDBForRole(role string) *sql.DB {
    // Before returning any connection, ensure it's still alive by pinging
    switch role {
    case "student":
        if StudentDB != nil {
            if err := StudentDB.Ping(); err == nil {
                return StudentDB
            }
            log.Println("Student DB connection lost, falling back to root")
        }
        // Fallback to root if student connection isn't available
        return RootDB
    case "teacher":
        if TeacherDB != nil {
            if err := TeacherDB.Ping(); err == nil {
                return TeacherDB
            }
            log.Println("Teacher DB connection lost, falling back to root")
        }
        // Fallback to root if teacher connection isn't available
        return RootDB
    default:
        // Admin and dev roles use the root connection
        return RootDB
    }
}

// CloseConnections closes all database connections
func CloseConnections() {
	if RootDB != nil {
		RootDB.Close()
	}
	if StudentDB != nil {
		StudentDB.Close()
	}
	if TeacherDB != nil {
		TeacherDB.Close()
	}
}
