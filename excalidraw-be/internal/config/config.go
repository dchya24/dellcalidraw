package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv    string          `mapstructure:"app_env"`
	Server    ServerConfig    `mapstructure:"server"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	Room      RoomConfig      `mapstructure:"room"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Auth      AuthConfig      `mapstructure:"auth"`
	AI        AIConfig        `mapstructure:"ai"`
	Log       LogConfig       `mapstructure:"log"`
	CORS      CORSConfig      `mapstructure:"cors"`
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development" || c.AppEnv == "dev"
}

type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type WebSocketConfig struct {
	ReadBufferSize  int           `mapstructure:"read_buffer_size"`
	WriteBufferSize int           `mapstructure:"write_buffer_size"`
	PingPeriod      time.Duration `mapstructure:"ping_period"`
	PongWait        time.Duration `mapstructure:"pong_wait"`
}

type RoomConfig struct {
	Capacity          int           `mapstructure:"capacity"`
	InactivityTimeout time.Duration `mapstructure:"inactivity_timeout"`
	CleanupInterval   time.Duration `mapstructure:"cleanup_interval"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type StorageConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Public    bool   `mapstructure:"public"`
}

type AuthConfig struct {
	SecretKey       string        `mapstructure:"secret_key"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

type AIConfig struct {
	Provider    string  `mapstructure:"provider"`    // openai | anthropic
	APIKey      string  `mapstructure:"api_key"`
	BaseURL     string  `mapstructure:"base_url"`    // For OpenAI-compatible endpoints
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func LoadFromEnv() (*Config, error) {
	// Load .env file if it exists
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	setDefaults()
	bindEnvVars()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func bindEnvVars() {
	// Server
	_ = viper.BindEnv("server.port", "EXCALIDRAW_SERVER_PORT")
	_ = viper.BindEnv("server.read_timeout", "EXCALIDRAW_SERVER_READ_TIMEOUT")
	_ = viper.BindEnv("server.write_timeout", "EXCALIDRAW_SERVER_WRITE_TIMEOUT")
	_ = viper.BindEnv("server.idle_timeout", "EXCALIDRAW_SERVER_IDLE_TIMEOUT")

	// WebSocket
	_ = viper.BindEnv("websocket.read_buffer_size", "EXCALIDRAW_WEBSOCKET_READ_BUFFER_SIZE")
	_ = viper.BindEnv("websocket.write_buffer_size", "EXCALIDRAW_WEBSOCKET_WRITE_BUFFER_SIZE")
	_ = viper.BindEnv("websocket.ping_period", "EXCALIDRAW_WEBSOCKET_PING_PERIOD")
	_ = viper.BindEnv("websocket.pong_wait", "EXCALIDRAW_WEBSOCKET_PONG_WAIT")

	// Room
	_ = viper.BindEnv("room.capacity", "EXCALIDRAW_ROOM_CAPACITY")
	_ = viper.BindEnv("room.inactivity_timeout", "EXCALIDRAW_ROOM_INACTIVITY_TIMEOUT")
	_ = viper.BindEnv("room.cleanup_interval", "EXCALIDRAW_ROOM_CLEANUP_INTERVAL")

	// Database
	_ = viper.BindEnv("database.host", "EXCALIDRAW_DATABASE_HOST")
	_ = viper.BindEnv("database.port", "EXCALIDRAW_DATABASE_PORT")
	_ = viper.BindEnv("database.user", "EXCALIDRAW_DATABASE_USER")
	_ = viper.BindEnv("database.password", "EXCALIDRAW_DATABASE_PASSWORD")
	_ = viper.BindEnv("database.dbname", "EXCALIDRAW_DATABASE_DBNAME")
	_ = viper.BindEnv("database.sslmode", "EXCALIDRAW_DATABASE_SSLMODE")
	_ = viper.BindEnv("database.max_open_conns", "EXCALIDRAW_DATABASE_MAX_OPEN_CONNS")
	_ = viper.BindEnv("database.max_idle_conns", "EXCALIDRAW_DATABASE_MAX_IDLE_CONNS")
	_ = viper.BindEnv("database.conn_max_lifetime", "EXCALIDRAW_DATABASE_CONN_MAX_LIFETIME")

	// Storage
	_ = viper.BindEnv("storage.endpoint", "EXCALIDRAW_STORAGE_ENDPOINT")
	_ = viper.BindEnv("storage.access_key", "EXCALIDRAW_STORAGE_ACCESS_KEY")
	_ = viper.BindEnv("storage.secret_key", "EXCALIDRAW_STORAGE_SECRET_KEY")
	_ = viper.BindEnv("storage.bucket", "EXCALIDRAW_STORAGE_BUCKET")
	_ = viper.BindEnv("storage.region", "EXCALIDRAW_STORAGE_REGION")
	_ = viper.BindEnv("storage.use_ssl", "EXCALIDRAW_STORAGE_USE_SSL")
	_ = viper.BindEnv("storage.public", "EXCALIDRAW_STORAGE_PUBLIC")

	// Auth
	_ = viper.BindEnv("auth.secret_key", "EXCALIDRAW_AUTH_SECRET_KEY")
	_ = viper.BindEnv("auth.access_token_ttl", "EXCALIDRAW_AUTH_ACCESS_TOKEN_TTL")
	_ = viper.BindEnv("auth.refresh_token_ttl", "EXCALIDRAW_AUTH_REFRESH_TOKEN_TTL")

	// AI
	_ = viper.BindEnv("ai.provider", "EXCALIDRAW_AI_PROVIDER")
	_ = viper.BindEnv("ai.api_key", "EXCALIDRAW_AI_API_KEY")
	_ = viper.BindEnv("ai.base_url", "EXCALIDRAW_AI_BASE_URL")
	_ = viper.BindEnv("ai.model", "EXCALIDRAW_AI_MODEL")
	_ = viper.BindEnv("ai.max_tokens", "EXCALIDRAW_AI_MAX_TOKENS")
	_ = viper.BindEnv("ai.temperature", "EXCALIDRAW_AI_TEMPERATURE")

	// Log
	_ = viper.BindEnv("log.level", "EXCALIDRAW_LOG_LEVEL")
	_ = viper.BindEnv("log.format", "EXCALIDRAW_LOG_FORMAT")

	// App Environment
	_ = viper.BindEnv("app_env", "APP_ENV")

	// CORS
	_ = viper.BindEnv("cors.allowed_origins", "EXCALIDRAW_CORS_ALLOWED_ORIGINS")
	_ = viper.BindEnv("cors.allowed_methods", "EXCALIDRAW_CORS_ALLOWED_METHODS")
	_ = viper.BindEnv("cors.allowed_headers", "EXCALIDRAW_CORS_ALLOWED_HEADERS")
	_ = viper.BindEnv("cors.allow_credentials", "EXCALIDRAW_CORS_ALLOW_CREDENTIALS")
	_ = viper.BindEnv("cors.max_age", "EXCALIDRAW_CORS_MAX_AGE")
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.read_timeout", 10*time.Second)
	viper.SetDefault("server.write_timeout", 10*time.Second)
	viper.SetDefault("server.idle_timeout", 60*time.Second)

	// WebSocket defaults
	viper.SetDefault("websocket.read_buffer_size", 1024)
	viper.SetDefault("websocket.write_buffer_size", 1024)
	viper.SetDefault("websocket.ping_period", 54*time.Second)
	viper.SetDefault("websocket.pong_wait", 60*time.Second)

	// Room defaults
	viper.SetDefault("room.capacity", 50)
	viper.SetDefault("room.inactivity_timeout", time.Hour)
	viper.SetDefault("room.cleanup_interval", 10*time.Minute)

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "excalidraw")
	viper.SetDefault("database.password", "excalidraw")
	viper.SetDefault("database.dbname", "excalidraw")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 5*time.Minute)

	// Storage defaults
	viper.SetDefault("storage.endpoint", "localhost:9000")
	viper.SetDefault("storage.access_key", "minioadmin")
	viper.SetDefault("storage.secret_key", "minioadmin")
	viper.SetDefault("storage.bucket", "excalidraw-files")
	viper.SetDefault("storage.region", "us-east-1")
	viper.SetDefault("storage.use_ssl", false)
	viper.SetDefault("storage.public", false)

	// Auth defaults
	viper.SetDefault("auth.secret_key", "change-me-in-production-please")
	viper.SetDefault("auth.access_token_ttl", 15*time.Minute)
	viper.SetDefault("auth.refresh_token_ttl", 7*24*time.Hour)

	// AI defaults
	viper.SetDefault("ai.provider", "openai")
	viper.SetDefault("ai.api_key", "")
	viper.SetDefault("ai.base_url", "")
	viper.SetDefault("ai.model", "gpt-4o")
	viper.SetDefault("ai.max_tokens", 4096)
	viper.SetDefault("ai.temperature", 0.7)

	// Log defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")

	// App Environment defaults
	viper.SetDefault("app_env", "development")

	// CORS defaults
	viper.SetDefault("cors.allowed_origins", []string{"http://localhost:3000", "http://localhost:5173"})
	viper.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.allowed_headers", []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"})
	viper.SetDefault("cors.allow_credentials", true)
	viper.SetDefault("cors.max_age", 300)
}
