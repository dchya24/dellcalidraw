package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"
)

// PasswordResetToken represents a password reset token
type PasswordResetToken struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// CreatePasswordResetToken creates a new password reset token for a user
func (p *PostgresClient) CreatePasswordResetToken(userID string, expiresIn time.Duration) (*PasswordResetToken, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(expiresIn)

	// Invalidate any existing tokens for this user
	_, _ = p.db.Exec(
		`UPDATE password_reset_tokens SET used_at = NOW() WHERE user_id = $1 AND used_at IS NULL`,
		userID,
	)

	var prt PasswordResetToken
	err := p.db.QueryRow(
		`INSERT INTO password_reset_tokens (user_id, token, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, token, expires_at, created_at`,
		userID, token, expiresAt,
	).Scan(&prt.ID, &prt.UserID, &prt.Token, &prt.ExpiresAt, &prt.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create password reset token: %w", err)
	}

	slog.Info("Password reset token created", "userID", userID)
	return &prt, nil
}

// GetPasswordResetToken retrieves a password reset token by its token string
func (p *PostgresClient) GetPasswordResetToken(token string) (*PasswordResetToken, error) {
	var prt PasswordResetToken
	var usedAt sql.NullTime

	err := p.db.QueryRow(
		`SELECT id, user_id, token, expires_at, used_at, created_at
		 FROM password_reset_tokens WHERE token = $1`,
		token,
	).Scan(&prt.ID, &prt.UserID, &prt.Token, &prt.ExpiresAt, &usedAt, &prt.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get password reset token: %w", err)
	}

	if usedAt.Valid {
		prt.UsedAt = &usedAt.Time
	}

	return &prt, nil
}

// UsePasswordResetToken marks a token as used
func (p *PostgresClient) UsePasswordResetToken(token string) error {
	result, err := p.db.Exec(
		`UPDATE password_reset_tokens SET used_at = NOW() WHERE token = $1 AND used_at IS NULL`,
		token,
	)
	if err != nil {
		return fmt.Errorf("failed to use password reset token: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("token not found or already used")
	}

	return nil
}

// UpdateUserPassword updates a user's password
func (p *PostgresClient) UpdateUserPassword(userID, hashedPassword string) error {
	result, err := p.db.Exec(
		`UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2`,
		hashedPassword, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("user not found")
	}

	slog.Info("User password updated", "userID", userID)
	return nil
}

// CleanExpiredPasswordResetTokens removes expired or used tokens
func (p *PostgresClient) CleanExpiredPasswordResetTokens() (int, error) {
	result, err := p.db.Exec(
		`DELETE FROM password_reset_tokens WHERE expires_at < NOW() OR used_at IS NOT NULL`,
	)
	if err != nil {
		return 0, err
	}
	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}
