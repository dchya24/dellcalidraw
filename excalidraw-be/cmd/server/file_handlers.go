package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/you/excalidraw-be/internal/database"
	"github.com/you/excalidraw-be/internal/room"
	"github.com/you/excalidraw-be/internal/storage"
)

const maxUploadSize = 50 << 20

var allowedMIMETypes = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/gif":     true,
	"image/webp":    true,
	"image/svg+xml": true,
	"image/bmp":     true,
}

type FileHandler struct {
	storage     *storage.StorageClient
	db          *database.PostgresClient
	roomManager *room.RoomManager
}

func NewFileHandler(s *storage.StorageClient, db *database.PostgresClient, rm *room.RoomManager) *FileHandler {
	return &FileHandler{
		storage:     s,
		db:          db,
		roomManager: rm,
	}
}

func (fh *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSONError(w, http.StatusBadRequest, "File too large (max 50MB)", "file_too_large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Missing file field", "missing_file")
		return
	}
	defer file.Close()

	roomID := r.FormValue("roomId")
	if roomID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing roomId", "missing_room_id")
		return
	}

	contentType := detectContentType(header.Filename, file)
	if !allowedMIMETypes[contentType] {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("File type %s not allowed", contentType), "invalid_file_type")
		return
	}

	if header.Size > maxUploadSize {
		writeJSONError(w, http.StatusBadRequest, "File too large (max 50MB)", "file_too_large")
		return
	}

	fileID := uuid.New().String()
	storageKey := fmt.Sprintf("%s/%s", roomID, fileID)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if err := fh.storage.Upload(ctx, storageKey, file, header.Size, contentType); err != nil {
		slog.Error("Failed to upload file to storage", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Upload failed", "upload_failed")
		return
	}

	dbID := fh.roomManager.GetRoomDBID(roomID)
	if dbID != "" && fh.db != nil {
		if err := fh.db.SaveFileRecord(dbID, fileID, contentType, header.Size, storageKey); err != nil {
			slog.Error("Failed to save file record", "error", err)
		}
	}

	fileURL := fh.storage.GetURL(storageKey)

	response := map[string]interface{}{
		"fileId":     fileID,
		"url":        fileURL,
		"mimeType":   contentType,
		"size":       header.Size,
		"storageKey": storageKey,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	slog.Info("File uploaded", "fileID", fileID, "roomID", roomID, "size", header.Size, "type", contentType)
}

func (fh *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	fileID := chi.URLParam(r, "fileId")

	if roomID == "" || fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing roomId or fileId", "missing_params")
		return
	}

	storageKey := fmt.Sprintf("%s/%s", roomID, fileID)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	obj, err := fh.storage.Download(ctx, storageKey)
	if err != nil {
		slog.Error("Failed to download file", "error", err, "key", storageKey)
		writeJSONError(w, http.StatusNotFound, "File not found", "file_not_found")
		return
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get file info", "stat_failed")
		return
	}

	contentType := "application/octet-stream"
	if ct, ok := info.Metadata["Content-Type"]; ok && len(ct) > 0 {
		contentType = ct[0]
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := io.Copy(w, obj); err != nil {
		slog.Error("Failed to stream file", "error", err)
	}
}

func (fh *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	fileID := chi.URLParam(r, "fileId")

	if roomID == "" || fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing roomId or fileId", "missing_params")
		return
	}

	storageKey := fmt.Sprintf("%s/%s", roomID, fileID)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := fh.storage.Delete(ctx, storageKey); err != nil {
		slog.Error("Failed to delete file from storage", "error", err)
	}

	dbID := fh.roomManager.GetRoomDBID(roomID)
	if dbID != "" && fh.db != nil {
		if err := fh.db.DeleteFileRecord(dbID, fileID); err != nil {
			slog.Error("Failed to delete file record", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"fileId":  fileID,
	})

	slog.Info("File deleted", "fileID", fileID, "roomID", roomID)
}

func (fh *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	if roomID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing roomId", "missing_room_id")
		return
	}

	dbID := fh.roomManager.GetRoomDBID(roomID)
	if dbID == "" || fh.db == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
		return
	}

	records, err := fh.db.ListFileRecords(dbID)
	if err != nil {
		slog.Error("Failed to list files", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to list files", "list_failed")
		return
	}

	type fileResponse struct {
		FileID   string `json:"fileId"`
		MimeType string `json:"mimeType"`
		Size     int64  `json:"size"`
		URL      string `json:"url"`
	}

	files := make([]fileResponse, 0, len(records))
	for _, rec := range records {
		files = append(files, fileResponse{
			FileID:   rec.FileID,
			MimeType: rec.MimeType,
			Size:     rec.Size,
			URL:      fh.storage.GetURL(rec.Key),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": files,
	})
}

func detectContentType(filename string, file io.ReadSeeker) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".jpg" {
		ext = ".jpeg"
	}

	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		return "application/octet-stream"
	}
	file.Seek(0, io.SeekStart)

	return http.DetectContentType(buf[:n])
}

func writeJSONError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"code":    code,
		"success": false,
	})
}
