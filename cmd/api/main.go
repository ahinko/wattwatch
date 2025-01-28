// Package main provides the entry point for the WattWatch API server
// @title WattWatch API
// @version 1.0
// @description WattWatch API server.
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer token authentication
// @Security BearerAuth
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"wattwatch/internal/api/routes"
	"wattwatch/internal/config"
	"wattwatch/internal/database"
	"wattwatch/internal/provider"
	"wattwatch/internal/validation"

	"github.com/joho/godotenv"
)

func main() {
	// Parse command line flags
	envFile := flag.String("env", ".env", "Path to env file")
	flag.Parse()

	// Load environment file
	if err := godotenv.Load(*envFile); err != nil && *envFile == ".env" {
		log.Printf("Warning: %v", err)
	}

	// Load configuration
	cfg := &config.Config{}
	if err := cfg.LoadFromEnv(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(cfg.Database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize validators
	validation.Initialize()

	// Initialize provider manager
	providerManager := provider.NewManager(db)

	// Setup routes
	router := routes.SetupRoutes(cfg, db, providerManager)

	// Convert port string to int
	port, err := strconv.Atoi(cfg.API.Port)
	if err != nil {
		log.Fatalf("Invalid port number: %v", err)
	}

	// Create server with graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on port %d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 5 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
