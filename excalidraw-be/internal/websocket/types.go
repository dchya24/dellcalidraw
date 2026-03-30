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
