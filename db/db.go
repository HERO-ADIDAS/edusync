package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"edusync/config"
)

// DB is the global database connection
var DB *sql.DB

// InitDatabaseConnection initializes the database connection
func InitDatabaseConnection() error {
	var err error
	DB, err = sql.Open("mysql", config.ConfigInstance.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	return nil
}

// CloseConnection closes the database connection
func CloseConnection() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}