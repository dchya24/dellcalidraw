package ai

import (
	"context"
	"fmt"
)

// MaxToolCalls is the hard cap on tool-call iterations per request.
const MaxToolCalls = 20

// wrapUpReminder is injected as the final system message when the
// iteration limit is reached. The follow-up LLM call is made with
// tools=nil so the model physically cannot emit more tool calls.
const wrapUpReminder = `You have used %d tool-calls for this request and reached the iteration limit. You may not call any more tools. Respond to the user with a final text summary of what you created, what is missing, and any caveats. Do not call any additional tools.`

// Run executes one iteration of the agent loop. It calls the LLM
// provider, emits events via state.Send, and returns. If the provider
// emitted tool-call events, the caller (handler) must wait for tool
// results via the /tool-result endpoint, append them to state.Messages,
// and call Run again. The loop terminates when:
//   - The provider emits only text (no tool calls) → reason "stop".
//   - The iteration count exceeds MaxToolCalls → a wrap-up call is made
//     with tools=nil, then reason "max_steps".
//   - A provider error occurs → reason "error".
func AgentRun(ctx context.Context, state *LoopState, onFinal func(string)) error {
	send := state.Send
	if send == nil {
		send = func(SSEEvent) error { return nil }
	}

	state.step++

	// Emit iteration event.
	_ = send(SSEEvent{
		Type:    "agent_iteration",
		Content: fmt.Sprintf("%d", state.step),
	})

	// Determine if this is a forced wrap-up call (no tools).
	isWrapUp := state.step > MaxToolCalls
	toolsForCall := state.Tools
	if isWrapUp {
		toolsForCall = nil
		state.Messages = append(state.Messages, Message{
			Role:    "system",
			Content: fmt.Sprintf(wrapUpReminder, MaxToolCalls),
		})
	}

	// Call the LLM provider. Events are streamed through send.
	if err := state.Provider.ChatStreamWithMemory(
		ctx, state.Messages, toolsForCall, state.Model, state.Memory, send,
	); err != nil {
		state.MarkEnd("error")
		return err
	}

	// If this was the wrap-up call, mark end and return.
	if isWrapUp {
		state.MarkEnd("max_steps")
		if onFinal != nil {
			onFinal("max_steps")
		}
		return nil
	}

	// For non-wrap-up calls, the handler inspects whether tool_call
	// events were emitted (via the send closure's side effects) and
	// decides whether to call Run again. We return nil to indicate
	// this iteration completed successfully.
	return nil
}

// SendDone emits the final "done" SSE event. Called by the handler
// after the last Run returns.
func SendDone(state *LoopState) {
	_ = state.Send(SSEEvent{
		Type:    "done",
		Content: "Generation complete",
	})
}
