package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gamemaster-scheduling/app/internal/database"
	"github.com/gamemaster-scheduling/app/internal/models"
	// "github.com/gorilla/mux" // Or use net/http path parsing
)

// GamesListPage displays all available games.
func GamesListPage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		games, err := database.GetAllGames(db)
		if err != nil {
			http.Error(w, "Failed to retrieve games: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// For now, we'll pass the games directly.
		// Later, we might add user login status to the data for conditional rendering in template.
		currentUser, _ := GetCurrentUser(r, db) // Ignore error for now, template will handle nil user

		data := map[string]interface{}{
			"Games": games,
			"User": currentUser,
		}
		RenderTemplate(w, "games/games_list.html", data)
	}
}

// GameDetailPage displays details for a specific game.
func GameDetailPage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Path: /games/{id}
		// This requires a router that can extract path parameters.
		// For standard net/http, you'd parse r.URL.Path.
		// Example with basic path parsing:
		// parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/games/"), "/")
		// if len(parts) < 1 || parts[0] == "" {
		//	 http.Error(w, "Game ID missing", http.StatusBadRequest)
		//	 return
		// }
		// gameIDStr := parts[0]

		// Assuming Gorilla Mux or similar is used for routing, like:
		// r := mux.NewRouter()
		// r.HandleFunc("/games/{id}", GameDetailPage(db)).Methods("GET")
		// vars := mux.Vars(r)
		// gameIDStr := vars["id"]
		// For now, we'll simulate this by getting it from query param for simplicity
		// until routing is fully set up in main.go.
		// Or, more robustly, parse the last segment of the path.
		
		pathParts := strings.Split(r.URL.Path, "/")
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

		game, err := database.GetGameByID(db, gameID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Game not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		currentUser, _ := GetCurrentUser(r, db) // Error ignored for now, template handles nil user

		allGameRSVPs, err := database.GetRSVPsForGame(db, gameID)
		if err != nil {
			// Log this error but don't necessarily fail the whole page load
			fmt.Printf("Error fetching all RSVPs for game %d: %v\n", gameID, err)
			// allGameRSVPs will be nil, template should handle this
		}

		var currentUserRSVP *models.RSVP
		if currentUser != nil {
			currentUserRSVP, err = database.GetRSVPByUserForGame(db, currentUser.ID, gameID)
			if err != nil && err != sql.ErrNoRows {
				// Log this error but don't necessarily fail the whole page load
				fmt.Printf("Error fetching current user's RSVP for game %d: %v\n", gameID, err)
				// currentUserRSVP will be nil, template should handle this
			}
			// If err is sql.ErrNoRows, currentUserRSVP remains nil, which is correct (user hasn't RSVP'd)
		}
		
		// Constants for RSVP status, to be used in templates if needed
		// Though for the current _rsvp_section.html, these are not directly used in hx-vals
		// as strings are directly embedded. But good to have if template logic changes.
		data := map[string]interface{}{
			"Game":              game,
			"User":              currentUser, // Renamed from LoggedInUser for consistency with other templates
			"CurrentUserRSVP":   currentUserRSVP, // Renamed from UserRSVP
			"AllGameRSVPs":      allGameRSVPs,
			"RSVPStatusAttending": models.RSVPStatusAttending,
			"RSVPStatusMaybe":     models.RSVPStatusMaybe,
			"RSVPStatusNotAttending": models.RSVPStatusNotAttending,
		}

		chatMessages, err := database.GetChatMessagesForGame(db, gameID)
		if err != nil {
			// Log this error but don't necessarily fail the whole page load
			fmt.Printf("Error fetching chat messages for game %d: %v\n", gameID, err)
			// chatMessages will be nil or empty, template should handle this
		}
		data["ChatMessages"] = chatMessages
		data["GameID"] = gameID // Already part of 'game' object, but explicit for chat form if needed

		RenderTemplate(w, "games/game_detail.html", data)
	}
}

// CreateGamePage renders the form for creating a new game.
// This handler should be wrapped by AuthMiddleware.
func CreateGamePage(w http.ResponseWriter, r *http.Request) {
	// Pass nil data if the form doesn't need any initial data.
	// If re-rendering with errors, this data object would contain error messages.
	RenderTemplate(w, "games/new_game.html", nil)
}

// CreateGame handles the submission of the new game form.
// This handler should be wrapped by AuthMiddleware.
func CreateGame(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}

		title := r.FormValue("title")
		description := r.FormValue("description")
		gameDateTimeStr := r.FormValue("game_datetime") // Format: "YYYY-MM-DDTHH:MM"
		location := r.FormValue("location")

		// Validation
		if title == "" || gameDateTimeStr == "" || location == "" {
			data := map[string]interface{}{"Error": "Title, Game Date/Time, and Location are required."}
			RenderTemplate(w, "games/new_game.html", data) // Re-render form with error
			return
		}

		// Parse game_datetime
		// HTML input type="datetime-local" sends data in "YYYY-MM-DDTHH:MM" format
		gameDateTime, err := time.Parse("2006-01-02T15:04", gameDateTimeStr)
		if err != nil {
			data := map[string]interface{}{
				"Error": "Invalid date/time format. Use YYYY-MM-DDTHH:MM.",
				"Form": map[string]string{ // Keep submitted values to repopulate form
					"title": title, "description": description, "game_datetime": gameDateTimeStr, "location": location,
				},
			}
			RenderTemplate(w, "games/new_game.html", data)
			return
		}

		currentUser, err := GetCurrentUser(r, db)
		if err != nil {
			// This should ideally not happen if AuthMiddleware is working correctly
			http.Error(w, "User not authenticated: "+err.Error(), http.StatusUnauthorized)
			return
		}

		game := &models.Game{
			GMID:         currentUser.ID,
			Title:        title,
			Description:  description,
			GameDateTime: gameDateTime,
			Location:     location,
		}

		createdGame, err := database.CreateGame(db, game)
		if err != nil {
			data := map[string]interface{}{
				"Error": "Failed to create game: " + err.Error(),
				"Form": map[string]string{
					"title": title, "description": description, "game_datetime": gameDateTimeStr, "location": location,
				},
			}
			RenderTemplate(w, "games/new_game.html", data)
			return
		}

		// Successful creation, redirect to the game's detail page.
		// For HTMX, a redirect can be triggered by HX-Redirect header.
		redirectURL := fmt.Sprintf("/games/%d", createdGame.ID)
		w.Header().Set("HX-Redirect", redirectURL) // For HTMX clients
		// http.Redirect(w, r, redirectURL, http.StatusSeeOther) // For non-HTMX clients, but HX-Redirect is often preferred with HTMX
	}
}
