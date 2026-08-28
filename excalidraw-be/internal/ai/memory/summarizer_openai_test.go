package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAISummarizer_Summarize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(r.URL.Path, "/chat/completions") {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"User likes blue."}}]
		}`))
	}))
	defer srv.Close()

	s := NewOpenAISummarizer("test-key", srv.URL, "gpt-4o-mini")
	out, err := s.Summarize(context.Background(), "user: hi\nassistant: hello")
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if out != "User likes blue." {
		t.Errorf("wrong summary: %q", out)
	}
}
