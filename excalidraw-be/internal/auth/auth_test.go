package auth

import (
	"strings"
	"testing"
	"time"
)

func newTestService(t *testing.T) *AuthService {
	t.Helper()
	return NewAuthService("test-secret-key-for-jwt-signing", 15*time.Minute, 7*24*time.Hour)
}

func TestHashPasswordAndCheck(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "hunter2" {
		t.Fatal("hash equals plaintext")
	}
	if !CheckPassword("hunter2", hash) {
		t.Fatal("CheckPassword failed for correct password")
	}
	if CheckPassword("wrong", hash) {
		t.Fatal("CheckPassword accepted wrong password")
	}
}

func TestHashPasswordNeverEqual(t *testing.T) {
	// bcrypt salts each hash, so the same plaintext must produce
	// different hashes that both still verify.
	a, _ := HashPassword("samepass")
	b, _ := HashPassword("samepass")
	if a == b {
		t.Fatal("two hashes of the same password must not be equal")
	}
	if !CheckPassword("samepass", a) || !CheckPassword("samepass", b) {
		t.Fatal("hashes must verify against the original password")
	}
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.GenerateTokenPair("user-1", "alice", "alice@example.com")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("empty access token")
	}
	if pair.RefreshToken == "" {
		t.Fatal("empty refresh token")
	}

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID: got %q want %q", claims.UserID, "user-1")
	}
	if claims.Username != "alice" {
		t.Errorf("Username: got %q want %q", claims.Username, "alice")
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("Email: got %q", claims.Email)
	}
}

func TestValidateAccessTokenRejectsTampered(t *testing.T) {
	svc := newTestService(t)
	pair, _ := svc.GenerateTokenPair("u", "u", "u@x")

	// JWT format: header.payload.signature. Flip a char in the
	// payload to invalidate the signature.
	parts := strings.Split(pair.AccessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format: %d parts", len(parts))
	}
	tampered := parts[0] + "." + parts[1] + "x" + "." + parts[2]
	if _, err := svc.ValidateAccessToken(tampered); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestValidateAccessTokenRejectsWrongSecret(t *testing.T) {
	svc := newTestService(t)
	pair, _ := svc.GenerateTokenPair("u", "u", "u@x")

	other := NewAuthService("a-different-secret-key", time.Minute, time.Minute)
	if _, err := other.ValidateAccessToken(pair.AccessToken); err == nil {
		t.Fatal("expected error: token must not validate against different secret")
	}
}

func TestValidateAccessTokenRejectsExpired(t *testing.T) {
	// Negative TTL means the token is already expired by the time it's issued.
	svc := NewAuthService("test-secret", -time.Hour, time.Hour)
	pair, _ := svc.GenerateTokenPair("u", "u", "u@x")

	if _, err := svc.ValidateAccessToken(pair.AccessToken); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateAccessTokenRejectsGarbage(t *testing.T) {
	svc := newTestService(t)
	for _, bad := range []string{"", "not-a-jwt", "a.b.c", "header.payload."} {
		if _, err := svc.ValidateAccessToken(bad); err == nil {
			t.Errorf("expected error for input %q", bad)
		}
	}
}

func TestRefreshTokenTTL(t *testing.T) {
	svc := NewAuthService("k", time.Minute, 42*time.Hour)
	if got := svc.RefreshTokenTTL(); got != 42*time.Hour {
		t.Fatalf("RefreshTokenTTL: got %v want %v", got, 42*time.Hour)
	}
}

func TestRefreshTokensAreRandom(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.GenerateTokenPair("u", "u", "u@x")
	b, _ := svc.GenerateTokenPair("u", "u", "u@x")

	if a.RefreshToken == b.RefreshToken {
		t.Fatal("refresh tokens must be unique per call")
	}
	// Refresh tokens are not JWTs (the service stores them server-side
	// for rotation); they're random opaque strings, so just verify
	// they're non-trivial.
	if len(a.RefreshToken) < 16 {
		t.Fatalf("refresh token unexpectedly short: %d", len(a.RefreshToken))
	}
}
