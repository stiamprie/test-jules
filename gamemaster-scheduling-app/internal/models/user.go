package models

import "time"

// User represents a user in the system.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
