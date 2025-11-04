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

// bindEnvWithFallback binds an environment variable with fallback options
// It tries each env var name in order until it finds one that exists
func bindEnvWithFallback(key string, envVars ...string) {
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			viper.BindEnv(key, envVar)
			return
		}
	}
	// If none found, bind to the first one (for viper.AutomaticEnv to handle)
	if len(envVars) > 0 {
		viper.BindEnv(key, envVars[0])
	}
}

// LoadConfig reads configuration from environment variables
func LoadConfig(configPath string) (*Config, error) {
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
	// Support both prefixed (PLACES_API_*) and non-prefixed versions
	// Check prefixed versions first, then fall back to non-prefixed
	bindEnvWithFallback("database.supabase_url", "PLACES_API_DATABASE_SUPABASE_URL", "SUPABASE_URL")
	bindEnvWithFallback("database.supabase_key", "PLACES_API_DATABASE_SUPABASE_KEY", "SUPABASE_KEY")
	bindEnvWithFallback("ai.openrouter_api_key", "PLACES_API_AI_OPENROUTER_API_KEY", "OPENROUTER_API_KEY")
	bindEnvWithFallback("ai.model", "PLACES_API_AI_MODEL", "AI_MODEL")
	bindEnvWithFallback("nats.url", "PLACES_API_NATS_URL", "NATS_URL")
	bindEnvWithFallback("nats.cluster_id", "PLACES_API_NATS_CLUSTER_ID", "NATS_CLUSTER_ID")
	bindEnvWithFallback("server.host", "PLACES_API_SERVER_HOST", "SERVER_HOST", "HOST")
	bindEnvWithFallback("server.port", "PLACES_API_SERVER_PORT", "SERVER_PORT")
	bindEnvWithFallback("logging.level", "PLACES_API_LOG_LEVEL", "LOG_LEVEL")
	bindEnvWithFallback("logging.format", "PLACES_API_LOG_FORMAT", "LOG_FORMAT")
	bindEnvWithFallback("logging.timeformat", "PLACES_API_LOG_TIME_FORMAT", "LOG_TIME_FORMAT")

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
