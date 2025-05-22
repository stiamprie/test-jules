package models

import "time"

type Game struct {
	ID           int64
	GMID         int64
	Title        string
	Description  string
	GameDateTime time.Time
	Location     string
	CreatedAt    time.Time
	// Optional: Add GMUsername string if you want to easily display it,
	// otherwise you'll need to join or do a separate query. For POC, keep it simple.
}
