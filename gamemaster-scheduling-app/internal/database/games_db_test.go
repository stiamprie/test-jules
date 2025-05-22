package database

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/gamemaster-scheduling/app/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB is a helper from users_db_test.go, duplicated here for brevity.
// In a real scenario, this might be in a shared test utility package.
func setupTestDBForGames(t *testing.T) (*sql.DB, func()) {
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

// createTestUser is a helper from users_db_test.go, duplicated here.
func createTestUserForGames(t *testing.T, db *sql.DB, email, password string) *models.User {
	t.Helper()
	user, err := CreateUser(db, email, password)
	if err != nil {
		t.Fatalf("Failed to create test user %s: %v", email, err)
	}
	return user
}


func TestCreateGameAndGetGame(t *testing.T) {
	db, teardown := setupTestDBForGames(t)
	defer teardown()

	gm := createTestUserForGames(t, db, "gamemaster@example.com", "gmpass")

	gameTime := time.Now().Add(24 * time.Hour).Round(time.Second) // Round to second for DB compatibility
	game := &models.Game{
		GMID:         gm.ID,
		Title:        "Test Game Adventure",
		Description:  "A fun game for testing.",
		GameDateTime: gameTime,
		Location:     "Virtual (Discord)",
	}

	t.Run("Create and Get Game", func(t *testing.T) {
		createdGame, err := CreateGame(db, game)
		if err != nil {
			t.Fatalf("CreateGame() error = %v", err)
		}
		if createdGame.ID == 0 {
			t.Errorf("CreateGame() returned game with ID 0")
		}
		if createdGame.Title != game.Title {
			t.Errorf("CreateGame() title = %v, want %v", createdGame.Title, game.Title)
		}
		if !createdGame.GameDateTime.Equal(gameTime) {
			t.Errorf("CreateGame() gameTime = %v, want %v", createdGame.GameDateTime, gameTime)
		}
		if createdGame.CreatedAt.IsZero() {
			t.Errorf("CreateGame() CreatedAt is zero")
		}
		// Check GMID
		if createdGame.GMID != gm.ID {
			t.Errorf("CreateGame() GMID = %v, want %v", createdGame.GMID, gm.ID)
		}


		retrievedGame, err := GetGameByID(db, createdGame.ID)
		if err != nil {
			t.Fatalf("GetGameByID() error = %v", err)
		}
		// Ensure time.Time fields are compared correctly, especially if one is from DB (might have different tz or precision)
		// For SQLite, timestamps are usually stored as strings and converted back.
		// Rounding or using .Equal for time.Time is important.
		// createdGame.CreatedAt and createdGame.GameDateTime come from DB retrieval in CreateGame.
		// retrievedGame.CreatedAt and retrievedGame.GameDateTime come from DB retrieval in GetGameByID.
		// They should be equal if DB handling is consistent.
		if !reflect.DeepEqual(retrievedGame, createdGame) {
			t.Errorf("GetGameByID() got = %+v, want %+v", retrievedGame, createdGame)
		}
	})
	
	t.Run("Get Non-existent Game", func(t *testing.T){
		_, err := GetGameByID(db, 99999)
		if err != sql.ErrNoRows {
			t.Errorf("GetGameByID() for non-existent ID, got err = %v, want sql.ErrNoRows", err)
		}
	})
}

func TestGetAllGames(t *testing.T) {
	db, teardown := setupTestDBForGames(t)
	defer teardown()

	gm1 := createTestUserForGames(t, db, "gm1@example.com", "gmpass")
	gm2 := createTestUserForGames(t, db, "gm2@example.com", "gmpass")

	gameTime1 := time.Now().Add(48 * time.Hour).Round(time.Second)
	gameTime2 := time.Now().Add(24 * time.Hour).Round(time.Second) // Earlier game

	game1 := &models.Game{GMID: gm1.ID, Title: "Game 1 (Later)", GameDateTime: gameTime1, Location: "Loc1"}
	game2 := &models.Game{GMID: gm2.ID, Title: "Game 2 (Earlier)", GameDateTime: gameTime2, Location: "Loc2"}

	createdGame1, err := CreateGame(db, game1)
	if err != nil {
		t.Fatalf("Failed to create game1: %v", err)
	}
	createdGame2, err := CreateGame(db, game2)
	if err != nil {
		t.Fatalf("Failed to create game2: %v", err)
	}

	allGames, err := GetAllGames(db)
	if err != nil {
		t.Fatalf("GetAllGames() error = %v", err)
	}

	if len(allGames) != 2 {
		t.Errorf("GetAllGames() count = %d, want 2", len(allGames))
	}

	// Check order (DESC by game_datetime by default in GetAllGames)
	// So game1 (later) should be first.
	if len(allGames) == 2 {
		if allGames[0].ID != createdGame1.ID {
			t.Errorf("GetAllGames() order incorrect. Expected game1 first. Got game ID %d, Title %s. Expected game ID %d, Title %s", 
				allGames[0].ID, allGames[0].Title, createdGame1.ID, createdGame1.Title)
		}
		if allGames[1].ID != createdGame2.ID {
			t.Errorf("GetAllGames() order incorrect. Expected game2 second. Got game ID %d, Title %s. Expected game ID %d, Title %s",
				allGames[1].ID, allGames[1].Title, createdGame2.ID, createdGame2.Title)
		}
	}
}
