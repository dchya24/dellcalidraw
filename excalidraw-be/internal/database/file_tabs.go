package database

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// FileTab represents a single tab/sheet within a user file
type FileTab struct {
	ID        string
	FileID    string
	TabKey    string
	Title     string
	RoomID    string
	Elements  json.RawMessage
	AppState  json.RawMessage
	FilesData json.RawMessage
	SortOrder int
	CreatedAt string
	UpdatedAt string
}

// CreateFileTab inserts a new tab for a file
func (p *PostgresClient) CreateFileTab(fileID, tabKey, title, roomID string, elements, appState, filesData json.RawMessage, sortOrder int) (*FileTab, error) {
	if elements == nil {
		elements = json.RawMessage(`[]`)
	}
	if appState == nil {
		appState = json.RawMessage(`{}`)
	}
	if filesData == nil {
		filesData = json.RawMessage(`{}`)
	}

	var tab FileTab
	err := p.db.QueryRow(
		`INSERT INTO file_tabs (file_id, tab_key, title, room_id, elements, app_state, files_data, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, file_id, tab_key, title, room_id, elements, app_state, files_data, sort_order, created_at::text, updated_at::text`,
		fileID, tabKey, title, roomID, elements, appState, filesData, sortOrder,
	).Scan(&tab.ID, &tab.FileID, &tab.TabKey, &tab.Title, &tab.RoomID, &tab.Elements, &tab.AppState, &tab.FilesData, &tab.SortOrder, &tab.CreatedAt, &tab.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create file tab: %w", err)
	}
	return &tab, nil
}

// GetFileTabs returns all tabs for a file, ordered by sort_order
func (p *PostgresClient) GetFileTabs(fileID string) ([]FileTab, error) {
	rows, err := p.db.Query(
		`SELECT id, file_id, tab_key, title, room_id, elements, app_state, files_data, sort_order, created_at::text, updated_at::text
		 FROM file_tabs WHERE file_id = $1 ORDER BY sort_order ASC`,
		fileID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file tabs: %w", err)
	}
	defer rows.Close()

	var tabs []FileTab
	for rows.Next() {
		var tab FileTab
		if err := rows.Scan(&tab.ID, &tab.FileID, &tab.TabKey, &tab.Title, &tab.RoomID, &tab.Elements, &tab.AppState, &tab.FilesData, &tab.SortOrder, &tab.CreatedAt, &tab.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan file tab: %w", err)
		}
		tabs = append(tabs, tab)
	}
	return tabs, rows.Err()
}

// UpdateFileTab updates a tab's canvas data
func (p *PostgresClient) UpdateFileTab(tabID string, elements, appState, filesData json.RawMessage) error {
	if elements == nil {
		elements = json.RawMessage(`[]`)
	}
	if appState == nil {
		appState = json.RawMessage(`{}`)
	}
	if filesData == nil {
		filesData = json.RawMessage(`{}`)
	}

	_, err := p.db.Exec(
		`UPDATE file_tabs SET elements = $2, app_state = $3, files_data = $4, updated_at = NOW()
		 WHERE id = $1`,
		tabID, elements, appState, filesData,
	)
	if err != nil {
		return fmt.Errorf("failed to update file tab: %w", err)
	}
	slog.Debug("File tab updated", "tabID", tabID)
	return nil
}

// DeleteFileTabsByFileID deletes all tabs for a file
func (p *PostgresClient) DeleteFileTabsByFileID(fileID string) error {
	_, err := p.db.Exec(`DELETE FROM file_tabs WHERE file_id = $1`, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file tabs: %w", err)
	}
	return nil
}
