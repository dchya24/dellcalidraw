package memory

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type countingRepo struct {
	summaries []MemoryEntry
	raws      []MemoryEntry
}

func (c *countingRepo) InsertSummary(_ context.Context, e MemoryEntry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	c.summaries = append(c.summaries, e)
	return nil
}

func (c *countingRepo) InsertRaw(_ context.Context, e MemoryEntry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	c.raws = append(c.raws, e)
	return nil
}

type stubEmbedder struct{}

func (stubEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

type stubSummarizer struct{ out string }

func (s stubSummarizer) Summarize(_ context.Context, _ string) (string, error) {
	return s.out, nil
}

func TestIngester_InsertsUserAndRoomSummariesPlusRaw(t *testing.T) {
	repo := &countingRepo{}
	ing := NewIngester(repo, stubEmbedder{}, stubSummarizer{out: "User likes blue"})
	if err := ing.Ingest(context.Background(), IngestRequest{
		UserID: "u1", RoomID: "r1", TabID: "", Transcript: "user: hi", RequestID: "req1",
	}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(repo.summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(repo.summaries))
	}
	if repo.summaries[0].OwnerType != OwnerUser || repo.summaries[1].OwnerType != OwnerRoom {
		t.Errorf("owner types wrong: %v %v", repo.summaries[0].OwnerType, repo.summaries[1].OwnerType)
	}
	if len(repo.raws) != 1 || repo.raws[0].Kind != KindRaw {
		t.Errorf("expected 1 raw row, got %d", len(repo.raws))
	}
}

func TestIngester_EmptySummarySkips(t *testing.T) {
	repo := &countingRepo{}
	ing := NewIngester(repo, stubEmbedder{}, stubSummarizer{out: ""})
	if err := ing.Ingest(context.Background(), IngestRequest{UserID: "u", RoomID: "r", Transcript: "x"}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(repo.summaries) != 0 || len(repo.raws) != 0 {
		t.Errorf("expected zero rows, got %d summaries, %d raws", len(repo.summaries), len(repo.raws))
	}
}
