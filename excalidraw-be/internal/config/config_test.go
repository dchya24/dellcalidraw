package config

import (
	"strings"
	"testing"
	"time"
)

func TestIsDevelopment(t *testing.T) {
	for _, env := range []string{"development", "dev"} {
		c := &Config{AppEnv: env}
		if !c.IsDevelopment() {
			t.Errorf("AppEnv=%q must be considered development", env)
		}
	}
	for _, env := range []string{"production", "prod", "staging", ""} {
		c := &Config{AppEnv: env}
		if c.IsDevelopment() {
			t.Errorf("AppEnv=%q must not be considered development", env)
		}
	}
}

func TestLoadFromEnvDefaults(t *testing.T) {
	// Ensure no .env file leaks defaults from previous tests
	t.Setenv("EXCALIDRAW_SERVER_PORT", "")
	t.Setenv("EXCALIDRAW_WEBSOCKET_ENCRYPTION_ENABLED", "")
	t.Setenv("EXCALIDRAW_SECURITY_ENABLE_HSTS", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}

	// Sanity-check the defaults exposed in the public docs / .env.example
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Server.Port", cfg.Server.Port, "8080"},
		{"Server.ReadTimeout", cfg.Server.ReadTimeout, 10 * time.Second},
		{"WebSocket.PingPeriod", cfg.WebSocket.PingPeriod, 54 * time.Second},
		{"WebSocket.EncryptionEnabled (default true)", cfg.WebSocket.EncryptionEnabled, true},
		{"Room.Capacity", cfg.Room.Capacity, 50},
		{"Room.InactivityTimeout", cfg.Room.InactivityTimeout, time.Hour},
		{"Database.MaxOpenConns", cfg.Database.MaxOpenConns, 25},
		{"Database.MaxIdleConns", cfg.Database.MaxIdleConns, 5},
		{"Auth.AccessTokenTTL", cfg.Auth.AccessTokenTTL, 15 * time.Minute},
		{"Auth.RefreshTokenTTL", cfg.Auth.RefreshTokenTTL, 7 * 24 * time.Hour},
		{"Security.EnableHSTS (off by default)", cfg.Security.EnableHSTS, false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v want %v", c.name, c.got, c.want)
		}
	}
}

func TestLoadFromEnvOverrides(t *testing.T) {
	t.Setenv("EXCALIDRAW_SERVER_PORT", "9090")
	t.Setenv("EXCALIDRAW_ROOM_CAPACITY", "200")
	t.Setenv("EXCALIDRAW_WEBSOCKET_ENCRYPTION_ENABLED", "false")
	t.Setenv("EXCALIDRAW_SECURITY_ENABLE_HSTS", "true")
	t.Setenv("EXCALIDRAW_SECURITY_HSTS_MAX_AGE", "600")
	t.Setenv("EXCALIDRAW_AI_TEMPERATURE", "0.3")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}

	if cfg.Server.Port != "9090" {
		t.Errorf("Server.Port: got %q want 9090", cfg.Server.Port)
	}
	if cfg.Room.Capacity != 200 {
		t.Errorf("Room.Capacity: got %d want 200", cfg.Room.Capacity)
	}
	if cfg.WebSocket.EncryptionEnabled {
		t.Error("WebSocket.EncryptionEnabled must be false when env says so")
	}
	if !cfg.Security.EnableHSTS {
		t.Error("Security.EnableHSTS must be true when env says so")
	}
	if cfg.Security.HSTSMaxAge != 600 {
		t.Errorf("Security.HSTSMaxAge: got %d want 600", cfg.Security.HSTSMaxAge)
	}
	if cfg.AI.Temperature != 0.3 {
		t.Errorf("AI.Temperature: got %v want 0.3", cfg.AI.Temperature)
	}
}

func TestLoadFromEnvAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if !cfg.IsDevelopment() {
		t.Errorf("APP_ENV=development must yield IsDevelopment()=true, got AppEnv=%q", cfg.AppEnv)
	}
}

func TestLoadFromEnvCORSList(t *testing.T) {
	// Viper splits comma-separated env values into []string for slice
	// fields. Make sure that path actually works.
	t.Setenv("EXCALIDRAW_CORS_ALLOWED_ORIGINS", "http://a.local,http://b.local")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	joined := strings.Join(cfg.CORS.AllowedOrigins, ",")
	if !strings.Contains(joined, "a.local") || !strings.Contains(joined, "b.local") {
		t.Errorf("CORS.AllowedOrigins not parsed: %v", cfg.CORS.AllowedOrigins)
	}
}
