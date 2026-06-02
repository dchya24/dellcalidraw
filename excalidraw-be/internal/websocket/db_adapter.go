package websocket

import (
	"time"

	"github.com/you/excalidraw-be/internal/database"
)

// DBClientAdapter adapts database.PostgresClient to the DBClient interface
type DBClientAdapter struct {
	db *database.PostgresClient
}

// NewDBClientAdapter creates a new adapter
func NewDBClientAdapter(db *database.PostgresClient) *DBClientAdapter {
	return &DBClientAdapter{db: db}
}

func (a *DBClientAdapter) GetRoomByKey(roomKey string) (string, error) {
	return a.db.GetRoomByKey(roomKey)
}

func (a *DBClientAdapter) GetOrCreateRoom(roomKey string) (string, error) {
	return a.db.GetOrCreateRoom(roomKey)
}

func (a *DBClientAdapter) GetOrCreateRoomEncryptionKey(roomDBID string, gen func() (string, error)) (string, error) {
	return a.db.GetOrCreateRoomEncryptionKey(roomDBID, gen)
}

func (a *DBClientAdapter) HasRoomPassword(roomDBID string) (bool, error) {
	return a.db.HasRoomPassword(roomDBID)
}

func (a *DBClientAdapter) VerifyRoomPassword(roomDBID string, password string) (bool, error) {
	return a.db.VerifyRoomPassword(roomDBID, password)
}

func (a *DBClientAdapter) GetRoomSettings(roomDBID string) (*RoomSettingsDB, error) {
	settings, err := a.db.GetRoomSettings(roomDBID)
	if err != nil {
		return nil, err
	}
	return &RoomSettingsDB{
		OwnerID:        settings.OwnerID,
		HasPassword:    settings.HasPassword,
		IsPublic:       settings.IsPublic,
		AllowAnonymous: settings.AllowAnonymous,
	}, nil
}

func (a *DBClientAdapter) UpdateRoomSettings(roomDBID string, isPublic, allowAnonymous bool) error {
	return a.db.UpdateRoomSettings(roomDBID, isPublic, allowAnonymous)
}

func (a *DBClientAdapter) SetRoomPassword(roomDBID string, password string) error {
	return a.db.SetRoomPassword(roomDBID, password)
}

func (a *DBClientAdapter) RemoveRoomPassword(roomDBID string) error {
	return a.db.RemoveRoomPassword(roomDBID)
}

func (a *DBClientAdapter) SetRoomOwner(roomDBID string, userID string) error {
	return a.db.SetRoomOwner(roomDBID, userID)
}

func (a *DBClientAdapter) GetRoomOwner(roomDBID string) (*string, error) {
	return a.db.GetRoomOwner(roomDBID)
}

func (a *DBClientAdapter) GetUserRole(roomDBID, userID string) (string, error) {
	role, err := a.db.GetUserRole(roomDBID, userID)
	if err != nil {
		return "", err
	}
	return string(role), nil
}

func (a *DBClientAdapter) CanUserPerformAction(roomDBID, userID string, action string) (bool, error) {
	return a.db.CanUserPerformAction(roomDBID, userID, action)
}

func (a *DBClientAdapter) GetRoomMembers(roomDBID string) ([]RoomMemberDB, error) {
	members, err := a.db.GetRoomMembers(roomDBID)
	if err != nil {
		return nil, err
	}
	result := make([]RoomMemberDB, len(members))
	for i, m := range members {
		result[i] = RoomMemberDB{
			ID:        m.ID,
			RoomID:    m.RoomID,
			UserID:    m.UserID,
			Username:  m.Username,
			Email:     m.Email,
			Role:      string(m.Role),
			InvitedBy: m.InvitedBy,
		}
	}
	return result, nil
}

func (a *DBClientAdapter) AddRoomMember(roomDBID, userID string, role string, invitedBy *string) error {
	return a.db.AddRoomMember(roomDBID, userID, database.RoomRole(role), invitedBy)
}

func (a *DBClientAdapter) UpdateRoomMemberRole(roomDBID, userID string, role string) error {
	return a.db.UpdateRoomMemberRole(roomDBID, userID, database.RoomRole(role))
}

func (a *DBClientAdapter) RemoveRoomMember(roomDBID, userID string) error {
	return a.db.RemoveRoomMember(roomDBID, userID)
}

func (a *DBClientAdapter) CreateRoomInvitation(roomDBID string, email string, role string, invitedBy *string, expiresIn interface{}) (*RoomInvitationDB, error) {
	duration, ok := expiresIn.(time.Duration)
	if !ok {
		duration = 7 * 24 * time.Hour // Default 7 days
	}
	inv, err := a.db.CreateRoomInvitation(roomDBID, email, database.RoomRole(role), invitedBy, duration)
	if err != nil {
		return nil, err
	}
	return &RoomInvitationDB{
		ID:        inv.ID,
		RoomID:    inv.RoomID,
		Email:     inv.Email,
		Role:      string(inv.Role),
		Token:     inv.Token,
		InvitedBy: inv.InvitedBy,
		ExpiresAt: inv.ExpiresAt,
		UsedAt:    inv.UsedAt,
	}, nil
}

func (a *DBClientAdapter) GetRoomInvitationByToken(token string) (*RoomInvitationDB, error) {
	inv, err := a.db.GetRoomInvitationByToken(token)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, nil
	}
	return &RoomInvitationDB{
		ID:        inv.ID,
		RoomID:    inv.RoomID,
		Email:     inv.Email,
		Role:      string(inv.Role),
		Token:     inv.Token,
		InvitedBy: inv.InvitedBy,
		ExpiresAt: inv.ExpiresAt,
		UsedAt:    inv.UsedAt,
	}, nil
}

func (a *DBClientAdapter) UseRoomInvitation(token string) error {
	return a.db.UseRoomInvitation(token)
}

func (a *DBClientAdapter) GetRoomInvitations(roomDBID string) ([]RoomInvitationDB, error) {
	invitations, err := a.db.GetRoomInvitations(roomDBID)
	if err != nil {
		return nil, err
	}
	result := make([]RoomInvitationDB, len(invitations))
	for i, inv := range invitations {
		result[i] = RoomInvitationDB{
			ID:        inv.ID,
			RoomID:    inv.RoomID,
			Email:     inv.Email,
			Role:      string(inv.Role),
			Token:     inv.Token,
			InvitedBy: inv.InvitedBy,
			ExpiresAt: inv.ExpiresAt,
			UsedAt:    inv.UsedAt,
		}
	}
	return result, nil
}

func (a *DBClientAdapter) DeleteRoomInvitation(invitationID string) error {
	return a.db.DeleteRoomInvitation(invitationID)
}

func (a *DBClientAdapter) GetUserByID(userID string) (*UserDB, error) {
	user, err := a.db.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &UserDB{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}
