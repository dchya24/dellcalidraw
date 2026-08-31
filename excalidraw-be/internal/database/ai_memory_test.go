package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/you/excalidraw-be/internal/ai/memory"
)

func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping DB-backed test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db, func() { _ = db.Close() }
}

func TestAIMemoryRepository_InsertSummaryAndTopK(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()
	repo := NewAIMemoryRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userID := uuid.NewString()
	closeVec := []float32{0.10, 0.20, 0.30}
	farVec := []float32{0.90, 0.80, 0.70}

	if err := repo.InsertSummary(ctx, memory.MemoryEntry{
		OwnerType: memory.OwnerUser, OwnerID: userID, Kind: memory.KindSummary,
		Content: "User prefers blue pastels", Embedding: closeVec,
	}); err != nil {
		t.Fatalf("insert close: %v", err)
	}
	if err := repo.InsertSummary(ctx, memory.MemoryEntry{
		OwnerType: memory.OwnerUser, OwnerID: userID, Kind: memory.KindSummary,
		Content: "User asked for ERD", Embedding: farVec,
	}); err != nil {
		t.Fatalf("insert far: %v", err)
	}

	got, err := repo.TopK(ctx, memory.OwnerUser, userID, closeVec, 1)
	if err != nil {
		t.Fatalf("topk: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Content != "User prefers blue pastels" {
		t.Errorf("expected closest entry, got %q", got[0].Content)
	}
}

func TestAIMemoryRepository_InsertRaw(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()
	repo := NewAIMemoryRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := repo.InsertRaw(ctx, memory.MemoryEntry{
		OwnerType: memory.OwnerRoom, OwnerID: "room-1", Kind: memory.KindRaw,
		Content: "user: hi\nassistant: hello",
	}); err != nil {
		t.Fatalf("insert raw: %v", err)
	}
}
