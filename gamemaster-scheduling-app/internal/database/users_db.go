package database

import (
	"database/sql"
	"time"

	"github.com/gamemaster-scheduling/app/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser hashes the password and inserts a new user into the database.
func CreateUser(db *sql.DB, email string, password string) (*models.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	stmt, err := db.Prepare("INSERT INTO users(email, password_hash) VALUES(?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(email, string(hashedPassword))
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	// To get CreatedAt, we could query the user again, or rely on application-level timestamping if preferred.
	// For simplicity, we'll query again here or assume default value is set by DB.
	// Let's retrieve the user to get all fields populated, including DB defaults like created_at.
	return GetUserByID(db, id)
}

// GetUserByEmail retrieves a user by their email address.
func GetUserByEmail(db *sql.DB, email string) (*models.User, error) {
	user := &models.User{}
	row := db.QueryRow("SELECT id, email, password_hash, created_at FROM users WHERE email = ?", email)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err // This will include sql.ErrNoRows if not found
	}
	return user, nil
}

// GetUserByID retrieves a user by their ID.
func GetUserByID(db *sql.DB, id int64) (*models.User, error) {
	user := &models.User{}
	row := db.QueryRow("SELECT id, email, password_hash, created_at FROM users WHERE id = ?", id)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err // This will include sql.ErrNoRows if not found
	}
	return user, nil
}

// VerifyPassword compares a stored hashed password with a plaintext password.
func VerifyPassword(hashedPassword string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
