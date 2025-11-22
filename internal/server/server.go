package server

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"places_api/internal/ai"
	"places_api/internal/config"
	"places_api/internal/handlers"
	"places_api/internal/services"
	"places_api/internal/worker"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	config      *config.Config
	router      *gin.Engine
	handler     *handlers.Handler
	natsService *services.NATSService
	worker      *worker.Worker
}

// New creates a new server instance
func New(cfg *config.Config) *Server {
	// Set gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Initialize Supabase service
	supabaseService, err := services.NewSupabaseService(&cfg.Database)
	if err != nil {
		log.Printf("Warning: Failed to initialize Supabase service: %v", err)
		log.Printf("API will run with mock data only. Set PLACES_API_DATABASE_SUPABASE_URL and PLACES_API_DATABASE_SUPABASE_KEY environment variables.")
	}

	// Initialize AI service
	aiService := ai.NewService(cfg)

	// Initialize Photon service
	photonService := services.NewPhotonService()

	// Initialize LocationIQ service (optional, for fallback geocoding)
	locationiqService := services.NewLocationIQService(cfg.LocationIQ.APIKey)

	// Initialize NATS service
	var natsService *services.NATSService
	var jobWorker *worker.Worker

	if cfg.NATS.URL != "" {
		log.Printf("Attempting to connect to NATS at: %s", cfg.NATS.URL)
		natsService, err = services.NewNATSService(&cfg.NATS)
		if err != nil {
			log.Printf("Warning: Failed to initialize NATS service: %v", err)
			log.Printf("Background job processing will be disabled. Ensure NATS service is running and accessible.")
			log.Printf("If using Docker, verify NATS container is on the same network and the hostname is resolvable.")
		} else {
			// Initialize job service and worker
			jobService := services.NewJobService(supabaseService, natsService)
			jobWorker = worker.NewWorker(jobService, natsService, supabaseService, aiService, photonService, locationiqService)
		}
	} else {
		log.Printf("NATS URL not configured. Set PLACES_API_NATS_URL or NATS_URL environment variable to enable background job processing.")
	}

	handler := handlers.New(cfg, supabaseService, natsService, photonService, aiService)

	server := &Server{
		config:      cfg,
		router:      router,
		handler:     handler,
		natsService: natsService,
		worker:      jobWorker,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all the routes using the handler
func (s *Server) setupRoutes() {
	s.handler.SetupRoutes(s.router)
}

// Start starts the HTTP server and background worker
func (s *Server) Start() error {
	// Start background worker if available
	if s.worker != nil {
		err := s.worker.Start()
		if err != nil {
			log.Printf("Warning: Failed to start background worker: %v", err)
		}
	}

	// Setup graceful shutdown
	go s.setupGracefulShutdown()

	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	fmt.Printf("Starting Places API server on %s\n", addr)

	if s.natsService != nil {
		fmt.Printf("NATS JetStream enabled for background job processing\n")
	} else {
		fmt.Printf("NATS disabled - background job processing unavailable\n")
	}

	return s.router.Run(addr)
}

// setupGracefulShutdown handles graceful shutdown of services
func (s *Server) setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	fmt.Printf("\nShutting down gracefully...\n")

	// Stop worker
	if s.worker != nil {
		s.worker.Stop()
	}

	// Close NATS connection
	if s.natsService != nil {
		s.natsService.Close()
	}

	fmt.Printf("Shutdown complete\n")
	os.Exit(0)
}
