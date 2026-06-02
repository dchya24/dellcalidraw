package database

import (
	"fmt"
	"log/slog"
)

// UserFile represents a user's file with metadata
type UserFile struct {
	ID        string
	UserID    string
	Name      string
	TabCount  int
	CreatedAt string
	UpdatedAt string
}

func (p *PostgresClient) CreateUserFile(userID, name string) (*UserFile, error) {
	var file UserFile
	err := p.db.QueryRow(
		`INSERT INTO user_files (user_id, name, tab_count)
		 VALUES ($1, $2, 1)
		 RETURNING id, user_id, name, tab_count, created_at::text, updated_at::text`,
		userID, name,
	).Scan(&file.ID, &file.UserID, &file.Name, &file.TabCount, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user file: %w", err)
	}
	slog.Info("User file created", "fileID", file.ID, "userID", userID)
	return &file, nil
}

func (p *PostgresClient) GetUserFiles(userID string) ([]UserFile, error) {
	rows, err := p.db.Query(
		`SELECT id, user_id, name, tab_count, created_at::text, updated_at::text
		 FROM user_files WHERE user_id = $1 ORDER BY updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user files: %w", err)
	}
	defer rows.Close()

	var files []UserFile
	for rows.Next() {
		var file UserFile
		if err := rows.Scan(&file.ID, &file.UserID, &file.Name, &file.TabCount, &file.CreatedAt, &file.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user file: %w", err)
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (p *PostgresClient) GetUserFile(fileID string) (*UserFile, error) {
	var file UserFile
	err := p.db.QueryRow(
		`SELECT id, user_id, name, tab_count, created_at::text, updated_at::text
		 FROM user_files WHERE id = $1`,
		fileID,
	).Scan(&file.ID, &file.UserID, &file.Name, &file.TabCount, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user file: %w", err)
	}
	return &file, nil
}

func (p *PostgresClient) UpdateUserFile(fileID, name string, tabCount int) (*UserFile, error) {
	var file UserFile
	err := p.db.QueryRow(
		`UPDATE user_files SET name = $2, tab_count = $3, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, user_id, name, tab_count, created_at::text, updated_at::text`,
		fileID, name, tabCount,
	).Scan(&file.ID, &file.UserID, &file.Name, &file.TabCount, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update user file: %w", err)
	}
	slog.Info("User file updated", "fileID", fileID)
	return &file, nil
}

func (p *PostgresClient) DeleteUserFile(fileID string) error {
	result, err := p.db.Exec(`DELETE FROM user_files WHERE id = $1`, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete user file: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("file not found: %s", fileID)
	}
	slog.Info("User file deleted", "fileID", fileID)
	return nil
}

func (p *PostgresClient) RenameUserFile(fileID, newName string) error {
	_, err := p.db.Exec(
		`UPDATE user_files SET name = $2, updated_at = NOW() WHERE id = $1`,
		fileID, newName,
	)
	if err != nil {
		return fmt.Errorf("failed to rename user file: %w", err)
	}
	slog.Info("User file renamed", "fileID", fileID, "newName", newName)
	return nil
}
