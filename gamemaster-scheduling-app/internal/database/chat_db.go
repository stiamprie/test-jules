package database

import (
	"database/sql"
	// "time" // Not strictly needed if not using time.Now() directly here

	"github.com/gamemaster-scheduling/app/internal/models"
)

// CreateChatMessage inserts a new chat message into the chat_messages table.
// The UserEmail field in the passed models.ChatMessage is ignored here,
// as it's not a column in the chat_messages table. It's populated by GetChatMessagesForGame.
func CreateChatMessage(db *sql.DB, message *models.ChatMessage) (*models.ChatMessage, error) {
	stmt, err := db.Prepare("INSERT INTO chat_messages(game_id, user_id, message_content) VALUES(?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(message.GameID, message.UserID, message.MessageContent)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	// To get CreatedAt and potentially UserEmail (if we were to fetch it here, but we won't),
	// we need to retrieve the message.
	// For this function, we'll construct the returned message partially,
	// as the schema sets created_at. A full select would be more robust.
	// Let's do a Get to ensure all fields are consistent.
	
	// Minimal approach:
	// createdMsg := *message // copy
	// createdMsg.ID = id
	// // createdMsg.CreatedAt will be zero time.Time, unless schema default is read back,
	// // which LastInsertId doesn't provide.
	// return &createdMsg, nil

	// Better approach: Retrieve the message to get all DB-generated fields (like created_at)
	// and to ensure the UserEmail is correctly associated if we were to fetch it here.
	// However, to keep this function focused, we will query for the specific message
	// and populate its UserEmail from the users table.
	// This is slightly redundant if the calling handler already has user email,
	// but makes this function more self-contained for returning a "complete" ChatMessage model.

	var createdMessage models.ChatMessage
	row := db.QueryRow(`
		SELECT cm.id, cm.game_id, cm.user_id, u.email, cm.message_content, cm.created_at 
		FROM chat_messages cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.id = ?
	`, id)
	err = row.Scan(
		&createdMessage.ID,
		&createdMessage.GameID,
		&createdMessage.UserID,
		&createdMessage.UserEmail, // Populate UserEmail
		&createdMessage.MessageContent,
		&createdMessage.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &createdMessage, nil
}

// GetChatMessagesForGame retrieves all chat messages for a given game,
// including the user's email, ordered by creation time (oldest first).
func GetChatMessagesForGame(db *sql.DB, gameID int64) ([]*models.ChatMessage, error) {
	rows, err := db.Query(`
		SELECT cm.id, cm.game_id, cm.user_id, u.email, cm.message_content, cm.created_at
		FROM chat_messages cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.game_id = ?
		ORDER BY cm.created_at ASC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.ChatMessage
	for rows.Next() {
		msg := &models.ChatMessage{}
		err := rows.Scan(&msg.ID, &msg.GameID, &msg.UserID, &msg.UserEmail, &msg.MessageContent, &msg.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}
