package database

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes and returns a database connection
func InitDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	// Load schema
	if err = loadSchema(db, "internal/database/schema.sql"); err != nil {
		return nil, err
	}

	return db, nil
}

// loadSchema reads and executes the SQL schema from the given path
func loadSchema(db *sql.DB, schemaPath string) error {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(content))
	if err != nil {
		return err
	}

	return nil
}
