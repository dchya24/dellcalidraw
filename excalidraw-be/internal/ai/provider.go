package ai

import (
	"context"
	"encoding/json"
)

// ToolCall represents a tool call from LLM
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult represents execution result of a tool
type ToolResult struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SSEEvent represents Server-Sent Events for streaming
type SSEEvent struct {
	Type    string      `json:"type"` // text, tool_call, tool_result, done, error
	Content string      `json:"content,omitempty"`
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name,omitempty"`
	CallID  string      `json:"callId,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ChatRequest represents incoming chat request
type ChatRequest struct {
	Message       string                 `json:"message"`
	CanvasContext map[string]interface{} `json:"canvasContext"`
	Model         string                 `json:"model,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
}

// ChatResponse represents non-streaming response
type ChatResponse struct {
	Response string                   `json:"response"`
	Elements []map[string]interface{} `json:"elements,omitempty"`
}

// LLMProvider defines interface for LLM providers
type LLMProvider interface {
	// Chat sends a message and returns the response
	Chat(ctx context.Context, messages []Message, tools []Tool, model string) (*ChatResult, error)
	
	// ChatStream sends a message and streams the response via SSE
	ChatStream(ctx context.Context, messages []Message, tools []Tool, model string, streamFunc func(SSEEvent) error) error
	
	// GetModels returns available models
	GetModels() []string
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // system, user, assistant, tool
	Content string `json:"content"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema struct {
		Type       string                 `json:"type"`
		Properties map[string]interface{} `json:"properties,omitempty"`
		Required   []string               `json:"required,omitempty"`
	} `json:"input_schema"`
}

// ChatResult represents LLM chat result
type ChatResult struct {
	Content   string
	ToolCalls []ToolCall
}

// BuildSystemPrompt creates system prompt with canvas context
func BuildSystemPrompt(canvasElements []interface{}) string {
	return `You are an AI assistant that helps users create and modify diagrams in Excalidraw.

You have access to tools that can create Excalidraw elements on a canvas. When a user asks you to create a diagram:
1. Use the appropriate tool (create_rectangle, create_text, create_arrow, create_ellipse, create_diamond)
2. Position elements logically with reasonable spacing
3. Add connecting arrows for flowcharts

Available tools:
- create_rectangle: Create a rectangle. Args: x, y, width, height, label (optional), strokeColor (optional), backgroundColor (optional)
- create_text: Create text element. Args: x, y, text, fontSize (optional), strokeColor (optional)
- create_arrow: Create an arrow. Args: startX, startY, endX, endY, label (optional), strokeColor (optional)
- create_ellipse: Create an ellipse. Args: x, y, width, height, label (optional), strokeColor (optional), backgroundColor (optional)
- create_diamond: Create a diamond shape. Args: x, y, width, height, label (optional), strokeColor (optional), backgroundColor (optional)
- create_line: Create a line. Args: points (array of [x,y] arrays), strokeColor (optional)
- get_canvas_state: Get current canvas state. Returns: element count, types, bounding box

IMPORTANT:
- Use tool_calls when you need to create elements, don't just describe
- Position elements starting around x=100, y=100 with reasonable spacing
- For flowcharts, use rectangles for processes, diamonds for decisions, ellipses for start/end
- Connect elements with arrows

Current canvas has ` + formatElementCount(canvasElements) + ` element(s).`
}

func formatElementCount(elements []interface{}) string {
	if len(elements) == 0 {
		return "no"
	}
	return jsonString(len(elements))
}

func jsonString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// GetDefaultTools returns list of available MCP tools
func GetDefaultTools() []Tool {
	return []Tool{
		{
			Name:        "create_rectangle",
			Description: "Create a rectangle element on the canvas",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":              map[string]interface{}{"type": "number", "description": "X position"},
					"y":              map[string]interface{}{"type": "number", "description": "Y position"},
					"width":          map[string]interface{}{"type": "number", "description": "Width of rectangle"},
					"height":         map[string]interface{}{"type": "number", "description": "Height of rectangle"},
					"label":          map[string]interface{}{"type": "string", "description": "Text label inside rectangle"},
					"strokeColor":    map[string]interface{}{"type": "string", "description": "Stroke color (hex)"},
					"backgroundColor": map[string]interface{}{"type": "string", "description": "Fill color (hex)"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		{
			Name:        "create_text",
			Description: "Create a text element on the canvas",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":          map[string]interface{}{"type": "number", "description": "X position"},
					"y":          map[string]interface{}{"type": "number", "description": "Y position"},
					"text":       map[string]interface{}{"type": "string", "description": "Text content"},
					"fontSize":   map[string]interface{}{"type": "number", "description": "Font size"},
					"strokeColor": map[string]interface{}{"type": "string", "description": "Text color (hex)"},
				},
				Required: []string{"x", "y", "text"},
			},
		},
		{
			Name:        "create_arrow",
			Description: "Create an arrow or connector between points",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"startX":     map[string]interface{}{"type": "number", "description": "Start X position"},
					"startY":     map[string]interface{}{"type": "number", "description": "Start Y position"},
					"endX":       map[string]interface{}{"type": "number", "description": "End X position"},
					"endY":       map[string]interface{}{"type": "number", "description": "End Y position"},
					"label":      map[string]interface{}{"type": "string", "description": "Label on arrow"},
					"strokeColor": map[string]interface{}{"type": "string", "description": "Arrow color (hex)"},
				},
				Required: []string{"startX", "startY", "endX", "endY"},
			},
		},
		{
			Name:        "create_ellipse",
			Description: "Create an ellipse (oval) element",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":              map[string]interface{}{"type": "number", "description": "X position"},
					"y":              map[string]interface{}{"type": "number", "description": "Y position"},
					"width":          map[string]interface{}{"type": "number", "description": "Width"},
					"height":         map[string]interface{}{"type": "number", "description": "Height"},
					"label":          map[string]interface{}{"type": "string", "description": "Text label inside"},
					"strokeColor":    map[string]interface{}{"type": "string", "description": "Stroke color (hex)"},
					"backgroundColor": map[string]interface{}{"type": "string", "description": "Fill color (hex)"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		{
			Name:        "create_diamond",
			Description: "Create a diamond (decision) shape",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":              map[string]interface{}{"type": "number", "description": "X position"},
					"y":              map[string]interface{}{"type": "number", "description": "Y position"},
					"width":          map[string]interface{}{"type": "number", "description": "Width"},
					"height":         map[string]interface{}{"type": "number", "description": "Height"},
					"label":          map[string]interface{}{"type": "string", "description": "Text label inside"},
					"strokeColor":    map[string]interface{}{"type": "string", "description": "Stroke color (hex)"},
					"backgroundColor": map[string]interface{}{"type": "string", "description": "Fill color (hex)"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		{
			Name:        "create_line",
			Description: "Create a line with multiple points",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"points":      map[string]interface{}{"type": "array", "description": "Array of [x, y] coordinates"},
					"strokeColor": map[string]interface{}{"type": "string", "description": "Line color (hex)"},
				},
				Required: []string{"points"},
			},
		},
		{
			Name:        "get_canvas_state",
			Description: "Get current canvas state summary",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type:       "object",
				Properties: map[string]interface{}{},
				Required:   []string{},
			},
		},
	}
}

// SupportedModels returns list of supported models
func SupportedModels() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"claude-sonnet-4-20250514",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
	}
}