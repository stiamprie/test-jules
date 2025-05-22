package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gamemaster-scheduling/app/internal/database"
	"github.com/gamemaster-scheduling/app/internal/handlers"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

func main() {
	// Initialize database
	db, err := database.InitDB("scheduler.db")
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	// Load HTML templates
	// The path should be relative to where the binary is run, or absolute.
	// For development, running from project root, "web/templates" is fine.
	err = handlers.LoadTemplates("web/templates")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}

	// Initialize ServeMux
	mux := http.NewServeMux()

	// Static File Server
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Root Handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/games", http.StatusSeeOther)
		} else {
			// Pass db to RenderErrorPage
			handlers.RenderErrorPage(w, r, db, http.StatusNotFound, "Page Not Found", "The page you are looking for does not exist.")
		}
	})

	// Authentication Routes
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.RegisterPage(w, r)
		case http.MethodPost:
			handlers.Register(db)(w, r)
		default:
			handlers.RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "This method is not supported for /register.")
		}
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.LoginPage(w, r)
		case http.MethodPost:
			// Login handler uses the global handlers.SessionStore
			handlers.Login(db)(w, r)
		default:
			handlers.RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "This method is not supported for /login.")
		}
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Logout handler uses the global handlers.SessionStore
			handlers.Logout(w, r)
		} else {
			handlers.RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "Logout requires POST method.")
		}
	})

	// Game Routes
	mux.HandleFunc("/games", handlers.GamesListPage(db)) // Handles only "/games", not "/games/"

	mux.HandleFunc("/games/new", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.AuthMiddleware(handlers.CreateGamePage)(w, r)
		case http.MethodPost:
			handlers.AuthMiddleware(handlers.CreateGame(db))(w, r)
		default:
			handlers.RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "This method is not supported for /games/new.")
		}
	})
	
	// Dynamic Game Path Router
	// This needs to be specific enough not to overlap with /games or /games/new if they were also handled by it.
	// Since /games and /games/new are handled above, this will catch /games/{id} and /games/{id}/action
	mux.HandleFunc("/games/", routeDynamicGamePaths(db))


	// Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func routeDynamicGamePaths(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		parts := strings.Split(strings.TrimPrefix(path, "/games/"), "/")
		// Expected parts:
		// /games/{id} -> ["{id}"] -> len 1
		// /games/{id}/rsvp -> ["{id}", "rsvp"] -> len 2
		// /games/{id}/chat -> ["{id}", "chat"] -> len 2

		if len(parts) == 0 || parts[0] == "" {
			// This case might occur if path is just "/games/" with trailing slash and no ID
			handlers.RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Game ID missing or invalid path.")
			return
		}

		gameIDStr := parts[0]
		_, err := strconv.ParseInt(gameIDStr, 10, 64) // Validate gameID format
		if err != nil {
			handlers.RenderErrorPage(w, r, db, http.StatusBadRequest, "Bad Request", "Invalid Game ID format.")
			return
		}

		// Route based on number of parts and the action part
		if len(parts) == 1 { // Path is /games/{id}
			if r.Method == http.MethodGet {
				handlers.GameDetailPage(db)(w, r)
			} else {
				handlers.RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "Only GET is allowed for game details.")
			}
		} else if len(parts) == 2 { // Path is /games/{id}/action
			action := parts[1]
			switch action {
			case "rsvp":
				if r.Method == http.MethodPost {
					handlers.AuthMiddleware(handlers.SubmitRSVP(db))(w, r)
				} else {
					handlers.RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "Only POST is allowed for RSVP.")
				}
			case "chat":
				if r.Method == http.MethodPost {
					handlers.AuthMiddleware(handlers.PostChatMessage(db))(w, r)
				} else {
					handlers.RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "Only POST is allowed for chat.")
				}
			default:
				handlers.RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Invalid action for game.")
			}
		} else {
			// Path is too long or malformed, e.g., /games/{id}/action/extra
			handlers.RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Invalid game path structure.")
		}
	}
}
