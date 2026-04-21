package websocket

import (
	"encoding/json"
	"log/slog"
)

func (h *Hub) handleFileUploaded(conn *Connection, payload map[string]interface{}) {
	if conn.RoomID == "" {
		h.sendError(conn, "Not in a room", "not_in_room")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var fileMsg FileUploadedPayload
	if err := json.Unmarshal(data, &fileMsg); err != nil {
		h.sendError(conn, "Invalid file upload payload", "invalid_payload")
		return
	}

	if fileMsg.RoomID != conn.RoomID {
		h.sendError(conn, "Room ID mismatch", "room_mismatch")
		return
	}

	username := getUsernameFromRoom(h.roomManager.GetRoom(conn.RoomID), conn.UserID)

	broadcast := FileUploadedBroadcast{
		UserID:   conn.UserID,
		Username: username,
		FileID:   fileMsg.FileID,
		URL:      fileMsg.URL,
		MimeType: fileMsg.MimeType,
		Size:     fileMsg.Size,
	}
	h.broadcastToRoom(conn.RoomID, "file_uploaded", broadcast, conn.ID)

	response := FileUploadedResponse{
		FileID:     fileMsg.FileID,
		URL:        fileMsg.URL,
		MimeType:   fileMsg.MimeType,
		Size:       fileMsg.Size,
		StorageKey: fileMsg.StorageKey,
	}
	h.sendMessage(conn, "file_upload_confirmed", response)

	slog.Info("File upload broadcast", "fileID", fileMsg.FileID, "roomID", conn.RoomID, "userID", conn.UserID)
}

func (h *Hub) handleFileDeleted(conn *Connection, payload map[string]interface{}) {
	if conn.RoomID == "" {
		h.sendError(conn, "Not in a room", "not_in_room")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var delMsg struct {
		RoomID string `json:"roomId"`
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal(data, &delMsg); err != nil {
		h.sendError(conn, "Invalid file delete payload", "invalid_payload")
		return
	}

	broadcast := FileDeletedBroadcast{
		UserID: conn.UserID,
		FileID: delMsg.FileID,
	}
	h.broadcastToRoom(conn.RoomID, "file_deleted", broadcast, conn.ID)

	slog.Info("File delete broadcast", "fileID", delMsg.FileID, "roomID", conn.RoomID, "userID", conn.UserID)
}
