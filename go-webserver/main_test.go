package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeHTTP(t *testing.T) {
	// Create a new HTTP request for a GET request to "/".
	req := httptest.NewRequest("GET", "/", nil)

	// Create a new HTTP response recorder.
	rr := httptest.NewRecorder()

	// Create a file server handler similar to the one in main.go.
	fs := http.FileServer(http.Dir("./static"))

	// Call the handler's ServeHTTP method with the response recorder and the request.
	fs.ServeHTTP(rr, req)

	// Check if the response status code is http.StatusOK.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check if the response body contains the string "Hello from Go Webserver!".
	expected := "Hello from Go Webserver!"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
