package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"sub-balance-implementation/internal/config"
	"sub-balance-implementation/internal/delivery/rest"
	"sub-balance-implementation/internal/infra/postgres"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func main() {
	// Optimize runtime for ultra-high performance
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

	logger.Info("FastHTTP runtime optimization applied",
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

	// Initialize FastHTTP handler
	fastHTTPHandler := rest.NewFastHTTPHandler(db, logger, cfg)

	// Configure ultra-high-performance FastHTTP server
	server := &fasthttp.Server{
		Handler: fastHTTPHandler.GetHandler(),

		// Ultra-optimized settings for maximum TPS
		ReadBufferSize:                64 * 1024, // 64KB read buffer
		WriteBufferSize:               64 * 1024, // 64KB write buffer
		ReadTimeout:                   5 * time.Second,
		WriteTimeout:                  10 * time.Second,
		IdleTimeout:                   120 * time.Second,
		MaxConnsPerIP:                 1000,  // Allow many connections per IP
		MaxRequestsPerConn:            10000, // Reuse connections extensively
		MaxKeepaliveDuration:          120 * time.Second,
		MaxIdleWorkerDuration:         10 * time.Second,
		TCPKeepalivePeriod:            10 * time.Second,
		DisableHeaderNamesNormalizing: true,  // Skip header normalization for speed
		NoDefaultServerHeader:         true,  // Remove default server header
		NoDefaultDate:                 false, // Keep date header
		NoDefaultContentType:          false, // Keep content-type
		DisablePreParseMultipartForm:  true,  // Skip multipart parsing
		ReduceMemoryUsage:             true,  // Reduce memory usage
		GetOnly:                       false, // Allow all methods
		DisableKeepalive:              false, // Enable keep-alive

		// Error handling
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
			logger.Error("FastHTTP error", zap.Error(err))
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.WriteString(`{"error":"Internal server error"}`)
		},

		// Logging (minimal for performance)
		Logger: nil, // Disable default logging for performance
	}

	// Start server in goroutine
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		logger.Info("Starting FastHTTP server",
			zap.String("address", addr),
			zap.Int("port", cfg.Server.Port),
			zap.String("framework", "FastHTTP"),
		)

		if err := server.ListenAndServe(addr); err != nil {
			logger.Fatal("Failed to start FastHTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down FastHTTP server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.ShutdownWithContext(ctx); err != nil {
		logger.Fatal("FastHTTP server forced to shutdown", zap.Error(err))
	}

	logger.Info("FastHTTP server exited")
}
