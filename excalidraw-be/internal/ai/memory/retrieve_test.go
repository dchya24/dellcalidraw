package memory

import (
	"strings"
	"testing"
	"time"
)

func TestFormatMemoryBlock_GroupsByOwner(t *testing.T) {
	now := time.Now()
	entries := []MemoryEntry{
		{OwnerType: OwnerUser, Content: "User likes blue", CreatedAt: now},
		{OwnerType: OwnerRoom, Content: "Team uses amber", CreatedAt: now},
	}
	out := FormatMemoryBlock(entries)
	if !strings.Contains(out, "## Relevant memory (user)") {
		t.Errorf("missing user header in:\n%s", out)
	}
	if !strings.Contains(out, "## Relevant memory (room)") {
		t.Errorf("missing room header in:\n%s", out)
	}
	if !strings.Contains(out, "User likes blue") || !strings.Contains(out, "Team uses amber") {
		t.Errorf("missing content in:\n%s", out)
	}
}

func TestTruncateToTokens_RespectsLimit(t *testing.T) {
	s := strings.Repeat("alpha ", 1000) // ~5000 chars ~ 1250 tokens
	out := TruncateToTokens(s, 100)
	if len(out) > 100*4+10 {
		t.Errorf("output too long: %d chars", len(out))
	}
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected truncation marker, got %q", out)
	}
}
