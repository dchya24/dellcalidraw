package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicProvider implements LLMProvider for Anthropic API
type AnthropicProvider struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, model string, maxTokens int, temperature float64) *AnthropicProvider {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return &AnthropicProvider{
		APIKey:      apiKey,
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
}

// Anthropic request/response types
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema struct {
		Type       string                 `json:"type"`
		Properties map[string]interface{} `json:"properties,omitempty"`
		Required   []string               `json:"required,omitempty"`
	} `json:"input_schema"`
}

// Chat implements LLMProvider.Chat (non-streaming)
func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []Tool, model string) (*ChatResult, error) {
	if model == "" {
		model = p.Model
	}

	// Extract system message
	systemPrompt := ""
	userMessages := make([]anthropicMessage, 0)
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			userMessages = append(userMessages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	req := anthropicRequest{
		Model:       model,
		Messages:    userMessages,
		System:      systemPrompt,
		MaxTokens:   p.MaxTokens,
		Temperature: p.Temperature,
	}

	if len(tools) > 0 {
		req.Tools = convertAnthropicTools(tools)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp struct {
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text,omitempty"`
			ID    string `json:"id,omitempty"`
			Name  string `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
		} `json:"content"`
	}

	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := &ChatResult{
		ToolCalls: make([]ToolCall, 0),
	}

	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			var args map[string]interface{}
			json.Unmarshal(block.Input, &args)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

// ChatStream implements LLMProvider.ChatStream for Anthropic
func (p *AnthropicProvider) ChatStream(ctx context.Context, messages []Message, tools []Tool, model string, streamFunc func(SSEEvent) error) error {
	if model == "" {
		model = p.Model
	}

	// Extract system message
	systemPrompt := ""
	userMessages := make([]anthropicMessage, 0)
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			userMessages = append(userMessages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	req := anthropicRequest{
		Model:       model,
		Messages:    userMessages,
		System:      systemPrompt,
		MaxTokens:   p.MaxTokens,
		Temperature: p.Temperature,
		Stream:      true,
	}

	if len(tools) > 0 {
		req.Tools = convertAnthropicTools(tools)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Track current tool use block
	var currentToolID string
	var currentToolName string
	var currentToolArgs strings.Builder

	// Token usage accumulators — populated from message_start (input)
	// and message_delta (output). Emitted at the end as a single 'usage'
	// SSEEvent so the client can render totals once.
	var promptTokens int
	var completionTokens int

	reader := io.Reader(resp.Body)
	lineBuf := make([]byte, 0, 4096)
	buf := make([]byte, 4096)

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

				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")

					var event struct {
						Type  string `json:"type"`
						Index int    `json:"index"`
						Delta struct {
							Type        string `json:"type"`
							Text        string `json:"text"`
							PartialJSON string `json:"partial_json"`
						} `json:"delta,omitempty"`
						ContentBlock struct {
							Type string `json:"type"`
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"content_block,omitempty"`
						Message struct {
							Usage struct {
								InputTokens  int `json:"input_tokens"`
								OutputTokens int `json:"output_tokens"`
							} `json:"usage"`
						} `json:"message,omitempty"`
						Usage struct {
							InputTokens  int `json:"input_tokens"`
							OutputTokens int `json:"output_tokens"`
						} `json:"usage,omitempty"`
					}

					if err := json.Unmarshal([]byte(data), &event); err != nil {
						continue
					}

					switch event.Type {
					case "message_start":
						// Anthropic delivers initial input/output usage on message_start
						if event.Message.Usage.InputTokens > 0 {
							promptTokens = event.Message.Usage.InputTokens
						}
						if event.Message.Usage.OutputTokens > 0 {
							completionTokens = event.Message.Usage.OutputTokens
						}

					case "message_delta":
						// message_delta carries the running output token count
						if event.Usage.OutputTokens > 0 {
							completionTokens = event.Usage.OutputTokens
						}
						if event.Usage.InputTokens > 0 {
							promptTokens = event.Usage.InputTokens
						}

					case "content_block_start":
						if event.ContentBlock.Type == "tool_use" {
							currentToolID = event.ContentBlock.ID
							currentToolName = event.ContentBlock.Name
							currentToolArgs.Reset()
						}

					case "content_block_delta":
						if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
							if err := streamFunc(SSEEvent{
								Type:    "text",
								Content: event.Delta.Text,
							}); err != nil {
								return err
							}
						}
						if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
							currentToolArgs.WriteString(event.Delta.PartialJSON)
						}

					case "content_block_stop":
						if currentToolName != "" {
							var args map[string]interface{}
							rawArgs := currentToolArgs.String()
							if rawArgs != "" {
								json.Unmarshal([]byte(rawArgs), &args)
							}
							if args == nil {
								args = map[string]interface{}{}
							}

							if err := streamFunc(SSEEvent{
								Type:   "tool_call",
								ID:     currentToolID,
								Name:   currentToolName,
								Result: args,
							}); err != nil {
								return err
							}

							currentToolID = ""
							currentToolName = ""
							currentToolArgs.Reset()
						}

					case "message_stop":
						// Done
					}
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

	// Emit accumulated usage once the stream is fully drained.
	if promptTokens > 0 || completionTokens > 0 {
		if err := streamFunc(SSEEvent{
			Type: "usage",
			Usage: &Usage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      promptTokens + completionTokens,
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetModels implements LLMProvider.GetModels
func (p *AnthropicProvider) GetModels() []string {
	return SupportedModels()
}

// DefaultModel returns the configured model from env
func (p *AnthropicProvider) DefaultModel() string {
	return p.Model
}

// convertAnthropicTools converts tools to Anthropic format
func convertAnthropicTools(tools []Tool) []anthropicTool {
	result := make([]anthropicTool, len(tools))
	for i, t := range tools {
		result[i] = anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return result
}
