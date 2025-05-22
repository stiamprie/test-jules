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

// SubmitRSVP handles the submission of an RSVP for a game.
// It expects a POST request with a 'status' field.
// This handler should be wrapped by AuthMiddleware.
func SubmitRSVP(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		currentUser, err := GetCurrentUser(r, db)
		if err != nil {
			// This should ideally not happen if AuthMiddleware is working correctly.
			// For HTMX, returning an error that can be displayed or causes a redirect.
			w.Header().Set("HX-Redirect", "/login") // Redirect to login if not authenticated
			http.Error(w, "User not authenticated", http.StatusUnauthorized)
			return
		}

		// Extract Game ID from URL path: /games/{id}/rsvp
		pathParts := strings.Split(strings.TrimSuffix(r.URL.Path, "/rsvp"), "/")
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

		// Parse form data for RSVP status
		err = r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing form data", http.StatusBadRequest)
			return
		}
		status := r.FormValue("status")

		// Validate status
		switch status {
		case models.RSVPStatusAttending, models.RSVPStatusNotAttending, models.RSVPStatusMaybe:
			// valid
		default:
			http.Error(w, "Invalid RSVP status value", http.StatusBadRequest)
			return
		}

		rsvp := &models.RSVP{
			UserID: currentUser.ID,
			GameID: gameID,
			Status: status,
		}

		err = database.CreateOrUpdateRSVP(db, rsvp)
		if err != nil {
			// Log the error for server-side diagnosis
			fmt.Printf("Error creating or updating RSVP: %v\n", err)
			// Provide a generic error message to the client
			// You might want to render an error message within the partial for HTMX
			http.Error(w, "Failed to update RSVP status. Please try again.", http.StatusInternalServerError)
			return
		}

		// Successfully updated RSVP. Re-render the RSVP section.
		// Fetch updated data for the partial.
		allGameRSVPs, err := database.GetRSVPsForGame(db, gameID)
		if err != nil {
			fmt.Printf("Error fetching RSVPs for game %d after update: %v\n", gameID, err)
			http.Error(w, "Failed to refresh RSVP list.", http.StatusInternalServerError)
			return
		}

		currentUserRSVP, err := database.GetRSVPByUserForGame(db, currentUser.ID, gameID)
		if err != nil && err != sql.ErrNoRows {
			fmt.Printf("Error fetching current user's RSVP for game %d after update: %v\n", gameID, err)
			// Non-critical, can proceed without it if it fails, partial should handle nil
		}
		
		// Also need the Game itself for the context of the RSVP section (e.g. Game.ID for form posts)
		game, err := database.GetGameByID(db, gameID)
		if err != nil {
			fmt.Printf("Error fetching game %d for RSVP partial: %v\n", gameID, err)
			http.Error(w, "Failed to load game context for RSVP.", http.StatusInternalServerError)
			return
		}


		data := map[string]interface{}{
			"Game":            game, // Needed for forming hx-post URLs in the partial
			"User":            currentUser, // For conditional rendering within the partial
			"CurrentUserRSVP": currentUserRSVP,
			"AllGameRSVPs":    allGameRSVPs,
		}
		
		// Render only the partial for the HTMX response
		RenderTemplate(w, "games/_rsvp_section.html", data)
	}
}
