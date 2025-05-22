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

// testServerRSVPChat struct holds a test server and its dependencies for RSVP/Chat tests
type testServerRSVPChat struct {
	server *httptest.Server
	db     *sql.DB
	client *http.Client // HTTP client that can handle cookies
	mux    *http.ServeMux
}

// setupTestServerForRSVPChat initializes a full test server for RSVP and Chat.
// It's largely similar to setupTestServerForGames.
func setupTestServerForRSVPChat(t *testing.T) *testServerRSVPChat {
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

	// Root Handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" { http.Redirect(w, r, "/games", http.StatusSeeOther) } else 
		{ RenderErrorPage(w, r, db, http.StatusNotFound, "Page Not Found", "Test: Page not found.")}
	})

	// Auth Routes
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
	
	// Game Routes (as in main.go, simplified for test focus)
	mux.HandleFunc("/games", GamesListPage(db))
	mux.HandleFunc("/games/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { AuthMiddleware(CreateGamePage)(w,r) } else
		if r.Method == http.MethodPost { AuthMiddleware(CreateGame(db))(w,r) } else
		{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "") }
	})

	// Dynamic Game Path Router (copied from main.go's structure for accuracy)
	mux.HandleFunc("/games/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		parts := strings.Split(strings.TrimPrefix(path, "/games/"), "/")

		if len(parts) == 0 || parts[0] == "" {
			RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Game ID missing or invalid path.")
			return
		}
		gameIDStr := parts[0]
		// Allow "new" to pass through if it wasn't caught by the more specific "/games/new"
		// This shouldn't happen if "/games/new" is registered before "/games/"
		if gameIDStr == "new" {
			// This case should be handled by the specific /games/new handler
			RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Should be handled by /games/new")
			return
		}
		
		_, err := strconv.ParseInt(gameIDStr, 10, 64) 
		if err != nil {
			RenderErrorPage(w, r, db, http.StatusBadRequest, "Bad Request", "Invalid Game ID format.")
			return
		}

		if len(parts) == 1 { 
			if r.Method == http.MethodGet { GameDetailPage(db)(w, r) } else 
			{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "Only GET for game detail") }
		} else if len(parts) == 2 { 
			action := parts[1]
			switch action {
			case "rsvp":
				if r.Method == http.MethodPost { AuthMiddleware(SubmitRSVP(db))(w,r) } else
				{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "RSVP requires POST") }
			case "chat":
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

	return &testServerRSVPChat{
		server: ts,
		db:     db,
		client: client,
		mux:    mux,
	}
}

func (ts *testServerRSVPChat) Teardown() {
	ts.server.Close()
	ts.db.Close()
}

// Helper to register and login a user
func (ts *testServerRSVPChat) registerAndLoginUser(t *testing.T, email, password string) (*http.Client, *models.User) {
	t.Helper()
	regData := url.Values{"email": {email}, "password": {password}, "confirm_password": {password}}
	respReg, errReg := ts.client.PostForm(ts.server.URL+"/register", regData)
	if errReg != nil { t.Fatalf("Helper register failed: %v", errReg) }
	defer respReg.Body.Close()
	if respReg.StatusCode != http.StatusSeeOther { t.Fatalf("Helper register expected redirect, got %d", respReg.StatusCode) }

	loginData := url.Values{"email": {email}, "password": {password}}
	respLogin, errLogin := ts.client.PostForm(ts.server.URL+"/login", loginData)
	if errLogin != nil { t.Fatalf("Helper login failed: %v", errLogin) }
	defer respLogin.Body.Close()
	if respLogin.StatusCode != http.StatusSeeOther { t.Fatalf("Helper login expected redirect, got %d", respLogin.StatusCode) }
	
	user, dbErr := database.GetUserByEmail(ts.db, email)
	if dbErr != nil { t.Fatalf("Helper: Failed to get user from DB after login: %v", dbErr) }
	return ts.client, user
}

// Helper to create a game directly in DB
func (ts *testServerRSVPChat) createTestGameDirectly(t *testing.T, gmID int64, title string) *models.Game {
	t.Helper()
	game := &models.Game{ GMID: gmID, Title: title, Description: "Test Desc", GameDateTime: time.Now().Add(7 * 24 * time.Hour), Location: "Test Location"}
	createdGame, err := database.CreateGame(ts.db, game)
	if err != nil { t.Fatalf("Failed to create test game directly: %v", err) }
	return createdGame
}


func TestSubmitRSVP(t *testing.T) {
	ts := setupTestServerForRSVPChat(t)
	defer ts.Teardown()

	authedClient, user := ts.registerAndLoginUser(t, "rsvptester@example.com", "password")
	_, gm := ts.registerAndLoginUser(t, "rsvp_gm@example.com", "gmpass") // GM needs separate login for cookie jar state if used
	
	testGame := ts.createTestGameDirectly(t, gm.ID, "RSVP Target Game")
	rsvpURL := ts.server.URL + "/games/" + strconv.FormatInt(testGame.ID, 10) + "/rsvp"

	t.Run("POST /games/{id}/rsvp authenticated", func(t *testing.T) {
		formData := url.Values{"status": {models.RSVPStatusAttending}}
		resp, err := authedClient.PostForm(rsvpURL, formData)
		if err != nil {
			t.Fatalf("POST RSVP failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK { // Expecting 200 OK with HTMX partial
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST RSVP status = %d; want %d. Body: %s", resp.StatusCode, http.StatusOK, string(bodyBytes))
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Your current status:") {
			t.Errorf("POST RSVP response does not contain RSVP section content. Body: %s", string(body))
		}
		if !strings.Contains(string(body), models.RSVPStatusAttending) {
			t.Errorf("POST RSVP response does not reflect new status. Body: %s", string(body))
		}

		// Check DB
		dbRSVP, dbErr := database.GetRSVPByUserForGame(ts.db, user.ID, testGame.ID)
		if dbErr != nil {
			t.Fatalf("Failed to get RSVP from DB: %v", dbErr)
		}
		if dbRSVP.Status != models.RSVPStatusAttending {
			t.Errorf("RSVP in DB status = %s; want %s", dbRSVP.Status, models.RSVPStatusAttending)
		}
	})

	t.Run("POST /games/{id}/rsvp unauthenticated", func(t *testing.T) {
		unauthClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		}
		formData := url.Values{"status": {models.RSVPStatusAttending}}
		resp, err := unauthClient.PostForm(rsvpURL, formData)
		if err != nil {
			t.Fatalf("POST RSVP unauth failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther { // Expect redirect to login
			t.Errorf("POST RSVP unauth status = %d; want %d", resp.StatusCode, http.StatusSeeOther)
		}
		location, _ := resp.Location()
		if location == nil || location.Path != "/login" {
			t.Errorf("POST RSVP unauth redirect = %v; want /login", location)
		}
	})
}


func TestPostChatMessage(t *testing.T) {
	ts := setupTestServerForRSVPChat(t)
	defer ts.Teardown()

	authedClient, user := ts.registerAndLoginUser(t, "chattester@example.com", "password")
	_, gm := ts.registerAndLoginUser(t, "chat_gm@example.com", "gmpass")
	
	testGame := ts.createTestGameDirectly(t, gm.ID, "Chat Target Game")
	chatURL := ts.server.URL + "/games/" + strconv.FormatInt(testGame.ID, 10) + "/chat"
	messageContent := "Hello from test! This is message " + strconv.FormatInt(time.Now().UnixNano(), 10)


	t.Run("POST /games/{id}/chat authenticated", func(t *testing.T) {
		formData := url.Values{"message_content": {messageContent}}
		resp, err := authedClient.PostForm(chatURL, formData)
		if err != nil {
			t.Fatalf("POST Chat failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK { // Expecting 200 OK with HTMX partial
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST Chat status = %d; want %d. Body: %s", resp.StatusCode, http.StatusOK, string(bodyBytes))
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), messageContent) {
			t.Errorf("POST Chat response does not contain new message. Body: %s", string(body))
		}
		if !strings.Contains(string(body), user.Email) { // Check if user's email is shown with message
			t.Errorf("POST Chat response does not contain user's email. Body: %s", string(body))
		}

		// Check DB
		dbMessages, dbErr := database.GetChatMessagesForGame(ts.db, testGame.ID)
		if dbErr != nil {
			t.Fatalf("Failed to get chat messages from DB: %v", dbErr)
		}
		foundInDB := false
		for _, msg := range dbMessages {
			if msg.MessageContent == messageContent && msg.UserID == user.ID {
				foundInDB = true
				break
			}
		}
		if !foundInDB {
			t.Errorf("Posted chat message not found in DB. Messages: %+v", dbMessages)
		}
	})

	t.Run("POST /games/{id}/chat unauthenticated", func(t *testing.T) {
		unauthClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		}
		formData := url.Values{"message_content": {"Unauthenticated message attempt"}}
		resp, err := unauthClient.PostForm(chatURL, formData)
		if err != nil {
			t.Fatalf("POST Chat unauth failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther { // Expect redirect to login
			t.Errorf("POST Chat unauth status = %d; want %d", resp.StatusCode, http.StatusSeeOther)
		}
		location, _ := resp.Location()
		if location == nil || !strings.HasSuffix(location.Path, "/login") { // Path might include query params from redirect
			t.Errorf("POST Chat unauth redirect = %v; want suffix /login", location)
		}
	})

	t.Run("POST /games/{id}/chat empty message", func(t *testing.T) {
		formData := url.Values{"message_content": {" "}} // Empty or whitespace only
		resp, err := authedClient.PostForm(chatURL, formData)
		if err != nil {
			t.Fatalf("POST Chat empty message failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK { // Handler should return 200 with error in partial
			t.Errorf("POST Chat empty message status = %d; want %d", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Message content cannot be empty.") {
			t.Errorf("POST Chat empty message response does not contain error message. Body: %s", string(body))
		}
	})
}
