package database

import (
	"database/sql"
	"time"

	"github.com/gamemaster-scheduling/app/internal/models"
)

// CreateGame inserts a new game into the games table.
func CreateGame(db *sql.DB, game *models.Game) (*models.Game, error) {
	stmt, err := db.Prepare("INSERT INTO games(gm_id, title, description, game_datetime, location) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// Ensure GameDateTime is in a format SQLite understands, or use Unix timestamp.
	// SQLite typically handles "YYYY-MM-DD HH:MM:SS" format well.
	res, err := stmt.Exec(game.GMID, game.Title, game.Description, game.GameDateTime, game.Location)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Retrieve the game to get all fields populated, including DB defaults like created_at and the ID.
	// This ensures the returned Game object is complete.
	return GetGameByID(db, id)
}

// GetGameByID retrieves a game by its ID.
func GetGameByID(db *sql.DB, id int64) (*models.Game, error) {
	game := &models.Game{}
	row := db.QueryRow("SELECT id, gm_id, title, description, game_datetime, location, created_at FROM games WHERE id = ?", id)
	err := row.Scan(&game.ID, &game.GMID, &game.Title, &game.Description, &game.GameDateTime, &game.Location, &game.CreatedAt)
	if err != nil {
		return nil, err // This will include sql.ErrNoRows if not found
	}
	return game, nil
}

// GetAllGames retrieves all games, ordered by game_datetime descending.
func GetAllGames(db *sql.DB) ([]*models.Game, error) {
	rows, err := db.Query("SELECT id, gm_id, title, description, game_datetime, location, created_at FROM games ORDER BY game_datetime DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		game := &models.Game{}
		err := rows.Scan(&game.ID, &game.GMID, &game.Title, &game.Description, &game.GameDateTime, &game.Location, &game.CreatedAt)
		if err != nil {
			return nil, err // Or collect errors and continue
		}
		games = append(games, game)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return games, nil
}
