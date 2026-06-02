package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// OpenAIProvider implements LLMProvider for OpenAI-compatible APIs
type OpenAIProvider struct {
	BaseURL     string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, baseURL, model string, maxTokens int, temperature float64) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o"
	}
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return &OpenAIProvider{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
}

// OpenAI request/response types
type openAIRequest struct {
	Model         string              `json:"model"`
	Messages      []openAIMessage     `json:"messages"`
	Tools         []openAITool        `json:"tools,omitempty"`
	ToolChoice    any                 `json:"tool_choice,omitempty"`
	MaxTokens     int                 `json:"max_tokens,omitempty"`
	Temperature   float64             `json:"temperature,omitempty"`
	Stream        bool                `json:"stream,omitempty"`
	StreamOptions *openAIStreamOptions `json:"stream_options,omitempty"`
}

type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Chat implements LLMProvider.Chat (non-streaming)
func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message, tools []Tool, model string) (*ChatResult, error) {
	if model == "" {
		model = p.Model
	}

	req := openAIRequest{
		Model:       model,
		Messages:    convertMessages(messages),
		MaxTokens:   p.MaxTokens,
		Temperature: p.Temperature,
	}

	if len(tools) > 0 {
		req.Tools = convertTools(tools)
		req.ToolChoice = "required"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.BaseURL, "/"))
	slog.Info("[OpenAI] Sending request", "url", reqURL, "model", model, "messages", len(messages))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	slog.Info("[OpenAI] Making HTTP request to", "url", reqURL)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		slog.Error("[OpenAI] HTTP request failed", "error", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("[OpenAI] Response status", "status", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("[OpenAI] API error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	slog.Info("[OpenAI] Full response body:", "length", len(respBody), "body", string(respBody))

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if openAIResp.Error != nil {
		return nil, fmt.Errorf("OpenAI error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	choice := openAIResp.Choices[0]
	result := &ChatResult{
		Content:   choice.Message.Content,
		ToolCalls: make([]ToolCall, 0),
	}

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			continue
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return result, nil
}

// ─── Streaming ──────────────────────────────────────────────────────────────

// pendingToolCall accumulates streamed tool call fragments
type pendingToolCall struct {
	ID   string
	Name string
	Args strings.Builder
}

// ChatStream implements LLMProvider.ChatStream with proper tool call accumulation
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message, tools []Tool, model string, streamFunc func(SSEEvent) error) error {
	if model == "" {
		model = p.Model
	}

	req := openAIRequest{
		Model:         model,
		Messages:      convertMessages(messages),
		MaxTokens:     p.MaxTokens,
		Temperature:   p.Temperature,
		Stream:        true,
		StreamOptions: &openAIStreamOptions{IncludeUsage: true},
	}

	if len(tools) > 0 {
		req.Tools = convertTools(tools)
		req.ToolChoice = "required"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	slog.Info("[AI] Request body", "body", string(body))

	// Log shortened request for debugging
	reqURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.BaseURL, "/"))
	slog.Info("[AI] Request", "url", reqURL, "model", model, "msgs", len(messages), "tools", len(tools))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	slog.Info("[AI] Sending request to AI provider...")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		slog.Error("[OpenAI Stream] HTTP request failed", "error", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("[AI] Response status", "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("[OpenAI Stream] API error", "status", resp.StatusCode)
		slog.Info("[OpenAI Stream] ===== FULL ERROR RESPONSE =====")
		slog.Info("[OpenAI Stream] Error body:", "body", string(respBody))
		slog.Info("[OpenAI Stream] ================================")
		return fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	slog.Info("[OpenAI Stream] Starting to stream response...")

	// Track response for logging (pre-allocate)
	var fullResponse strings.Builder
	fullResponse.Grow(4096)

	// Stream directly from response body (don't read it first!)
	pendingCalls := make(map[int]*pendingToolCall)
	lineBuf := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	reader := resp.Body

	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			lineBuf = append(lineBuf, buf[:n]...)

			for {
				lineEnd := bytes.IndexByte(lineBuf, '\n')
				if lineEnd < 0 {
					break
				}

				line := string(lineBuf[:lineEnd])
				lineBuf = lineBuf[lineEnd+1:]
				line = strings.TrimSpace(line)

					slog.Info("[AI] Stream line", "line", line)

				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					// Flush any remaining pending tool calls
					for _, pc := range pendingCalls {
						if err := flushToolCall(pc, streamFunc); err != nil {
							return err
						}
					}
					continue
				}

				// Log each SSE event
				fullResponse.WriteString(data)
				fullResponse.WriteByte('\n')

				if err := p.processStreamChunk(data, pendingCalls, streamFunc); err != nil {
					return err
				}
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	// Log response summary (not full content)
	respStr := fullResponse.String()
	if len(respStr) > 200 {
		slog.Info("[AI] Response preview", "data", respStr[:200]+"...")
	} else if len(respStr) > 0 {
		slog.Info("[AI] Response data", "data", respStr)
	}

	// Flush any remaining
	for _, pc := range pendingCalls {
		if err := flushToolCall(pc, streamFunc); err != nil {
			return err
		}
	}

	return nil
}

// processStreamChunk handles a single SSE data chunk
func (p *OpenAIProvider) processStreamChunk(data string, pendingCalls map[int]*pendingToolCall, streamFunc func(SSEEvent) error) error {
	var chunk struct {
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage,omitempty"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil // skip malformed
	}

	// OpenAI emits a final chunk with empty choices and a populated usage.
	if chunk.Usage != nil {
		if err := streamFunc(SSEEvent{
			Type: "usage",
			Usage: &Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			},
		}); err != nil {
			return err
		}
	}

	if len(chunk.Choices) == 0 {
		return nil
	}

	choice := chunk.Choices[0]
	delta := choice.Delta

	// Stream text content immediately
	if delta.Content != "" {
		if err := streamFunc(SSEEvent{
			Type:    "text",
			Content: delta.Content,
		}); err != nil {
			return err
		}
	}

	// Accumulate tool call arguments
	for _, tc := range delta.ToolCalls {
		idx := tc.Index
		pc, exists := pendingCalls[idx]

		if !exists {
			// New tool call
			pc = &pendingToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
			}
			pendingCalls[idx] = pc
		}

		// OpenAI may send ID and Name only in the first chunk
		if tc.ID != "" {
			pc.ID = tc.ID
		}
		if tc.Function.Name != "" {
			pc.Name = tc.Function.Name
		}

		// Accumulate argument fragments
		if tc.Function.Arguments != "" {
			pc.Args.WriteString(tc.Function.Arguments)
		}
	}

	// If finish_reason is "tool_calls", flush all pending
	if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
		for idx, pc := range pendingCalls {
			if err := flushToolCall(pc, streamFunc); err != nil {
				return err
			}
			delete(pendingCalls, idx)
		}
	}

	return nil
}

// flushToolCall sends accumulated tool call as SSE event
func flushToolCall(pc *pendingToolCall, streamFunc func(SSEEvent) error) error {
	if pc.Name == "" {
		return nil
	}

	var args map[string]interface{}
	rawArgs := pc.Args.String()
	if rawArgs != "" {
		json.Unmarshal([]byte(rawArgs), &args)
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	// Send tool_call event with complete arguments
	if err := streamFunc(SSEEvent{
		Type:   "tool_call",
		ID:     pc.ID,
		Name:   pc.Name,
		Result: args,
	}); err != nil {
		return err
	}

	return nil
}

// GetModels implements LLMProvider.GetModels
func (p *OpenAIProvider) GetModels() []string {
	return SupportedModels()
}

// DefaultModel returns the configured model from env
func (p *OpenAIProvider) DefaultModel() string {
	return p.Model
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func convertMessages(messages []Message) []openAIMessage {
	result := make([]openAIMessage, len(messages))
	for i, m := range messages {
		result[i] = openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}

func convertTools(tools []Tool) []openAITool {
	result := make([]openAITool, len(tools))
	for i, t := range tools {
		result[i] = openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return result
}
