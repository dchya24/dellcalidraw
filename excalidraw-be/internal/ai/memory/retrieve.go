package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const approxCharsPerToken = 4

// Retriever fetches relevant memory entries from the vector store,
// merges results across owners, and renders them for system-prompt injection.
type Retriever struct {
	Repo         interface {
		TopK(ctx context.Context, ownerType, ownerID string, embedding []float32, k int) ([]MemoryEntry, error)
	}
	Embeddings   interface {
		Embed(ctx context.Context, texts []string) ([][]float32, error)
	}
	TopKPerOwner int
	MaxTokens    int
}

func NewRetriever(repo RetrieverRepo, emb *EmbeddingsClient, topK, maxTokens int) *Retriever {
	if topK <= 0 {
		topK = 5
	}
	if maxTokens <= 0 {
		maxTokens = 800
	}
	return &Retriever{Repo: repo, Embeddings: emb, TopKPerOwner: topK, MaxTokens: maxTokens}
}

// RetrieverRepo is the subset of AIMemoryRepository that Retriever needs.
type RetrieverRepo interface {
	TopK(ctx context.Context, ownerType, ownerID string, embedding []float32, k int) ([]MemoryEntry, error)
}

// Retrieve embeds the query, runs TopK per owner, and returns the merged
// list sorted by distance (closest first).
func (r *Retriever) Retrieve(ctx context.Context, query string, owners []Owner) ([]MemoryEntry, error) {
	if r.Embeddings == nil || r.Repo == nil || len(owners) == 0 {
		return nil, nil
	}
	vecs, err := r.Embeddings.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	merged := make([]MemoryEntry, 0, r.TopKPerOwner*len(owners))
	for _, o := range owners {
		got, err := r.Repo.TopK(ctx, o.Type, o.ID, vecs[0], r.TopKPerOwner)
		if err != nil {
			return nil, err
		}
		for _, e := range got {
			e.OwnerType = o.Type // ensure stable grouping for formatting
			merged = append(merged, e)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Distance < merged[j].Distance })
	return merged, nil
}

// FormatMemoryBlock renders entries grouped by owner type, newest first,
// as a markdown-ish section that gets injected into the system prompt.
func FormatMemoryBlock(entries []MemoryEntry) string {
	if len(entries) == 0 {
		return ""
	}
	byOwner := map[string][]MemoryEntry{}
	for _, e := range entries {
		byOwner[e.OwnerType] = append(byOwner[e.OwnerType], e)
	}
	var out strings.Builder
	for _, ownerType := range []string{OwnerUser, OwnerRoom} {
		list, ok := byOwner[ownerType]
		if !ok {
			continue
		}
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
		header := "## Relevant memory (user)"
		if ownerType == OwnerRoom {
			header = "## Relevant memory (room)"
		}
		fmt.Fprintf(&out, "\n%s\n", header)
		for _, e := range list {
			fmt.Fprintf(&out, "- %s: %s\n", e.CreatedAt.UTC().Format("2006-01-02"), e.Content)
		}
	}
	return out.String()
}

// TruncateToTokens caps text to approximately maxTokens using a
// character-per-token heuristic.  It cuts on the last newline before
// the limit so paragraph structure is preserved.
func TruncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	limit := maxTokens * approxCharsPerToken
	if len(text) <= limit {
		return text
	}
	cut := strings.LastIndex(text[:limit], "\n")
	if cut < 0 {
		cut = limit
	}
	return text[:cut] + "\n..."
}
