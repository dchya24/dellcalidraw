package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler holds AI HTTP handlers
type Handler struct {
	provider LLMProvider
	tools    []Tool
}

// NewHandler creates new AI handler
func NewHandler(provider LLMProvider) *Handler {
	return &Handler{
		provider: provider,
		tools:    GetDefaultTools(),
	}
}

// RegisterRoutes registers AI routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Post("/chat", h.HandleChat)
		r.Get("/models", h.HandleModels)
		r.Get("/health", h.HandleHealth)
	})
}

// HandleChat handles AI chat requests with SSE streaming
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid request JSON", http.StatusBadRequest)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Build messages
	canvasElements := []interface{}{}
	if req.CanvasContext != nil {
		if elements, ok := req.CanvasContext["elements"].([]interface{}); ok {
			canvasElements = elements
		}
	}

	messages := []Message{
		{Role: "system", Content: BuildSystemPrompt(canvasElements)},
		{Role: "user", Content: req.Message},
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	// Stream response
	ctx := r.Context()
	sendEvent := func(event SSEEvent) error {
		data, _ := json.Marshal(event)
		_, err := fmt.Fprintf(w, "data: %s\n\n", string(data))
		if err == nil {
			flusher.Flush()
		}
		return err
	}

	err = h.provider.ChatStream(ctx, messages, h.tools, model, sendEvent)
	if err != nil {
		// Send error event
		if !strings.Contains(err.Error(), "context canceled") {
			sendEvent(SSEEvent{
				Type:    "error",
				Content: err.Error(),
			})
		}
		return
	}

	// Send done event
	sendEvent(SSEEvent{
		Type:    "done",
		Content: "Generation complete",
	})
}

// HandleModels returns available models
func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	models := h.provider.GetModels()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
	})
}

// HandleHealth returns AI service health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"models":  h.provider.GetModels(),
	})
}

// SendSSENotice sends a notice message (non-streaming fallback)
func SendSSENotice(w http.ResponseWriter, message string) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("flusher not available")
	}
	
		data, _ := json.Marshal(SSEEvent{
		Type:    "text",
		Content: message,
	})
	
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()
	return nil
}

// SendSSEError sends an error event
func SendSSEError(w http.ResponseWriter, message string) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("flusher not available")
	}
	
	data, _ := json.Marshal(SSEEvent{
		Type:    "error",
		Content: message,
	})
	
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()
	return nil
}

// ValidateAPIKey validates the API key is configured
func (h *Handler) ValidateAPIKey() bool {
	return h.provider != nil
}

// GetTools returns available tools
func (h *Handler) GetTools() []Tool {
	return h.tools
}

// GetProvider returns the LLM provider
func (h *Handler) GetProvider() LLMProvider {
	return h.provider
}

// ExecuteTool executes a tool based on name and arguments
func ExecuteTool(name string, args map[string]interface{}) ToolResult {
	switch name {
	case "create_rectangle", "create_text", "create_arrow", 
	     "create_ellipse", "create_diamond", "create_line":
		return ToolResult{
			Success: true,
			Result: map[string]interface{}{
				"type":       name,
				"id":         fmt.Sprintf("el_%d", len(args)),
				"x":          args["x"],
				"y":          args["y"],
				"width":      args["width"],
				"height":     args["height"],
				"strokeColor": "#000000",
				"backgroundColor": "#ffffff",
			},
		}
	case "get_canvas_state":
		return ToolResult{
			Success: true,
			Result: map[string]interface{}{
				"elementCount": 0,
				"message":      "Canvas state retrieved",
			},
		}
	default:
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Unknown tool: %s", name),
		}
	}
}

// Context key for request ID
type contextKey string

const requestIDKey contextKey = "requestID"

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// GetRequestID gets request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}