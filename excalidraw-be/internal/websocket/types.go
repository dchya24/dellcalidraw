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
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	X          float64                `json:"x"`
	Y          float64                `json:"y"`
	Width      float64                `json:"width,omitempty"`
	Height     float64                `json:"height,omitempty"`
	Angle      float64                `json:"angle,omitempty"`
	Stroke     string                 `json:"stroke,omitempty"`
	Background string                 `json:"background,omitempty"`
	Fill       string                 `json:"fill,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
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
