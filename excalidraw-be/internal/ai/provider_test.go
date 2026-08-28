package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/you/excalidraw-be/internal/ai/memory"
)

func TestBuildSystemPromptIncludesElementCount(t *testing.T) {
	for _, n := range []int{0, 1, 5} {
		elements := make([]interface{}, n)
		prompt := BuildSystemPrompt(elements)
		if n == 0 {
			if !strings.Contains(prompt, "no element") {
				t.Errorf("n=0: expected 'no element', got prompt without it")
			}
		} else {
			marker := "Current canvas has "
			idx := strings.Index(prompt, marker)
			if idx < 0 {
				t.Fatalf("n=%d: marker missing", n)
			}
			tail := prompt[idx+len(marker):]
			if !strings.HasPrefix(tail, "1") && n == 1 {
				t.Errorf("n=1: prompt should mention 1 element, got tail %q", tail[:20])
			}
		}
	}
}

func TestBuildSystemPromptListsAllTools(t *testing.T) {
	prompt := BuildSystemPrompt(nil)
	for _, name := range []string{
		"create_rectangle",
		"create_ellipse",
		"create_diamond",
		"create_text",
		"create_arrow",
		"create_line",
		"camera_update",
		"convert_mermaid",
		"auto_layout",
	} {
		if !strings.Contains(prompt, name) {
			t.Errorf("system prompt missing tool name %q", name)
		}
	}
}

func TestGetDefaultToolsHasExpectedSet(t *testing.T) {
	tools := GetDefaultTools()
	got := make(map[string]bool, len(tools))
	for _, tt := range tools {
		if got[tt.Name] {
			t.Errorf("duplicate tool name in GetDefaultTools: %q", tt.Name)
		}
		got[tt.Name] = true
	}

	// All 19 tools must be registered. If you add or remove a tool,
	// update this list AND the AI_CHAT_DIAGRAM.md docs.
	want := []string{
		"create_rectangle",
		"create_ellipse",
		"create_diamond",
		"create_text",
		"create_arrow",
		"create_line",
		"create_zone",
		"move_elements",
		"delete_elements",
		"update_element_style",
		"edit_text",
		"camera_update",
		"get_canvas_state",
		"convert_mermaid",
		"auto_layout",
		"create_group",
		"duplicate_elements",
		"resize_elements",
		"align_elements",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing tool %q", name)
		}
	}
	if len(tools) != len(want) {
		t.Errorf("tool count: got %d want %d (tools=%v)", len(tools), len(want), keysOf(got))
	}
}

func TestEachToolHasObjectSchema(t *testing.T) {
	for _, tt := range GetDefaultTools() {
		if tt.Name == "" {
			t.Errorf("tool with empty name")
		}
		if tt.Description == "" {
			t.Errorf("%s: empty description", tt.Name)
		}
		if tt.InputSchema.Type != "object" {
			t.Errorf("%s: schema type should be 'object', got %q", tt.Name, tt.InputSchema.Type)
		}
		if tt.InputSchema.Properties == nil {
			t.Errorf("%s: nil properties map (must be at least empty {})", tt.Name)
		}
	}
}

func TestRequiredFieldsHaveProperties(t *testing.T) {
	// Every entry in tool.Required must also appear in tool.Properties,
	// otherwise the LLM can't satisfy the schema.
	for _, tt := range GetDefaultTools() {
		for _, req := range tt.InputSchema.Required {
			if _, ok := tt.InputSchema.Properties[req]; !ok {
				t.Errorf("%s: required field %q has no property definition", tt.Name, req)
			}
		}
	}
}

func TestSupportedModelsNonEmpty(t *testing.T) {
	models := SupportedModels()
	if len(models) == 0 {
		t.Fatal("SupportedModels must not be empty")
	}
	seen := make(map[string]bool, len(models))
	for _, m := range models {
		if m == "" {
			t.Error("empty model name in list")
		}
		if seen[m] {
			t.Errorf("duplicate model: %q", m)
		}
		seen[m] = true
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBuildSystemPromptWithMemory_IncludesBlock(t *testing.T) {
	now := time.Now()
	block := buildSystemPromptWithMemory(nil, []memory.MemoryEntry{
		{OwnerType: memory.OwnerUser, Content: "User likes blue pastels", CreatedAt: now},
	})
	if !strings.Contains(block, "## Relevant memory (user)") {
		t.Errorf("expected memory header in prompt")
	}
	if !strings.Contains(block, "User likes blue pastels") {
		t.Errorf("expected memory content in prompt")
	}
}

func TestBuildSystemPromptWithoutMemory_NoBlock(t *testing.T) {
	block := buildSystemPromptWithMemory(nil, nil)
	if strings.Contains(block, "## Relevant memory") {
		t.Errorf("did not expect memory block when none provided")
	}
}
