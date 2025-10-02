package rest

import (
	"context"
	"sync"
	"time"

	"sub-balance-implementation/internal/config"
	"sub-balance-implementation/internal/infra/postgres"
	"sub-balance-implementation/internal/usecase"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TransactionJob represents a transaction processing job
type TransactionJob struct {
	Request interface{}
	Result  chan TransactionResult
	Context context.Context
}

// TransactionResult represents the result of a transaction job
type TransactionResult struct {
	Data interface{}
	Err  error
}

// Handler contains all REST handlers
type Handler struct {
	db             *gorm.DB
	logger         *zap.Logger
	repos          *postgres.AllRepositories
	usecases       *usecase.AllUsecases
	jobQueue       chan TransactionJob
	workerPool     sync.WaitGroup
	circuitBreaker *CircuitBreaker
	config         *config.Config
}

// NewHandler creates a new REST handler with async processing
func NewHandler(db *gorm.DB, logger *zap.Logger, cfg *config.Config) *Handler {
	repoFactory := postgres.NewRepositoryFactory(db, logger)
	repos := repoFactory.GetAllRepositories()

	usecaseFactory := usecase.NewUsecaseFactory(db, repos, logger)
	usecases := usecaseFactory.GetAllUsecases()

	// Create ultra-high-performance job queue (massive buffer for sustained high TPS)
	jobQueue := make(chan TransactionJob, 200000) // 200k job buffer for sustained high TPS

	// Create circuit breaker for overload protection with ultra-high threshold
	// Increased to 10000 failures in 60s to prevent premature opening during high load
	circuitBreaker := NewCircuitBreaker(10000, 60*time.Second) // 10000 failures in 60s = open

	handler := &Handler{
		db:             db,
		logger:         logger,
		repos:          repos,
		usecases:       usecases,
		jobQueue:       jobQueue,
		circuitBreaker: circuitBreaker,
		config:         cfg,
	}

	// Start worker pool for async transaction processing
	handler.startWorkerPool()

	return handler
}

// startWorkerPool starts the worker pool for async processing
func (h *Handler) startWorkerPool() {
	// Start massive worker pool for sustained ultra-high concurrency
	numWorkers := 500 // Increased from 200 for sustained high TPS
	for i := 0; i < numWorkers; i++ {
		h.workerPool.Add(1)
		go h.worker(i)
	}

	h.logger.Info("Started ultra-high-performance worker pool", zap.Int("workers", numWorkers))
}

// worker processes transaction jobs from the queue
func (h *Handler) worker(workerID int) {
	defer h.workerPool.Done()

	h.logger.Info("Worker started", zap.Int("worker_id", workerID))

	for job := range h.jobQueue {
		// Process the transaction job
		result := h.processTransactionJob(job)

		// Send result back
		select {
		case job.Result <- result:
		case <-job.Context.Done():
			h.logger.Warn("Job context cancelled", zap.Int("worker_id", workerID))
		}
	}

	h.logger.Info("Worker stopped", zap.Int("worker_id", workerID))
}

// processTransactionJob processes a single transaction job
func (h *Handler) processTransactionJob(job TransactionJob) TransactionResult {
	// Use circuit breaker for overload protection
	var result TransactionResult

	err := h.circuitBreaker.Execute(job.Context, func() error {
		// This is a placeholder - actual transaction processing will be implemented
		// in the handlers.go file
		result = TransactionResult{
			Data: "processed",
			Err:  nil,
		}
		return nil
	})

	if err != nil {
		result = TransactionResult{
			Data: nil,
			Err:  err,
		}
	}

	return result
}

// RegisterRoutes registers all REST routes
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	// API v1 routes
	v1 := e.Group("/api/v1")

	// Account routes
	accountGroup := v1.Group("/account")
	accountGroup.POST("/create", h.CreateAccount)
	accountGroup.GET("/:id", h.GetAccount)
	accountGroup.PUT("/:id", h.UpdateAccount)
	accountGroup.DELETE("/:id", h.DeleteAccount)
	accountGroup.GET("/", h.ListAccounts)
	accountGroup.GET("/:id/balance", h.GetAccountBalance)
	accountGroup.POST("/:id/migrate-to-sub-balance", h.MigrateToSubBalance)

	// Transaction routes
	transactionGroup := v1.Group("/transaction")
	transactionGroup.POST("/execute", h.ProcessTransaction)
	transactionGroup.GET("/account/:accountId", h.GetTransactionHistory)
	transactionGroup.GET("/account/:accountId/summary", h.GetTransactionSummary)
	transactionGroup.GET("/:id", h.GetTransaction)

	// Sub Balance routes
	subBalanceGroup := v1.Group("/sub-balance")
	subBalanceGroup.GET("/:accountId", h.GetSubBalanceInfo)
	subBalanceGroup.POST("/:accountId/initialize", h.InitializeSubBalance)
	subBalanceGroup.POST("/:accountId/rebalance", h.RebalanceShards)
	subBalanceGroup.GET("/:accountId/consistency", h.ValidateConsistency)
	subBalanceGroup.GET("/:accountId/shard/:shardId", h.GetShardInfo)

	// Health and metrics routes
	e.GET("/health", h.HealthCheck)
	e.GET("/metrics", h.Metrics)
}
