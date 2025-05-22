package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Template helper functions
var funcMap = template.FuncMap{
	"FormatDateTime": FormatDateTime,
	"Nl2br":          Nl2br,
	"TitleCase":      TitleCase,
}

// TitleCase converts a string to title case.
// e.g., "not_attending" -> "Not Attending"
func TitleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Title(s)
}

// FormatDateTime formats a time.Time object into a more readable string.
func FormatDateTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	// Example format: "January 2, 2006 at 3:04 PM"
	// You can customize this format as needed.
	return t.Format("January 2, 2006 at 3:04 PM")
}

// Nl2br replaces newline characters with <br> tags.
func Nl2br(s string) template.HTML {
	return template.HTML(strings.ReplaceAll(s, "\n", "<br>"))
}


// templates holds all parsed templates.
// The key is the template name relative to the templates directory
// e.g., "auth/login.html" or "games/index.html"
var (
	templates     map[string]*template.Template
	templatesOnce sync.Once
	templatesDir  string
)

// LoadTemplates parses all HTML templates from the given directory and its subdirectories.
// It should be called once at application startup.
func LoadTemplates(dir string) error {
	templatesDir = dir
	var loadErr error
	templatesOnce.Do(func() {
		templates = make(map[string]*template.Template)
		layoutFile := filepath.Join(dir, "layout.html")

		// Check if layout file exists
		if _, err := os.Stat(layoutFile); os.IsNotExist(err) {
			loadErr = fmt.Errorf("layout.html not found in %s", dir)
			return
		}

		// Find all partial files (e.g., _header.html, _rsvp_section.html)
		partialFiles, err := filepath.Glob(filepath.Join(dir, "**/_*.html"))
		if err != nil {
			loadErr = fmt.Errorf("error globbing partial templates: %w", err)
			return
		}

		// Find all page template files (excluding layout and partials)
		allFiles, err := filepath.Glob(filepath.Join(dir, "**/*.html"))
		if err != nil {
			loadErr = fmt.Errorf("error globbing all templates: %w", err)
			return
		}

		pageFiles := []string{}
		for _, file := range allFiles {
			isLayout := (file == layoutFile)
			isPartial := false
			for _, pf := range partialFiles {
				if file == pf {
					isPartial = true
					break
				}
			}
			if !isLayout && !isPartial {
				pageFiles = append(pageFiles, file)
			}
		}
		
		if len(pageFiles) == 0 && len(partialFiles) == 0 {
			// This might be an issue if layout.html is the only file, but typically pages are expected.
			// If only layout.html exists, it won't be processed by the loops below.
			// However, layout.html is not meant to be rendered directly.
			fmt.Printf("Warning: No page or partial template files found in %s, only layout.html perhaps?\n", dir)
		}


		// Parse page templates (which include layout and partials)
		for _, pageFile := range pageFiles {
			name := strings.TrimPrefix(pageFile, dir+string(filepath.Separator))
			name = filepath.ToSlash(name) // Use relative path as template name

			// All files to parse for a page template: the page itself, the layout, and all partials
			filesToParse := append([]string{pageFile, layoutFile}, partialFiles...)
			
			// Create a new template with the page's name (e.g., "auth/login.html")
			// This name is used when calling Execute() on the template.
			// The template definitions within the files (e.g. {{define "layout"}}, {{define "content"}})
			// are associated with this named template set.
			tmpl, parseErr := template.New(name).Funcs(funcMap).ParseFiles(filesToParse...)
			if parseErr != nil {
				loadErr = fmt.Errorf("error parsing page template %s with layout and partials: %w", name, parseErr)
				return
			}
			templates[name] = tmpl
			// fmt.Printf("Loaded page template: %s\n", name)
		}

		// Parse partial templates standalone (they don't use the main layout)
		for _, partialFile := range partialFiles {
			name := strings.TrimPrefix(partialFile, dir+string(filepath.Separator))
			name = filepath.ToSlash(name) // Use relative path as template name

			// Partials are named by their file name (e.g., "games/_rsvp_section.html")
			// but they also define a template, often matching their base name.
			// template.New() here should ideally use the base name if ExecuteTemplate is to be called with it.
			// Or, the template name for Execute() should be the relative path.
			// Let's assume templates[name].Execute(w, data) will execute the primary definition in the partial.
			tmpl, parseErr := template.New(name).Funcs(funcMap).ParseFiles(partialFile)
			if parseErr != nil {
				loadErr = fmt.Errorf("error parsing partial template %s: %w", name, parseErr)
				return
			}
			templates[name] = tmpl
			// fmt.Printf("Loaded partial template: %s\n", name)
		}
	})
	return loadErr
}

// RenderErrorPage renders a standardized error page using the error.html template.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, db *sql.DB, statusCode int, title string, message string) {
	w.WriteHeader(statusCode)
	
	// Get current year for the footer
	currentYear := time.Now().Year()

	// Check authentication status and get current user for the layout
	// db might be nil if the error occurs before DB is initialized.
	// The GetCurrentUser and IsAuthenticated functions should handle db == nil if they are called.
	// However, GetCurrentUser as modified requires a non-nil db.
	// For error pages, we might not always have a DB connection available.
	// Let's make GetCurrentUser more resilient or only pass user if db is available.
	
	var currentUser *models.User
	isAuthenticated := false // Assume not authenticated if DB is nil or error occurs

	if db != nil {
		isAuthenticated = IsAuthenticated(r) // IsAuthenticated doesn't need DB
		if isAuthenticated {
			// Only try to get user if authenticated and DB is available
			var err error
			currentUser, err = GetCurrentUser(r, db)
			if err != nil {
				// Log error but don't fail error page rendering
				fmt.Printf("RenderErrorPage: could not get current user: %v\n", err)
				// currentUser will remain nil
			}
		}
	}


	data := map[string]interface{}{
		"Title":        fmt.Sprintf("Error %d - %s", statusCode, title), // Page Title
		"StatusCode":   statusCode,
		"StatusText":   http.StatusText(statusCode),
		"ErrorTitle":   title, // More specific title for the error content area
		"Message":      message,
		"User":         currentUser,       // For layout navbar
		"CurrentYear":  currentYear,       // For layout footer
		// IsAuthenticated is implicitly handled by whether .User is nil in layout.
	}
	
	// Ensure error.html is loaded by LoadTemplates.
	// It should be treated as a "page" template that uses the layout.
	RenderTemplate(w, "error.html", data)
}


// RenderTemplate executes the named template.
// For full pages, 'name' is the path like "auth/login.html".
// For partials (like "_rsvp_section.html"), 'name' is also its path.
func RenderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := templates[name]
	if !ok {
		http.Error(w, fmt.Sprintf("Template not found: %s. Available: %v", name, getTemplateKeys()), http.StatusInternalServerError)
		return
	}

	// For full page templates, Execute() will render the template named after the page file (e.g. "auth/login.html"),
	// which then calls {{template "layout" .}}.
	// For partials, Execute() will render the primary template defined in that partial file.
	err := tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing template %s: %s", name, err.Error()), http.StatusInternalServerError)
	}
}

func getTemplateKeys() []string {
	keys := make([]string, 0, len(templates))
	for k := range templates {
		keys = append(keys, k)
	}
	return keys
}
