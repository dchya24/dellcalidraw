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

// OpenAIProvider implements LLMProvider for OpenAI-compatible APIs
type OpenAIProvider struct {
	BaseURL    string
	APIKey     string
	Model      string
	MaxTokens  int
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
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
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
			Role         string `json:"role"`
			Content      string `json:"content"`
			ToolCalls    []struct {
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

// Chat implements LLMProvider.Chat
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
		req.ToolChoice = "auto"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.BaseURL, "/"))
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

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
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

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

	// Parse tool calls
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

// ChatStream implements LLMProvider.ChatStream
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message, tools []Tool, model string, streamFunc func(SSEEvent) error) error {
	if model == "" {
		model = p.Model
	}

	req := openAIRequest{
		Model:       model,
		Messages:    convertMessages(messages),
		MaxTokens:   p.MaxTokens,
		Temperature: p.Temperature,
		Stream:      true,
	}

	if len(tools) > 0 {
		req.Tools = convertTools(tools)
		req.ToolChoice = "auto"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.BaseURL, "/"))
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse SSE stream
	reader := io.Reader(resp.Body)
	lineBuf := make([]byte, 0, 1024)
	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			lineBuf = append(lineBuf, buf[:n]...)
			
			// Process complete lines
			for {
				lineEnd := -1
				for i, b := range lineBuf {
					if b == '\n' {
						lineEnd = i
						break
					}
				}
				
				if lineEnd < 0 {
					break // No complete line
				}
				
				line := string(lineBuf[:lineEnd])
				lineBuf = lineBuf[lineEnd+1:]
				
				// Parse SSE line
				if strings.HasPrefix(line, "data: ") {
					data := line[6:]
					if data == "[DONE]" {
						continue
					}
					
					// Parse streaming chunk
					if err := p.parseStreamChunk(data, streamFunc); err != nil {
						return err
					}
				}
			}
		}
		
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	return nil
}

// parseStreamChunk parses a streaming response chunk
func (p *OpenAIProvider) parseStreamChunk(data string, streamFunc func(SSEEvent) error) error {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil // Skip malformed chunks
	}

	if len(chunk.Choices) == 0 {
		return nil
	}

	delta := chunk.Choices[0].Delta

	// Send text content
	if delta.Content != "" {
		if err := streamFunc(SSEEvent{
			Type:    "text",
			Content: delta.Content,
		}); err != nil {
			return err
		}
	}

	// Send tool calls
	for _, tc := range delta.ToolCalls {
		var args map[string]interface{}
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		
		if err := streamFunc(SSEEvent{
			Type: "tool_call",
			ID:   tc.ID,
			Name: tc.Function.Name,
		}); err != nil {
			return err
		}
		
		// Also send tool arguments as result
		if err := streamFunc(SSEEvent{
			Type:   "tool_result",
			CallID: tc.ID,
			Name:   tc.Function.Name,
			Result: args,
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetModels implements LLMProvider.GetModels
func (p *OpenAIProvider) GetModels() []string {
	return SupportedModels()
}

// Helper functions
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