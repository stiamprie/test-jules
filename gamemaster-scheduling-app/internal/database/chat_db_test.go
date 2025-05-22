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
func setupTestDBForChat(t *testing.T) (*sql.DB, func()) {
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
func createTestUserForChat(t *testing.T, db *sql.DB, email, password string) *models.User {
	t.Helper()
	user, err := CreateUser(db, email, password)
	if err != nil {
		t.Fatalf("Failed to create test user %s: %v", email, err)
	}
	return user
}

// createTestGame is a helper, duplicated here.
func createTestGameForChat(t *testing.T, db *sql.DB, gm *models.User, title string) *models.Game {
	t.Helper()
	gameTime := time.Now().Add(24 * time.Hour).Round(time.Second)
	game := &models.Game{
		GMID:         gm.ID,
		Title:        title,
		GameDateTime: gameTime,
		Location:     "Test Location for Chat",
	}
	createdGame, err := CreateGame(db, game)
	if err != nil {
		t.Fatalf("Failed to create test game %s: %v", title, err)
	}
	return createdGame
}

func TestCreateChatMessageAndGet(t *testing.T) {
	db, teardown := setupTestDBForChat(t)
	defer teardown()

	user1 := createTestUserForChat(t, db, "chatuser1@example.com", "pass1")
	user2 := createTestUserForChat(t, db, "chatuser2@example.com", "pass2")
	gm := createTestUserForChat(t, db, "gmchat@example.com", "gmpass")
	game1 := createTestGameForChat(t, db, gm, "Chat Test Game")

	msgContent1 := "Hello world from user1!"
	msgContent2 := "Hello back from user2!"

	t.Run("Create and Get Chat Messages", func(t *testing.T) {
		// Message 1
		chatMsg1 := &models.ChatMessage{
			GameID:         game1.ID,
			UserID:         user1.ID,
			MessageContent: msgContent1,
		}
		createdMsg1, err := CreateChatMessage(db, chatMsg1)
		if err != nil {
			t.Fatalf("CreateChatMessage() for msg1 error = %v", err)
		}
		if createdMsg1.ID == 0 {
			t.Errorf("CreateChatMessage() msg1 ID is 0")
		}
		if createdMsg1.MessageContent != msgContent1 {
			t.Errorf("CreateChatMessage() msg1 content = %s, want %s", createdMsg1.MessageContent, msgContent1)
		}
		if createdMsg1.UserEmail != user1.Email {
			t.Errorf("CreateChatMessage() msg1 UserEmail = %s, want %s", createdMsg1.UserEmail, user1.Email)
		}
		if createdMsg1.CreatedAt.IsZero() {
			t.Errorf("CreateChatMessage() msg1 CreatedAt is zero")
		}

		// Add a small delay to ensure CreatedAt timestamps are distinct for ordering test
		time.Sleep(10 * time.Millisecond)

		// Message 2
		chatMsg2 := &models.ChatMessage{
			GameID:         game1.ID,
			UserID:         user2.ID,
			MessageContent: msgContent2,
		}
		createdMsg2, err := CreateChatMessage(db, chatMsg2)
		if err != nil {
			t.Fatalf("CreateChatMessage() for msg2 error = %v", err)
		}
		if createdMsg2.UserEmail != user2.Email {
			t.Errorf("CreateChatMessage() msg2 UserEmail = %s, want %s", createdMsg2.UserEmail, user2.Email)
		}

		// Get all messages for the game
		allMessages, err := GetChatMessagesForGame(db, game1.ID)
		if err != nil {
			t.Fatalf("GetChatMessagesForGame() error = %v", err)
		}

		if len(allMessages) != 2 {
			t.Fatalf("GetChatMessagesForGame() count = %d, want 2. Messages: %+v", len(allMessages), allMessages)
		}

		// Check order (ASC by created_at) and content
		// Message 1 should be first
		if !reflect.DeepEqual(allMessages[0], createdMsg1) {
			// UserEmail is populated by CreateChatMessage itself now, so it should be equal.
			t.Errorf("GetChatMessagesForGame() msg1 got = %+v, want %+v", allMessages[0], createdMsg1)
		}
		// Message 2 should be second
		if !reflect.DeepEqual(allMessages[1], createdMsg2) {
			t.Errorf("GetChatMessagesForGame() msg2 got = %+v, want %+v", allMessages[1], createdMsg2)
		}
	})

	t.Run("Get Messages for Game with No Messages", func(t *testing.T) {
		gameNoMsg := createTestGameForChat(t, db, gm, "No Message Game")
		messages, err := GetChatMessagesForGame(db, gameNoMsg.ID)
		if err != nil {
			t.Fatalf("GetChatMessagesForGame() for empty game error = %v", err)
		}
		if len(messages) != 0 {
			t.Errorf("GetChatMessagesForGame() for empty game count = %d, want 0", len(messages))
		}
	})
}
