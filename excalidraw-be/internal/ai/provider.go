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

	// DefaultModel returns the configured default model from env
	DefaultModel() string
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
	return `You are an AI assistant that helps users create professional Excalidraw diagrams.

## COLOR PALETTE (use consistently)

**Primary Colors:**
- Blue #4a9eed - Primary actions, links, data series 1
- Amber #f59e0b - Warnings, highlights
- Green #22c55e - Success, positive
- Red #ef4444 - Errors, negative
- Purple #8b5cf6 - Special items

**Pastel Fills (for shape backgrounds):**
- Light Blue #a5d8ff - Input, sources, primary nodes
- Light Green #b2f2bb - Success, output, completed
- Light Orange #ffd8a8 - Warning, pending, external
- Light Purple #d0bfff - Processing, middleware
- Light Red #ffc9c9 - Error, critical

**Background Zones (use with opacity 30-50):**
- Blue zone #dbe4ff - UI/Frontend layer
- Purple zone #e5dbff - Logic/Agent layer  
- Green zone #d3f9d8 - Data/Tool layer

## ELEMENT FORMAT

**Required fields for all elements:** type, id (unique string), x, y, width, height

**Defaults (you can skip):** strokeColor="#1e1e1e", backgroundColor="transparent", fillStyle="solid", strokeWidth=2, roughness=1, opacity=100

**Labeled shapes (PREFERRED):** Add label as object for auto-centered text. No separate text element needed.
Example: { "type": "rectangle", "x": 100, "y": 100, "width": 200, "height": 80, "label": { "text": "Start", "fontSize": 18 }, "backgroundColor": "#a5d8ff", "fillStyle": "solid" }

**Standalone text:** { "type": "text", "id": "t1", "x": 150, "y": 138, "text": "Title", "fontSize": 24 }
- x is LEFT edge of text. To center at cx: x = cx - text.length * fontSize * 0.5

**Rounded corners:** Add "roundness": { "type": 3 } for rounded rectangle corners

## ARROW BINDING

Connect arrows to shapes using binding:
- startBinding: { "elementId": "box1", "fixedPoint": [1, 0.5] }  // right-center of box1
- endBinding: { "elementId": "box2", "fixedPoint": [0, 0.5] }   // left-center of box2

**fixedPoint values:**
- Top: [0.5, 0], Bottom: [0.5, 1]
- Left: [0, 0.5], Right: [1, 0.5]
- Center: [0.5, 0.5]

**Arrow heads:** "startArrowhead" and "endArrowhead" can be: "arrow", "bar", "dot", "triangle", or null

## CAMERA SYSTEM

Control viewport with camera_update:
- { "type": "camera_update", "x": 0, "y": 0, "width": 800, "height": 600 }
- ALWAYS use 4:3 aspect ratio: 400x300, 600x450, 800x600, 1200x900, 1600x1200
- x, y: top-left corner of visible area (scene coordinates)
- Start with camera_update before drawing elements

**Camera sizes:**
- S: 400x300 (2-3 elements close-up)
- M: 600x450 (medium view)
- L: 800x600 (DEFAULT - full diagram)
- XL: 1200x900 (complex overview)
- XXL: 1600x1200 (panorama)

## DRAWING ORDER (CRITICAL)

1. camera_update (viewport) - FIRST
2. Background zones (large rectangles with opacity)
3. Main shapes with labels
4. Connecting arrows with labels
5. Annotations/text
6. Art/decorations - LAST

## FONT SIZES

- Titles/headings: minimum 20
- Labels/body text: minimum 16
- Annotations only: minimum 14
- NEVER go below 14 - unreadable!

## ELEMENT SIZING

- Min shape: 120x60 for labeled rectangles
- Min gap: 20-30px between elements
- Prefer fewer larger elements > many tiny ones
- Leave padding from camera edges

## STYLING OPTIONS

**fillStyle:** "solid", "hatching", or "cross-hatch"
**strokeWidth:** 1-3 for normal, use 1 for subtle
**roughness:** 0 for clean, 1 for sketchy, 2 for rough
**opacity:** 0-100, use 30-50 for background zones

## AVAILABLE TOOLS

**Shape Creation:**
- create_rectangle: x, y, width, height, label?, strokeColor?, backgroundColor?, roundness?, fillStyle?, strokeWidth?, opacity?
- create_ellipse: x, y, width, height, label?, strokeColor?, backgroundColor?, fillStyle?, opacity?
- create_diamond: x, y, width, height, label?, strokeColor?, backgroundColor?, fillStyle?, opacity?
- create_text: x, y, text, fontSize?, strokeColor?
- create_arrow: startX, startY, endX, endY, label?, strokeColor?, strokeWidth?, startArrowhead?, endArrowhead?, startBinding?, endBinding?
- create_line: points (array of [x,y]), strokeColor?, strokeWidth?

**Modifications:**
- move_elements: elementIds (array), deltaX, deltaY
- delete_elements: elementIds (array)
- update_element_style: elementIds, strokeColor?, backgroundColor?, strokeWidth?, opacity?, fillStyle?

**Camera & Canvas:**
- camera_update: x, y, width, height
- get_canvas_state: Returns element count, types, bounding box

## TIPS

1. Use labeled shapes instead of separate text + shape
2. Connect arrows between elements, not to coordinates
3. Use consistent color palette for visual harmony
4. Camera animation guides user attention - use it!
5. Start with camera_update to set the viewport
6. Dark mode: Use #1e1e2e background, lighter colors for elements

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

// GetDefaultTools returns list of available MCP tools with enhanced properties
func GetDefaultTools() []Tool {
	return []Tool{
		// === SHAPE CREATION TOOLS ===
		{
			Name:        "create_rectangle",
			Description: "Create a rectangle element on the canvas. Use label for auto-centered text inside.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":               map[string]interface{}{"type": "number", "description": "X position (left edge)"},
					"y":               map[string]interface{}{"type": "number", "description": "Y position (top edge)"},
					"width":           map[string]interface{}{"type": "number", "description": "Width of rectangle (min 120)"},
					"height":          map[string]interface{}{"type": "number", "description": "Height of rectangle (min 60)"},
					"label":           map[string]interface{}{"type": "object", "description": "Label object with text and fontSize", "properties": map[string]interface{}{
						"text":     map[string]interface{}{"type": "string", "description": "Label text"},
						"fontSize": map[string]interface{}{"type": "number", "description": "Font size (min 16, default 18)"},
					}},
					"strokeColor":     map[string]interface{}{"type": "string", "description": "Stroke color (hex, default #1e1e1e)"},
					"backgroundColor": map[string]interface{}{"type": "string", "description": "Fill color (hex, use pastel colors)"},
					"fillStyle":       map[string]interface{}{"type": "string", "description": "Fill style: solid, hatching, cross-hatch"},
					"roundness":       map[string]interface{}{"type": "object", "description": "Roundness for corners", "properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "number", "description": "Type: 1=sharp, 2=slight, 3=rounded, 4=very rounded"},
					}},
					"strokeWidth":     map[string]interface{}{"type": "number", "description": "Stroke width (1-3)"},
					"roughness":       map[string]interface{}{"type": "number", "description": "Roughness: 0=clean, 1=sketchy, 2=rough"},
					"opacity":         map[string]interface{}{"type": "number", "description": "Opacity 0-100, use 30-50 for zones"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		{
			Name:        "create_ellipse",
			Description: "Create an ellipse (oval) element. Good for start/end nodes in flowcharts.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":               map[string]interface{}{"type": "number", "description": "X position (left edge)"},
					"y":               map[string]interface{}{"type": "number", "description": "Y position (top edge)"},
					"width":           map[string]interface{}{"type": "number", "description": "Width (min 120)"},
					"height":          map[string]interface{}{"type": "number", "description": "Height (min 60)"},
					"label":           map[string]interface{}{"type": "object", "description": "Label object with text and fontSize"},
					"strokeColor":     map[string]interface{}{"type": "string", "description": "Stroke color (hex)"},
					"backgroundColor":  map[string]interface{}{"type": "string", "description": "Fill color (hex)"},
					"fillStyle":       map[string]interface{}{"type": "string", "description": "Fill style: solid, hatching, cross-hatch"},
					"strokeWidth":     map[string]interface{}{"type": "number", "description": "Stroke width (1-3)"},
					"opacity":         map[string]interface{}{"type": "number", "description": "Opacity 0-100"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		{
			Name:        "create_diamond",
			Description: "Create a diamond (decision) shape. Good for decision points in flowcharts.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":               map[string]interface{}{"type": "number", "description": "X position (left edge)"},
					"y":               map[string]interface{}{"type": "number", "description": "Y position (top edge)"},
					"width":           map[string]interface{}{"type": "number", "description": "Width"},
					"height":          map[string]interface{}{"type": "number", "description": "Height"},
					"label":           map[string]interface{}{"type": "object", "description": "Label object with text and fontSize"},
					"strokeColor":     map[string]interface{}{"type": "string", "description": "Stroke color (hex)"},
					"backgroundColor": map[string]interface{}{"type": "string", "description": "Fill color (hex)"},
					"fillStyle":       map[string]interface{}{"type": "string", "description": "Fill style: solid, hatching, cross-hatch"},
					"strokeWidth":     map[string]interface{}{"type": "number", "description": "Stroke width (1-3)"},
					"opacity":         map[string]interface{}{"type": "number", "description": "Opacity 0-100"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		{
			Name:        "create_text",
			Description: "Create standalone text element. Use labels on shapes instead when possible.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":           map[string]interface{}{"type": "number", "description": "X position (LEFT edge, not center)"},
					"y":           map[string]interface{}{"type": "number", "description": "Y position (top edge)"},
					"text":        map[string]interface{}{"type": "string", "description": "Text content"},
					"fontSize":    map[string]interface{}{"type": "number", "description": "Font size (min 16, use 20+ for titles)"},
					"strokeColor": map[string]interface{}{"type": "string", "description": "Text color (hex, min contrast #757575 on white)"},
				},
				Required: []string{"x", "y", "text"},
			},
		},
		{
			Name:        "create_arrow",
			Description: "Create an arrow/connector. Use binding to connect to shapes.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"startX":       map[string]interface{}{"type": "number", "description": "Start X position (relative to shape or absolute)"},
					"startY":       map[string]interface{}{"type": "number", "description": "Start Y position"},
					"endX":         map[string]interface{}{"type": "number", "description": "End X position"},
					"endY":         map[string]interface{}{"type": "number", "description": "End Y position"},
					"label":        map[string]interface{}{"type": "object", "description": "Label on arrow", "properties": map[string]interface{}{
						"text":     map[string]interface{}{"type": "string", "description": "Label text"},
						"fontSize": map[string]interface{}{"type": "number", "description": "Font size (min 14)"},
					}},
					"strokeColor":   map[string]interface{}{"type": "string", "description": "Arrow color (hex)"},
					"strokeWidth":   map[string]interface{}{"type": "number", "description": "Stroke width (1-3, default 2)"},
					"startArrowhead": map[string]interface{}{"type": "string", "description": "Start arrowhead: arrow, bar, dot, triangle, null"},
					"endArrowhead":  map[string]interface{}{"type": "string", "description": "End arrowhead: arrow, bar, dot, triangle, null (default arrow)"},
					"startBinding":  map[string]interface{}{"type": "object", "description": "Bind start to element", "properties": map[string]interface{}{
						"elementId":   map[string]interface{}{"type": "string", "description": "ID of element to bind to"},
						"fixedPoint":  map[string]interface{}{"type": "array", "description": "[x, y] relative position: [0,0]=top, [1,1]=bottom, [0,0.5]=left-center, [1,0.5]=right-center, [0.5,0.5]=center"},
					}},
					"endBinding":    map[string]interface{}{"type": "object", "description": "Bind end to element", "properties": map[string]interface{}{
						"elementId":   map[string]interface{}{"type": "string", "description": "ID of element to bind to"},
						"fixedPoint":  map[string]interface{}{"type": "array", "description": "[x, y] relative position"},
					}},
					"strokeStyle":   map[string]interface{}{"type": "string", "description": "Stroke style: solid, dashed, dotted"},
				},
				Required: []string{"startX", "startY", "endX", "endY"},
			},
		},
		{
			Name:        "create_line",
			Description: "Create a multi-point line with optional arrowheads.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"points":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "array"}, "description": "Array of [x, y] coordinate arrays [[x1,y1], [x2,y2], ...]"},
					"strokeColor":   map[string]interface{}{"type": "string", "description": "Line color (hex)"},
					"strokeWidth":   map[string]interface{}{"type": "number", "description": "Stroke width (1-3)"},
					"startArrowhead": map[string]interface{}{"type": "string", "description": "Start arrowhead: arrow, bar, dot, triangle, null"},
					"endArrowhead":  map[string]interface{}{"type": "string", "description": "End arrowhead: arrow, bar, dot, triangle, null"},
					"strokeStyle":   map[string]interface{}{"type": "string", "description": "Stroke style: solid, dashed, dotted"},
				},
				Required: []string{"points"},
			},
		},
		{
			Name:        "create_zone",
			Description: "Create a background zone (large rectangle with low opacity for grouping elements).",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":               map[string]interface{}{"type": "number", "description": "X position"},
					"y":               map[string]interface{}{"type": "number", "description": "Y position"},
					"width":           map[string]interface{}{"type": "number", "description": "Width"},
					"height":          map[string]interface{}{"type": "number", "description": "Height"},
					"label":           map[string]interface{}{"type": "object", "description": "Optional zone label"},
					"strokeColor":     map[string]interface{}{"type": "string", "description": "Border color (hex, use darker variant of background)"},
					"backgroundColor": map[string]interface{}{"type": "string", "description": "Fill color (hex, use zone colors like #dbe4ff)"},
					"opacity":         map[string]interface{}{"type": "number", "description": "Opacity 30-50 for zones"},
					"fillStyle":       map[string]interface{}{"type": "string", "description": "Fill style: solid (use hatching for empty zones)"},
					"strokeWidth":     map[string]interface{}{"type": "number", "description": "Stroke width (use 1 for subtle)"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		// === MODIFICATION TOOLS ===
		{
			Name:        "move_elements",
			Description: "Move existing elements by offset (dx, dy). Use after create to adjust positions.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"elementIds": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Array of element IDs to move"},
					"deltaX":     map[string]interface{}{"type": "number", "description": "Horizontal offset (positive = right)"},
					"deltaY":     map[string]interface{}{"type": "number", "description": "Vertical offset (positive = down)"},
				},
				Required: []string{"elementIds", "deltaX", "deltaY"},
			},
		},
		{
			Name:        "delete_elements",
			Description: "Delete elements by ID. Use for corrections or cleanup.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"elementIds": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Array of element IDs to delete"},
				},
				Required: []string{"elementIds"},
			},
		},
		{
			Name:        "update_element_style",
			Description: "Update visual style of elements. Use for changing colors, opacity, etc.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"elementIds":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Array of element IDs to update"},
					"strokeColor":     map[string]interface{}{"type": "string", "description": "Stroke/border color (hex)"},
					"backgroundColor": map[string]interface{}{"type": "string", "description": "Fill color (hex)"},
					"strokeWidth":     map[string]interface{}{"type": "number", "description": "Stroke width (1-3)"},
					"opacity":         map[string]interface{}{"type": "number", "description": "Opacity 0-100"},
					"fillStyle":       map[string]interface{}{"type": "string", "description": "Fill style: solid, hatching, cross-hatch"},
				},
				Required: []string{"elementIds"},
			},
		},
		// === CAMERA & CANVAS TOOLS ===
		{
			Name:        "camera_update",
			Description: "Set viewport/camera position. ALWAYS call this FIRST before drawing elements.",
			InputSchema: struct {
				Type       string                 `json:"type"`
				Properties map[string]interface{} `json:"properties,omitempty"`
				Required   []string               `json:"required,omitempty"`
			}{
				Type: "object",
				Properties: map[string]interface{}{
					"x":      map[string]interface{}{"type": "number", "description": "Top-left X of visible area"},
					"y":      map[string]interface{}{"type": "number", "description": "Top-left Y of visible area"},
					"width":  map[string]interface{}{"type": "number", "description": "Viewport width (use 4:3 ratio: 400, 600, 800, 1200, 1600)"},
					"height": map[string]interface{}{"type": "number", "description": "Viewport height (use 4:3 ratio: 300, 450, 600, 900, 1200)"},
				},
				Required: []string{"x", "y", "width", "height"},
			},
		},
		{
			Name:        "get_canvas_state",
			Description: "Get current canvas state summary. Returns element count, types, and bounding box.",
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