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
	// Models are implicitly used via handlers and db functions
	// _ "github.com/gamemaster-scheduling/app/internal/models"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// testServer struct holds a test server and its dependencies
type testServer struct {
	server *httptest.Server
	db     *sql.DB
	client *http.Client // HTTP client that can handle cookies
}

// setupTestServer initializes an in-memory SQLite database, loads templates,
// sets up the application router, and starts an httptest.Server.
// It mimics the setup in main.go.
func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	// Initialize database
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Load HTML templates - path relative to this test file
	// Assuming this file is in internal/handlers, web/templates is ../../web/templates
	templatePath := "../../web/templates" 
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		// Fallback for different CWD (e.g. if running tests from project root)
		templatePath = "web/templates" 
	}
	err = LoadTemplates(templatePath)
	if err != nil {
		t.Fatalf("Error loading templates from %s: %v", templatePath, err)
	}

	// Setup router (mimicking main.go's mux setup)
	mux := http.NewServeMux()

	// Static File Server (not strictly needed for handler logic tests, but good for completeness)
	// fs := http.FileServer(http.Dir("../../web/static")) // Adjust path if testing static files
	// mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Root Handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/games", http.StatusSeeOther)
		} else {
			RenderErrorPage(w, r, db, http.StatusNotFound, "Page Not Found", "Test: The page you are looking for does not exist.")
		}
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

	// Game Routes (simplified for now, will expand in game_handlers_test.go)
	mux.HandleFunc("/games", GamesListPage(db))
	mux.HandleFunc("/games/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet { AuthMiddleware(CreateGamePage)(w,r) } else
		if r.Method == http.MethodPost { AuthMiddleware(CreateGame(db))(w,r) } else
		{ RenderErrorPage(w, r, db, http.StatusMethodNotAllowed, "Method Not Allowed", "") }
	})
	// Placeholder for dynamic game paths, actual router needed for /games/{id} etc.
	// For now, this is enough for auth tests.
	// The `routeDynamicGamePaths` function from main.go would be used here in a more complete setup.
	// For now, we'll add it when testing game/rsvp/chat handlers.
	// This simplified version won't correctly route /games/{id} yet.
	mux.HandleFunc("/games/", func(w http.ResponseWriter, r *http.Request) {
		// A basic catch-all for /games/* to avoid 404s where not explicitly handled
		// This will be replaced by a proper dynamic router in game_handlers_test.go
		if strings.HasPrefix(r.URL.Path, "/games/") && r.URL.Path != "/games/new" {
			// Simulate a simple game detail page or an error for now
			// RenderErrorPage(w, r, db, http.StatusNotFound, "Not Found", "Game detail page placeholder")
			// For login redirect tests, it's better if this route exists somewhat.
			// Let's assume GamesListPage can handle some of these or we just test the redirect target.
			GamesListPage(db)(w,r) // Or a more specific handler if needed for tests
		}
	})


	// Create a new httptest.Server
	ts := httptest.NewServer(mux)
	
	// Create a client with a cookie jar to handle sessions
	jar, err := cookiejar.New(nil)
	if err != nil {
		ts.Close() // Clean up server
		db.Close() // Clean up db
		t.Fatalf("Failed to create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		// Prevent auto-redirects to inspect intermediate responses (e.g. 302 redirect from POST)
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Important for testing redirects
		},
	}

	return &testServer{
		server: ts,
		db:     db,
		client: client,
	}
}

// Teardown closes the test server and database connection.
func (ts *testServer) Teardown() {
	ts.server.Close()
	ts.db.Close()
}

func TestRegisterAndLoginLogout(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Teardown()

	uniqueEmail := "testuser" + strconvFormatInt(time.Now().UnixNano()) + "@example.com"
	password := "password123"

	// 1. GET /register
	t.Run("GET /register", func(t *testing.T) {
		resp, err := ts.client.Get(ts.server.URL + "/register")
		if err != nil {
			t.Fatalf("GET /register failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /register status = %d; want %d", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `<form hx-post="/register"`) {
			t.Errorf("GET /register response does not contain registration form")
		}
	})

	// 2. POST /register with valid data
	t.Run("POST /register valid", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("email", uniqueEmail)
		formData.Set("password", password)
		formData.Set("confirm_password", password)

		resp, err := ts.client.PostForm(ts.server.URL + "/register", formData)
		if err != nil {
			t.Fatalf("POST /register failed: %v", err)
		}
		defer resp.Body.Close()
		
		// Expect redirect to /login
		if resp.StatusCode != http.StatusSeeOther {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST /register valid status = %d; want %d. Body: %s", resp.StatusCode, http.StatusSeeOther, string(bodyBytes))
		}
		location, err := resp.Location()
		if err != nil {
			t.Fatalf("POST /register valid redirect location error: %v", err)
		}
		if location.Path != "/login" {
			t.Errorf("POST /register valid redirect location = %s; want /login", location.Path)
		}

		// Check if user is in DB
		_, err = database.GetUserByEmail(ts.db, uniqueEmail)
		if err != nil {
			t.Errorf("User not found in DB after registration: %v", err)
		}
	})

	// 3. POST /register with existing email
	t.Run("POST /register existing email", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("email", uniqueEmail) // Same email as above
		formData.Set("password", password)
		formData.Set("confirm_password", password)

		resp, err := ts.client.PostForm(ts.server.URL + "/register", formData)
		if err != nil {
			t.Fatalf("POST /register existing email failed: %v", err)
		}
		defer resp.Body.Close()

		// Expect form to re-render with an error
		if resp.StatusCode != http.StatusOK { // Or whatever status code your handler returns for this
			t.Errorf("POST /register existing email status = %d; want %d (or error status)", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Email already registered.") {
			t.Errorf("POST /register existing email response does not contain error message. Body: %s", string(body))
		}
	})

	// 4. GET /login
	t.Run("GET /login", func(t *testing.T) {
		resp, err := ts.client.Get(ts.server.URL + "/login")
		if err != nil {
			t.Fatalf("GET /login failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /login status = %d; want %d", resp.StatusCode, http.StatusOK)
		}
	})
	
	// 5. POST /login with valid credentials
	t.Run("POST /login valid", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("email", uniqueEmail)
		formData.Set("password", password)

		resp, err := ts.client.PostForm(ts.server.URL + "/login", formData)
		if err != nil {
			t.Fatalf("POST /login valid failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST /login valid status = %d; want %d. Body: %s", resp.StatusCode, http.StatusSeeOther, string(bodyBytes))
		}
		location, err := resp.Location()
		if err != nil {
			t.Fatalf("POST /login valid redirect location error: %v", err)
		}
		// The Login handler redirects to "/games"
		if location.Path != "/games" { 
			t.Errorf("POST /login valid redirect location = %s; want /games", location.Path)
		}

		// Check for session cookie
		foundCookie := false
		for _, cookie := range ts.client.Jar.Cookies(mustParseURL(t, ts.server.URL)) {
			if cookie.Name == sessionCookieName {
				foundCookie = true
				break
			}
		}
		if !foundCookie {
			t.Errorf("Session cookie not found after valid login")
		}
	})

	// 6. POST /login with invalid credentials
	t.Run("POST /login invalid", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("email", uniqueEmail)
		formData.Set("password", "wrongpassword")

		resp, err := ts.client.PostForm(ts.server.URL + "/login", formData)
		if err != nil {
			t.Fatalf("POST /login invalid failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK { // Assuming re-render with error
			t.Errorf("POST /login invalid status = %d; want %d", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Invalid email or password.") {
			t.Errorf("POST /login invalid response does not contain error message. Body: %s", string(body))
		}
	})

	// 7. POST /logout (with active session)
	// Ensure user is logged in from previous test ("POST /login valid")
	t.Run("POST /logout", func(t *testing.T) {
		// First, verify we have a session cookie
		var initialSessionCookie *http.Cookie
		for _, cookie := range ts.client.Jar.Cookies(mustParseURL(t, ts.server.URL)) {
			if cookie.Name == sessionCookieName {
				initialSessionCookie = cookie
				break
			}
		}
		if initialSessionCookie == nil {
			t.Fatal("Cannot test logout: No session cookie found from previous login.")
		}


		resp, err := ts.client.PostForm(ts.server.URL + "/logout", url.Values{})
		if err != nil {
			t.Fatalf("POST /logout failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("POST /logout status = %d; want %d", resp.StatusCode, http.StatusSeeOther)
		}
		location, err := resp.Location()
		if err != nil {
			t.Fatalf("POST /logout redirect location error: %v", err)
		}
		if location.Path != "/login" { // Logout redirects to /login
			t.Errorf("POST /logout redirect location = %s; want /login", location.Path)
		}

		// Check if session cookie is cleared/expired
		// The cookie might still exist but be expired or its value removed from server's sessionStore
		cookieCleared := true
		for _, cookie := range ts.client.Jar.Cookies(mustParseURL(t, ts.server.URL)) {
			if cookie.Name == sessionCookieName {
				// Check if its value is empty or if it's expired (MaxAge < 0 or Expires in the past)
				if cookie.Value != "" && (cookie.MaxAge >= 0 && !cookie.Expires.Before(time.Now().Add(-time.Minute))) {
					// If MaxAge is 0, it's a session cookie, should be gone if value is empty.
					// If MaxAge > 0, it's persistent, check Expires.
					// Here, the handler sets Expires to time.Unix(0,0).
					cookieCleared = false
					t.Logf("Logout check: Found session cookie with value '%s', MaxAge %d, Expires %v", cookie.Value, cookie.MaxAge, cookie.Expires)
					break
				}
			}
		}
		if !cookieCleared {
			t.Errorf("Session cookie not properly cleared after logout by client jar inspection.")
		}
		
		// Also, check server-side session store (if accessible, or by trying an authenticated route)
		// Here, SessionStore is global in handlers package.
		if _, exists := SessionStore[initialSessionCookie.Value]; exists {
			t.Errorf("Session token for value %s still exists in server-side SessionStore after logout", initialSessionCookie.Value)
		}
	})
}


// Helper to parse URL for cookie jar, fatal on error
func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Failed to parse URL '%s': %v", rawURL, err)
	}
	return u
}

// Helper for unique email generation (alternative to just timestamp)
func strconvFormatInt(i int64) string {
    return strconv.FormatInt(i, 10)
}
