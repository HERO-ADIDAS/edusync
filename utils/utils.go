package utils

import (
	"errors"
	"os"
	"unicode"
)

// SetDefaultEnvVars sets default environment variables if not already set
func SetDefaultEnvVars() {
	// Set default values if environment variables are not set
	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost:3306")
	}
	if os.Getenv("DB_NAME") == "" {
		os.Setenv("DB_NAME", "edusync_db")
	}
	if os.Getenv("DB_ROOT_PASSWORD") == "" {
		os.Setenv("DB_ROOT_PASSWORD", "adidas") // Default for development
	}
	if os.Getenv("DB_STUDENT_PASSWORD") == "" {
		os.Setenv("DB_STUDENT_PASSWORD", "student") // Default for development
	}
	if os.Getenv("DB_TEACHER_PASSWORD") == "" {
		os.Setenv("DB_TEACHER_PASSWORD", "teacher") // Default for development
	}
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "3f8a3d6ea42995bcb8003a4d85a62c93cfae6f3bc8fbdf71817a0bda7c054cb3")
	}
}

// ValidatePassword checks if a password meets security requirements:
// - At least 8 characters long
// - Contains at least one capital letter
// - Contains alphanumeric characters (both letters and numbers)
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	var hasUpper, hasLetter, hasDigit bool
	for _, char := range password {
		if unicode.IsUpper(char) {
			hasUpper = true
		}
		if unicode.IsLetter(char) {
			hasLetter = true
		}
		if unicode.IsDigit(char) {
			hasDigit = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}

	if !hasLetter || !hasDigit {
		return errors.New("password must contain both letters and numbers")
	}

	return nil
}
