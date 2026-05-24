package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"

	"github.com/you/excalidraw-be/internal/ai"
	"github.com/you/excalidraw-be/internal/auth"
	"github.com/you/excalidraw-be/internal/config"
	"github.com/you/excalidraw-be/internal/database"
	appmiddleware "github.com/you/excalidraw-be/internal/middleware"
	"github.com/you/excalidraw-be/internal/room"
	"github.com/you/excalidraw-be/internal/storage"
	"github.com/you/excalidraw-be/internal/websocket"
)

func main() {
	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := initLogger(cfg.Log)
	if err != nil {
		slog.Error("Failed to initialize logger", "error", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Initialize room manager
	roomManager := room.NewRoomManager()
	roomManager.StartCleanup(
		cfg.Room.CleanupInterval,
		cfg.Room.InactivityTimeout,
	)

	// Initialize database (optional, graceful degradation)
	var dbClient *database.PostgresClient
	dbClient, err = database.NewPostgresClient(database.DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
		MaxOpen:  cfg.Database.MaxOpenConns,
		MaxIdle:  cfg.Database.MaxIdleConns,
		MaxLife:  cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		slog.Warn("Database connection failed, running without persistence", "error", err)
	} else {
		defer dbClient.Close()

		if err := database.RunMigrations(dbClient.DB(), cfg.Database.DBName); err != nil {
			slog.Error("Failed to run migrations", "error", err)
			os.Exit(1)
		}

		if err := room.InitPersistence(dbClient, roomManager, 3*time.Second); err != nil {
			slog.Warn("Failed to initialize persistence, running without it", "error", err)
		}
	}

	var storageClient *storage.StorageClient
	storageClient, err = storage.NewStorageClient(storage.StorageConfig{
		Endpoint:  cfg.Storage.Endpoint,
		AccessKey: cfg.Storage.AccessKey,
		SecretKey: cfg.Storage.SecretKey,
		Bucket:    cfg.Storage.Bucket,
		Region:    cfg.Storage.Region,
		UseSSL:    cfg.Storage.UseSSL,
		Public:    cfg.Storage.Public,
	})
	if err != nil {
		slog.Warn("Storage connection failed, running without file storage", "error", err)
	} else {
		defer storageClient.Close()
	}

	authService := auth.NewAuthService(
		cfg.Auth.SecretKey,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
	)

	// Initialize WebSocket hub
	hub := websocket.NewHub(roomManager, authService)

	// Set database client for permission checks (Phase 11)
	if dbClient != nil {
		hub.SetDBClient(websocket.NewDBClientAdapter(dbClient))
	}

	// Initialize AI provider
	var aiHandler *ai.Handler
	if cfg.AI.APIKey != "" {
		var provider ai.LLMProvider
		switch cfg.AI.Provider {
		case "anthropic":
			provider = ai.NewAnthropicProvider(
				cfg.AI.APIKey,
				cfg.AI.Model,
				cfg.AI.MaxTokens,
				cfg.AI.Temperature,
			)
		default: // "openai" or any OpenAI-compatible
			provider = ai.NewOpenAIProvider(
				cfg.AI.APIKey,
				cfg.AI.BaseURL,
				cfg.AI.Model,
				cfg.AI.MaxTokens,
				cfg.AI.Temperature,
			)
		}
		aiHandler = ai.NewHandler(provider)
		aiHandler.SetProviderName(cfg.AI.Provider)

		// Enable request logging to PostgreSQL (development only)
		if cfg.IsDevelopment() && dbClient != nil {
			aiLogRepo := database.NewAIRequestLogRepository(dbClient.DB())
			aiHandler.SetRequestLogger(&aiRequestLoggerAdapter{repo: aiLogRepo})
			logger.Info("AI request logging to PostgreSQL enabled (development mode)")
		}
	}

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middlewareLogger)

	// CORS middleware - must be before route handlers
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
	}))

	// IMPORTANT: WebSocket route must be registered BEFORE AllowContentType middleware
	// because WebSocket upgrade doesn't have Content-Type header
	r.Get("/ws", hub.HandleWebSocket)

	// Routes that require JSON content type
	r.Group(func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get("/health", healthHandler)
		r.Get("/api/stats", statsHandler(roomManager))
		r.Get("/api/rooms/{id}/link", roomLinkHandler)
	})

	// Auth routes (public)
	if dbClient != nil {
		authHandler := NewAuthHandler(authService, dbClient)
		r.Post("/api/auth/register", authHandler.Register)
		r.Post("/api/auth/login", authHandler.Login)
		r.Post("/api/auth/refresh", authHandler.Refresh)
		r.Post("/api/auth/logout", authHandler.Logout)

		// Password reset routes (public)
		r.Post("/api/auth/forgot-password", authHandler.ForgotPassword)
		r.Post("/api/auth/validate-reset-token", authHandler.ValidateResetToken)
		r.Post("/api/auth/reset-password", authHandler.ResetPassword)

		// File management routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(authService))
			fileMgmtHandler := NewFileManagementHandler(dbClient, authService)
			r.Get("/api/files", fileMgmtHandler.ListUserFiles)
			r.Post("/api/files", fileMgmtHandler.CreateUserFile)
			r.Post("/api/files/migrate", fileMgmtHandler.MigrateLocalFiles)
			r.Get("/api/files/{fileId}", fileMgmtHandler.GetUserFile)
			r.Put("/api/files/{fileId}", fileMgmtHandler.UpdateUserFile)
			r.Patch("/api/files/{fileId}/rename", fileMgmtHandler.RenameUserFile)
			r.Delete("/api/files/{fileId}", fileMgmtHandler.DeleteUserFile)
			r.Put("/api/files/{fileId}/tabs", fileMgmtHandler.SaveFileTabs)
		})
	}

	// File upload/download routes (multipart support)
	if storageClient != nil {
		fileHandler := NewFileHandler(storageClient, dbClient, roomManager)
		r.Post("/api/rooms/{roomId}/files", fileHandler.Upload)
		r.Get("/api/rooms/{roomId}/files/{fileId}", fileHandler.Download)
		r.Delete("/api/rooms/{roomId}/files/{fileId}", fileHandler.Delete)
		r.Get("/api/rooms/{roomId}/files", fileHandler.ListFiles)
	}

	// Canvas save/load routes (manual persistence)
	if dbClient != nil {
		canvasHandler := NewCanvasHandler(dbClient, roomManager)
		r.Post("/api/rooms/{roomId}/canvas/save", canvasHandler.SaveCanvas)
		r.Get("/api/rooms/{roomId}/canvas/load", canvasHandler.LoadCanvas)
		r.Post("/api/rooms/{roomId}/canvas/restore", canvasHandler.RestoreCanvas)
		r.Delete("/api/rooms/{roomId}/canvas", canvasHandler.ClearCanvas)
	}

	// AI routes (optional, only if configured)
	if aiHandler != nil {
		// Rate limiter for AI endpoints: 2 requests/second, burst 6 per IP
		aiRateLimiter := appmiddleware.NewIPRateLimiter(2, 6)

		r.Route("/api/ai", func(r chi.Router) {
			r.Use(aiRateLimiter.Middleware)

			// Remove write deadline for SSE streaming (AI chat can take time)
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Try to remove write deadline for SSE
					if rc, ok := w.(interface{ SetWriteDeadline(time.Time) error }); ok {
						rc.SetWriteDeadline(time.Time{})
					}
					_ = w // avoid unused warning
					next.ServeHTTP(w, r)
				})
			})
			aiHandler.RegisterRoutes(r)

			// Development-only log endpoints
			if cfg.IsDevelopment() && dbClient != nil {
				aiLogRepo := database.NewAIRequestLogRepository(dbClient.DB())
				r.Get("/logs", aiLogsHandler(aiLogRepo))
				r.Get("/logs/stats", aiLogsStatsHandler(aiLogRepo))
				r.Delete("/logs/cleanup", aiLogsCleanupHandler(aiLogRepo))
			}
		})
	}

	// Start server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: 5 * time.Minute, // Extended for SSE streaming (AI chat can take time)
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Starting server",
			zap.String("port", cfg.Server.Port),
			zap.Bool("persistence", roomManager.HasPersistence()),
			zap.Bool("file_storage", storageClient != nil),
			zap.Bool("auth", dbClient != nil),
			zap.Bool("ai", aiHandler != nil),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	roomManager.StopPersistence()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("Server shutdown complete")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC(),
	})
}

func statsHandler(rm *room.RoomManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := rm.GetStats()
		stats["timestamp"] = time.Now().UTC()
		json.NewEncoder(w).Encode(stats)
	}
}

// roomLinkHandler handles GET /api/rooms/:id/link (Phase 5)
func roomLinkHandler(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")

	// Generate shareable link
	shareURL := fmt.Sprintf("%s?room=%s", "http://localhost:3000", roomID)

	// Response
	response := map[string]interface{}{
		"shareUrl": shareURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func initLogger(cfg config.LogConfig) (*zap.Logger, error) {
	var logger *zap.Logger
	var err error

	if cfg.Format == "json" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}

	if err != nil {
		return nil, err
	}

	// Set log level
	switch cfg.Level {
	case "debug":
		logger = logger.WithOptions(zap.IncreaseLevel(zap.DebugLevel))
	case "info":
		logger = logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))
	case "warn":
		logger = logger.WithOptions(zap.IncreaseLevel(zap.WarnLevel))
	case "error":
		logger = logger.WithOptions(zap.IncreaseLevel(zap.ErrorLevel))
	default:
		logger = logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))
	}

	return logger, nil
}

func middlewareLogger(next http.Handler) http.Handler {
	return appmiddleware.Logger(next)
}

// ─── AI Request Logger Adapter ─────────────────────────────────────────────

// aiRequestLoggerAdapter adapts database.AIRequestLogRepository to ai.RequestLogger
// This avoids importing the ai package from database or vice versa
type aiRequestLoggerAdapter struct {
	repo *database.AIRequestLogRepository
}

func (a *aiRequestLoggerAdapter) LogRequest(entry *ai.RequestLogEntry) {
	toolCallsJSON, _ := json.Marshal(entry.ToolCalls)

	dbLog := &database.AIRequestLog{
		RequestID:         entry.RequestID,
		Model:             entry.Model,
		Provider:          entry.Provider,
		UserMessage:       entry.UserMessage,
		SystemPrompt:      entry.SystemPrompt,
		CanvasElementCount: entry.CanvasElementCount,
		ToolsCount:        entry.ToolsCount,
		ResponseText:      entry.ResponseText,
		ToolCalls:         toolCallsJSON,
		FinishReason:      entry.FinishReason,
		RequestDurationMs: entry.RequestDurationMs,
		PromptTokens:      entry.PromptTokens,
		CompletionTokens:  entry.CompletionTokens,
		TotalTokens:       entry.TotalTokens,
		Status:            entry.Status,
		ErrorMessage:      entry.ErrorMessage,
		ClientIP:          entry.ClientIP,
		UserAgent:         entry.UserAgent,
	}

	if err := a.repo.Insert(dbLog); err != nil {
		slog.Error("[AI Log] Failed to insert request log", "error", err)
		return
	}

	// Copy back the generated ID and timestamps
	entry.ID = dbLog.ID
}

func (a *aiRequestLoggerAdapter) UpdateLog(entry *ai.RequestLogEntry) {
	toolCallsJSON, _ := json.Marshal(entry.ToolCalls)

	dbLog := &database.AIRequestLog{
		ID:                entry.ID,
		ResponseText:      entry.ResponseText,
		ToolCalls:         toolCallsJSON,
		FinishReason:      entry.FinishReason,
		RequestDurationMs: entry.RequestDurationMs,
		PromptTokens:      entry.PromptTokens,
		CompletionTokens:  entry.CompletionTokens,
		TotalTokens:       entry.TotalTokens,
		Status:            entry.Status,
		ErrorMessage:      entry.ErrorMessage,
	}

	if err := a.repo.Update(dbLog); err != nil {
		slog.Error("[AI Log] Failed to update request log", "error", err, "log_id", entry.ID)
	}
}

// ─── AI Log API Handlers (Development Only) ───────────────────────────────

func aiLogsHandler(repo *database.AIRequestLogRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
				limit = parsed
			}
		}

		logs, err := repo.GetRecent(limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":  logs,
			"count": len(logs),
		})
	}
}

func aiLogsStatsHandler(repo *database.AIRequestLogRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := repo.GetStats()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func aiLogsCleanupHandler(repo *database.AIRequestLogRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deleted, err := repo.CleanupOldLogs(7 * 24 * time.Hour)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted": deleted,
			"message": fmt.Sprintf("Cleaned up %d old log entries", deleted),
		})
	}
}
