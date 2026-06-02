package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/you/excalidraw-be/internal/database"
	"github.com/you/excalidraw-be/internal/room"
)

type CanvasHandler struct {
	db          *database.PostgresClient
	roomManager *room.RoomManager
}

func NewCanvasHandler(db *database.PostgresClient, rm *room.RoomManager) *CanvasHandler {
	return &CanvasHandler{
		db:          db,
		roomManager: rm,
	}
}

type SaveCanvasRequest struct {
	Elements []json.RawMessage `json:"elements"`
}

type SaveCanvasResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type LoadCanvasResponse struct {
	Success  bool              `json:"success"`
	Elements []json.RawMessage `json:"elements"`
	Count    int               `json:"count"`
}

// SaveCanvas manually saves the current canvas state to database
// POST /api/rooms/{roomId}/canvas/save
func (ch *CanvasHandler) SaveCanvas(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	if roomID == "" {
		writeJSONError(w, http.StatusBadRequest, "Room ID is required", "missing_room_id")
		return
	}

	// Get room from memory
	rm := ch.roomManager.GetRoom(roomID)
	if rm == nil {
		writeJSONError(w, http.StatusNotFound, "Room not found", "room_not_found")
		return
	}

	// Get or create room in database
	dbID, err := ch.db.GetOrCreateRoom(roomID)
	if err != nil {
		slog.Error("Failed to get/create room in database", "roomID", roomID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to save canvas", "db_error")
		return
	}

	// Get current elements from room
	elements := rm.GetElements()

	// Convert to raw JSON
	rawElements := make([]json.RawMessage, 0, len(elements))
	for _, elem := range elements {
		data, err := json.Marshal(elem)
		if err != nil {
			slog.Warn("Failed to marshal element", "elementID", elem.ID, "error", err)
			continue
		}
		rawElements = append(rawElements, data)
	}

	// Save to database
	if err := ch.db.SaveAllElementsRaw(dbID, rawElements); err != nil {
		slog.Error("Failed to save canvas to database", "roomID", roomID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to save canvas", "save_error")
		return
	}

	slog.Info("Canvas saved manually", "roomID", roomID, "elementCount", len(rawElements))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SaveCanvasResponse{
		Success: true,
		Message: "Canvas saved successfully",
		Count:   len(rawElements),
	})
}

// LoadCanvas manually loads the canvas state from database
// GET /api/rooms/{roomId}/canvas/load
func (ch *CanvasHandler) LoadCanvas(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	if roomID == "" {
		writeJSONError(w, http.StatusBadRequest, "Room ID is required", "missing_room_id")
		return
	}

	// Get room DB ID
	dbID, err := ch.db.GetRoomByKey(roomID)
	if err != nil {
		slog.Error("Failed to get room from database", "roomID", roomID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to load canvas", "db_error")
		return
	}

	if dbID == "" {
		// Room doesn't exist in database yet, return empty
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoadCanvasResponse{
			Success:  true,
			Elements: []json.RawMessage{},
			Count:    0,
		})
		return
	}

	// Load elements from database
	rawElements, err := ch.db.GetRawElements(dbID)
	if err != nil {
		slog.Error("Failed to load elements from database", "roomID", roomID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to load canvas", "load_error")
		return
	}

	slog.Info("Canvas loaded manually", "roomID", roomID, "elementCount", len(rawElements))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoadCanvasResponse{
		Success:  true,
		Elements: rawElements,
		Count:    len(rawElements),
	})
}

// LoadCanvasToRoom loads canvas from database and updates the in-memory room
// POST /api/rooms/{roomId}/canvas/restore
func (ch *CanvasHandler) RestoreCanvas(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	if roomID == "" {
		writeJSONError(w, http.StatusBadRequest, "Room ID is required", "missing_room_id")
		return
	}

	// Get room DB ID
	dbID, err := ch.db.GetRoomByKey(roomID)
	if err != nil {
		slog.Error("Failed to get room from database", "roomID", roomID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to restore canvas", "db_error")
		return
	}

	if dbID == "" {
		writeJSONError(w, http.StatusNotFound, "No saved canvas found", "no_saved_canvas")
		return
	}

	// Load elements from database
	rawElements, err := ch.db.GetRawElements(dbID)
	if err != nil {
		slog.Error("Failed to load elements from database", "roomID", roomID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to restore canvas", "load_error")
		return
	}

	// Parse elements
	elements := make([]room.Element, 0, len(rawElements))
	for _, raw := range rawElements {
		var elem room.Element
		if err := json.Unmarshal(raw, &elem); err != nil {
			slog.Warn("Failed to unmarshal element", "error", err)
			continue
		}
		elements = append(elements, elem)
	}

	// Get or create room in memory
	rm := ch.roomManager.GetOrCreateRoom(roomID)

	// Replace room elements
	rm.Mu.Lock()
	rm.Elements = elements
	rm.Mu.Unlock()

	slog.Info("Canvas restored to room", "roomID", roomID, "elementCount", len(elements))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SaveCanvasResponse{
		Success: true,
		Message: "Canvas restored successfully",
		Count:   len(elements),
	})
}

// ClearCanvas clears all elements from the room and database
// DELETE /api/rooms/{roomId}/canvas
func (ch *CanvasHandler) ClearCanvas(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	if roomID == "" {
		writeJSONError(w, http.StatusBadRequest, "Room ID is required", "missing_room_id")
		return
	}

	// Clear in-memory room
	rm := ch.roomManager.GetRoom(roomID)
	if rm != nil {
		rm.Mu.Lock()
		rm.Elements = make([]room.Element, 0)
		rm.Mu.Unlock()
	}

	// Clear database
	dbID, _ := ch.db.GetRoomByKey(roomID)
	if dbID != "" {
		if err := ch.db.SaveAllElementsRaw(dbID, []json.RawMessage{}); err != nil {
			slog.Error("Failed to clear canvas in database", "roomID", roomID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to clear canvas", "clear_error")
			return
		}
	}

	slog.Info("Canvas cleared", "roomID", roomID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SaveCanvasResponse{
		Success: true,
		Message: "Canvas cleared successfully",
		Count:   0,
	})
}
