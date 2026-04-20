package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

type ElementUpdate struct {
	ElementID string
	Data      json.RawMessage
	Version   int
}

func (p *PostgresClient) CreateRoom(roomKey, roomName string) (string, error) {
	var id string
	err := p.db.QueryRow(
		`INSERT INTO rooms (key, name) VALUES ($1, $2) RETURNING id`,
		roomKey, roomName,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create room: %w", err)
	}
	slog.Info("Room created in database", "key", roomKey, "id", id)
	return id, nil
}

func (p *PostgresClient) GetRoomByKey(roomKey string) (string, error) {
	var id string
	err := p.db.QueryRow(
		`SELECT id FROM rooms WHERE key = $1`, roomKey,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func (p *PostgresClient) GetOrCreateRoom(roomKey string) (string, error) {
	id, err := p.GetRoomByKey(roomKey)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	return p.CreateRoom(roomKey, "")
}

func (p *PostgresClient) BatchSaveElements(roomDBID string, updates []ElementUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO room_elements (room_id, element_id, data, version, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (room_id, element_id)
		DO UPDATE SET data = $3, version = $4, updated_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, u := range updates {
		_, err := stmt.Exec(roomDBID, u.ElementID, u.Data, u.Version)
		if err != nil {
			return fmt.Errorf("failed to save element %s: %w", u.ElementID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Debug("Batch saved elements", "roomDBID", roomDBID, "count", len(updates))
	return nil
}

func (p *PostgresClient) DeleteElements(roomDBID string, elementIDs []string) error {
	if len(elementIDs) == 0 {
		return nil
	}

	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		DELETE FROM room_elements WHERE room_id = $1 AND element_id = $2
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare delete statement: %w", err)
	}
	defer stmt.Close()

	for _, id := range elementIDs {
		_, err := stmt.Exec(roomDBID, id)
		if err != nil {
			return fmt.Errorf("failed to delete element %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete transaction: %w", err)
	}

	slog.Debug("Deleted elements from database", "roomDBID", roomDBID, "count", len(elementIDs))
	return nil
}

func (p *PostgresClient) GetRawElements(roomDBID string) ([]json.RawMessage, error) {
	rows, err := p.db.Query(
		`SELECT data FROM room_elements WHERE room_id = $1`, roomDBID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query elements: %w", err)
	}
	defer rows.Close()

	elements := make([]json.RawMessage, 0)
	for rows.Next() {
		var data json.RawMessage
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan element: %w", err)
		}
		elements = append(elements, data)
	}

	return elements, rows.Err()
}

func (p *PostgresClient) SaveAllElementsRaw(roomDBID string, elements []json.RawMessage) error {
	if len(elements) == 0 {
		return nil
	}

	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM room_elements WHERE room_id = $1`, roomDBID)
	if err != nil {
		return fmt.Errorf("failed to clear elements: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO room_elements (room_id, element_id, data, version, updated_at)
		VALUES ($1, $2, $3, 1, NOW())
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, raw := range elements {
		var partial struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &partial); err != nil {
			continue
		}
		if partial.ID == "" {
			continue
		}
		_, err = stmt.Exec(roomDBID, partial.ID, raw)
		if err != nil {
			return fmt.Errorf("failed to insert element %s: %w", partial.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	slog.Debug("Saved all elements (raw)", "roomDBID", roomDBID, "count", len(elements))
	return nil
}

func (p *PostgresClient) UpdateRoomActivity(roomDBID string) error {
	_, err := p.db.Exec(
		`UPDATE rooms SET updated_at = NOW() WHERE id = $1`, roomDBID,
	)
	return err
}

func (p *PostgresClient) DeleteRoom(roomDBID string) error {
	_, err := p.db.Exec(`DELETE FROM rooms WHERE id = $1`, roomDBID)
	return err
}

func (p *PostgresClient) DeleteOldRooms(olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := p.db.Exec(
		`DELETE FROM rooms WHERE updated_at < $1 AND id NOT IN (
			SELECT DISTINCT room_id FROM room_elements
		)`, cutoff,
	)
	if err != nil {
		return 0, err
	}
	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}
