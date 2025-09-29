package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sub-balance-implementation/internal/config"
	"sub-balance-implementation/internal/delivery/rest"
	"sub-balance-implementation/internal/infra"
	"sub-balance-implementation/internal/infra/postgres"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize database connection(s)
	var db *gorm.DB
	var shardRouter *infra.ShardRouter

	if cfg.Sharding.Enabled {
		logger.Info("Initializing database sharding", zap.Int("shard_count", cfg.Sharding.ShardCount))

		// Initialize shard router
		shardConfigs := make(map[string]string)
		for i, shard := range cfg.Sharding.Shards {
			dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
				shard.Host, shard.Port, shard.User, shard.Password, shard.DBName, shard.SSLMode)
			shardConfigs[fmt.Sprintf("%d", i)] = dsn
		}

		shardRouter, err = infra.NewShardRouter(shardConfigs)
		if err != nil {
			logger.Fatal("Failed to initialize shard router", zap.Error(err))
		}
		defer shardRouter.Close()

		// Use first shard as primary for now
		db, err = postgres.NewConnection(cfg.Sharding.Shards[0])
		if err != nil {
			logger.Fatal("Failed to connect to primary database", zap.Error(err))
		}
	} else {
		// Single database connection
		db, err = postgres.NewConnection(cfg.Database)
		if err != nil {
			logger.Fatal("Failed to connect to database", zap.Error(err))
		}
	}

	// Run migrations (skip if configured)
	if !cfg.Database.SkipMigrations {
		if err := postgres.RunMigrations(cfg.Database); err != nil {
			logger.Fatal("Failed to run migrations", zap.Error(err))
		}
	} else {
		logger.Info("Skipping database migrations")
	}

	// Initialize repository factory
	var repoFactory *postgres.RepositoryFactory
	if cfg.Sharding.Enabled {
		logger.Info("Creating RepositoryFactory with sharding", zap.Bool("sharding_enabled", cfg.Sharding.Enabled))
		repoFactory = postgres.NewRepositoryFactoryWithSharding(db, shardRouter, logger)
	} else {
		logger.Info("Creating RepositoryFactory without sharding", zap.Bool("sharding_enabled", cfg.Sharding.Enabled))
		repoFactory = postgres.NewRepositoryFactory(db, logger)
	}
	defer repoFactory.Close()

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})

	// Metrics endpoint
	e.GET("/metrics", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"uptime": time.Since(time.Now()).String(),
		})
	})

	// Initialize REST handlers
	handlerFactory := rest.NewHandlerFactoryWithRepos(repoFactory, logger)
	restHandler := handlerFactory.GetHandler()
	restHandler.RegisterRoutes(e)

	// Start server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      e,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting server",
			zap.String("address", server.Addr),
			zap.Int("port", cfg.Server.Port),
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
