package models

import "time"

const (
	RSVPStatusAttending    = "attending"
	RSVPStatusNotAttending = "not_attending"
	RSVPStatusMaybe        = "maybe"
)

type RSVP struct {
	ID        int64
	UserID    int64
	GameID    int64
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	UserEmail string // Optional: For easier display in templates
}
