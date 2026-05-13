package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/you/excalidraw-be/internal/auth"
	"github.com/you/excalidraw-be/internal/database"
)

type FileManagementHandler struct {
	db          *database.PostgresClient
	authService *auth.AuthService
}

func NewFileManagementHandler(db *database.PostgresClient, authSvc *auth.AuthService) *FileManagementHandler {
	return &FileManagementHandler{
		db:          db,
		authService: authSvc,
	}
}

// ListUserFiles returns all files for the authenticated user
// GET /api/files
func (h *FileManagementHandler) ListUserFiles(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	files, err := h.db.GetUserFiles(userID)
	if err != nil {
		slog.Error("Failed to list user files", "error", err, "userID", userID)
		writeJSONError(w, http.StatusInternalServerError, "Failed to list files", "list_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   files,
		"count":   len(files),
	})
}

// CreateUserFile creates a new file for the user
// POST /api/files
func (h *FileManagementHandler) CreateUserFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to "Untitled" if no body provided
		req.Name = "Untitled"
	}
	if req.Name == "" {
		req.Name = "Untitled"
	}

	file, err := h.db.CreateUserFile(userID, req.Name)
	if err != nil {
		slog.Error("Failed to create user file", "error", err, "userID", userID)
		writeJSONError(w, http.StatusInternalServerError, "Failed to create file", "create_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"file":    file,
	})
}

// GetUserFile returns a specific file by ID
// GET /api/files/:fileId
func (h *FileManagementHandler) GetUserFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	fileID := chi.URLParam(r, "fileId")
	if fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing fileId", "missing_file_id")
		return
	}

	file, err := h.db.GetUserFile(fileID)
	if err != nil {
		slog.Error("Failed to get user file", "error", err, "fileID", fileID)
		writeJSONError(w, http.StatusNotFound, "File not found", "file_not_found")
		return
	}

	// Verify ownership
	if file.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Access denied", "forbidden")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"file":    file,
	})
}

// UpdateUserFile updates a file's metadata (name, tab count)
// PUT /api/files/:fileId
func (h *FileManagementHandler) UpdateUserFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	fileID := chi.URLParam(r, "fileId")
	if fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing fileId", "missing_file_id")
		return
	}

	// Check ownership first
	existing, err := h.db.GetUserFile(fileID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "File not found", "file_not_found")
		return
	}
	if existing.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Access denied", "forbidden")
		return
	}

	var req struct {
		Name     string `json:"name,omitempty"`
		TabCount int    `json:"tabCount,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	name := existing.Name
	if req.Name != "" {
		name = req.Name
	}
	tabCount := existing.TabCount
	if req.TabCount > 0 {
		tabCount = req.TabCount
	}

	file, err := h.db.UpdateUserFile(fileID, name, tabCount)
	if err != nil {
		slog.Error("Failed to update user file", "error", err, "fileID", fileID)
		writeJSONError(w, http.StatusInternalServerError, "Failed to update file", "update_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"file":    file,
	})
}

// RenameUserFile renames a file
// PATCH /api/files/:fileId/rename
func (h *FileManagementHandler) RenameUserFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	fileID := chi.URLParam(r, "fileId")
	if fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing fileId", "missing_file_id")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSONError(w, http.StatusBadRequest, "Name is required", "missing_name")
		return
	}

	// Check ownership
	existing, err := h.db.GetUserFile(fileID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "File not found", "file_not_found")
		return
	}
	if existing.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Access denied", "forbidden")
		return
	}

	if err := h.db.RenameUserFile(fileID, req.Name); err != nil {
		slog.Error("Failed to rename user file", "error", err, "fileID", fileID)
		writeJSONError(w, http.StatusInternalServerError, "Failed to rename file", "rename_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"fileId":  fileID,
		"name":    req.Name,
	})
}

// DeleteUserFile deletes a file
// DELETE /api/files/:fileId
func (h *FileManagementHandler) DeleteUserFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	fileID := chi.URLParam(r, "fileId")
	if fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing fileId", "missing_file_id")
		return
	}

	// Check ownership
	existing, err := h.db.GetUserFile(fileID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "File not found", "file_not_found")
		return
	}
	if existing.UserID != userID {
		writeJSONError(w, http.StatusForbidden, "Access denied", "forbidden")
		return
	}

	if err := h.db.DeleteUserFile(fileID); err != nil {
		slog.Error("Failed to delete user file", "error", err, "fileID", fileID)
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete file", "delete_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"fileId":  fileID,
	})
}