package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/KOFI-GYIMAH/github-monitor/docs"
	"github.com/KOFI-GYIMAH/github-monitor/internal/config"
	"github.com/KOFI-GYIMAH/github-monitor/internal/db"
	"github.com/KOFI-GYIMAH/github-monitor/internal/github"
	"github.com/KOFI-GYIMAH/github-monitor/internal/handler"
	md "github.com/KOFI-GYIMAH/github-monitor/internal/middleware"

	// "github.com/KOFI-GYIMAH/github-monitor/internal/queue"
	"github.com/KOFI-GYIMAH/github-monitor/internal/service"
	"github.com/KOFI-GYIMAH/github-monitor/internal/worker"
	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title GitHub Monitory Service
// @version 1.0.0
// @description This is a sample server.
// @host localhost:8081
// @BasePath /v1
func main() {
	if os.Getenv("DEBUG") == "true" {
		logger.SetLevel(logger.LevelDebug)
	}

	// * Load configuration
	cfg, err := config.LoadConfiguration()
	if err != nil {
		logger.Error("‼️ Failed to load config: %v", err)
	}

	// * Initialize PostgreSQL database
	database, err := db.NewPostgresDB(cfg.DBURL)
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// * Run migrations
	if err := database.Migrate(); err != nil {
		logger.Error("Failed to run migrations: %v", err)
	} else {
		logger.Info("Successfully ran migrations")
	}

	// * Initialize GitHub client
	githubClient := github.NewClient(cfg.GitHubToken)

	// rabbitMQ, err := queue.NewRabbitMQ(cfg.RabbitMQURL)
	// if err != nil {
	// 	logger.Error("Failed to initialize RabbitMQ: %v", err)
	// 	os.Exit(1)
	// }
	// defer rabbitMQ.Close()

	// * Create services
	repoService := service.NewRepositoryService(githubClient, database)

	// * Parse sync interval
	syncInterval, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		logger.Error("Invalid sync interval: %v", err)
	}

	// * Parse repository owner/name
	owner, name, err := config.ParseRepository(cfg.Repository)
	if err != nil {
		logger.Error("Invalid repository format: %v", err)
	}

	// * Create and start worker
	worker := worker.NewSyncWorker(repoService, syncInterval, owner, name)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.Run(ctx)

	// * Create API server
	apiHandler := handler.NewRepositoryHandler(repoService)
	router := mux.NewRouter()
	router.Use(md.LoggingMiddleware)
	api := router.PathPrefix("/v1").Subrouter()

	apiHandler.RegisterRoutes(api)
	router.PathPrefix("/v1/swagger/").Handler(httpSwagger.WrapHandler)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = ":8081"
	}

	server := &http.Server{
		Addr:    port,
		Handler: router,
	}

	go func() {
		logger.Info("Starting API server on %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error: %v", err)
			os.Exit(1)
		}
	}()

	// * Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
}
