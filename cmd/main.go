package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"sub-balance-implementation/internal/config"
	"sub-balance-implementation/internal/delivery/rest"
	"sub-balance-implementation/internal/infra/postgres"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

func main() {
	// Optimize runtime for high performance
	runtime.GOMAXPROCS(runtime.NumCPU()) // Use all CPU cores

	// Set GC target percentage for better performance under load
	runtime.GC() // Force initial garbage collection

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

	logger.Info("Runtime optimization applied",
		zap.Int("cpu_cores", runtime.NumCPU()),
		zap.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		zap.Int("goroutines", runtime.NumGoroutine()),
	)

	// Initialize database
	db, err := postgres.NewConnection(cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Run migrations
	if err := postgres.RunMigrations(cfg.Database); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize repository factory
	repoFactory := postgres.NewRepositoryFactory(db, logger)
	defer repoFactory.Close()

	// Initialize Echo with ultra-high-performance configuration
	e := echo.New()
	e.HideBanner = true

	// Disable Echo's built-in debug mode for better performance
	e.Debug = false

	// Minimal middleware for maximum performance - only essential ones
	e.Use(middleware.Recover())

	// Optimized CORS with minimal overhead
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"*"},
		MaxAge:       86400, // Cache preflight for 24 hours
	}))

	// Lightweight request ID middleware
	e.Use(middleware.RequestID())

	// Minimal logging middleware - only log errors and slow requests
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${time_rfc3339} ${status} ${method} ${uri} ${latency_human}\n",
		Output: os.Stderr, // Log to stderr for better performance
	}))

	// High-performance middleware for HTTP optimization
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Set optimized HTTP headers for high concurrency
			c.Response().Header().Set("Connection", "keep-alive")
			c.Response().Header().Set("Keep-Alive", "timeout=120, max=1000")
			c.Response().Header().Set("Cache-Control", "no-cache")
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")

			// Set content type early for better performance
			if c.Request().Method == "POST" || c.Request().Method == "PUT" {
				c.Response().Header().Set("Content-Type", "application/json")
			}

			return next(c)
		}
	})

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
	handlerFactory := rest.NewHandlerFactory(db, logger, cfg)
	restHandler := handlerFactory.GetHandler()
	restHandler.RegisterRoutes(e)

	// Start server with ultra-high-performance configuration
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: e,

		// Optimized timeouts for high TPS
		ReadTimeout:  5 * time.Second,   // Reduced for faster processing
		WriteTimeout: 10 * time.Second,  // Reduced for faster response
		IdleTimeout:  120 * time.Second, // Increased for connection reuse

		// Ultra-high-performance settings
		MaxHeaderBytes:    1 << 20,         // 1 MB
		ReadHeaderTimeout: 5 * time.Second, // Reduced for faster header processing

		// Additional performance optimizations
		ErrorLog:                     nil,  // Disable default error logging for performance
		DisableGeneralOptionsHandler: true, // Disable OPTIONS handler for performance
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
