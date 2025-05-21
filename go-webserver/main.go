package main

import (
	"log"
	"net/http"
)

func main() {
	// Create a file server handler that serves files from the "static" directory.
	fs := http.FileServer(http.Dir("./static"))

	// Register the file server handler to serve requests for the root path ("/").
	http.Handle("/", fs)

	// Print a message to the console indicating that the server is starting on port 8080.
	log.Println("Starting server on :8080")

	// Start the HTTP server on port 8080 and log any errors.
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
