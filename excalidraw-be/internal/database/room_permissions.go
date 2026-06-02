package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// RoomRole represents user roles in a room
type RoomRole string

const (
	RoleOwner  RoomRole = "owner"
	RoleEditor RoomRole = "editor"
	RoleViewer RoomRole = "viewer"
)

// RoomSettings contains room configuration
type RoomSettings struct {
	OwnerID        *string `json:"ownerId,omitempty"`
	HasPassword    bool    `json:"hasPassword"`
	IsPublic       bool    `json:"isPublic"`
	AllowAnonymous bool    `json:"allowAnonymous"`
}

// RoomMember represents a user's membership in a room
type RoomMember struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"roomId"`
	UserID    string    `json:"userId"`
	Username  string    `json:"username,omitempty"`
	Email     string    `json:"email,omitempty"`
	Role      RoomRole  `json:"role"`
	InvitedBy *string   `json:"invitedBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RoomInvitation represents a pending room invitation
type RoomInvitation struct {
	ID        string     `json:"id"`
	RoomID    string     `json:"roomId"`
	Email     string     `json:"email,omitempty"`
	Role      RoomRole   `json:"role"`
	Token     string     `json:"token"`
	InvitedBy *string    `json:"invitedBy,omitempty"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// SetRoomPassword sets or updates the room password
func (p *PostgresClient) SetRoomPassword(roomDBID string, password string) error {
	var passwordHash *string
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		hashStr := string(hash)
		passwordHash = &hashStr
	}

	_, err := p.db.Exec(
		`UPDATE rooms SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
		passwordHash, roomDBID,
	)
	return err
}

// VerifyRoomPassword checks if the provided password matches the room password
func (p *PostgresClient) VerifyRoomPassword(roomDBID string, password string) (bool, error) {
	var passwordHash sql.NullString
	err := p.db.QueryRow(
		`SELECT password_hash FROM rooms WHERE id = $1`, roomDBID,
	).Scan(&passwordHash)
	if err != nil {
		return false, err
	}

	// No password set
	if !passwordHash.Valid || passwordHash.String == "" {
		return true, nil
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(password))
	return err == nil, nil
}

// HasRoomPassword checks if a room has a password set
func (p *PostgresClient) HasRoomPassword(roomDBID string) (bool, error) {
	var passwordHash sql.NullString
	err := p.db.QueryRow(
		`SELECT password_hash FROM rooms WHERE id = $1`, roomDBID,
	).Scan(&passwordHash)
	if err != nil {
		return false, err
	}
	return passwordHash.Valid && passwordHash.String != "", nil
}

// RemoveRoomPassword removes the password from a room
func (p *PostgresClient) RemoveRoomPassword(roomDBID string) error {
	_, err := p.db.Exec(
		`UPDATE rooms SET password_hash = NULL, updated_at = NOW() WHERE id = $1`,
		roomDBID,
	)
	return err
}

// SetRoomOwner sets the owner of a room
func (p *PostgresClient) SetRoomOwner(roomDBID string, userID string) error {
	_, err := p.db.Exec(
		`UPDATE rooms SET owner_id = $1, updated_at = NOW() WHERE id = $2`,
		userID, roomDBID,
	)
	return err
}

// GetRoomOwner returns the owner ID of a room
func (p *PostgresClient) GetRoomOwner(roomDBID string) (*string, error) {
	var ownerID sql.NullString
	err := p.db.QueryRow(
		`SELECT owner_id FROM rooms WHERE id = $1`, roomDBID,
	).Scan(&ownerID)
	if err != nil {
		return nil, err
	}
	if !ownerID.Valid {
		return nil, nil
	}
	return &ownerID.String, nil
}

// GetRoomSettings returns the settings for a room
func (p *PostgresClient) GetRoomSettings(roomDBID string) (*RoomSettings, error) {
	var ownerID sql.NullString
	var passwordHash sql.NullString
	var isPublic, allowAnonymous bool

	err := p.db.QueryRow(
		`SELECT owner_id, password_hash, COALESCE(is_public, true), COALESCE(allow_anonymous, true) 
		 FROM rooms WHERE id = $1`, roomDBID,
	).Scan(&ownerID, &passwordHash, &isPublic, &allowAnonymous)
	if err != nil {
		return nil, err
	}

	settings := &RoomSettings{
		HasPassword:    passwordHash.Valid && passwordHash.String != "",
		IsPublic:       isPublic,
		AllowAnonymous: allowAnonymous,
	}
	if ownerID.Valid {
		settings.OwnerID = &ownerID.String
	}

	return settings, nil
}

// UpdateRoomSettings updates room settings
func (p *PostgresClient) UpdateRoomSettings(roomDBID string, isPublic, allowAnonymous bool) error {
	_, err := p.db.Exec(
		`UPDATE rooms SET is_public = $1, allow_anonymous = $2, updated_at = NOW() WHERE id = $3`,
		isPublic, allowAnonymous, roomDBID,
	)
	return err
}

// AddRoomMember adds a user as a member of a room
func (p *PostgresClient) AddRoomMember(roomDBID, userID string, role RoomRole, invitedBy *string) error {
	_, err := p.db.Exec(
		`INSERT INTO room_members (room_id, user_id, role, invited_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (room_id, user_id) DO UPDATE SET role = $3, updated_at = NOW()`,
		roomDBID, userID, role, invitedBy,
	)
	return err
}

// GetRoomMember returns a user's membership in a room
func (p *PostgresClient) GetRoomMember(roomDBID, userID string) (*RoomMember, error) {
	var member RoomMember
	var invitedBy sql.NullString

	err := p.db.QueryRow(
		`SELECT rm.id, rm.room_id, rm.user_id, u.username, u.email, rm.role, rm.invited_by, rm.created_at, rm.updated_at
		 FROM room_members rm
		 JOIN users u ON rm.user_id = u.id
		 WHERE rm.room_id = $1 AND rm.user_id = $2`,
		roomDBID, userID,
	).Scan(&member.ID, &member.RoomID, &member.UserID, &member.Username, &member.Email,
		&member.Role, &invitedBy, &member.CreatedAt, &member.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if invitedBy.Valid {
		member.InvitedBy = &invitedBy.String
	}

	return &member, nil
}

// GetRoomMembers returns all members of a room
func (p *PostgresClient) GetRoomMembers(roomDBID string) ([]RoomMember, error) {
	rows, err := p.db.Query(
		`SELECT rm.id, rm.room_id, rm.user_id, u.username, u.email, rm.role, rm.invited_by, rm.created_at, rm.updated_at
		 FROM room_members rm
		 JOIN users u ON rm.user_id = u.id
		 WHERE rm.room_id = $1
		 ORDER BY rm.created_at ASC`,
		roomDBID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []RoomMember
	for rows.Next() {
		var member RoomMember
		var invitedBy sql.NullString

		err := rows.Scan(&member.ID, &member.RoomID, &member.UserID, &member.Username, &member.Email,
			&member.Role, &invitedBy, &member.CreatedAt, &member.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if invitedBy.Valid {
			member.InvitedBy = &invitedBy.String
		}

		members = append(members, member)
	}

	return members, rows.Err()
}

// UpdateRoomMemberRole updates a member's role in a room
func (p *PostgresClient) UpdateRoomMemberRole(roomDBID, userID string, role RoomRole) error {
	result, err := p.db.Exec(
		`UPDATE room_members SET role = $1, updated_at = NOW() WHERE room_id = $2 AND user_id = $3`,
		role, roomDBID, userID,
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("member not found")
	}

	return nil
}

// RemoveRoomMember removes a user from a room
func (p *PostgresClient) RemoveRoomMember(roomDBID, userID string) error {
	_, err := p.db.Exec(
		`DELETE FROM room_members WHERE room_id = $1 AND user_id = $2`,
		roomDBID, userID,
	)
	return err
}

// GetUserRole returns the role of a user in a room
func (p *PostgresClient) GetUserRole(roomDBID, userID string) (RoomRole, error) {
	// First check if user is owner
	ownerID, err := p.GetRoomOwner(roomDBID)
	if err != nil {
		return "", err
	}
	if ownerID != nil && *ownerID == userID {
		return RoleOwner, nil
	}

	// Check room_members table
	var role RoomRole
	err = p.db.QueryRow(
		`SELECT role FROM room_members WHERE room_id = $1 AND user_id = $2`,
		roomDBID, userID,
	).Scan(&role)

	if err == sql.ErrNoRows {
		return "", nil // No role assigned
	}
	if err != nil {
		return "", err
	}

	return role, nil
}

// CreateRoomInvitation creates a new room invitation
func (p *PostgresClient) CreateRoomInvitation(roomDBID string, email string, role RoomRole, invitedBy *string, expiresIn time.Duration) (*RoomInvitation, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(expiresIn)

	var invitation RoomInvitation
	err := p.db.QueryRow(
		`INSERT INTO room_invitations (room_id, email, role, token, invited_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, room_id, email, role, token, invited_by, expires_at, created_at`,
		roomDBID, email, role, token, invitedBy, expiresAt,
	).Scan(&invitation.ID, &invitation.RoomID, &invitation.Email, &invitation.Role,
		&invitation.Token, &invitation.InvitedBy, &invitation.ExpiresAt, &invitation.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &invitation, nil
}

// GetRoomInvitationByToken returns an invitation by its token
func (p *PostgresClient) GetRoomInvitationByToken(token string) (*RoomInvitation, error) {
	var invitation RoomInvitation
	var email sql.NullString
	var invitedBy sql.NullString
	var usedAt sql.NullTime

	err := p.db.QueryRow(
		`SELECT id, room_id, email, role, token, invited_by, expires_at, used_at, created_at
		 FROM room_invitations WHERE token = $1`,
		token,
	).Scan(&invitation.ID, &invitation.RoomID, &email, &invitation.Role,
		&invitation.Token, &invitedBy, &invitation.ExpiresAt, &usedAt, &invitation.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		invitation.Email = email.String
	}
	if invitedBy.Valid {
		invitation.InvitedBy = &invitedBy.String
	}
	if usedAt.Valid {
		invitation.UsedAt = &usedAt.Time
	}

	return &invitation, nil
}

// UseRoomInvitation marks an invitation as used
func (p *PostgresClient) UseRoomInvitation(token string) error {
	result, err := p.db.Exec(
		`UPDATE room_invitations SET used_at = NOW() WHERE token = $1 AND used_at IS NULL`,
		token,
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("invitation not found or already used")
	}

	return nil
}

// GetRoomInvitations returns all pending invitations for a room
func (p *PostgresClient) GetRoomInvitations(roomDBID string) ([]RoomInvitation, error) {
	rows, err := p.db.Query(
		`SELECT id, room_id, email, role, token, invited_by, expires_at, used_at, created_at
		 FROM room_invitations 
		 WHERE room_id = $1 AND used_at IS NULL AND expires_at > NOW()
		 ORDER BY created_at DESC`,
		roomDBID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []RoomInvitation
	for rows.Next() {
		var invitation RoomInvitation
		var email sql.NullString
		var invitedBy sql.NullString
		var usedAt sql.NullTime

		err := rows.Scan(&invitation.ID, &invitation.RoomID, &email, &invitation.Role,
			&invitation.Token, &invitedBy, &invitation.ExpiresAt, &usedAt, &invitation.CreatedAt)
		if err != nil {
			return nil, err
		}

		if email.Valid {
			invitation.Email = email.String
		}
		if invitedBy.Valid {
			invitation.InvitedBy = &invitedBy.String
		}
		if usedAt.Valid {
			invitation.UsedAt = &usedAt.Time
		}

		invitations = append(invitations, invitation)
	}

	return invitations, rows.Err()
}

// DeleteRoomInvitation deletes an invitation
func (p *PostgresClient) DeleteRoomInvitation(invitationID string) error {
	_, err := p.db.Exec(
		`DELETE FROM room_invitations WHERE id = $1`,
		invitationID,
	)
	return err
}

// CleanExpiredInvitations removes expired invitations
func (p *PostgresClient) CleanExpiredInvitations() (int, error) {
	result, err := p.db.Exec(
		`DELETE FROM room_invitations WHERE expires_at < NOW() OR used_at IS NOT NULL`,
	)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// CanUserPerformAction checks if a user can perform an action based on their role
func (p *PostgresClient) CanUserPerformAction(roomDBID, userID string, action string) (bool, error) {
	role, err := p.GetUserRole(roomDBID, userID)
	if err != nil {
		return false, err
	}

	// Check room settings for anonymous access
	settings, err := p.GetRoomSettings(roomDBID)
	if err != nil {
		return false, err
	}

	// If no role and anonymous not allowed, deny
	if role == "" && !settings.AllowAnonymous {
		return false, nil
	}

	// Define permissions per role
	switch action {
	case "view":
		// Everyone can view if room is public or they have any role
		return settings.IsPublic || role != "", nil

	case "edit":
		// Owner and Editor can edit, anonymous can edit if allowed
		if role == RoleOwner || role == RoleEditor {
			return true, nil
		}
		// Anonymous users can edit in public rooms that allow anonymous
		return settings.IsPublic && settings.AllowAnonymous, nil

	case "manage_members":
		// Only owner can manage members
		return role == RoleOwner, nil

	case "change_settings":
		// Only owner can change settings
		return role == RoleOwner, nil

	case "delete_room":
		// Only owner can delete room
		return role == RoleOwner, nil

	default:
		return false, nil
	}
}
