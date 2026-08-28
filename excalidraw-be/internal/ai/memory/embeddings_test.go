package memory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddingsClient_Embed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/embeddings") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"embedding": [0.1, 0.2, 0.3]},
				{"embedding": [0.4, 0.5, 0.6]}
			]
		}`))
	}))
	defer srv.Close()

	c := NewEmbeddingsClient("test-key", srv.URL, "text-embedding-3-small")
	got, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(got))
	}
	if got[0][0] != 0.1 || got[1][2] != 0.6 {
		t.Errorf("vectors wrong: %v %v", got[0], got[1])
	}
}
