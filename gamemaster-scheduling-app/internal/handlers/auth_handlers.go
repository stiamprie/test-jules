package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gamemaster-scheduling/app/internal/database"
	"github.com/gamemaster-scheduling/app/internal/models"
	"github.com/google/uuid"
)

// SessionStore holds active session IDs and their corresponding user IDs.
// For POC only. In production, use a persistent store like Redis.
var SessionStore = make(map[string]int64) // Made public

const sessionCookieName = "session_token"

// RegisterPage renders the user registration page.
func RegisterPage(w http.ResponseWriter, r *http.Request) {
	// Assumes LoadTemplates has been called at startup.
	// The key "auth/register.html" must match how it's stored by LoadTemplates.
	RenderTemplate(w, "auth/register.html", nil)
}

// Register handles the user registration form submission.
func Register(db *sql.DB) http.HandlerFunc {
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

		email := r.FormValue("email")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		// Basic validation
		if email == "" || password == "" {
			// Re-render form with error. HTMX can target an error div.
			// For simplicity, just sending a bad request status now.
			// In a real app, you'd pass data back to the template.
			data := map[string]interface{}{"Error": "Email and password are required."}
			// If using HTMX and want to re-render the form part:
			// RenderTemplate(w, "auth/register.html#registration-form-container", data) // Fictional syntax for fragment
			RenderTemplate(w, "auth/register.html", data)
			return
		}

		if password != confirmPassword {
			data := map[string]interface{}{"Error": "Passwords do not match."}
			RenderTemplate(w, "auth/register.html", data)
			return
		}

		// Check if user already exists
		_, err = database.GetUserByEmail(db, email)
		if err == nil { // If err is nil, user was found
			data := map[string]interface{}{"Error": "Email already registered."}
			RenderTemplate(w, "auth/register.html", data)
			return
		}
		if err != sql.ErrNoRows { // Some other database error
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Create user
		_, err = database.CreateUser(db, email, password)
		if err != nil {
			http.Error(w, "Could not create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// For HTMX, if successful, you might want to redirect via a special HTMX header,
		// or return a snippet that indicates success and then the client-side JS redirects.
		// For now, a simple redirect.
		// Consider "HX-Redirect" header for HTMX if you want server-side redirect after AJAX.
		// w.Header().Set("HX-Redirect", "/login") // Example for HTMX
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// LoginPage renders the user login page.
func LoginPage(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, "auth/login.html", nil)
}

// Login handles the user login form submission.
// SessionStore is now passed as an argument for clarity, or can be accessed globally if public.
// For consistency with Logout, let's assume it's passed if handlers are to be more testable.
// However, the main.go spec suggests handlers.SessionStore, so we will use the public global var.
func Login(db *sql.DB) http.HandlerFunc {
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

		email := r.FormValue("email")
		password := r.FormValue("password")

		if email == "" || password == "" {
			data := map[string]interface{}{"Error": "Email and password are required."}
			RenderTemplate(w, "auth/login.html", data)
			return
		}

		user, err := database.GetUserByEmail(db, email)
		if err != nil {
			if err == sql.ErrNoRows {
				data := map[string]interface{}{"Error": "Invalid email or password."}
				RenderTemplate(w, "auth/login.html", data)
			} else {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		err = database.VerifyPassword(user.PasswordHash, password)
		if err != nil { // Password mismatch
			data := map[string]interface{}{"Error": "Invalid email or password."}
			RenderTemplate(w, "auth/login.html", data)
			return
		}

		// Create session
		sessionID, err := uuid.NewRandom()
		if err != nil {
			http.Error(w, "Could not create session ID", http.StatusInternalServerError)
			return
		}
		sessionToken := sessionID.String()
		SessionStore[sessionToken] = user.ID // Use public SessionStore

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionToken,
			Path:     "/",
			Expires:  time.Now().Add(24 * time.Hour), // Example: 24-hour session
			HttpOnly: true,
			Secure:   r.TLS != nil, // Set to true if using HTTPS
			SameSite: http.SameSiteLaxMode,
		})
		
		// For HTMX, you might want to use HX-Redirect
		// w.Header().Set("HX-Redirect", "/games") // Assuming /games is a protected route
		http.Redirect(w, r, "/games", http.StatusSeeOther) // Redirect to a protected area
	}
}

// Logout handles user logout.
// SessionStore is now passed as an argument for clarity, or can be accessed globally if public.
// For consistency with Login, using the public global var.
func Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil { // Cookie exists
		sessionToken := cookie.Value
		delete(SessionStore, sessionToken) // Use public SessionStore

		// Expire the cookie
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0), // Set expiry to the past
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		})
	}
	// Redirect to login or home page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Middleware to protect routes that require authentication.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// If IsAuthenticated, proceed. We could also fetch the user and add to context here.
		next.ServeHTTP(w, r)
	}
}

// IsAuthenticated checks if a user is currently authenticated based on session cookie.
func IsAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil { // No session cookie
		return false
	}
	sessionToken := cookie.Value
	_, ok := SessionStore[sessionToken] // Check if session token is valid in our store
	return ok
}


// GetCurrentUser retrieves the currently authenticated user from the session.
// Returns the User object or an error if not authenticated or user not found.
// db can be nil if only checking authentication status without fetching user details,
// but for GetCurrentUser, db is required.
func GetCurrentUser(r *http.Request, db *sql.DB) (*models.User, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to get current user")
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie: %w", err)
	}
	sessionToken := cookie.Value
	userID, ok := SessionStore[sessionToken]
	if !ok {
		return nil, fmt.Errorf("invalid session token")
	}
	return database.GetUserByID(db, userID)
}
