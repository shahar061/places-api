package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	AI       AIConfig       `mapstructure:"ai"`
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

// LoadConfig reads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
	}

	// Set default values
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("ai.model", "x-ai/grok-4-fast:free")

	// Enable reading from environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("PLACES_API")

	// Bind specific environment variables for nested configs
	viper.BindEnv("database.supabase_url", "PLACES_API_DATABASE_SUPABASE_URL")
	viper.BindEnv("database.supabase_key", "PLACES_API_DATABASE_SUPABASE_KEY")
	viper.BindEnv("ai.openrouter_api_key", "PLACES_API_AI_OPENROUTER_API_KEY")
	viper.BindEnv("ai.model", "PLACES_API_AI_MODEL")

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		// If config file is not found, we'll use defaults and env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Override with standard PORT env var (used by hosting platforms)
	// This must be after reading config file to override file settings
	if port := viper.GetString("PORT"); port != "" {
		viper.Set("server.port", port)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
