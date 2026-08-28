package agent

import (
	"sync"

	"github.com/you/excalidraw-be/internal/ai"
	"github.com/you/excalidraw-be/internal/ai/memory"
)

// LoopState holds the mutable state for one agent loop iteration set.
type LoopState struct {
	RequestID string
	Messages  []ai.Message
	Tools     []ai.Tool
	Model     string
	Provider  ai.LLMProvider
	Memory    []memory.MemoryEntry

	// Send writes SSE events back to the client stream owned by the
	// /api/ai/chat handler.
	Send func(ai.SSEEvent) error

	// Done is closed when the loop finishes (for any reason).
	Done chan struct{}

	// step is the current iteration count (incremented by Run).
	step int

	// Pending collects tool-result messages that arrive between
	// LLM iterations. Protected by mu because the handler writes
	// from the /tool-result goroutine while Run reads.
	Pending   []ai.Message
	mu        sync.Mutex
	ended     bool
	EndReason string
}

// AppendPending adds tool-result messages. It is safe to call from
// the /tool-result HTTP handler while Run is blocked.
func (s *LoopState) AppendPending(m ai.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.Pending = append(s.Pending, m)
}

// DrainPending returns and clears the pending queue. Called by Run
// before each LLM call to fold tool results into the transcript.
func (s *LoopState) DrainPending() []ai.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.Pending
	s.Pending = nil
	return out
}

// MarkEnd records the loop's final reason and closes Done. It is
// idempotent: the first call wins; subsequent calls are no-ops.
func (s *LoopState) MarkEnd(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ended = true
	s.EndReason = reason
	close(s.Done)
}

// Registry maps request IDs to LoopState values. It is safe for
// concurrent use.
type Registry struct{ m sync.Map }

// GetOrCreate returns the existing LoopState for id, or calls init()
// to create and store a new one. The init function must not block.
func (r *Registry) GetOrCreate(id string, init func() *LoopState) *LoopState {
	if v, ok := r.m.Load(id); ok {
		return v.(*LoopState)
	}
	s := init()
	actual, _ := r.m.LoadOrStore(id, s)
	return actual.(*LoopState)
}

// Drop removes a loop state from the registry. Safe to call even if
// the key does not exist.
func (r *Registry) Drop(id string) { r.m.Delete(id) }

// Has reports whether a loop state exists for the given id.
func (r *Registry) Has(id string) bool { _, ok := r.m.Load(id); return ok }
