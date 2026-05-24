package main

import (
	"encoding/json"
	"fmt"
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

// ListUserFiles returns all files with tabs for the authenticated user
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

	type TabResponse struct {
		TabKey    string          `json:"tabKey"`
		Title     string          `json:"title"`
		RoomID    string          `json:"roomId"`
		Elements  json.RawMessage `json:"elements"`
		AppState  json.RawMessage `json:"appState"`
		FilesData json.RawMessage `json:"files"`
	}

	type FileWithTabs struct {
		ID        string        `json:"id"`
		UserID    string        `json:"userId"`
		Name      string        `json:"name"`
		TabCount  int           `json:"tabCount"`
		CreatedAt string        `json:"createdAt"`
		UpdatedAt string        `json:"updatedAt"`
		Tabs      []TabResponse `json:"tabs"`
	}

	result := make([]FileWithTabs, 0, len(files))
	for _, f := range files {
		tabs, err := h.db.GetFileTabs(f.ID)
		fileWithTabs := FileWithTabs{
			ID:        f.ID,
			UserID:    f.UserID,
			Name:      f.Name,
			TabCount:  f.TabCount,
			CreatedAt: f.CreatedAt,
			UpdatedAt: f.UpdatedAt,
		}

		if err == nil && len(tabs) > 0 {
			for _, t := range tabs {
				fileWithTabs.Tabs = append(fileWithTabs.Tabs, TabResponse{
					TabKey:    t.TabKey,
					Title:     t.Title,
					RoomID:    t.RoomID,
					Elements:  t.Elements,
					AppState:  t.AppState,
					FilesData: t.FilesData,
				})
			}
		}
		fileWithTabs.TabCount = len(fileWithTabs.Tabs)

		result = append(result, fileWithTabs)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   result,
		"count":   len(result),
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

// MigrateLocalFiles bulk-creates files with tabs from local storage
// POST /api/files/migrate
func (h *FileManagementHandler) MigrateLocalFiles(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSONError(w, http.StatusUnauthorized, "User not authenticated", "unauthorized")
		return
	}

	var req struct {
		Files []struct {
			Name      string `json:"name"`
			ActiveTab string `json:"activeTabId"`
			Tabs      []struct {
				Title    string          `json:"title"`
				RoomID   string          `json:"roomId"`
				Elements json.RawMessage `json:"elements"`
				AppState json.RawMessage `json:"appState"`
				Files    json.RawMessage `json:"files"`
			} `json:"tabs"`
		} `json:"files"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	if len(req.Files) == 0 {
		writeJSONError(w, http.StatusBadRequest, "No files to migrate", "empty_files")
		return
	}

	type MigratedTab struct {
		TabKey    string          `json:"tabKey"`
		Title     string          `json:"title"`
		RoomID    string          `json:"roomId"`
		Elements  json.RawMessage `json:"elements"`
		AppState  json.RawMessage `json:"appState"`
		FilesData json.RawMessage `json:"files"`
	}

	type MigratedFile struct {
		ID        string        `json:"id"`
		Name      string        `json:"name"`
		ActiveTab string        `json:"activeTabId"`
		Tabs      []MigratedTab `json:"tabs"`
	}

	migratedFiles := make([]MigratedFile, 0, len(req.Files))

	for _, f := range req.Files {
		file, err := h.db.CreateUserFile(userID, f.Name)
		if err != nil {
			slog.Error("Failed to create file during migration", "error", err, "userID", userID)
			continue
		}

		migrated := MigratedFile{
			ID:        file.ID,
			Name:      file.Name,
			ActiveTab: f.ActiveTab,
			Tabs:      make([]MigratedTab, 0, len(f.Tabs)),
		}

		for i, t := range f.Tabs {
			tabKey := fmt.Sprintf("tab_%d", i)
			elements := t.Elements
			if elements == nil {
				elements = json.RawMessage(`[]`)
			}
			appState := t.AppState
			if appState == nil {
				appState = json.RawMessage(`{}`)
			}
			filesData := t.Files
			if filesData == nil {
				filesData = json.RawMessage(`{}`)
			}

			tab, err := h.db.CreateFileTab(file.ID, tabKey, t.Title, t.RoomID, elements, appState, filesData, i)
			if err != nil {
				slog.Error("Failed to create tab during migration", "error", err, "fileID", file.ID)
				continue
			}

			migrated.Tabs = append(migrated.Tabs, MigratedTab{
				TabKey:    tab.TabKey,
				Title:     tab.Title,
				RoomID:    tab.RoomID,
				Elements:  tab.Elements,
				AppState:  tab.AppState,
				FilesData: tab.FilesData,
			})
		}

		_, _ = h.db.UpdateUserFile(file.ID, file.Name, len(f.Tabs))

		migratedFiles = append(migratedFiles, migrated)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   migratedFiles,
		"count":   len(migratedFiles),
	})
}

// SaveFileTabs saves/updates all tabs for a file
// PUT /api/files/:fileId/tabs
func (h *FileManagementHandler) SaveFileTabs(w http.ResponseWriter, r *http.Request) {
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
		Tabs []struct {
			TabKey   string          `json:"tabKey"`
			Title    string          `json:"title"`
			RoomID   string          `json:"roomId"`
			Elements json.RawMessage `json:"elements"`
			AppState json.RawMessage `json:"appState"`
			Files    json.RawMessage `json:"files"`
		} `json:"tabs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	if err := h.db.DeleteFileTabsByFileID(fileID); err != nil {
		slog.Error("Failed to clear existing tabs", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to save tabs", "save_failed")
		return
	}

	for i, t := range req.Tabs {
		elements := t.Elements
		if elements == nil {
			elements = json.RawMessage(`[]`)
		}
		appState := t.AppState
		if appState == nil {
			appState = json.RawMessage(`{}`)
		}
		filesData := t.Files
		if filesData == nil {
			filesData = json.RawMessage(`{}`)
		}

		tabKey := t.TabKey
		if tabKey == "" {
			tabKey = fmt.Sprintf("tab_%d", i)
		}

		_, err := h.db.CreateFileTab(fileID, tabKey, t.Title, t.RoomID, elements, appState, filesData, i)
		if err != nil {
			slog.Error("Failed to create tab", "error", err, "fileID", fileID)
			continue
		}
	}

	_, _ = h.db.UpdateUserFile(fileID, existing.Name, len(req.Tabs))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(req.Tabs),
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