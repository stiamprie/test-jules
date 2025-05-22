package handlers

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gamemaster-scheduling/app/internal/database"
	"github.com/gamemaster-scheduling/app/internal/models"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// testServerGame struct holds a test server and its dependencies for game tests
type testServerGame struct {
	server *httptest.Server
	db     *sql.DB
	client *http.Client // HTTP client that can handle cookies
	mux    *http.ServeMux // Expose mux to add more routes if needed
}

// setupTestServerForGames initializes an in-memory SQLite database, loads templates,
// sets up the application router (including dynamic game paths), and starts an httptest.Server.
func setupTestServerForGames(t *testing.T) *testServerGame {
	t.Helper()

	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	templatePath := "../../web/templates"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		templatePath = "web/templates"
	}
	err = LoadTemplates(templatePath)
	if err != nil {
		t.Fatalf("Error loading templates from %s: %v", templatePath, err)
	}

	mux := http.NewServeMux()

	// Static File Server
	// fs := http.FileServer(http.Dir("../../web/static"))
	// mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Root Handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" { http.Redirect(w, r, "/games", http.StatusSeeOther) } else 
		{ RenderErrorPage(w, r, db, http.StatusNotFound, "Page Not Found", "Test: Page not found.")}
	})

	// Auth Routes (needed for testing auth-protected game routes)
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { RegisterPage(w, r) } else 
		if r.Method == http.MethodPost { Register(db)(w, r) } else 
		{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "")}
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { LoginPage(w, r) } else 
		if r.Method == http.MethodPost { Login(db)(w, r) } else
		{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "")}
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost { Logout(w, r) } else
		{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "")}
	})

	// Game Routes
	mux.HandleFunc("/games", GamesListPage(db))
	mux.HandleFunc("/games/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { AuthMiddleware(CreateGamePage)(w,r) } else
		if r.Method == http.MethodPost { AuthMiddleware(CreateGame(db))(w,r) } else
		{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "") }
	})
	
	// Dynamic Game Path Router (simplified from main.go for test focus)
	// This needs to be robust enough for the tests.
	mux.HandleFunc("/games/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		parts := strings.Split(strings.TrimPrefix(path, "/games/"), "/")

		if len(parts) == 0 || parts[0] == "" {
			RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Game ID missing or invalid path.")
			return
		}
		gameIDStr := parts[0]
		if _, err := strconv.ParseInt(gameIDStr, 10, 64); err != nil && gameIDStr != "new" { 
			// "new" is handled by its own more specific mux.HandleFunc
			RenderErrorPage(w, r, db, http.StatusBadRequest, "Bad Request", "Invalid Game ID format.")
			return
		}

		if len(parts) == 1 { // Path is /games/{id}
			if r.Method == http.MethodGet { GameDetailPage(db)(w, r) } else 
			{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "") }
		} else if len(parts) == 2 { // Path is /games/{id}/action
			action := parts[1]
			// RSVP and Chat handlers will be tested in their own file,
			// but the routing logic needs to be aware of them.
			// For now, just a placeholder or error if not testing those actions here.
			switch action {
			case "rsvp": // Placeholder for RSVP
				if r.Method == http.MethodPost { AuthMiddleware(SubmitRSVP(db))(w,r) } else
				{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "RSVP requires POST") }
			case "chat": // Placeholder for Chat
				if r.Method == http.MethodPost { AuthMiddleware(PostChatMessage(db))(w,r) } else
				{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "Chat requires POST") }
			default:
				RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Invalid game action.")
			}
		} else {
			RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Invalid game path structure.")
		}
	})


	ts := httptest.NewServer(mux)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse 
		},
	}

	return &testServerGame{
		server: ts,
		db:     db,
		client: client,
		mux:    mux,
	}
}

func (ts *testServerGame) Teardown() {
	ts.server.Close()
	ts.db.Close()
}

// Helper to register and login a user, returns the authenticated client and user model
func (ts *testServerGame) registerAndLoginUser(t *testing.T, email, password string) (*http.Client, *models.User) {
	t.Helper()
	// Register
	regData := url.Values{"email": {email}, "password": {password}, "confirm_password": {password}}
	resp, err := ts.client.PostForm(ts.server.URL+"/register", regData)
	if err != nil {
		t.Fatalf("Helper register failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther { // Should redirect to /login
		t.Fatalf("Helper register expected redirect, got %d", resp.StatusCode)
	}

	// Login
	loginData := url.Values{"email": {email}, "password": {password}}
	resp, err = ts.client.PostForm(ts.server.URL+"/login", loginData)
	if err != nil {
		t.Fatalf("Helper login failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther { // Should redirect to /games
		t.Fatalf("Helper login expected redirect, got %d", resp.StatusCode)
	}
	
	user, dbErr := database.GetUserByEmail(ts.db, email)
	if dbErr != nil {
		t.Fatalf("Helper: Failed to get user from DB after login: %v", dbErr)
	}
	return ts.client, user // client now has session cookie
}

// Helper to create a game directly in DB for testing reads
func (ts *testServerGame) createTestGameDirectly(t *testing.T, gmID int64, title string) *models.Game {
	t.Helper()
	game := &models.Game{
		GMID:         gmID,
		Title:        title,
		Description:  "Test Desc",
		GameDateTime: time.Now().Add(7 * 24 * time.Hour),
		Location:     "Test Location",
	}
	createdGame, err := database.CreateGame(ts.db, game)
	if err != nil {
		t.Fatalf("Failed to create test game directly: %v", err)
	}
	return createdGame
}


func TestGamesListAndDetailPages(t *testing.T) {
	ts := setupTestServerForGames(t)
	defer ts.Teardown()

	// Register a GM user to create games
	_, gm := ts.registerAndLoginUser(t, "gamemaster@example.com", "password123")

	t.Run("GET /games empty", func(t *testing.T) {
		resp, err := ts.client.Get(ts.server.URL + "/games")
		if err != nil {
			t.Fatalf("GET /games failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /games status = %d; want %d", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "No games are currently scheduled") {
			t.Errorf("GET /games (empty) did not find 'No games' message. Body: %s", string(body))
		}
	})

	// Create a test game
	testGame1 := ts.createTestGameDirectly(t, gm.ID, "My Awesome Game")
	testGame2 := ts.createTestGameDirectly(t, gm.ID, "Another Cool Game")


	t.Run("GET /games with games", func(t *testing.T) {
		resp, err := ts.client.Get(ts.server.URL + "/games")
		if err != nil {
			t.Fatalf("GET /games with games failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /games with games status = %d; want %d", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), testGame1.Title) {
			t.Errorf("GET /games with games response does not contain game1 title '%s'. Body: %s", testGame1.Title, string(body))
		}
		if !strings.Contains(string(body), testGame2.Title) {
			t.Errorf("GET /games with games response does not contain game2 title '%s'. Body: %s", testGame2.Title, string(body))
		}
	})

	t.Run("GET /games/{id} valid", func(t *testing.T) {
		gameURL := ts.server.URL + "/games/" + strconv.FormatInt(testGame1.ID, 10)
		resp, err := ts.client.Get(gameURL)
		if err != nil {
			t.Fatalf("GET %s failed: %v", gameURL, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d; want %d", gameURL, resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), testGame1.Title) {
			t.Errorf("GET %s response does not contain game title. Body: %s", gameURL, string(body))
		}
		if !strings.Contains(string(body), testGame1.Description) {
			t.Errorf("GET %s response does not contain game description. Body: %s", gameURL, string(body))
		}
	})

	t.Run("GET /games/{non_existent_id}", func(t *testing.T) {
		gameURL := ts.server.URL + "/games/999999"
		resp, err := ts.client.Get(gameURL)
		if err != nil {
			t.Fatalf("GET %s failed: %v", gameURL, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d; want %d (NotFound)", gameURL, resp.StatusCode, http.StatusNotFound)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Error 404: Page Not Found") && !strings.Contains(string(body), "Error 404: Not Found"){
			t.Errorf("GET %s response does not contain standard 404 error message. Body: %s", gameURL, string(body))
		}
	})
}


func TestCreateGame(t *testing.T) {
	ts := setupTestServerForGames(t)
	defer ts.Teardown()

	// Unauthenticated client
	unauthClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}

	t.Run("GET /games/new unauthenticated", func(t *testing.T) {
		resp, err := unauthClient.Get(ts.server.URL + "/games/new")
		if err != nil {
			t.Fatalf("GET /games/new unauth failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther { // Expect redirect to login
			t.Errorf("GET /games/new unauth status = %d; want %d", resp.StatusCode, http.StatusSeeOther)
		}
		location, _ := resp.Location()
		if location == nil || location.Path != "/login" {
			t.Errorf("GET /games/new unauth redirect = %v; want /login", location)
		}
	})

	t.Run("POST /games/new unauthenticated", func(t *testing.T) {
		resp, err := unauthClient.PostForm(ts.server.URL+"/games/new", url.Values{})
		if err != nil {
			t.Fatalf("POST /games/new unauth failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther { // Expect redirect to login
			t.Errorf("POST /games/new unauth status = %d; want %d", resp.StatusCode, http.StatusSeeOther)
		}
	})

	// Authenticated client
	authedClient, _ := ts.registerAndLoginUser(t, "gamecreator@example.com", "securepass")

	t.Run("GET /games/new authenticated", func(t *testing.T) {
		resp, err := authedClient.Get(ts.server.URL + "/games/new")
		if err != nil {
			t.Fatalf("GET /games/new auth failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET /games/new auth status = %d; want %d. Body: %s", resp.StatusCode, http.StatusOK, string(bodyBytes))
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `Host a New Game`) {
			t.Errorf("GET /games/new auth response does not contain form title. Body: %s", string(body))
		}
	})

	t.Run("POST /games/new authenticated valid", func(t *testing.T) {
		gameTitle := "Epic Test Quest"
		gameTime := time.Now().Add(72 * time.Hour).Format("2006-01-02T15:04") // HTML datetime-local format

		formData := url.Values{}
		formData.Set("title", gameTitle)
		formData.Set("description", "A grand adventure for brave testers.")
		formData.Set("game_datetime", gameTime)
		formData.Set("location", "The Test Server")
		
		resp, err := authedClient.PostForm(ts.server.URL+"/games/new", formData)
		if err != nil {
			t.Fatalf("POST /games/new auth valid failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK { // HTMX target expects 200
			// Check if it's a redirect (HX-Redirect header from server)
			// The client itself won't follow this if CheckRedirect is ErrUseLastResponse
			// but the status code might be 200 if handler sets HX-Redirect and writes content,
			// or it could be a 302/303 if it's a plain http.Redirect.
			// The CreateGame handler uses w.Header().Set("HX-Redirect", redirectURL) and no explicit status.
			// net/http default is 200 OK if not set.
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST /games/new auth valid status = %d; want %d. Body: %s. HX-Redirect: %s",
				resp.StatusCode, http.StatusOK, string(bodyBytes), resp.Header.Get("HX-Redirect"))
		}

		hxRedirect := resp.Header.Get("HX-Redirect")
		if hxRedirect == "" {
			t.Errorf("POST /games/new auth valid expected HX-Redirect header, got none")
		}
		if !strings.HasPrefix(hxRedirect, "/games/") {
			t.Errorf("POST /games/new auth valid HX-Redirect = %s; want prefix /games/", hxRedirect)
		}

		// Verify game in DB
		// We need to parse the ID from hxRedirect
		parts := strings.Split(strings.Trim(hxRedirect, "/"), "/") // e.g. ["games", "1"]
		if len(parts) != 2 {
			t.Fatalf("Could not parse game ID from HX-Redirect: %s", hxRedirect)
		}
		gameID, convErr := strconv.ParseInt(parts[1], 10, 64)
		if convErr != nil {
			t.Fatalf("Could not convert game ID '%s' to int: %v", parts[1], convErr)
		}

		dbGame, dbErr := database.GetGameByID(ts.db, gameID)
		if dbErr != nil {
			t.Errorf("Game not found in DB after creation: %v", dbErr)
		}
		if dbGame.Title != gameTitle {
			t.Errorf("Game in DB title = %s; want %s", dbGame.Title, gameTitle)
		}
	})
}
