package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const summarizeSystemPrompt = `You are a memory summarizer for an Excalidraw AI assistant.
Given a conversation transcript, produce 1-3 short paragraphs (max ~250 words)
covering:
- the topic of the conversation
- key decisions or parameters for any diagram the user asked for
- the user's apparent style preferences (colors, layout, language)
- any concrete element IDs the user explicitly cared about

Output plain text only. No markdown. No preamble.`

type OpenAISummarizer struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewOpenAISummarizer(apiKey, baseURL, model string) *OpenAISummarizer {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAISummarizer{
		APIKey: apiKey, BaseURL: baseURL, Model: model,
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string    `json:"model"`
	Messages []chatMsg `json:"messages"`
}

type chatResp struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *OpenAISummarizer) Summarize(ctx context.Context, transcript string) (string, error) {
	body, err := json.Marshal(chatReq{
		Model: s.Model,
		Messages: []chatMsg{
			{Role: "system", Content: summarizeSystemPrompt},
			{Role: "user", Content: transcript},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST",
		strings.TrimSuffix(s.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("summarizer %d: %s", resp.StatusCode, string(b))
	}
	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("summarizer error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("summarizer: no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
