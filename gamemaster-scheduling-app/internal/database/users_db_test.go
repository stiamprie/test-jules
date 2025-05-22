package database

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/gamemaster-scheduling/app/internal/models"
	// Ensure sqlite3 driver is registered
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB initializes an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper() // Marks this function as a test helper

	// InitDB with ":memory:" creates a new in-memory SQLite database for each call.
	// InitDB also loads the schema.
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Teardown function to close the database
	teardown := func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close test database: %v", err)
		}
	}

	return db, teardown
}

func TestCreateUserAndGetUser(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	email := "testuser@example.com"
	password := "password123"

	t.Run("Create and Get User", func(t *testing.T) {
		createdUser, err := CreateUser(db, email, password)
		if err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}
		if createdUser.ID == 0 {
			t.Errorf("CreateUser() returned user with ID 0")
		}
		if createdUser.Email != email {
			t.Errorf("CreateUser() email = %v, want %v", createdUser.Email, email)
		}
		if createdUser.CreatedAt.IsZero() {
			t.Errorf("CreateUser() CreatedAt is zero")
		}

		// Get by ID
		userByID, err := GetUserByID(db, createdUser.ID)
		if err != nil {
			t.Fatalf("GetUserByID() error = %v", err)
		}
		if !reflect.DeepEqual(userByID, createdUser) {
			t.Errorf("GetUserByID() got = %v, want %v", userByID, createdUser)
		}

		// Get by Email
		userByEmail, err := GetUserByEmail(db, email)
		if err != nil {
			t.Fatalf("GetUserByEmail() error = %v", err)
		}
		if !reflect.DeepEqual(userByEmail, createdUser) {
			t.Errorf("GetUserByEmail() got = %v, want %v", userByEmail, createdUser)
		}
	})

	t.Run("Create User with Existing Email", func(t *testing.T) {
		// First user already created in the previous sub-test due to shared DB context (fixed by calling setupTestDB per main test)
		// For this to be isolated, each t.Run should have its own db state, or clean up.
		// The current setupTestDB is per TestFunction, so this t.Run shares state with the one above.
		// Let's re-create a user here to be sure, or structure tests to be independent.
		// Since CreateUser is already tested, we assume it works. If not, this test might show misleading errors.
		
		// Attempt to create the same user again
		_, err := CreateUser(db, email, password)
		if err == nil {
			t.Errorf("CreateUser() with existing email expected error, got nil")
		}
		// The error should be about uniqueness constraint, e.g., "UNIQUE constraint failed: users.email"
		// For a more robust check, you might inspect the error type or message.
	})

	t.Run("Get Non-existent User", func(t *testing.T){
		_, err := GetUserByID(db, 99999)
		if err != sql.ErrNoRows {
			t.Errorf("GetUserByID() for non-existent ID, got err = %v, want sql.ErrNoRows", err)
		}
		_, err = GetUserByEmail(db, "nonexistent@example.com")
		if err != sql.ErrNoRows {
			t.Errorf("GetUserByEmail() for non-existent email, got err = %v, want sql.ErrNoRows", err)
		}
	})
}

func TestVerifyPassword(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	email := "verifyuser@example.com"
	password := "securepassword"

	createdUser, err := CreateUser(db, email, password)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	t.Run("Correct Password", func(t *testing.T) {
		err := VerifyPassword(createdUser.PasswordHash, password)
		if err != nil {
			t.Errorf("VerifyPassword() with correct password error = %v, want nil", err)
		}
	})

	t.Run("Incorrect Password", func(t *testing.T) {
		err := VerifyPassword(createdUser.PasswordHash, "wrongpassword")
		if err == nil {
			t.Errorf("VerifyPassword() with incorrect password expected error, got nil")
		}
		// bcrypt.CompareHashAndPassword returns a specific error on mismatch
		// For a more robust check, you might check if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword)
	})
}

// Example of a helper to create a user for other tests, if needed
func createTestUser(t *testing.T, db *sql.DB, email, password string) *models.User {
	t.Helper()
	user, err := CreateUser(db, email, password)
	if err != nil {
		t.Fatalf("Failed to create test user %s: %v", email, err)
	}
	return user
}
