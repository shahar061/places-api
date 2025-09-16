package server

import (
	"fmt"

	"places_api/internal/config"
	"places_api/internal/handlers"

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

	handler := handlers.New(cfg)

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
