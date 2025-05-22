package models

import "time"

type ChatMessage struct {
	ID             int64
	GameID         int64
	UserID         int64
	UserEmail      string // For display
	MessageContent string
	CreatedAt      time.Time
}
