package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	AI         AIConfig         `mapstructure:"ai"`
	NATS       NATSConfig       `mapstructure:"nats"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	LocationIQ LocationIQConfig `mapstructure:"locationiq"`
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

// LocationIQConfig holds LocationIQ API configuration
type LocationIQConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// bindEnvWithFallback binds an environment variable with fallback options
// It tries each env var name in order until it finds one that exists
func bindEnvWithFallback(key string, envVars ...string) {
	for _, envVar := range envVars {
		value := os.Getenv(envVar)
		if value != "" {
			// Bind and also set directly to ensure it's loaded
			viper.BindEnv(key, envVar)
			viper.Set(key, value)
			fmt.Printf("Bound config key '%s' to env var '%s' (value length: %d)\n", key, envVar, len(value))
			return
		}
	}
	// If none found, bind to the first one (for viper.AutomaticEnv to handle)
	if len(envVars) > 0 {
		viper.BindEnv(key, envVars[0])
		fmt.Printf("Bound config key '%s' to env var '%s' (not found, using for fallback)\n", key, envVars[0])
	}
}

// LoadConfig reads configuration from environment variables
func LoadConfig(configPath string) (*Config, error) {
	// Debug: Print all environment variables that contain PLACES_API or OPENROUTER
	fmt.Println("=== Environment Variables Debug ===")
	allEnvVars := os.Environ()
	relevantVars := []string{}
	for _, envVar := range allEnvVars {
		if strings.Contains(envVar, "PLACES_API") ||
			strings.Contains(envVar, "OPENROUTER") ||
			strings.Contains(envVar, "AI") ||
			strings.Contains(envVar, "OPENROUTER") {
			relevantVars = append(relevantVars, envVar)
		}
	}
	if len(relevantVars) > 0 {
		fmt.Println("Relevant environment variables found:")
		for _, v := range relevantVars {
			// Mask API keys for security
			parts := strings.SplitN(v, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]
				if strings.Contains(key, "KEY") || strings.Contains(key, "SECRET") || strings.Contains(key, "TOKEN") {
					if len(value) > 8 {
						value = value[:8] + "..."
					}
				}
				fmt.Printf("  %s=%s\n", key, value)
			} else {
				fmt.Printf("  %s\n", v)
			}
		}
	} else {
		fmt.Println("No relevant environment variables found (PLACES_API, OPENROUTER, AI)")
	}

	// Check specific env vars
	fmt.Println("\nDirect environment variable checks:")
	fmt.Printf("  PLACES_API_AI_OPENROUTER_API_KEY: '%s' (len=%d)\n",
		os.Getenv("PLACES_API_AI_OPENROUTER_API_KEY"),
		len(os.Getenv("PLACES_API_AI_OPENROUTER_API_KEY")))
	fmt.Printf("  OPENROUTER_API_KEY: '%s' (len=%d)\n",
		os.Getenv("OPENROUTER_API_KEY"),
		len(os.Getenv("OPENROUTER_API_KEY")))
	fmt.Println("====================================")

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
	fmt.Println("\nBinding environment variables:")
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
	bindEnvWithFallback("locationiq.api_key", "PLACES_API_LOCATIONIQ_API_KEY", "LOCATIONIQ_API_KEY")
	fmt.Println()

	// Override with standard PORT env var (used by hosting platforms)
	if port := viper.GetString("PORT"); port != "" {
		viper.Set("server.port", port)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Debug: Log API key status (masked for security)
	if config.AI.OpenRouterAPIKey != "" {
		keyPreview := config.AI.OpenRouterAPIKey
		if len(keyPreview) > 8 {
			keyPreview = keyPreview[:8] + "..."
		}
		fmt.Printf("AI OpenRouter API key loaded: %s (model: %s)\n", keyPreview, config.AI.Model)
	} else {
		fmt.Printf("Warning: AI OpenRouter API key is NOT configured (checking env vars: PLACES_API_AI_OPENROUTER_API_KEY, OPENROUTER_API_KEY)\n")
		// Debug: Check what viper sees
		fmt.Printf("Debug: viper.Get('ai.openrouter_api_key') = '%s'\n", viper.GetString("ai.openrouter_api_key"))
		fmt.Printf("Debug: viper.Get('PLACES_API_AI_OPENROUTER_API_KEY') = '%s'\n", viper.GetString("PLACES_API_AI_OPENROUTER_API_KEY"))
	}

	return &config, nil
}
