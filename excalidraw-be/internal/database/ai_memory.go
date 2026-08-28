package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/you/excalidraw-be/internal/ai/memory"
)

type AIMemoryRepository struct {
	db *sql.DB
}

func NewAIMemoryRepository(db *sql.DB) *AIMemoryRepository {
	return &AIMemoryRepository{db: db}
}

func (r *AIMemoryRepository) InsertSummary(ctx context.Context, e memory.MemoryEntry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Kind != "summary" {
		return fmt.Errorf("InsertSummary requires Kind='summary', got %q", e.Kind)
	}
	if len(e.Embedding) == 0 {
		return fmt.Errorf("InsertSummary requires embedding")
	}
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
        INSERT INTO ai_memory_entries (id, owner_type, owner_id, tab_id, kind, content, embedding, metadata)
        VALUES ($1, $2, $3, $4, 'summary', $5, $6::vector, $7)`,
		e.ID, e.OwnerType, e.OwnerID, e.TabID, e.Content, pgVectorString(e.Embedding), meta)
	if err != nil {
		return fmt.Errorf("insert summary: %w", err)
	}
	slog.Debug("[ai_memory] inserted summary", "id", e.ID, "owner", e.OwnerType)
	return nil
}

func (r *AIMemoryRepository) InsertRaw(ctx context.Context, e memory.MemoryEntry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Kind != "raw" {
		return fmt.Errorf("InsertRaw requires Kind='raw', got %q", e.Kind)
	}
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
        INSERT INTO ai_memory_entries (id, owner_type, owner_id, tab_id, kind, content, metadata)
        VALUES ($1, $2, $3, $4, 'raw', $5, $6)`,
		e.ID, e.OwnerType, e.OwnerID, e.TabID, e.Content, meta)
	if err != nil {
		return fmt.Errorf("insert raw: %w", err)
	}
	return nil
}

func (r *AIMemoryRepository) TopK(ctx context.Context, ownerType, ownerID string, embedding []float32, k int) ([]memory.MemoryEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, owner_type, owner_id, tab_id, content, metadata, created_at,
               (embedding <=> $1::vector) AS distance
        FROM ai_memory_entries
        WHERE owner_type = $2 AND owner_id = $3 AND kind = 'summary' AND embedding IS NOT NULL
        ORDER BY embedding <=> $1::vector
        LIMIT $4`,
		pgVectorString(embedding), ownerType, ownerID, k)
	if err != nil {
		return nil, fmt.Errorf("topk: %w", err)
	}
	defer rows.Close()

	var out []memory.MemoryEntry
	for rows.Next() {
		var e memory.MemoryEntry
		var meta sql.NullString
		var tabID sql.NullString
		var distance float64
		if err := rows.Scan(&e.ID, &e.OwnerType, &e.OwnerID, &tabID, &e.Content, &meta, &e.CreatedAt, &distance); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if tabID.Valid {
			id, _ := uuid.Parse(tabID.String)
			e.TabID = &id
		}
		if meta.Valid {
			_ = json.Unmarshal([]byte(meta.String), &e.Metadata)
		}
		e.Distance = distance
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *AIMemoryRepository) GetByID(ctx context.Context, id uuid.UUID) (memory.MemoryEntry, error) {
	var e memory.MemoryEntry
	var tabID sql.NullString
	var meta sql.NullString
	err := r.db.QueryRowContext(ctx, `
        SELECT id, owner_type, owner_id, tab_id, kind, content, metadata, created_at
        FROM ai_memory_entries WHERE id = $1`, id).
		Scan(&e.ID, &e.OwnerType, &e.OwnerID, &tabID, &e.Kind, &e.Content, &meta, &e.CreatedAt)
	if err != nil {
		return e, err
	}
	if tabID.Valid {
		u, _ := uuid.Parse(tabID.String)
		e.TabID = &u
	}
	if meta.Valid {
		_ = json.Unmarshal([]byte(meta.String), &e.Metadata)
	}
	return e, nil
}

// pgVectorString renders a []float32 as the literal '[a,b,c]' that pgvector accepts.
func pgVectorString(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
