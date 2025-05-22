# Game Master Scheduling App - Proof of Concept

A web application to help tabletop gamers schedule their gaming sessions with ease. This is a Proof of Concept (POC) demonstrating core functionalities.

## Features

*   **User Authentication**: Secure user registration, login, and logout.
*   **Game Creation**: Game Masters (GMs) can create new game sessions, providing details like title, description, date/time, and location (physical or virtual).
*   **Game Listings**: Users can view a list of all scheduled games.
*   **Game Details**: Users can view detailed information for a specific game.
*   **RSVP Functionality**: Logged-in users can RSVP to games (Attending, Maybe, Not Attending). RSVP status updates dynamically on the page.
*   **Basic Chat**: A real-time chat feature per game session for communication between participants. Chat messages update dynamically.
*   **HTMX-Powered UI**: Frontend interactions (forms, RSVPs, chat) are enhanced with HTMX for partial page updates, providing a smoother user experience without full page reloads.

## Technology Stack

*   **Backend**: Go (Golang)
    *   Standard library `net/http` for HTTP handling and routing.
    *   `golang.org/x/crypto/bcrypt` for password hashing.
*   **Database**: SQLite 3 (single file `scheduler.db`)
    *   `mattn/go-sqlite3` driver.
*   **Frontend**:
    *   HTML5
    *   HTMX for dynamic UI updates.
    *   Basic CSS for styling.
*   **Testing**:
    *   Go standard library `testing`.
    *   `net/http/httptest` for handler integration tests.

## Prerequisites

*   Go (version 1.21 or newer recommended, though the current implementation is compatible with older versions like 1.18+). Download and install from [golang.org](https://golang.org/dl/).
*   Git for cloning the repository.

## Running Locally

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd gamemaster-scheduling-app
    ```
    (Replace `<repository-url>` with the actual URL of the repository. If working within an environment that already has the code, this step is just for context).

2.  **Tidy dependencies (optional, if you want to ensure `go.mod` and `go.sum` are pristine):**
    ```bash
    go mod tidy
    ```

3.  **Run the application:**
    ```bash
    go run cmd/server/main.go
    ```
    This will compile and run the `main.go` application.

4.  **Access the application:**
    Open your web browser and go to `http://localhost:8080`. The default port is `8080`.

5.  **Database:**
    *   The application uses SQLite, and the database file (`scheduler.db`) will be created in the root directory of the project (`gamemaster-scheduling-app`) when the application starts and the first database operation occurs.

## Running Tests

To run the unit and integration tests:

```bash
go test ./...
```
This command will run all tests in the current directory and its subdirectories.

## Deployment Considerations (Cloud Hosted Server)

Deploying a Go web application like this to a cloud server (e.g., AWS EC2, Google Cloud Run, DigitalOcean Droplets, Heroku) involves several considerations:

1.  **Build a Binary:**
    Compile your application into a self-contained executable:
    ```bash
    GOOS=linux GOARCH=amd64 go build -o game-scheduler-app cmd/server/main.go
    ```
    Adjust `GOOS` and `GOARCH` for your target environment. Upload this binary to your server.

2.  **Environment Variables:**
    *   **`PORT`**: The application respects the `PORT` environment variable. Cloud platforms often set this.
    *   **Database Path**: Consider making the `scheduler.db` path configurable via an environment variable for flexibility.

3.  **SQLite on Cloud Platforms:**
    *   **File System Persistence**: Ensure your server's file system is persistent. Ephemeral systems might lose the `scheduler.db` file. Consider managed databases for critical persistence or if SQLite limitations are an issue.
    *   **Backups**: Implement a backup strategy for `scheduler.db`.
    *   **Concurrency**: SQLite has serialized writes. For high write loads, this could be a bottleneck.

4.  **Serving Static Files:**
    *   The app serves static files directly. For larger scale, consider a reverse proxy (Nginx, Caddy) or CDN.

5.  **Process Management:**
    *   Use `systemd`, `supervisor`, or Docker to keep the application running as a service.

6.  **CORS (Cross-Origin Resource Sharing):**
    *   Not implemented. Required if accessing from different domains/frontends.

## Directory Structure

```
gamemaster-scheduling-app/
├── cmd/server/main.go        # Main application entry point
├── internal/                 # Internal application logic
│   ├── database/             # Database interaction logic, schema
│   ├── handlers/             # HTTP handlers
│   └── models/               # Data models (structs)
├── web/
│   ├── static/css/style.css  # CSS
│   └── templates/            # HTML templates (layout, pages, partials)
├── go.mod                    # Go module definition
├── go.sum                    # Go module checksums
├── scheduler.db              # SQLite database file (created at runtime)
└── README.md                 # This file
```
