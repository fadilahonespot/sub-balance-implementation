package rest

import (
	"sub-balance-implementation/internal/infra/postgres"
	"sub-balance-implementation/internal/usecase"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Handler contains all REST handlers
type Handler struct {
	db       *gorm.DB
	logger   *zap.Logger
	repos    *postgres.AllRepositories
	usecases *usecase.AllUsecases
}

// NewHandler creates a new REST handler
func NewHandler(db *gorm.DB, logger *zap.Logger) *Handler {
	repoFactory := postgres.NewRepositoryFactory(db, logger)
	repos := repoFactory.GetAllRepositories()

	usecaseFactory := usecase.NewUsecaseFactory(db, repos, logger)
	usecases := usecaseFactory.GetAllUsecases()

	return &Handler{
		db:       db,
		logger:   logger,
		repos:    repos,
		usecases: usecases,
	}
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
	transactionGroup.POST("/execute", h.ProcessTransactionWithBP)
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
