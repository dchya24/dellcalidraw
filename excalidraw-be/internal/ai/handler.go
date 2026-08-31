package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/you/excalidraw-be/internal/ai/memory"
)

// RequestLogger is an interface for logging AI requests (development only)
type RequestLogger interface {
	LogRequest(log *RequestLogEntry)
	UpdateLog(log *RequestLogEntry)
}

// RequestLogEntry holds data to be logged
// This is a duplicate-free struct that maps to database.AIRequestLog
// without importing the database package directly
type RequestLogEntry struct {
	ID                 int64         `json:"id,omitempty"`
	RequestID          string        `json:"request_id"`
	Model              string        `json:"model"`
	Provider           string        `json:"provider"`
	UserMessage        string        `json:"user_message"`
	SystemPrompt       string        `json:"system_prompt,omitempty"`
	CanvasElementCount int           `json:"canvas_element_count"`
	ToolsCount         int           `json:"tools_count"`
	ResponseText       string        `json:"response_text,omitempty"`
	ToolCalls          []ToolCallLog `json:"tool_calls,omitempty"`
	FinishReason       string        `json:"finish_reason,omitempty"`
	RequestDurationMs  int           `json:"request_duration_ms,omitempty"`
	PromptTokens       int           `json:"prompt_tokens,omitempty"`
	CompletionTokens   int           `json:"completion_tokens,omitempty"`
	TotalTokens        int           `json:"total_tokens,omitempty"`
	Status             string        `json:"status"`
	ErrorMessage       string        `json:"error_message,omitempty"`
	ClientIP           string        `json:"client_ip,omitempty"`
	UserAgent          string        `json:"user_agent,omitempty"`
}

// ToolCallLog represents a logged tool call (for JSON serialization)
type ToolCallLog struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"arguments"`
}

// Handler holds AI HTTP handlers
type Handler struct {
	provider        LLMProvider
	tools           []Tool
	requestLogger   RequestLogger // nil if not in development mode
	providerName    string
	retriever       *memory.Retriever
	ingester        *memory.Ingester
	resolveIdentity func(*http.Request) (string, string)
	maxMemoryTokens int
	agentReg        *Registry
}

// NewHandler creates new AI handler
func NewHandler(provider LLMProvider) *Handler {
	return &Handler{
		provider: provider,
		tools:    GetDefaultTools(),
	}
}

// SetRequestLogger sets the request logger (development only)
func (h *Handler) SetRequestLogger(logger RequestLogger) {
	h.requestLogger = logger
}

// SetProviderName sets the provider name for logging
func (h *Handler) SetProviderName(name string) {
	h.providerName = name
}

func (h *Handler) SetRetriever(r *memory.Retriever) { h.retriever = r }
func (h *Handler) SetIngester(i *memory.Ingester)   { h.ingester = i }
func (h *Handler) SetIdentityResolver(f func(*http.Request) (string, string)) {
	h.resolveIdentity = f
}
func (h *Handler) SetMaxMemoryTokens(n int)     { h.maxMemoryTokens = n }
func (h *Handler) SetAgentRegistry(r *Registry) { h.agentReg = r }

// RegisterRoutes registers AI routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/chat", h.HandleChat)
	r.Post("/tool-result", h.HandleToolResult)
	r.Get("/models", h.HandleModels)
	r.Get("/health", h.HandleHealth)
}

// HandleChat handles AI chat requests with SSE streaming
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers BEFORE anything else
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")          // Disable nginx buffering
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins for SSE

	// Write a 200 OK status so the client sees a successful response
	w.WriteHeader(http.StatusOK)

	// Flush headers to client immediately
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("[AI Handler] Failed to read request body", "error", err)
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"content\":\"Failed to read request\"}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Error("[AI Handler] Failed to parse request", "error", err)
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"content\":\"Invalid request JSON\"}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	// Build canvas context
	canvasElements := []interface{}{}
	if req.CanvasContext != nil {
		if elements, ok := req.CanvasContext["elements"].([]interface{}); ok {
			canvasElements = elements
		}
	}

	// Build messages: use client transcript if provided, else system+user.
	var messages []Message
	if len(req.Messages) > 0 {
		// Trust the transcript but always ensure a system message first.
		if req.Messages[0].Role != "system" {
			messages = append([]Message{{Role: "system", Content: BuildSystemPrompt(canvasElements)}}, req.Messages...)
		} else {
			// Keep client's system message but append canvas context section.
			req.Messages[0].Content = BuildSystemPrompt(canvasElements)
			messages = req.Messages
		}
	} else {
		messages = []Message{
			{Role: "system", Content: BuildSystemPrompt(canvasElements)},
			{Role: "user", Content: req.Message},
		}
	}
	// The new user message is always appended (transcript excludes it).
	if len(messages) == 0 || messages[len(messages)-1].Content != req.Message {
		messages = append(messages, Message{Role: "user", Content: req.Message})
	}

	// Use model from request if provided and valid, otherwise use server default
	model := h.provider.DefaultModel()
	if req.Model != "" {
		validModels := h.provider.GetModels()
		for _, m := range validModels {
			if m == req.Model {
				model = req.Model
				break
			}
		}
	}
	slog.Info("[AI Handler] Processing chat request", "message", req.Message, "model", model, "canvas_elements", len(canvasElements))

	// Start request logging (development only)
	requestID := uuid.New().String()
	startTime := time.Now()
	var logEntry *RequestLogEntry

	if h.requestLogger != nil {
		logEntry = &RequestLogEntry{
			RequestID:          requestID,
			Model:              model,
			Provider:           h.providerName,
			UserMessage:        req.Message,
			SystemPrompt:       truncate(BuildSystemPrompt(canvasElements), 5000),
			CanvasElementCount: len(canvasElements),
			ToolsCount:         len(h.tools),
			Status:             "pending",
			ClientIP:           r.RemoteAddr,
			UserAgent:          r.UserAgent(),
		}
		h.requestLogger.LogRequest(logEntry)
		slog.Info("[AI Log] Request logged", "request_id", requestID, "log_id", logEntry.ID)
	}

	// Stream response via SSE
	ctx := r.Context()
	// Track response data for logging
	var responseText strings.Builder
	var toolCalls []ToolCallLog
	var tokenUsage *Usage

	// Retrieve memory if the handler is configured for it.
	var memEntries []memory.MemoryEntry
	if h.retriever != nil && h.resolveIdentity != nil {
		userID, roomID := h.resolveIdentity(r)
		if userID != "" {
			owners := []memory.Owner{memory.UserOwner(userID)}
			if roomID != "" {
				owners = append(owners, memory.RoomOwner(roomID))
			}
			ctxRetrieve, cancel := context.WithTimeout(ctx, 5*time.Second)
			got, err := h.retriever.Retrieve(ctxRetrieve, req.Message, owners)
			cancel()
			if err != nil {
				slog.Warn("[AI Handler] memory retrieve failed, continuing without", "error", err)
			} else {
				memEntries = got
			}
		}
	}

	sendEvent := func(event SSEEvent) error {
		slog.Info("[AI Handler] Sending event", "type", event.Type, "content", event.Content)

		// Accumulate response data for logging
		if h.requestLogger != nil {
			switch event.Type {
			case "text":
				responseText.WriteString(event.Content)
			case "tool_call":
				args, _ := event.Result.(map[string]interface{})
				if args == nil {
					args = map[string]interface{}{}
				}
				toolCalls = append(toolCalls, ToolCallLog{
					ID:   event.ID,
					Name: event.Name,
					Args: args,
				})
			case "usage":
				if event.Usage != nil {
					tokenUsage = event.Usage
				}
			case "error":
				if logEntry != nil {
					logEntry.Status = "error"
					logEntry.ErrorMessage = event.Content
				}
			}
		} else if event.Type == "usage" && event.Usage != nil {
			// Even without a logger we still need to forward usage to the client.
			tokenUsage = event.Usage
		}
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		// Use fmt.Fprintf with explicit flush
		_, err = fmt.Fprintf(w, "data: %s\n\n", data)
		if err != nil {
			slog.Warn("[AI Handler] Failed to write SSE data", "error", err)
			return err
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return nil
	}

	// Emit the start event so the frontend knows the requestId.
	_ = sendEvent(SSEEvent{Type: "start", Content: requestID, Result: 20})

	// Create a loop state and run the first iteration.
	hasToolCalls := false
	trackingSend := func(event SSEEvent) error {
		if event.Type == "tool_call" {
			hasToolCalls = true
		}
		return sendEvent(event)
	}

	state := &LoopState{
		RequestID: requestID,
		Messages:  messages,
		Tools:     h.tools,
		Model:     model,
		Provider:  h.provider,
		Memory:    memEntries,
		Send:      trackingSend,
		Done:      make(chan struct{}),
	}
	h.agentReg.GetOrCreate(requestID, func() *LoopState { return state })

	slog.Info("[AI Handler] Starting agent loop...")
	err = AgentRun(ctx, state, func(reason string) {
		_ = sendEvent(SSEEvent{Type: "agent_final", Content: reason})
		SendDone(state)
	})
	if err != nil {
		slog.Error("[AI Handler] agent loop error", "error", err)
		if !strings.Contains(err.Error(), "context canceled") {
			_ = sendEvent(SSEEvent{
				Type:    "error",
				Content: fmt.Sprintf("AI provider error: %v", err),
			})
		}
		h.agentReg.Drop(requestID)

		if h.requestLogger != nil && logEntry != nil {
			logEntry.Status = "error"
			logEntry.ErrorMessage = truncate(err.Error(), 2000)
			logEntry.RequestDurationMs = int(time.Since(startTime).Milliseconds())
			logEntry.ResponseText = responseText.String()
			h.requestLogger.UpdateLog(logEntry)
		}
		return
	}

	// If tool_call events were emitted, the frontend will process them
	// and POST results to /tool-result. The handler returns here,
	// leaving the SSE connection open for subsequent iterations.
	if hasToolCalls {
		slog.Info("[AI Handler] Iteration complete, waiting for tool results")
		return
	}

	// Text-only response: loop is done. Send final events and clean up.
	slog.Info("[AI Handler] Agent loop complete (text-only)")
	h.agentReg.Drop(requestID)
	_ = sendEvent(SSEEvent{Type: "agent_final", Content: "stop"})
	SendDone(state)

	// Ingest the conversation into memory (async, best-effort).
	if h.ingester != nil && h.resolveIdentity != nil {
		userID, roomID := h.resolveIdentity(r)
		if userID != "" {
			tabID, _ := req.CanvasContext["activeTabId"].(string)
			transcript := "User: " + req.Message + "\nAssistant: " + responseText.String()
			h.ingester.IngestAsync(memory.IngestRequest{
				UserID: userID, RoomID: roomID, TabID: tabID,
				Transcript: transcript, RequestID: requestID, Model: model,
			})
		}
	}

	// Finalize request log (development only)
	if h.requestLogger != nil && logEntry != nil {
		logEntry.Status = "success"
		logEntry.RequestDurationMs = int(time.Since(startTime).Milliseconds())
		logEntry.ResponseText = responseText.String()
		logEntry.ToolCalls = toolCalls
		logEntry.FinishReason = "stop"
		if tokenUsage != nil {
			logEntry.PromptTokens = tokenUsage.PromptTokens
			logEntry.CompletionTokens = tokenUsage.CompletionTokens
			logEntry.TotalTokens = tokenUsage.TotalTokens
		}
		h.requestLogger.UpdateLog(logEntry)
		slog.Info("[AI Log] Request completed",
			"request_id", requestID,
			"duration_ms", logEntry.RequestDurationMs,
			"tool_calls", len(toolCalls),
			"prompt_tokens", logEntry.PromptTokens,
			"completion_tokens", logEntry.CompletionTokens,
		)
	}

	// Final flush to ensure all data is sent
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// HandleModels returns available models
func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models":      h.provider.GetModels(),
		"activeModel": h.provider.DefaultModel(),
	})
}

// HandleHealth returns AI service health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"models":      h.provider.GetModels(),
		"activeModel": h.provider.DefaultModel(),
	})
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

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ToolResultRequest is the body of POST /api/ai/tool-result.
type ToolResultRequest struct {
	RequestID string                  `json:"requestId"`
	Results   []browserToolResultBody `json:"results"`
}

// browserToolResultBody carries the outcome of one tool call executed
// in the browser.
type browserToolResultBody struct {
	CallID  string `json:"callId"`
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Result  any    `json:"result"`
	Error   string `json:"error"`
}

// HandleToolResult receives tool results from the browser, appends
// them to the loop state, and triggers the next agent iteration.
func (h *Handler) HandleToolResult(w http.ResponseWriter, r *http.Request) {
	if h.agentReg == nil {
		http.Error(w, "agent not configured", http.StatusServiceUnavailable)
		return
	}

	var req ToolResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.RequestID == "" {
		http.Error(w, "missing requestId", http.StatusBadRequest)
		return
	}

	v := h.agentReg.Get(req.RequestID)
	if v == nil {
		http.Error(w, "unknown requestId", http.StatusNotFound)
		return
	}
	state := v

	// Append tool messages; preserve order.
	for _, res := range req.Results {
		payload, _ := json.Marshal(map[string]any{
			"success": res.Success,
			"result":  res.Result,
			"error":   res.Error,
		})
		state.AppendPending(Message{
			Role:       "tool",
			ToolCallID: res.CallID,
			Name:       res.Name,
			Content:    string(payload),
		})
	}

	w.WriteHeader(http.StatusOK)

	// Trigger the next iteration in a goroutine. The goroutine drains
	// pending results, folds them into state.Messages, calls Run, and
	// cleans up when the loop finishes.
	go func() {
		for {
			drain := state.DrainPending()
			for _, m := range drain {
				state.Messages = append(state.Messages, m)
			}

			err := AgentRun(r.Context(), state, func(reason string) {
				_ = state.Send(SSEEvent{Type: "agent_final", Content: reason})
				SendDone(state)
			})
			if err != nil {
				slog.Warn("[ai/agent] continuation failed", "error", err)
				break
			}

			// If the loop ended (wrap-up or error), we're done.
			if state.EndReason != "" {
				break
			}

			// If no new tool_calls were emitted (only text), we're done.
			// The frontend will not POST more results.
			// We detect this by checking if any pending results exist.
			// If not, the provider emitted text and we should stop.
			if len(state.DrainPending()) == 0 {
				// No more tool results to process. Check if the provider
				// emitted tool_calls in this iteration by looking at the
				// state.Messages — if the last assistant message has no
				// tool_call_id, the provider emitted text and we're done.
				break
			}
		}

		h.agentReg.Drop(req.RequestID)
	}()
}
