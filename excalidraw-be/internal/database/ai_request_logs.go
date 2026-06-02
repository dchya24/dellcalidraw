package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// AIRequestLog represents a logged AI API request
type AIRequestLog struct {
	ID                int64           `json:"id"`
	CreatedAt         time.Time       `json:"created_at"`
	RequestID         string          `json:"request_id"`
	Model             string          `json:"model"`
	Provider          string          `json:"provider"`
	UserMessage       string          `json:"user_message"`
	SystemPrompt      string          `json:"system_prompt,omitempty"`
	CanvasElementCount int            `json:"canvas_element_count"`
	ToolsCount        int             `json:"tools_count"`
	ResponseText      string          `json:"response_text,omitempty"`
	ToolCalls         json.RawMessage `json:"tool_calls,omitempty"`
	FinishReason      string          `json:"finish_reason,omitempty"`
	RequestDurationMs int             `json:"request_duration_ms,omitempty"`
	PromptTokens      int             `json:"prompt_tokens,omitempty"`
	CompletionTokens  int             `json:"completion_tokens,omitempty"`
	TotalTokens       int             `json:"total_tokens,omitempty"`
	Status            string          `json:"status"`
	ErrorMessage      string          `json:"error_message,omitempty"`
	ClientIP          string          `json:"client_ip,omitempty"`
	UserAgent         string          `json:"user_agent,omitempty"`
}

// AIRequestLogRepository handles persistence of AI request logs
type AIRequestLogRepository struct {
	db *sql.DB
}

// NewAIRequestLogRepository creates a new repository
func NewAIRequestLogRepository(db *sql.DB) *AIRequestLogRepository {
	return &AIRequestLogRepository{db: db}
}

// Insert creates a new AI request log entry
func (r *AIRequestLogRepository) Insert(log *AIRequestLog) error {
	query := `
		INSERT INTO ai_request_logs (
			request_id, model, provider,
			user_message, system_prompt, canvas_element_count, tools_count,
			response_text, tool_calls, finish_reason,
			request_duration_ms, prompt_tokens, completion_tokens, total_tokens,
			status, error_message,
			client_ip, user_agent
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id, created_at`

	var toolCallsJSON interface{}
	if log.ToolCalls != nil {
		toolCallsJSON = log.ToolCalls
	}

	err := r.db.QueryRow(query,
		log.RequestID, log.Model, log.Provider,
		log.UserMessage, log.SystemPrompt, log.CanvasElementCount, log.ToolsCount,
		log.ResponseText, toolCallsJSON, log.FinishReason,
		log.RequestDurationMs, log.PromptTokens, log.CompletionTokens, log.TotalTokens,
		log.Status, log.ErrorMessage,
		log.ClientIP, log.UserAgent,
	).Scan(&log.ID, &log.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert AI request log: %w", err)
	}

	slog.Debug("[AI Log] Stored request log",
		"request_id", log.RequestID,
		"model", log.Model,
		"status", log.Status,
		"duration_ms", log.RequestDurationMs,
	)
	return nil
}

// Update updates an existing AI request log (for streaming completion)
func (r *AIRequestLogRepository) Update(log *AIRequestLog) error {
	query := `
		UPDATE ai_request_logs SET
			response_text = $2,
			tool_calls = $3,
			finish_reason = $4,
			request_duration_ms = $5,
			prompt_tokens = $6,
			completion_tokens = $7,
			total_tokens = $8,
			status = $9,
			error_message = $10
		WHERE id = $1`

	var toolCallsJSON interface{}
	if log.ToolCalls != nil {
		toolCallsJSON = log.ToolCalls
	}

	_, err := r.db.Exec(query,
		log.ID,
		log.ResponseText,
		toolCallsJSON,
		log.FinishReason,
		log.RequestDurationMs,
		log.PromptTokens,
		log.CompletionTokens,
		log.TotalTokens,
		log.Status,
		log.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to update AI request log: %w", err)
	}
	return nil
}

// GetRecent returns the most recent N logs
func (r *AIRequestLogRepository) GetRecent(limit int) ([]*AIRequestLog, error) {
	query := `
		SELECT id, created_at, request_id, model, provider,
			user_message, canvas_element_count, tools_count,
			response_text, finish_reason,
			request_duration_ms, prompt_tokens, completion_tokens, total_tokens,
			status, error_message, client_ip
		FROM ai_request_logs
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query AI request logs: %w", err)
	}
	defer rows.Close()

	var logs []*AIRequestLog
	for rows.Next() {
		log := &AIRequestLog{}
		err := rows.Scan(
			&log.ID, &log.CreatedAt, &log.RequestID, &log.Model, &log.Provider,
			&log.UserMessage, &log.CanvasElementCount, &log.ToolsCount,
			&log.ResponseText, &log.FinishReason,
			&log.RequestDurationMs, &log.PromptTokens, &log.CompletionTokens, &log.TotalTokens,
			&log.Status, &log.ErrorMessage, &log.ClientIP,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan AI request log: %w", err)
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// CleanupOldLogs removes logs older than the given duration
func (r *AIRequestLogRepository) CleanupOldLogs(olderThan time.Duration) (int64, error) {
	result, err := r.db.Exec(
		`DELETE FROM ai_request_logs WHERE created_at < NOW() - $1::interval`,
		fmt.Sprintf("%d seconds", int(olderThan.Seconds())),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup AI request logs: %w", err)
	}
	return result.RowsAffected()
}

// GetStats returns summary statistics of AI request logs
func (r *AIRequestLogRepository) GetStats() (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COUNT(*) FILTER (WHERE status = 'success') as successful,
			COUNT(*) FILTER (WHERE status = 'error') as errors,
			COALESCE(AVG(request_duration_ms), 0) as avg_duration_ms,
			COALESCE(SUM(total_tokens), 0) as total_tokens_used,
			COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as total_completion_tokens
		FROM ai_request_logs
		WHERE created_at > NOW() - INTERVAL '24 hours'`

	var total, successful, errors int
	var avgDuration float64
	var totalTokens, promptTokens, completionTokens int

	err := r.db.QueryRow(query).Scan(
		&total, &successful, &errors,
		&avgDuration,
		&totalTokens, &promptTokens, &completionTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI log stats: %w", err)
	}

	return map[string]interface{}{
		"total_requests_24h":        total,
		"successful_requests_24h":   successful,
		"error_requests_24h":        errors,
		"avg_duration_ms_24h":       avgDuration,
		"total_tokens_24h":          totalTokens,
		"prompt_tokens_24h":         promptTokens,
		"completion_tokens_24h":     completionTokens,
	}, nil
}
