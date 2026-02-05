package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/you/excalidraw-be/internal/config"
	appmiddleware "github.com/you/excalidraw-be/internal/middleware"
	"github.com/you/excalidraw-be/internal/room"
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

	// Initialize WebSocket hub
	hub := websocket.NewHub(roomManager)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middlewareLogger)

	// IMPORTANT: WebSocket route must be registered BEFORE AllowContentType middleware
	// because WebSocket upgrade doesn't have Content-Type header
	r.Get("/ws", hub.HandleWebSocket)

	// Routes that require JSON content type
	r.Group(func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get("/health", healthHandler)
		r.Get("/api/stats", statsHandler(roomManager))
	})

	// Start server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Starting server",
			zap.String("port", cfg.Server.Port),
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
