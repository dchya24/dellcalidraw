package websocket

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// Client → Server message types
type JoinRoomPayload struct {
	RoomID   string `json:"roomId"`
	Username string `json:"username"`
}

type LeaveRoomPayload struct {
	RoomID string `json:"roomId"`
}

type UpdateElementsPayload struct {
	RoomID  string         `json:"roomId"`
	Changes ElementChanges `json:"changes"`
}

type ElementChanges struct {
	Added   []ElementPayload `json:"added,omitempty"`
	Updated []ElementPayload `json:"updated,omitempty"`
	Deleted []string         `json:"deleted,omitempty"`
}

type ElementPayload struct {
	ID              string                `json:"id"`
	Type            string                `json:"type"`
	X               float64               `json:"x"`
	Y               float64               `json:"y"`
	Width           float64               `json:"width,omitempty"`
	Height          float64               `json:"height,omitempty"`
	Angle           float64               `json:"angle,omitempty"`
	StrokeColor     string                `json:"strokeColor,omitempty"`
	BackgroundColor string                `json:"backgroundColor,omitempty"`
	FillStyle       string                `json:"fillStyle,omitempty"`
	StrokeWidth     int                   `json:"strokeWidth,omitempty"`
	StrokeStyle     string                `json:"strokeStyle,omitempty"`
	Roughness       int                   `json:"roughness,omitempty"`
	Opacity         int                   `json:"opacity,omitempty"`
	Seed            int64                 `json:"seed,omitempty"`
	Version         int                   `json:"version,omitempty"`
	VersionNonce    int                   `json:"versionNonce,omitempty"`
	IsDeleted       bool                  `json:"isDeleted,omitempty"`
	GroupIds        []string              `json:"groupIds,omitempty"`
	FrameId         string                `json:"frameId,omitempty"`
	BoundElements   []BoundElementPayload `json:"boundElements,omitempty"`
	Updated         int64                 `json:"updated,omitempty"`
	Link            string                `json:"link,omitempty"`
	Locked          bool                  `json:"locked,omitempty"`
	// Type-specific fields stored in Data for flexibility
	Data map[string]interface{} `json:"data,omitempty"`
}

type BoundElementPayload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type CursorMovePayload struct {
	RoomID   string   `json:"roomId"`
	Position Position `json:"position"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Server → Client message types
type RoomStatePayload struct {
	Elements     []ElementPayload `json:"elements"`
	Participants []UserPayload    `json:"participants"`
}

type UserPayload struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Color    string `json:"color"`
}

type UserJoinedPayload struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Color    string `json:"color"`
}

type UserLeftPayload struct {
	UserID string `json:"userId"`
}

type ElementsUpdatedPayload struct {
	UserID  string         `json:"userId"`
	Changes ElementChanges `json:"changes"`
}

type CursorUpdatedPayload struct {
	UserID   string   `json:"userId"`
	Username string   `json:"username"`
	Color    string   `json:"color"`
	Position Position `json:"position"`
}

type ErrorPayload struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Room Link Management (Phase 5)
type GetRoomLinkPayload struct {
	RoomID string `json:"roomId"`
}

type RoomLinkPayload struct {
	ShareURL string `json:"shareUrl"`
	QRCode   string `json:"qrCode,omitempty"`
}

// Selection & Interaction Awareness (Phase 6)
type SelectionChangePayload struct {
	RoomID      string   `json:"roomId"`
	SelectedIDs []string `json:"selectedIds"`
}

type SelectionUpdatedPayload struct {
	UserID      string   `json:"userId"`
	Username    string   `json:"username"`
	Color       string   `json:"color"`
	SelectedIDs []string `json:"selectedIds"`
}

// File Storage (Phase 9)
type FileUploadedPayload struct {
	RoomID     string `json:"roomId"`
	FileID     string `json:"fileId"`
	URL        string `json:"url"`
	MimeType   string `json:"mimeType"`
	Size       int64  `json:"size"`
	StorageKey string `json:"storageKey"`
}

type FileUploadedBroadcast struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	FileID   string `json:"fileId"`
	URL      string `json:"url"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

type FileDeletedBroadcast struct {
	UserID string `json:"userId"`
	FileID string `json:"fileId"`
}

type FileUploadedResponse struct {
	FileID     string `json:"fileId"`
	URL        string `json:"url"`
	MimeType   string `json:"mimeType"`
	Size       int64  `json:"size"`
	StorageKey string `json:"storageKey"`
}

// Phase 11–12: WebSocket message encryption handshake.
// Sent once from server → client immediately after a successful
// join_room, plaintext, before any encrypted traffic.
type EncryptionHandshakePayload struct {
	RoomID string `json:"roomId"`
	Key    string `json:"key"` // base64 AES-256 key, 44 chars
}

// Phase 11: Room Permissions & Advanced Features

// JoinRoomPayload extended with password support
type JoinRoomWithPasswordPayload struct {
	RoomID   string `json:"roomId"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// Room settings payload
type RoomSettingsPayload struct {
	RoomID         string `json:"roomId"`
	HasPassword    bool   `json:"hasPassword"`
	IsPublic       bool   `json:"isPublic"`
	AllowAnonymous bool   `json:"allowAnonymous"`
	IsOwner        bool   `json:"isOwner"`
	UserRole       string `json:"userRole,omitempty"` // owner, editor, viewer, or empty for anonymous
}

// Set room password request
type SetRoomPasswordPayload struct {
	RoomID   string `json:"roomId"`
	Password string `json:"password"` // Empty string to remove password
}

// Update room settings request
type UpdateRoomSettingsPayload struct {
	RoomID         string `json:"roomId"`
	IsPublic       *bool  `json:"isPublic,omitempty"`
	AllowAnonymous *bool  `json:"allowAnonymous,omitempty"`
}

// Room member management
type RoomMemberPayload struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"` // owner, editor, viewer
}

type GetRoomMembersPayload struct {
	RoomID string `json:"roomId"`
}

type RoomMembersListPayload struct {
	RoomID  string              `json:"roomId"`
	Members []RoomMemberPayload `json:"members"`
}

type UpdateMemberRolePayload struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
	Role   string `json:"role"` // editor, viewer
}

type RemoveMemberPayload struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

type MemberRoleUpdatedPayload struct {
	RoomID    string `json:"roomId"`
	UserID    string `json:"userId"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	UpdatedBy string `json:"updatedBy"`
}

type MemberRemovedPayload struct {
	RoomID    string `json:"roomId"`
	UserID    string `json:"userId"`
	RemovedBy string `json:"removedBy"`
}

// Room invitation
type CreateInvitationPayload struct {
	RoomID string `json:"roomId"`
	Email  string `json:"email,omitempty"`
	Role   string `json:"role"` // editor, viewer
}

type InvitationPayload struct {
	ID        string `json:"id"`
	RoomID    string `json:"roomId"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role"`
	Token     string `json:"token"`
	InviteURL string `json:"inviteUrl"`
	ExpiresAt string `json:"expiresAt"`
}

type AcceptInvitationPayload struct {
	Token string `json:"token"`
}

type GetInvitationsPayload struct {
	RoomID string `json:"roomId"`
}

type InvitationsListPayload struct {
	RoomID      string              `json:"roomId"`
	Invitations []InvitationPayload `json:"invitations"`
}

type DeleteInvitationPayload struct {
	RoomID       string `json:"roomId"`
	InvitationID string `json:"invitationId"`
}

// Permission denied response
type PermissionDeniedPayload struct {
	Action  string `json:"action"`
	Message string `json:"message"`
}

// Room state extended with settings
type RoomStateWithSettingsPayload struct {
	Elements     []ElementPayload    `json:"elements"`
	Participants []UserPayload       `json:"participants"`
	Settings     RoomSettingsPayload `json:"settings"`
}

// Password required response (sent when joining password-protected room)
type PasswordRequiredPayload struct {
	RoomID string `json:"roomId"`
}
