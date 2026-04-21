package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

type User struct {
	ID        string
	Username  string
	Email     string
	Password  string
	AvatarURL string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
	Revoked   bool
}

func (p *PostgresClient) CreateUser(username, email, hashedPassword string) (*User, error) {
	var user User
	err := p.db.QueryRow(
		`INSERT INTO users (username, email, password) VALUES ($1, $2, $3)
		 RETURNING id, username, email, password, avatar_url, created_at, updated_at`,
		username, email, hashedPassword,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	slog.Info("User created", "username", username, "email", email)
	return &user, nil
}

func (p *PostgresClient) GetUserByEmail(email string) (*User, error) {
	var user User
	err := p.db.QueryRow(
		`SELECT id, username, email, password, avatar_url, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

func (p *PostgresClient) GetUserByUsername(username string) (*User, error) {
	var user User
	err := p.db.QueryRow(
		`SELECT id, username, email, password, avatar_url, created_at, updated_at
		 FROM users WHERE username = $1`, username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return &user, nil
}

func (p *PostgresClient) GetUserByID(id string) (*User, error) {
	var user User
	err := p.db.QueryRow(
		`SELECT id, username, email, password, avatar_url, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &user, nil
}

func (p *PostgresClient) UpdateUserProfile(id, username, avatarURL string) (*User, error) {
	var user User
	err := p.db.QueryRow(
		`UPDATE users SET username = $1, avatar_url = $2, updated_at = NOW()
		 WHERE id = $3
		 RETURNING id, username, email, password, avatar_url, created_at, updated_at`,
		username, avatarURL, id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}
	slog.Info("User profile updated", "userID", id)
	return &user, nil
}

func (p *PostgresClient) SaveRefreshToken(userID, token string, expiresAt time.Time) error {
	_, err := p.db.Exec(
		`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, token, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}

func (p *PostgresClient) GetRefreshToken(token string) (*RefreshToken, error) {
	var rt RefreshToken
	err := p.db.QueryRow(
		`SELECT id, user_id, token, expires_at, created_at, revoked
		 FROM refresh_tokens WHERE token = $1`, token,
	).Scan(&rt.ID, &rt.UserID, &rt.Token, &rt.ExpiresAt, &rt.CreatedAt, &rt.Revoked)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &rt, nil
}

func (p *PostgresClient) RevokeRefreshToken(token string) error {
	_, err := p.db.Exec(
		`UPDATE refresh_tokens SET revoked = TRUE WHERE token = $1`, token,
	)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (p *PostgresClient) RevokeAllUserTokens(userID string) error {
	_, err := p.db.Exec(
		`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1`, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", err)
	}
	slog.Info("All refresh tokens revoked for user", "userID", userID)
	return nil
}

func (p *PostgresClient) CleanExpiredTokens() (int, error) {
	result, err := p.db.Exec(
		`DELETE FROM refresh_tokens WHERE expires_at < NOW() OR revoked = TRUE`,
	)
	if err != nil {
		return 0, err
	}
	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}
