package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAlwaysOnHeaders(t *testing.T) {
	cfg := DefaultSecurityHeaders()
	mw := SecurityHeaders(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	checks := map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "strict-origin-when-cross-origin",
		"Permissions-Policy":         "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
		"Cross-Origin-Opener-Policy": "same-origin",
	}
	for h, want := range checks {
		if got := rec.Header().Get(h); got != want {
			t.Errorf("%s: got %q, want %q", h, got, want)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP must be set by default")
	}
	for _, frag := range []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"connect-src 'self' ws: wss:",
	} {
		if !strings.Contains(csp, frag) {
			t.Errorf("CSP missing %q\nfull: %s", frag, csp)
		}
	}
}

func TestSecurityHeadersHSTSDisabledByDefault(t *testing.T) {
	mw := SecurityHeaders(DefaultSecurityHeaders())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS must be off by default, got %q", got)
	}
}

func TestSecurityHeadersHSTSWhenEnabled(t *testing.T) {
	cfg := DefaultSecurityHeaders()
	cfg.EnableHSTS = true
	cfg.HSTSMaxAge = 600
	mw := SecurityHeaders(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	got := rec.Header().Get("Strict-Transport-Security")
	if !strings.Contains(got, "max-age=600") {
		t.Errorf("HSTS max-age missing: %q", got)
	}
	if !strings.Contains(got, "includeSubDomains") {
		t.Errorf("HSTS must include subdomains: %q", got)
	}
	if strings.Contains(got, "preload") {
		t.Errorf("preload must be opt-in: %q", got)
	}
}

func TestSecurityHeadersHSTSPreload(t *testing.T) {
	cfg := DefaultSecurityHeaders()
	cfg.EnableHSTS = true
	cfg.SetHSTSPreload = true
	mw := SecurityHeaders(cfg)

	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	got := rec.Header().Get("Strict-Transport-Security")
	if !strings.Contains(got, "preload") {
		t.Errorf("preload must be set: %q", got)
	}
}

func TestSecurityHeadersHSTSDefaultMaxAge(t *testing.T) {
	cfg := DefaultSecurityHeaders()
	cfg.EnableHSTS = true
	cfg.HSTSMaxAge = 0 // expect 1 year fallback
	mw := SecurityHeaders(cfg)

	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=31536000") {
		t.Errorf("expected 1y default max-age, got %q", got)
	}
}

func TestSecurityHeadersOverrideCSP(t *testing.T) {
	cfg := DefaultSecurityHeaders()
	cfg.CSP = "default-src 'none'"
	mw := SecurityHeaders(cfg)

	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'" {
		t.Fatalf("CSP override not applied: %q", got)
	}
}

func TestSecurityHeadersDisableIndividualHeaders(t *testing.T) {
	cfg := SecurityHeadersConfig{} // all empty → only nosniff is forced
	mw := SecurityHeaders(cfg)

	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Error("X-Content-Type-Options must always be on")
	}
	for _, h := range []string{
		"Content-Security-Policy",
		"X-Frame-Options",
		"Referrer-Policy",
		"Permissions-Policy",
		"Cross-Origin-Opener-Policy",
		"Strict-Transport-Security",
	} {
		if got := rec.Header().Get(h); got != "" {
			t.Errorf("%s should be unset, got %q", h, got)
		}
	}
}
