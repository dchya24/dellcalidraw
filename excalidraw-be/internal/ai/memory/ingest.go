package memory

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// LLMSummarizer summarizes a transcript into 1-3 paragraphs.
type LLMSummarizer interface {
	Summarize(ctx context.Context, transcript string) (string, error)
}

// RepoIface is the narrow subset of AIMemoryRepository used by Ingester.
type RepoIface interface {
	InsertSummary(ctx context.Context, e MemoryEntry) error
	InsertRaw(ctx context.Context, e MemoryEntry) error
}

// EmbedIface is the narrow subset of EmbeddingsClient used by Ingester.
type EmbedIface interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type Ingester struct {
	Repo       RepoIface
	Embeddings EmbedIface
	Summarizer LLMSummarizer
}

func NewIngester(repo RepoIface, emb EmbedIface, sum LLMSummarizer) *Ingester {
	return &Ingester{Repo: repo, Embeddings: emb, Summarizer: sum}
}

type IngestRequest struct {
	UserID     string
	RoomID     string
	TabID      string
	Transcript string
	RequestID  string
	Model      string
}

func (i *Ingester) Ingest(ctx context.Context, req IngestRequest) error {
	if i.Summarizer == nil || i.Embeddings == nil || i.Repo == nil {
		return errors.New("ingester not fully wired")
	}
	summary, err := i.Summarizer.Summarize(ctx, req.Transcript)
	if err != nil {
		return err
	}
	if summary == "" {
		return nil // nothing worth remembering
	}
	vecs, err := i.Embeddings.Embed(ctx, []string{summary})
	if err != nil {
		return err
	}
	if len(vecs) == 0 {
		return errors.New("empty embedding")
	}
	meta := map[string]any{
		"request_id": req.RequestID,
		"model":      req.Model,
		"ts":         time.Now().UTC().Format(time.RFC3339),
	}
	_, _ = json.Marshal(meta) // ensure meta serializes; not used further

	var tabPtr *uuid.UUID
	if req.TabID != "" {
		u, err := uuid.Parse(req.TabID)
		if err == nil {
			tabPtr = &u
		}
	}

	// Insert summary twice — once for user, once for room — sharing the
	// same embedding. This is intentional: each owner has its own row
	// so retrieval per-owner is one indexed query.
	summaryEntry := MemoryEntry{
		OwnerType: OwnerUser, OwnerID: req.UserID, TabID: tabPtr,
		Kind: KindSummary, Content: summary, Embedding: vecs[0], Metadata: meta,
	}
	if err := i.Repo.InsertSummary(ctx, summaryEntry); err != nil {
		return err
	}
	roomEntry := summaryEntry
	roomEntry.OwnerType = OwnerRoom
	roomEntry.OwnerID = req.RoomID
	if err := i.Repo.InsertSummary(ctx, roomEntry); err != nil {
		return err
	}

	// Insert raw transcript.
	rawEntry := MemoryEntry{
		OwnerType: OwnerUser, OwnerID: req.UserID, TabID: tabPtr,
		Kind: KindRaw, Content: req.Transcript, Metadata: meta,
	}
	if err := i.Repo.InsertRaw(ctx, rawEntry); err != nil {
		return err
	}
	return nil
}

// IngestAsync runs Ingest in a goroutine with 3 retries (exponential backoff).
// Failures are logged and dropped.
func (i *Ingester) IngestAsync(req IngestRequest) {
	go func() {
		backoff := time.Second
		for attempt := 1; attempt <= 3; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			err := i.Ingest(ctx, req)
			cancel()
			if err == nil {
				return
			}
			slog.Warn("[ai/memory] ingest failed",
				"attempt", attempt, "request_id", req.RequestID, "error", err)
			time.Sleep(backoff)
			backoff *= 2
		}
		slog.Error("[ai/memory] ingest abandoned after 3 attempts",
			"request_id", req.RequestID)
	}()
}
