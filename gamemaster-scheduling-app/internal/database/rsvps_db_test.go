package database

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/gamemaster-scheduling/app/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB is a helper, duplicated here for brevity.
func setupTestDBForRSVPs(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	teardown := func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close test database: %v", err)
		}
	}
	return db, teardown
}

// createTestUser is a helper, duplicated here.
func createTestUserForRSVPs(t *testing.T, db *sql.DB, email, password string) *models.User {
	t.Helper()
	user, err := CreateUser(db, email, password)
	if err != nil {
		t.Fatalf("Failed to create test user %s: %v", email, err)
	}
	return user
}

// createTestGame is a helper, duplicated here.
func createTestGameForRSVPs(t *testing.T, db *sql.DB, gm *models.User, title string) *models.Game {
	t.Helper()
	gameTime := time.Now().Add(24 * time.Hour).Round(time.Second)
	game := &models.Game{
		GMID:         gm.ID,
		Title:        title,
		GameDateTime: gameTime,
		Location:     "Test Location",
	}
	createdGame, err := CreateGame(db, game)
	if err != nil {
		t.Fatalf("Failed to create test game %s: %v", title, err)
	}
	return createdGame
}

func TestCreateOrUpdateRSVPAndGet(t *testing.T) {
	db, teardown := setupTestDBForRSVPs(t)
	defer teardown()

	user1 := createTestUserForRSVPs(t, db, "rsvpuser1@example.com", "pass1")
	gm := createTestUserForRSVPs(t, db, "gmrsvp@example.com", "gmpass")
	game1 := createTestGameForRSVPs(t, db, gm, "RSVP Test Game")

	t.Run("Create and Get RSVP", func(t *testing.T) {
		rsvp := &models.RSVP{
			UserID: user1.ID,
			GameID: game1.ID,
			Status: models.RSVPStatusAttending,
		}
		err := CreateOrUpdateRSVP(db, rsvp)
		if err != nil {
			t.Fatalf("CreateOrUpdateRSVP() error = %v", err)
		}

		retrievedRSVP, err := GetRSVPByUserForGame(db, user1.ID, game1.ID)
		if err != nil {
			t.Fatalf("GetRSVPByUserForGame() error = %v", err)
		}
		if retrievedRSVP.Status != models.RSVPStatusAttending {
			t.Errorf("RSVP status got = %v, want %v", retrievedRSVP.Status, models.RSVPStatusAttending)
		}
		if retrievedRSVP.UserID != user1.ID {
			t.Errorf("RSVP UserID got = %v, want %v", retrievedRSVP.UserID, user1.ID)
		}
		if retrievedRSVP.GameID != game1.ID {
			t.Errorf("RSVP GameID got = %v, want %v", retrievedRSVP.GameID, game1.ID)
		}
		if retrievedRSVP.UserEmail != user1.Email {
			t.Errorf("RSVP UserEmail got = %v, want %v", retrievedRSVP.UserEmail, user1.Email)
		}
		if retrievedRSVP.CreatedAt.IsZero() || retrievedRSVP.UpdatedAt.IsZero() {
			t.Errorf("RSVP CreatedAt or UpdatedAt is zero")
		}
	})

	t.Run("Update RSVP", func(t *testing.T) {
		// Ensure user1 and game1 exist from previous context or recreate if tests are isolated
		// This test run shares DB state with the one above.
		
		rsvpUpdate := &models.RSVP{
			UserID: user1.ID,
			GameID: game1.ID,
			Status: models.RSVPStatusNotAttending,
		}
		err := CreateOrUpdateRSVP(db, rsvpUpdate)
		if err != nil {
			t.Fatalf("CreateOrUpdateRSVP() for update error = %v", err)
		}

		retrievedRSVP, err := GetRSVPByUserForGame(db, user1.ID, game1.ID)
		if err != nil {
			t.Fatalf("GetRSVPByUserForGame() after update error = %v", err)
		}
		if retrievedRSVP.Status != models.RSVPStatusNotAttending {
			t.Errorf("Updated RSVP status got = %v, want %v", retrievedRSVP.Status, models.RSVPStatusNotAttending)
		}
		// CreatedAt should remain the same, UpdatedAt should change.
		// This requires storing the old UpdatedAt time to compare.
		// For simplicity, we're mainly checking the status update.
	})
	
	t.Run("Get Non-existent RSVP", func(t *testing.T) {
		userNonExist := createTestUserForRSVPs(t, db, "nonexist@example.com", "pass")
		_, err := GetRSVPByUserForGame(db, userNonExist.ID, game1.ID)
		if err != sql.ErrNoRows {
			t.Errorf("GetRSVPByUserForGame() for non-existent RSVP, got err = %v, want sql.ErrNoRows", err)
		}
	})
}

func TestGetRSVPsForGame(t *testing.T) {
	db, teardown := setupTestDBForRSVPs(t)
	defer teardown()

	user1 := createTestUserForRSVPs(t, db, "user1@rsvplist.com", "pass1")
	user2 := createTestUserForRSVPs(t, db, "user2@rsvplist.com", "pass2")
	user3 := createTestUserForRSVPs(t, db, "user3@rsvplist.com", "pass3")
	gm := createTestUserForRSVPs(t, db, "gmrsvplist@example.com", "gmpass")
	game := createTestGameForRSVPs(t, db, gm, "Multi-RSVP Game")

	rsvpsToCreate := []*models.RSVP{
		{UserID: user1.ID, GameID: game.ID, Status: models.RSVPStatusAttending},
		{UserID: user2.ID, GameID: game.ID, Status: models.RSVPStatusNotAttending},
		{UserID: user3.ID, GameID: game.ID, Status: models.RSVPStatusMaybe},
	}

	for _, r := range rsvpsToCreate {
		if err := CreateOrUpdateRSVP(db, r); err != nil {
			t.Fatalf("Failed to create RSVP for user %d: %v", r.UserID, err)
		}
		// Add a small delay to ensure UpdatedAt timestamps are distinct for ordering test
		time.Sleep(10 * time.Millisecond)
	}

	allRSVPs, err := GetRSVPsForGame(db, game.ID)
	if err != nil {
		t.Fatalf("GetRSVPsForGame() error = %v", err)
	}

	if len(allRSVPs) != len(rsvpsToCreate) {
		t.Errorf("GetRSVPsForGame() count = %d, want %d", len(allRSVPs), len(rsvpsToCreate))
	}

	// Check if emails are populated and statuses are correct
	// The order is by updated_at DESC. So user3 should be first.
	expectedOrderUserIDs := []int64{user3.ID, user2.ID, user1.ID} 
	expectedStatuses := map[int64]string{
		user1.ID: models.RSVPStatusAttending,
		user2.ID: models.RSVPStatusNotAttending,
		user3.ID: models.RSVPStatusMaybe,
	}
	expectedEmails := map[int64]string{
		user1.ID: user1.Email,
		user2.ID: user2.Email,
		user3.ID: user3.Email,
	}

	for i, rsvp := range allRSVPs {
		if len(allRSVPs) == len(expectedOrderUserIDs) { // Only check order if counts match
			if rsvp.UserID != expectedOrderUserIDs[i] {
				t.Errorf("GetRSVPsForGame() order incorrect. Pos %d: UserID got %d, want %d", i, rsvp.UserID, expectedOrderUserIDs[i])
			}
		}
		if rsvp.Status != expectedStatuses[rsvp.UserID] {
			t.Errorf("GetRSVPsForGame() status for UserID %d got %s, want %s", rsvp.UserID, rsvp.Status, expectedStatuses[rsvp.UserID])
		}
		if rsvp.UserEmail != expectedEmails[rsvp.UserID] {
			t.Errorf("GetRSVPsForGame() email for UserID %d got %s, want %s", rsvp.UserID, rsvp.UserEmail, expectedEmails[rsvp.UserID])
		}
	}
}
