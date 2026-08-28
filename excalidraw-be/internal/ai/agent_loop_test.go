package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/you/excalidraw-be/internal/ai/memory"
)

type scriptedProvider struct {
	responses [][]SSEEvent
	callIndex int
}

func (p *scriptedProvider) Chat(_ context.Context, _ []Message, _ []Tool, _ string) (*ChatResult, error) {
	return nil, nil
}

func (p *scriptedProvider) ChatStream(_ context.Context, _ []Message, _ []Tool, _ string, _ func(SSEEvent) error) error {
	return nil
}

func (p *scriptedProvider) ChatStreamWithMemory(_ context.Context, _ []Message, _ []Tool, _ string, _ []memory.MemoryEntry, streamFunc func(SSEEvent) error) error {
	p.callIndex++
	if p.callIndex > len(p.responses) {
		return fmt.Errorf("scripted provider exhausted (call %d)", p.callIndex)
	}
	for _, ev := range p.responses[p.callIndex-1] {
		if err := streamFunc(ev); err != nil {
			return err
		}
	}
	return nil
}

func (p *scriptedProvider) GetModels() []string  { return nil }
func (p *scriptedProvider) DefaultModel() string { return "test-model" }

func collectEvents(out *[]SSEEvent) func(SSEEvent) error {
	return func(e SSEEvent) error {
		*out = append(*out, e)
		return nil
	}
}

func TestAgent_StopsOnTextOnly(t *testing.T) {
	p := &scriptedProvider{responses: [][]SSEEvent{
		{{Type: "text", Content: "Hello from AI"}},
	}}
	var got []SSEEvent
	state := &LoopState{
		RequestID: "test-stop",
		Messages:  []Message{{Role: "user", Content: "hi"}},
		Tools:     []Tool{},
		Model:     "m",
		Provider:  p,
		Memory:    nil,
		Send:      collectEvents(&got),
		Done:      make(chan struct{}),
	}

	err := AgentRun(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sawIteration bool
	for _, e := range got {
		if e.Type == "agent_iteration" {
			sawIteration = true
			if e.Content != "1" {
				t.Errorf("expected step 1, got %q", e.Content)
			}
		}
	}
	if !sawIteration {
		t.Errorf("expected agent_iteration event")
	}

	var sawText bool
	for _, e := range got {
		if e.Type == "text" && e.Content == "Hello from AI" {
			sawText = true
		}
	}
	if !sawText {
		t.Errorf("expected text event from provider")
	}
}

func TestAgent_ForcesWrapUpAtMax(t *testing.T) {
	responses := make([][]SSEEvent, MaxToolCalls+1)
	for i := 0; i < MaxToolCalls; i++ {
		responses[i] = []SSEEvent{{
			Type:   "tool_call",
			ID:     fmt.Sprintf("c%d", i),
			Name:   "noop",
			Result: map[string]any{"ok": true},
		}}
	}
	responses[MaxToolCalls] = []SSEEvent{{Type: "text", Content: "wrap-up summary"}}

	p := &scriptedProvider{responses: responses}
	var got []SSEEvent

	state := &LoopState{
		RequestID: "test-wrapup",
		Messages:  []Message{{Role: "user", Content: "do 20 things"}},
		Tools:     []Tool{{Name: "noop"}},
		Model:     "m",
		Provider:  p,
		Memory:    nil,
		Send:      collectEvents(&got),
		Done:      make(chan struct{}),
	}

	var lastErr error
	for i := 0; i < MaxToolCalls+2; i++ {
		lastErr = AgentRun(context.Background(), state, nil)
		if lastErr != nil {
			break
		}
		if state.EndReason != "" {
			break
		}
		state.Messages = append(state.Messages, Message{
			Role:       "tool",
			Content:    `{"success":true}`,
			ToolCallID: fmt.Sprintf("c%d", state.step-1),
		})
	}
	if lastErr != nil {
		t.Fatalf("unexpected error: %v", lastErr)
	}

	var sawWrapUp bool
	for _, e := range got {
		if e.Type == "text" && strings.Contains(e.Content, "wrap-up summary") {
			sawWrapUp = true
		}
	}
	if !sawWrapUp {
		t.Errorf("expected wrap-up text in final iteration")
	}

	if state.EndReason != "max_steps" {
		t.Errorf("expected EndReason 'max_steps', got %q", state.EndReason)
	}

	var iterCount int
	for _, e := range got {
		if e.Type == "agent_iteration" {
			iterCount++
		}
	}
	if iterCount < MaxToolCalls+1 {
		t.Errorf("expected at least %d agent_iteration events, got %d", MaxToolCalls+1, iterCount)
	}
}
