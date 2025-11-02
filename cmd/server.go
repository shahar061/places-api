package cmd

import (
	"log"

	"places_api/internal/config"
	"places_api/internal/logger"
	"places_api/internal/server"

	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Places API server",
	Long: `Start the Places API HTTP server with the specified configuration.
The server will listen for incoming requests on the configured host and port.`,
	Run: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger.InitGlobal(logger.LogConfig{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		TimeFormat: cfg.Logging.TimeFormat,
	})

	logger.Info().Msg("Starting Places API server")
	logger.Info().Str("host", cfg.Server.Host).Int("port", cfg.Server.Port).Msg("Server configuration")

	// Create and start server
	srv := server.New(cfg)
	if err := srv.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start server")
	}
}
