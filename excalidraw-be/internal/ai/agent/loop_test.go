package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/you/excalidraw-be/internal/ai"
	"github.com/you/excalidraw-be/internal/ai/memory"
)

type scriptedProvider struct {
	responses [][]ai.SSEEvent
	callIndex int
}

func (p *scriptedProvider) Chat(_ context.Context, _ []ai.Message, _ []ai.Tool, _ string) (*ai.ChatResult, error) {
	return nil, nil
}

func (p *scriptedProvider) ChatStream(_ context.Context, _ []ai.Message, _ []ai.Tool, _ string, _ func(ai.SSEEvent) error) error {
	return nil
}

func (p *scriptedProvider) ChatStreamWithMemory(_ context.Context, _ []ai.Message, _ []ai.Tool, _ string, _ []memory.MemoryEntry, streamFunc func(ai.SSEEvent) error) error {
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

// collectEvents is a Send function that records all events.
func collectEvents(out *[]ai.SSEEvent) func(ai.SSEEvent) error {
	return func(e ai.SSEEvent) error {
		*out = append(*out, e)
		return nil
	}
}

func TestAgent_StopsOnTextOnly(t *testing.T) {
	p := &scriptedProvider{responses: [][]ai.SSEEvent{
		{{Type: "text", Content: "Hello from AI"}},
	}}
	var got []ai.SSEEvent
	state := &LoopState{
		RequestID: "test-stop",
		Messages:  []ai.Message{{Role: "user", Content: "hi"}},
		Tools:     []ai.Tool{},
		Model:     "m",
		Provider:  p,
		Memory:    nil,
		Send:      collectEvents(&got),
		Done:      make(chan struct{}),
	}

	err := Run(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify agent_iteration was emitted.
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

	// Verify text event was forwarded.
	var sawText bool
	for _, e := range got {
		if e.Type == "text" && e.Content == "Hello from AI" {
			sawText = true
		}
	}
	if !sawText {
		t.Errorf("expected text event from provider")
	}

	// After a text-only call, state should NOT be ended (the handler
	// decides when to send done). But since no tool_call events were
	// emitted, the handler will know to call SendDone.
	if state.EndReason != "" {
		t.Errorf("state should not be ended yet, got %q", state.EndReason)
	}
}

func TestAgent_ForcesWrapUpAtMax(t *testing.T) {
	// Build 20 tool-call responses + 1 wrap-up text response.
	responses := make([][]ai.SSEEvent, MaxToolCalls+1)
	for i := 0; i < MaxToolCalls; i++ {
		responses[i] = []ai.SSEEvent{{
			Type:   "tool_call",
			ID:     fmt.Sprintf("c%d", i),
			Name:   "noop",
			Result: map[string]any{"ok": true},
		}}
	}
	responses[MaxToolCalls] = []ai.SSEEvent{{Type: "text", Content: "wrap-up summary"}}

	p := &scriptedProvider{responses: responses}
	var got []ai.SSEEvent

	state := &LoopState{
		RequestID: "test-wrapup",
		Messages:  []ai.Message{{Role: "user", Content: "do 20 things"}},
		Tools:     []ai.Tool{{Name: "noop"}},
		Model:     "m",
		Provider:  p,
		Memory:    nil,
		Send:      collectEvents(&got),
		Done:      make(chan struct{}),
	}

	// Simulate the handler loop: call Run until step exceeds MaxToolCalls.
	// After each Run that doesn't mark end, the handler would wait for
	// tool results; here we immediately call Run again to simulate.
	var lastErr error
	for i := 0; i < MaxToolCalls+2; i++ { // +2 for safety margin
		lastErr = Run(context.Background(), state, nil)
		if lastErr != nil {
			break
		}
		if state.EndReason != "" {
			break
		}
		// Simulate tool results arriving: append a dummy tool message.
		state.Messages = append(state.Messages, ai.Message{
			Role:       "tool",
			Content:    `{"success":true}`,
			ToolCallID: fmt.Sprintf("c%d", state.step-1),
		})
	}
	if lastErr != nil {
		t.Fatalf("unexpected error: %v", lastErr)
	}

	// Verify we saw the wrap-up text.
	var sawWrapUp bool
	for _, e := range got {
		if e.Type == "text" && strings.Contains(e.Content, "wrap-up summary") {
			sawWrapUp = true
		}
	}
	if !sawWrapUp {
		t.Errorf("expected wrap-up text in final iteration")
	}

	// Verify state is ended. (agent_final is emitted by the handler, not Run.)
	if state.EndReason != "max_steps" {
		t.Errorf("expected EndReason 'max_steps', got %q", state.EndReason)
	}

	// Verify we saw enough agent_iteration events.
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
