package server

import (
	"fmt"
	"log"

	"places_api/internal/ai"
	"places_api/internal/config"
	"places_api/internal/handlers"
	"places_api/internal/services"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	config  *config.Config
	router  *gin.Engine
	handler *handlers.Handler
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

	// Initialize location services
	overpassService := services.NewOverpassService()
	nominatimService := services.NewNominatimService()

	// Initialize AI Planner service
	aiPlannerService, err := ai.NewAIPlannerService(&cfg.AI, supabaseService, overpassService, nominatimService)
	if err != nil {
		log.Printf("Warning: Failed to initialize AI Planner service: %v", err)
		log.Printf("AI endpoints will not be available. Set PLACES_API_AI_OPENROUTER_API_KEY environment variable.")
	}

	handler := handlers.New(cfg, supabaseService, aiPlannerService)

	server := &Server{
		config:  cfg,
		router:  router,
		handler: handler,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all the routes using the handler
func (s *Server) setupRoutes() {
	s.handler.SetupRoutes(s.router)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	fmt.Printf("Starting server on %s\n", addr)
	return s.router.Run(addr)
}
