package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	Room      RoomConfig      `mapstructure:"room"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Log       LogConfig       `mapstructure:"log"`
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

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
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
	setDefaults()

	// Override with environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("EXCALIDRAW")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
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

	// Log defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
}
