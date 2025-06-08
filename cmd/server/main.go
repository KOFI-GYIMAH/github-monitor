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
	"github.com/KOFI-GYIMAH/github-monitor/internal/service"
	"github.com/KOFI-GYIMAH/github-monitor/internal/worker"
	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title GitHub Monitory Service
// @version 1.0.0
// @description A Go service that monitors GitHub repositories, tracks commits, and stores data in a persistent database.
// @host localhost:8081
// @BasePath /api/v1
func main() {
	if os.Getenv("DEBUG") == "true" {
		logger.SetLevel(logger.LevelDebug)
	}

	// * Load configuration
	cfg, err := config.LoadConfiguration()
	if err != nil {
		logger.Error("Failed to load config: %v", err)
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
		logger.Info("Successfully ran migrations 🎉")
	}

	// * Initialize GitHub client
	githubClient := github.NewClient(cfg.GitHubToken)

	// * Create services
	repoService := service.NewRepositoryService(githubClient, database)

	// * Parse sync interval
	syncInterval, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		logger.Error("Invalid sync interval: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// * List repositories from DB
	repositories, err := repoService.ListAllRepositories(ctx)
	if err != nil {
		logger.Error("Failed to load repositories from DB: %v", err)
	}

	if len(repositories) == 0 {
		logger.Warn("No repositories found in the database. Falling back to default repository...")

		owner, name, err := config.ParseRepository(cfg.DefaultRepository)
		if err != nil {
			logger.Error("Invalid default repository format: %v", err)
			os.Exit(1)
		}

		// * Sync the default repo
		logger.Info("Syncing default repository: %s/%s", owner, name)
		if err := repoService.SyncRepository(ctx, owner, name, time.Time{}); err != nil {
			logger.Error("Failed to sync default repository: %v", err)
			os.Exit(1)
		}

		// * Reload repositories after syncing
		repositories, err = repoService.ListAllRepositories(ctx)
		if err != nil {
			logger.Error("Failed to reload repositories from DB: %v", err)
			os.Exit(1)
		}
	}

	for _, repo := range repositories {
		owner, name, err := config.ParseRepository(repo.Name)
		if err != nil {
			logger.Error("Invalid repository format: %v", err)
			os.Exit(1)
		}
		w := worker.NewSyncWorker(repoService, syncInterval, owner, name)
		go w.Run(ctx)
	}

	// * Create API server
	apiHandler := handler.NewRepositoryHandler(ctx, repoService)
	router := mux.NewRouter()
	router.Use(md.LoggingMiddleware)
	api := router.PathPrefix("/api/v1").Subrouter()

	apiHandler.RegisterRoutes(api)
	router.PathPrefix("/api/v1/swagger/").Handler(httpSwagger.WrapHandler)

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
