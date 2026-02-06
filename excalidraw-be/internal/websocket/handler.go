package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/you/excalidraw-be/internal/room"
)

// Connection represents an active WebSocket connection
type Connection struct {
	ID     string
	Conn   *websocket.Conn
	Send   chan []byte
	RoomID string
	UserID string
	mu     sync.Mutex
}

// Hub manages active connections
type Hub struct {
	connections       map[string]*Connection // connID -> Connection
	userConns         map[string]string      // userID -> connID
	roomManager       *room.RoomManager
	rateLimiter       *RateLimiter
	cursorRateLimiter *CursorRateLimiter
	mu                sync.RWMutex
}

// NewHub creates a new connection hub
func NewHub(rm *room.RoomManager) *Hub {
	return &Hub{
		connections:       make(map[string]*Connection),
		userConns:         make(map[string]string),
		roomManager:       rm,
		rateLimiter:       NewRateLimiter(),
		cursorRateLimiter: NewCursorRateLimiter(),
	}
}

// HandleWebSocket handles WebSocket connections
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade connection", "error", err)
		return
	}

	// Create connection object
	connection := &Connection{
		ID:   uuid.New().String(),
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	// Register connection
	h.mu.Lock()
	h.connections[connection.ID] = connection
	h.mu.Unlock()

	slog.Info("WebSocket connected", "connID", connection.ID)

	// Start goroutines for reading and writing
	go h.readPump(connection)
	go h.writePump(connection)
}

// readPump handles incoming messages from WebSocket
func (h *Hub) readPump(conn *Connection) {
	defer func() {
		h.unregisterConnection(conn)
		conn.Conn.Close()
	}()

	// Set read deadline to 60 seconds (matches pong wait time)
	conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	slog.Info("readPump started for connection", "connID", conn.ID)

	for {
		_, message, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket error", "error", err, "connID", conn.ID)
			} else {
				slog.Info("WebSocket connection closed", "connID", conn.ID, "error", err)
			}
			break
		}

		slog.Info("📨 Raw message received", "connID", conn.ID, "messageLength", len(message), "message", string(message))
		h.handleMessage(conn, message)
	}
}

// writePump handles outgoing messages to WebSocket
func (h *Hub) writePump(conn *Connection) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-conn.Send:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			conn.mu.Lock()
			w, err := conn.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				conn.mu.Unlock()
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(conn.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-conn.Send)
			}

			if err := w.Close(); err != nil {
				conn.mu.Unlock()
				return
			}
			conn.mu.Unlock()

		case <-ticker.C:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (h *Hub) handleMessage(conn *Connection, message []byte) {
	slog.Info("🔧 handleMessage called", "connID", conn.ID, "message", string(message))

	var wsMsg WSMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		slog.Error("Failed to parse message", "error", err, "connID", conn.ID, "rawMessage", string(message))
		return
	}

	slog.Info("✅ Message parsed successfully", "connID", conn.ID, "type", wsMsg.Type, "payload", wsMsg.Payload)

	switch wsMsg.Type {
	case "join_room":
		slog.Info("🚪 Routing to handleJoinRoom", "connID", conn.ID)
		h.handleJoinRoom(conn, wsMsg.Payload)
	case "leave_room":
		slog.Info("🚪 Routing to handleLeaveRoom", "connID", conn.ID)
		h.handleLeaveRoom(conn, wsMsg.Payload)
	case "update_elements":
		slog.Info("🎨 Routing to handleUpdateElements", "connID", conn.ID)
		h.handleUpdateElements(conn, wsMsg.Payload)
	case "cursor_move":
		slog.Info("🖱️ Routing to handleCursorMove", "connID", conn.ID)
		h.handleCursorMove(conn, wsMsg.Payload)
	default:
		slog.Warn("❓ Unknown message type", "type", wsMsg.Type, "connID", conn.ID)
	}
}

// handleJoinRoom handles join_room messages
func (h *Hub) handleJoinRoom(conn *Connection, payload map[string]interface{}) {
	// Parse payload
	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var joinMsg JoinRoomPayload
	if err := json.Unmarshal(data, &joinMsg); err != nil {
		h.sendError(conn, "Invalid join room payload", "invalid_payload")
		return
	}

	// Get or create room
	r := h.roomManager.GetOrCreateRoom(joinMsg.RoomID)

	// Check capacity
	if !r.HasCapacity(50) { // TODO: Make configurable
		h.sendError(conn, "Room is full", "room_full")
		return
	}

	// Generate user color (random for now)
	color := generateColor()

	// Create user
	user := &room.User{
		ID:       uuid.New().String(),
		Username: joinMsg.Username,
		Color:    color,
		ConnID:   conn.ID,
		JoinedAt: time.Now(),
		LastSeen: time.Now(),
	}

	// Update connection
	conn.RoomID = joinMsg.RoomID
	conn.UserID = user.ID

	// Register user connection
	h.mu.Lock()
	h.userConns[user.ID] = conn.ID
	h.mu.Unlock()

	// Add user to room
	r.AddUser(user)

	// Send current room state to new user
	roomState := RoomStatePayload{
		Elements:     elementsToPayload(r.GetElements()),
		Participants: usersToPayload(r.GetParticipants()),
	}
	h.sendMessage(conn, "room_state", roomState)

	// Notify other participants
	userJoined := UserJoinedPayload{
		UserID:   user.ID,
		Username: user.Username,
		Color:    user.Color,
	}
	h.broadcastToRoom(joinMsg.RoomID, "user_joined", userJoined, conn.ID)

	slog.Info("User joined room", "userID", user.ID, "roomID", joinMsg.RoomID, "username", user.Username)
}

// handleLeaveRoom handles leave_room messages
func (h *Hub) handleLeaveRoom(conn *Connection, payload map[string]interface{}) {
	if conn.RoomID == "" {
		return
	}

	r := h.roomManager.GetRoom(conn.RoomID)
	if r == nil {
		return
	}

	r.RemoveUser(conn.UserID)

	// Notify other participants
	userLeft := UserLeftPayload{
		UserID: conn.UserID,
	}
	h.broadcastToRoom(conn.RoomID, "user_left", userLeft, conn.ID)

	// Clear connection room
	conn.RoomID = ""
	conn.UserID = ""

	slog.Info("User left room", "userID", conn.UserID, "roomID", conn.RoomID)
}

// handleUpdateElements handles update_elements messages
func (h *Hub) handleUpdateElements(conn *Connection, payload map[string]interface{}) {
	if conn.RoomID == "" {
		h.sendError(conn, "Not in a room", "not_in_room")
		return
	}

	// Check rate limit
	if !h.rateLimiter.Allow(conn.ID) {
		h.sendError(conn, "Rate limit exceeded. Please slow down.", "rate_limit_exceeded")
		return
	}

	r := h.roomManager.GetRoom(conn.RoomID)
	if r == nil {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	// Parse payload
	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	fmt.Println(string(data))

	var updateMsg UpdateElementsPayload
	if err := json.Unmarshal(data, &updateMsg); err != nil {
		h.sendError(conn, "Invalid update elements payload", "invalid_payload")
		return
	}

	// Validate elements
	elementsToValidate := []room.Element{}
	elementsToValidate = append(elementsToValidate, payloadToElements(updateMsg.Changes.Added)...)
	elementsToValidate = append(elementsToValidate, payloadToElements(updateMsg.Changes.Updated)...)

	if len(elementsToValidate) > 0 {
		validationErrors := room.ValidateElementsBatch(elementsToValidate)
		if len(validationErrors) > 0 {
			h.sendError(conn, "Element validation failed", "validation_error")
			slog.Warn("Element validation failed", "errors", validationErrors, "userID", conn.UserID)
			return
		}
	}

	// Check element count limit
	currentElementCount := len(r.GetElements())
	totalAdding := len(updateMsg.Changes.Added)
	if err := room.ValidateElementCount(currentElementCount, totalAdding); err != nil {
		h.sendError(conn, err.Error(), "element_limit_exceeded")
		return
	}

	// Apply changes
	if len(updateMsg.Changes.Added) > 0 {
		elements := payloadToElements(updateMsg.Changes.Added)
		r.AddElements(elements)
	}

	if len(updateMsg.Changes.Updated) > 0 {
		elements := payloadToElements(updateMsg.Changes.Updated)
		r.UpdateElements(elements)
	}

	if len(updateMsg.Changes.Deleted) > 0 {
		r.DeleteElements(updateMsg.Changes.Deleted)
	}

	// Broadcast to other participants with sender info
	elementsUpdated := ElementsUpdatedPayload{
		UserID:  conn.UserID,
		Changes: updateMsg.Changes,
	}
	h.broadcastToRoom(conn.RoomID, "elements_updated", elementsUpdated, conn.ID)
}

// handleCursorMove handles cursor_move messages
func (h *Hub) handleCursorMove(conn *Connection, payload map[string]interface{}) {
	if conn.RoomID == "" {
		return
	}

	// Check cursor rate limit (allow 20 updates per second)
	if !h.cursorRateLimiter.Allow(conn.ID) {
		return // Silently skip throttled cursor updates
	}

	r := h.roomManager.GetRoom(conn.RoomID)
	if r == nil {
		return
	}

	// Parse payload
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	var cursorMsg CursorMovePayload
	if err := json.Unmarshal(data, &cursorMsg); err != nil {
		return
	}

	// Update cursor in room
	r.UpdateCursor(conn.UserID, cursorMsg.Position.X, cursorMsg.Position.Y)

	// Get user info
	users := r.GetParticipants()
	var username, color string
	for _, u := range users {
		if u.ID == conn.UserID {
			username = u.Username
			color = u.Color
			break
		}
	}

	// Broadcast to other participants
	cursorUpdated := CursorUpdatedPayload{
		UserID:   conn.UserID,
		Username: username,
		Color:    color,
		Position: cursorMsg.Position,
	}
	h.broadcastToRoom(conn.RoomID, "cursor_updated", cursorUpdated, conn.ID)
}

// unregisterConnection removes a connection from the hub
func (h *Hub) unregisterConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.connections[conn.ID]; ok {
		delete(h.connections, conn.ID)

		// Also remove from userConns
		for userID, connID := range h.userConns {
			if connID == conn.ID {
				delete(h.userConns, userID)

				// Remove from room
				if conn.RoomID != "" {
					r := h.roomManager.GetRoom(conn.RoomID)
					if r != nil {
						r.RemoveUser(userID)

						// Notify other participants
						userLeft := UserLeftPayload{UserID: userID}
						h.broadcastToRoom(conn.RoomID, "user_left", userLeft, "")
					}
				}
				break
			}
		}

		close(conn.Send)
	}

	slog.Info("WebSocket disconnected", "connID", conn.ID)
}

// sendMessage sends a message to a specific connection
func (h *Hub) sendMessage(conn *Connection, msgType string, payload interface{}) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":    msgType,
		"payload": payload,
	})
	if err != nil {
		return err
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		return context.DeadlineExceeded
	}
}

// broadcastToRoom sends a message to all connections in a room except sender
func (h *Hub) broadcastToRoom(roomID, msgType string, payload interface{}, excludeConnID string) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    msgType,
		"payload": payload,
	})
	if err != nil {
		slog.Error("Failed to marshal broadcast message", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conn := range h.connections {
		if conn.RoomID == roomID && conn.ID != excludeConnID {
			select {
			case conn.Send <- data:
			default:
				slog.Warn("Failed to send message, channel full", "connID", conn.ID)
			}
		}
	}
}

// sendError sends an error message to a connection
func (h *Hub) sendError(conn *Connection, message, code string) {
	errPayload := ErrorPayload{
		Message: message,
		Code:    code,
	}
	h.sendMessage(conn, "error", errPayload)
}

// Helper functions
func elementsToPayload(elements []room.Element) []ElementPayload {
	payload := make([]ElementPayload, len(elements))
	for i, elem := range elements {
		payload[i] = ElementPayload{
			ID:         elem.ID,
			Type:       elem.Type,
			X:          elem.X,
			Y:          elem.Y,
			Width:      elem.Width,
			Height:     elem.Height,
			Angle:      elem.Angle,
			Stroke:     elem.Stroke,
			Background: elem.Background,
			Fill:       elem.Fill,
			Data:       elem.Data,
		}
	}
	return payload
}

func usersToPayload(users []*room.User) []UserPayload {
	payload := make([]UserPayload, len(users))
	for i, user := range users {
		payload[i] = UserPayload{
			ID:       user.ID,
			Username: user.Username,
			Color:    user.Color,
		}
	}
	return payload
}

func payloadToElements(elements []ElementPayload) []room.Element {
	result := make([]room.Element, len(elements))
	for i, elem := range elements {
		result[i] = room.Element{
			ID:         elem.ID,
			Type:       elem.Type,
			X:          elem.X,
			Y:          elem.Y,
			Width:      elem.Width,
			Height:     elem.Height,
			Angle:      elem.Angle,
			Stroke:     elem.Stroke,
			Background: elem.Background,
			Fill:       elem.Fill,
			Data:       elem.Data,
		}
	}
	return result
}

// generateColor generates a random color for a user
func generateColor() string {
	colors := []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#FFA07A",
		"#98D8C8", "#F7DC6F", "#BB8FCE", "#85C1E2",
		"#F8B739", "#52B788",
	}
	return colors[len(colors)-1] // Simple for now, will make random
}
