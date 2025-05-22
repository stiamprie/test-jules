package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gamemaster-scheduling/app/internal/database"
	"github.com/gamemaster-scheduling/app/internal/models"
)

// PostChatMessage handles the submission of a chat message for a game.
// This handler should be wrapped by AuthMiddleware.
func PostChatMessage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		currentUser, err := GetCurrentUser(r, db)
		if err != nil {
			// For HTMX, returning an error that can be displayed or causes a redirect.
			// A 401 Unauthorized might trigger client-side logic or HTMX error handling.
			// Or, redirect to login.
			w.Header().Set("HX-Redirect", "/login")
			http.Error(w, "User not authenticated", http.StatusUnauthorized)
			return
		}

		// Extract Game ID from URL path: /games/{id}/chat
		pathParts := strings.Split(strings.TrimSuffix(r.URL.Path, "/chat"), "/")
		gameIDStr := ""
		if len(pathParts) > 0 {
			gameIDStr = pathParts[len(pathParts)-1]
		}
		if gameIDStr == "" {
			http.Error(w, "Game ID missing in URL path", http.StatusBadRequest)
			return
		}
		gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid Game ID format", http.StatusBadRequest)
			return
		}

		// Parse form data for message content
		err = r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing form data", http.StatusBadRequest)
			return
		}
		messageContent := r.FormValue("message_content")

		if strings.TrimSpace(messageContent) == "" {
			// Re-render chat section with an error message.
			// This requires fetching existing messages and adding an error to the template data.
			// For simplicity here, we'll return a Bad Request.
			// A more user-friendly approach would be to return the chat partial with an error message.
			// To do that, we'd fetch existing messages, then pass an error string to the partial.
			
			// Fetch existing messages to re-render the chat area
			chatMessages, fetchErr := database.GetChatMessagesForGame(db, gameID)
			if fetchErr != nil {
				fmt.Printf("Error fetching chat messages for game %d: %v\n", gameID, fetchErr)
				http.Error(w, "Failed to refresh chat messages after validation error.", http.StatusInternalServerError)
				return
			}
			data := map[string]interface{}{
				"ChatMessages": chatMessages,
				"GameID": gameID, // For the form action URL in the partial
				"User": currentUser, // For form display logic
				"Error": "Message content cannot be empty.",
			}
			// It's important that the client-side target for this error is correct.
			// If the form itself is inside the hx-target, this will replace the form and messages.
			RenderTemplate(w, "games/_chat_messages.html", data)
			return
		}

		chatMessage := &models.ChatMessage{
			GameID:         gameID,
			UserID:         currentUser.ID,
			UserEmail:      currentUser.Email, // This is for the model, DB layer might re-fetch or use UserID
			MessageContent: messageContent,
		}

		_, err = database.CreateChatMessage(db, chatMessage)
		if err != nil {
			fmt.Printf("Error creating chat message: %v\n", err)
			// In a real app, you might want to return a more user-friendly error
			// or re-render the chat section with an error.
			http.Error(w, "Failed to post message. Please try again.", http.StatusInternalServerError)
			return
		}

		// Successfully posted. Re-render the chat messages section.
		updatedChatMessages, err := database.GetChatMessagesForGame(db, gameID)
		if err != nil {
			fmt.Printf("Error fetching chat messages for game %d after post: %v\n", gameID, err)
			http.Error(w, "Failed to refresh chat messages.", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"ChatMessages": updatedChatMessages,
			"GameID": gameID, // For the form action URL in the partial, if form is part of it
			"User": currentUser, // For conditional rendering in the partial (e.g. showing form)
		}
		RenderTemplate(w, "games/_chat_messages.html", data)
	}
}
