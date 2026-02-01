package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	Room RoomConfig `mapstructure:"room"`
	Log LogConfig `mapstructure:"log"`
}

type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type WebSocketConfig struct {
	ReadBufferSize  int `mapstructure:"read_buffer_size"`
	WriteBufferSize int `mapstructure:"write_buffer_size"`
	PingPeriod      time.Duration `mapstructure:"ping_period"`
	PongWait        time.Duration `mapstructure:"pong_wait"`
}

type RoomConfig struct {
	Capacity          int           `mapstructure:"capacity"`
	InactivityTimeout time.Duration `mapstructure:"inactivity_timeout"`
	CleanupInterval   time.Duration `mapstructure:"cleanup_interval"`
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

	// Log defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
}
