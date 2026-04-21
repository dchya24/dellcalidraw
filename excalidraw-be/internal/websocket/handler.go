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

	"github.com/you/excalidraw-be/internal/auth"
	"github.com/you/excalidraw-be/internal/room"
)

// Connection represents an active WebSocket connection
type Connection struct {
	ID           string
	Conn         *websocket.Conn
	Send         chan []byte
	RoomID       string
	UserID       string
	AuthUserID   string
	AuthUsername string
	mu           sync.Mutex
}

// Hub manages active connections
type Hub struct {
	connections       map[string]*Connection // connID -> Connection
	userConns         map[string]string      // userID -> connID
	roomManager       *room.RoomManager
	rateLimiter       *RateLimiter
	cursorRateLimiter *CursorRateLimiter
	authService       *auth.AuthService
	mu                sync.RWMutex
}

// NewHub creates a new connection hub
func NewHub(rm *room.RoomManager, authService *auth.AuthService) *Hub {
	return &Hub{
		connections:       make(map[string]*Connection),
		userConns:         make(map[string]string),
		roomManager:       rm,
		rateLimiter:       NewRateLimiter(),
		cursorRateLimiter: NewCursorRateLimiter(),
		authService:       authService,
	}
}

// HandleWebSocket handles WebSocket connections
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade connection", "error", err)
		return
	}

	var authenticatedUserID string
	var authenticatedUsername string

	if h.authService != nil {
		tokenStr := r.URL.Query().Get("token")
		if tokenStr != "" {
			claims, err := h.authService.ValidateAccessToken(tokenStr)
			if err == nil && claims != nil {
				authenticatedUserID = claims.UserID
				authenticatedUsername = claims.Username
				slog.Info("WebSocket authenticated", "userID", authenticatedUserID, "username", authenticatedUsername)
			} else {
				slog.Debug("WebSocket token invalid, connecting as guest", "error", err)
			}
		}
	}

	// Create connection object
	connection := &Connection{
		ID:           uuid.New().String(),
		Conn:         conn,
		Send:         make(chan []byte, 256),
		AuthUserID:   authenticatedUserID,
		AuthUsername: authenticatedUsername,
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

	// Handle heartbeat ping from client
	if wsMsg.Type == "ping" {
		slog.Debug("💓 Heartbeat ping received", "connID", conn.ID)
		h.sendMessage(conn, "pong", map[string]interface{}{})
		return
	}

	// Handle pong from client (optional - for server-initiated ping)
	if wsMsg.Type == "pong" {
		slog.Debug("💓 Heartbeat pong received", "connID", conn.ID)
		return
	}

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
	case "get_room_link":
		slog.Info("🔗 Routing to handleGetRoomLink", "connID", conn.ID)
		h.handleGetRoomLink(conn, wsMsg.Payload)
	case "selection_change":
		slog.Info("👆 Routing to handleSelectionChange", "connID", conn.ID)
		h.handleSelectionChange(conn, wsMsg.Payload)
	case "file_uploaded":
		slog.Info("📎 Routing to handleFileUploaded", "connID", conn.ID)
		h.handleFileUploaded(conn, wsMsg.Payload)
	case "file_deleted":
		slog.Info("🗑️ Routing to handleFileDeleted", "connID", conn.ID)
		h.handleFileDeleted(conn, wsMsg.Payload)
	default:
		slog.Warn("❓ Unknown message type", "type", wsMsg.Type, "connID", conn.ID)
	}
}

// handleGetRoomLink handles get_room_link messages (Phase 5)
func (h *Hub) handleGetRoomLink(conn *Connection, payload map[string]interface{}) {
	// Parse payload
	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var linkMsg GetRoomLinkPayload
	if err := json.Unmarshal(data, &linkMsg); err != nil {
		h.sendError(conn, "Invalid room link payload", "invalid_payload")
		return
	}

	// Get room
	r := h.roomManager.GetRoom(linkMsg.RoomID)
	if r == nil {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	// Generate shareable link
	shareURL := fmt.Sprintf("%s?room=%s", "http://localhost:3000", linkMsg.RoomID)

	// Send room link
	linkPayload := RoomLinkPayload{
		ShareURL: shareURL,
	}
	h.sendMessage(conn, "room_link", linkPayload)

	slog.Info("Room link generated", "roomID", linkMsg.RoomID, "shareURL", shareURL)
}

// handleSelectionChange handles selection_change messages (Phase 6)
func (h *Hub) handleSelectionChange(conn *Connection, payload map[string]interface{}) {
	// Parse payload
	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var selectionMsg SelectionChangePayload
	if err := json.Unmarshal(data, &selectionMsg); err != nil {
		h.sendError(conn, "Invalid selection change payload", "invalid_payload")
		return
	}

	// Get room
	r := h.roomManager.GetRoom(selectionMsg.RoomID)
	if r == nil {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	// Update user's selected elements
	r.UpdateSelectedIDs(conn.UserID, selectionMsg.SelectedIDs)

	// Broadcast selection to other participants
	selectionUpdated := SelectionUpdatedPayload{
		UserID:      conn.UserID,
		Username:    getUsernameFromRoom(r, conn.UserID),
		Color:       getUserColorFromRoom(r, conn.UserID),
		SelectedIDs: selectionMsg.SelectedIDs,
	}
	h.broadcastToRoom(selectionMsg.RoomID, "selection_updated", selectionUpdated, conn.ID)

	slog.Info("Selection updated", "userID", conn.UserID, "selectedIds", selectionMsg.SelectedIDs, "roomID", selectionMsg.RoomID)
}

// Helper function to get username from room
func getUsernameFromRoom(r *room.Room, userID string) string {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	if user, exists := r.Participants[userID]; exists {
		return user.Username
	}
	return ""
}

// Helper function to get user color from room
func getUserColorFromRoom(r *room.Room, userID string) string {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	if user, exists := r.Participants[userID]; exists {
		return user.Color
	}
	return ""
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

	username := joinMsg.Username
	if conn.AuthUsername != "" {
		username = conn.AuthUsername
	}

	// Create user
	user := &room.User{
		ID:       uuid.New().String(),
		Username: username,
		Color:    color,
		ConnID:   conn.ID,
		JoinedAt: time.Now(),
		LastSeen: time.Now(),
	}

	if conn.AuthUserID != "" {
		user.ID = conn.AuthUserID
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

	h.roomManager.FlushRoom(conn.RoomID)

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

	// Check element count limit
	currentElementCount := len(r.GetElements())
	totalAdding := len(updateMsg.Changes.Added)
	if err := room.ValidateElementCount(currentElementCount, totalAdding); err != nil {
		h.sendError(conn, err.Error(), "element_limit_exceeded")
		return
	}

	var addedElements, updatedElements []room.Element

	if len(updateMsg.Changes.Added) > 0 {
		addedElements = payloadToElements(updateMsg.Changes.Added)
		r.AddElements(addedElements)
	}

	if len(updateMsg.Changes.Updated) > 0 {
		updatedElements = payloadToElements(updateMsg.Changes.Updated)
		r.UpdateElements(updatedElements)
	}

	if len(updateMsg.Changes.Deleted) > 0 {
		r.DeleteElements(updateMsg.Changes.Deleted)
	}

	h.roomManager.QueuePersistence(conn.RoomID, addedElements, updatedElements, updateMsg.Changes.Deleted)

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

						h.roomManager.FlushRoom(conn.RoomID)

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
			ID:              elem.ID,
			Type:            elem.Type,
			X:               elem.X,
			Y:               elem.Y,
			Width:           elem.Width,
			Height:          elem.Height,
			Angle:           elem.Angle,
			StrokeColor:     elem.StrokeColor,
			BackgroundColor: elem.BackgroundColor,
			FillStyle:       elem.FillStyle,
			StrokeWidth:     elem.StrokeWidth,
			StrokeStyle:     elem.StrokeStyle,
			Roughness:       elem.Roughness,
			Opacity:         elem.Opacity,
			Seed:            elem.Seed,
			Version:         elem.Version,
			VersionNonce:    elem.VersionNonce,
			IsDeleted:       elem.IsDeleted,
			GroupIds:        elem.GroupIds,
			FrameId:         elem.FrameId,
			BoundElements:   convertBoundElements(elem.BoundElements),
			Updated:         elem.Updated,
			Link:            elem.Link,
			Locked:          elem.Locked,
			Data:            elem.Data,
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
			ID:              elem.ID,
			Type:            elem.Type,
			X:               elem.X,
			Y:               elem.Y,
			Width:           elem.Width,
			Height:          elem.Height,
			Angle:           elem.Angle,
			StrokeColor:     elem.StrokeColor,
			BackgroundColor: elem.BackgroundColor,
			FillStyle:       elem.FillStyle,
			StrokeWidth:     elem.StrokeWidth,
			StrokeStyle:     elem.StrokeStyle,
			Roughness:       elem.Roughness,
			Opacity:         elem.Opacity,
			Seed:            elem.Seed,
			Version:         elem.Version,
			VersionNonce:    elem.VersionNonce,
			IsDeleted:       elem.IsDeleted,
			GroupIds:        elem.GroupIds,
			FrameId:         elem.FrameId,
			BoundElements:   convertPayloadBoundElements(elem.BoundElements),
			Updated:         elem.Updated,
			Link:            elem.Link,
			Locked:          elem.Locked,
			Data:            elem.Data,
		}
	}
	return result
}

// Helper function to convert room.BoundElement to websocket BoundElementPayload
func convertBoundElements(elements []room.BoundElement) []BoundElementPayload {
	if elements == nil {
		return nil
	}
	result := make([]BoundElementPayload, len(elements))
	for i, elem := range elements {
		result[i] = BoundElementPayload{
			ID:   elem.ID,
			Type: elem.Type,
		}
	}
	return result
}

// Helper function to convert websocket BoundElementPayload to room.BoundElement
func convertPayloadBoundElements(elements []BoundElementPayload) []room.BoundElement {
	if elements == nil {
		return nil
	}
	result := make([]room.BoundElement, len(elements))
	for i, elem := range elements {
		result[i] = room.BoundElement{
			ID:   elem.ID,
			Type: elem.Type,
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
