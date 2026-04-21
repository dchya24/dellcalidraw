package database

import (
	"fmt"
	"log/slog"
)

type FileRecord struct {
	ID       string
	RoomID   string
	FileID   string
	MimeType string
	Size     int64
	Key      string
}

func (p *PostgresClient) SaveFileRecord(roomDBID, fileID, mimeType string, size int64, storageKey string) error {
	_, err := p.db.Exec(
		`INSERT INTO room_files (room_id, file_id, mime_type, size, storage_key, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (room_id, file_id)
		 DO UPDATE SET mime_type = $3, size = $4, storage_key = $5, created_at = NOW()`,
		roomDBID, fileID, mimeType, size, storageKey,
	)
	if err != nil {
		return fmt.Errorf("failed to save file record: %w", err)
	}
	slog.Info("File record saved", "fileID", fileID, "roomDBID", roomDBID)
	return nil
}

func (p *PostgresClient) GetFileRecord(roomDBID, fileID string) (*FileRecord, error) {
	var rec FileRecord
	err := p.db.QueryRow(
		`SELECT room_id, file_id, mime_type, size, storage_key
		 FROM room_files WHERE room_id = $1 AND file_id = $2`,
		roomDBID, fileID,
	).Scan(&rec.RoomID, &rec.FileID, &rec.MimeType, &rec.Size, &rec.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to get file record: %w", err)
	}
	return &rec, nil
}

func (p *PostgresClient) DeleteFileRecord(roomDBID, fileID string) error {
	_, err := p.db.Exec(
		`DELETE FROM room_files WHERE room_id = $1 AND file_id = $2`,
		roomDBID, fileID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}
	slog.Info("File record deleted", "fileID", fileID, "roomDBID", roomDBID)
	return nil
}

func (p *PostgresClient) ListFileRecords(roomDBID string) ([]FileRecord, error) {
	rows, err := p.db.Query(
		`SELECT room_id, file_id, mime_type, size, storage_key
		 FROM room_files WHERE room_id = $1 ORDER BY created_at DESC`,
		roomDBID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list file records: %w", err)
	}
	defer rows.Close()

	var records []FileRecord
	for rows.Next() {
		var rec FileRecord
		if err := rows.Scan(&rec.RoomID, &rec.FileID, &rec.MimeType, &rec.Size, &rec.Key); err != nil {
			return nil, fmt.Errorf("failed to scan file record: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
