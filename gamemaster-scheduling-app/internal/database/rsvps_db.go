package database

import (
	"database/sql"
	"time"

	"github.com/gamemaster-scheduling/app/internal/models"
)

// CreateOrUpdateRSVP inserts a new RSVP or updates an existing one.
// It uses SQLite's "ON CONFLICT" clause to handle the upsert.
func CreateOrUpdateRSVP(db *sql.DB, rsvp *models.RSVP) error {
	stmt, err := db.Prepare(`
		INSERT INTO rsvps (user_id, game_id, status, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, game_id) DO UPDATE SET
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(rsvp.UserID, rsvp.GameID, rsvp.Status)
	return err
}

// GetRSVPsForGame retrieves all RSVPs for a given game, including the user's email.
func GetRSVPsForGame(db *sql.DB, gameID int64) ([]*models.RSVP, error) {
	rows, err := db.Query(`
		SELECT r.id, r.user_id, r.game_id, r.status, r.created_at, r.updated_at, u.email
		FROM rsvps r
		JOIN users u ON r.user_id = u.id
		WHERE r.game_id = ?
		ORDER BY r.updated_at DESC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rsvps []*models.RSVP
	for rows.Next() {
		rsvp := &models.RSVP{}
		err := rows.Scan(&rsvp.ID, &rsvp.UserID, &rsvp.GameID, &rsvp.Status, &rsvp.CreatedAt, &rsvp.UpdatedAt, &rsvp.UserEmail)
		if err != nil {
			return nil, err
		}
		rsvps = append(rsvps, rsvp)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return rsvps, nil
}

// GetRSVPByUserForGame retrieves a specific user's RSVP for a specific game.
func GetRSVPByUserForGame(db *sql.DB, userID int64, gameID int64) (*models.RSVP, error) {
	rsvp := &models.RSVP{}
	// We can also join with users table here if UserEmail is needed, though it's less critical for this specific function
	// if its primary use is just to check status for the current user.
	// For consistency and if UserEmail might be useful on the RSVP object returned, let's include it.
	row := db.QueryRow(`
		SELECT r.id, r.user_id, r.game_id, r.status, r.created_at, r.updated_at, u.email
		FROM rsvps r
		JOIN users u ON r.user_id = u.id
		WHERE r.user_id = ? AND r.game_id = ?
	`, userID, gameID)

	err := row.Scan(&rsvp.ID, &rsvp.UserID, &rsvp.GameID, &rsvp.Status, &rsvp.CreatedAt, &rsvp.UpdatedAt, &rsvp.UserEmail)
	if err != nil {
		return nil, err // This will include sql.ErrNoRows if not found
	}
	return rsvp, nil
}
