package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	AI       AIConfig       `mapstructure:"ai"`
	NATS     NATSConfig     `mapstructure:"nats"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// DatabaseConfig holds database-specific configuration
type DatabaseConfig struct {
	SupabaseURL string `mapstructure:"supabase_url"`
	SupabaseKey string `mapstructure:"supabase_key"`
}

// AIConfig holds AI service configuration
type AIConfig struct {
	OpenRouterAPIKey string `mapstructure:"openrouter_api_key"`
	Model            string `mapstructure:"model"`
}

// NATSConfig holds NATS-specific configuration
type NATSConfig struct {
	URL       string `mapstructure:"url"`
	ClusterID string `mapstructure:"cluster_id"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	TimeFormat string `mapstructure:"timeformat"`
}

// LoadConfig reads configuration from environment variables
func LoadConfig(configPath string) (*Config, error) {
	fmt.Printf("Environment variables: %v\n", os.Environ())
	// Set default values
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("ai.model", "deepseek/deepseek-chat-v3.1:free")
	viper.SetDefault("nats.url", "nats://localhost:4222")
	viper.SetDefault("nats.cluster_id", "places-cluster")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.timeformat", "rfc3339")

	// Enable reading from environment variables
	viper.AutomaticEnv()

	// Bind specific environment variables for nested configs
	viper.BindEnv("database.supabase_url", "SUPABASE_URL")
	viper.BindEnv("database.supabase_key", "SUPABASE_KEY")
	viper.BindEnv("ai.openrouter_api_key", "OPENROUTER_API_KEY")
	viper.BindEnv("ai.model", "AI_MODEL")
	viper.BindEnv("nats.url", "NATS_URL")
	viper.BindEnv("nats.cluster_id", "NATS_CLUSTER_ID")
	viper.BindEnv("server.host", "SERVER_HOST")
	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("logging.level", "LOG_LEVEL")
	viper.BindEnv("logging.format", "LOG_FORMAT")
	viper.BindEnv("logging.timeformat", "LOG_TIME_FORMAT")

	// Override with standard PORT env var (used by hosting platforms)
	if port := viper.GetString("PORT"); port != "" {
		viper.Set("server.port", port)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
