package rest

import (
	"encoding/json"
	"runtime"
	"strings"
	"time"

	"sub-balance-implementation/internal/config"
	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/sub_balance_manager"
	"sub-balance-implementation/internal/domain/transaction"
	"sub-balance-implementation/internal/infra/postgres"
	"sub-balance-implementation/internal/usecase"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// FastHTTPHandler contains all FastHTTP handlers
type FastHTTPHandler struct {
	db       *gorm.DB
	logger   *zap.Logger
	repos    *postgres.AllRepositories
	usecases *usecase.AllUsecases
	config   *config.Config
	router   *fasthttp.RequestHandler
}

// NewFastHTTPHandler creates a new FastHTTP handler
func NewFastHTTPHandler(db *gorm.DB, logger *zap.Logger, cfg *config.Config) *FastHTTPHandler {
	repoFactory := postgres.NewRepositoryFactory(db, logger)
	repos := repoFactory.GetAllRepositories()

	usecaseFactory := usecase.NewUsecaseFactory(db, repos, logger)
	usecases := usecaseFactory.GetAllUsecases()

	handler := &FastHTTPHandler{
		db:       db,
		logger:   logger,
		repos:    repos,
		usecases: usecases,
		config:   cfg,
	}

	return handler
}

// GetHandler returns the FastHTTP request handler
func (h *FastHTTPHandler) GetHandler() fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		method := string(ctx.Method())

		// Set common headers for performance
		ctx.SetContentType("application/json")
		ctx.Response.Header.Set("Connection", "keep-alive")
		ctx.Response.Header.Set("Keep-Alive", "timeout=120, max=1000")
		ctx.Response.Header.Set("Cache-Control", "no-cache")
		ctx.Response.Header.Set("X-Content-Type-Options", "nosniff")

		// Route handling
		switch {
		case method == "GET" && path == "/health":
			h.HealthCheck(ctx)
		case method == "GET" && path == "/metrics":
			h.Metrics(ctx)
		case method == "POST" && strings.HasPrefix(path, "/api/v1/account/create"):
			h.CreateAccount(ctx)
		case method == "GET" && strings.HasPrefix(path, "/api/v1/account/"):
			if strings.Contains(path, "/balance") {
				h.GetAccountBalance(ctx)
			} else {
				h.GetAccount(ctx)
			}
		case method == "POST" && strings.HasPrefix(path, "/api/v1/account/") && strings.Contains(path, "/migrate-to-sub-balance"):
			h.MigrateToSubBalance(ctx)
		case method == "POST" && strings.HasPrefix(path, "/api/v1/transaction/execute"):
			h.ProcessTransaction(ctx)
		case method == "GET" && strings.HasPrefix(path, "/api/v1/sub-balance/"):
			h.GetSubBalanceInfo(ctx)
		case method == "OPTIONS":
			// Handle CORS preflight
			ctx.SetStatusCode(fasthttp.StatusOK)
		default:
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			h.writeJSONResponse(ctx, map[string]string{"error": "Not found"})
		}
	}
}

// HealthCheck handles health check requests
func (h *FastHTTPHandler) HealthCheck(ctx *fasthttp.RequestCtx) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
		"service":   "sub-balance-implementation",
	}
	h.writeJSONResponse(ctx, response)
}

// Metrics handles metrics requests
func (h *FastHTTPHandler) Metrics(ctx *fasthttp.RequestCtx) {
	// Get database stats
	sqlDB, err := h.db.DB()
	if err != nil {
		h.logger.Error("Failed to get database connection", zap.Error(err))
		h.writeJSONResponse(ctx, map[string]string{"error": "Failed to get database connection"})
		return
	}
	dbStats := sqlDB.Stats()

	response := map[string]interface{}{
		"service": map[string]interface{}{
			"status":    "healthy",
			"uptime":    time.Since(time.Now()).String(),
			"timestamp": time.Now().Format(time.RFC3339),
		},
		"database": map[string]interface{}{
			"max_open_connections": dbStats.MaxOpenConnections,
			"open_connections":     dbStats.OpenConnections,
			"in_use":               dbStats.InUse,
			"idle":                 dbStats.Idle,
			"wait_count":           dbStats.WaitCount,
			"wait_duration":        dbStats.WaitDuration.String(),
		},
		"runtime": map[string]interface{}{
			"cpu_cores":  runtime.NumCPU(),
			"goroutines": runtime.NumGoroutine(),
			"gomaxprocs": runtime.GOMAXPROCS(0),
			"memory_mb":  runtime.MemStats{}.HeapAlloc / 1024 / 1024,
		},
	}
	h.writeJSONResponse(ctx, response)
}

// CreateAccount handles account creation
func (h *FastHTTPHandler) CreateAccount(ctx *fasthttp.RequestCtx) {
	var req account.CreateAccountRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid request format"})
		return
	}

	// Validate required fields
	if req.WalletNo == "" || req.WalletTypeId == "" || req.InstanceType == "" ||
		req.CurrencyId == "" || req.OwnerId == "" {
		h.writeJSONResponse(ctx, map[string]string{"error": "Missing required fields"})
		return
	}

	account, err := h.usecases.Account.CreateAccount(ctx, req)
	if err != nil {
		h.logger.Error("Failed to create account", zap.Error(err))
		h.writeJSONResponse(ctx, map[string]string{"error": "Failed to create account"})
		return
	}

	ctx.SetStatusCode(fasthttp.StatusCreated)
	h.writeJSONResponse(ctx, account)
}

// GetAccount handles account retrieval
func (h *FastHTTPHandler) GetAccount(ctx *fasthttp.RequestCtx) {
	accountID := h.extractAccountID(ctx)
	if accountID == "" {
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid account ID"})
		return
	}

	account, err := h.usecases.Account.GetAccount(ctx, accountID)
	if err != nil {
		h.logger.Error("Failed to get account", zap.Error(err))
		h.writeJSONResponse(ctx, map[string]string{"error": "Account not found"})
		return
	}

	h.writeJSONResponse(ctx, account)
}

// GetAccountBalance handles account balance retrieval
func (h *FastHTTPHandler) GetAccountBalance(ctx *fasthttp.RequestCtx) {
	accountID := h.extractAccountID(ctx)
	if accountID == "" {
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid account ID"})
		return
	}

	balance, err := h.usecases.Account.GetAccountBalance(ctx, accountID)
	if err != nil {
		h.logger.Error("Failed to get account balance", zap.Error(err))
		h.writeJSONResponse(ctx, map[string]string{"error": "Failed to get account balance"})
		return
	}

	h.writeJSONResponse(ctx, balance)
}

// MigrateToSubBalance handles account migration to sub-balance
func (h *FastHTTPHandler) MigrateToSubBalance(ctx *fasthttp.RequestCtx) {
	accountID := h.extractAccountID(ctx)
	if accountID == "" {
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid account ID"})
		return
	}

	var req struct {
		ShardCount int `json:"shard_count"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid request format"})
		return
	}

	err := h.usecases.Account.MigrateToSubBalance(ctx, accountID, req.ShardCount)
	if err != nil {
		h.logger.Error("Failed to migrate to sub balance", zap.Error(err))
		h.writeJSONResponse(ctx, map[string]string{"error": "Failed to migrate to sub balance"})
		return
	}

	response := map[string]interface{}{
		"account_id":  accountID,
		"message":     "Account migrated to sub balance successfully",
		"shard_count": req.ShardCount,
	}
	h.writeJSONResponse(ctx, response)
}

// ProcessTransaction handles transaction processing
func (h *FastHTTPHandler) ProcessTransaction(ctx *fasthttp.RequestCtx) {
	var req transaction.ProcessTransactionWithBPRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid request format"})
		return
	}

	// Process transaction with sub balance
	var result interface{}
	var err error

	switch req.TransactionType {
	case transaction.TransactionTypeDebit:
		// Process debit transaction
		debitReq := sub_balance_manager.ProcessTransactionRequest{
			TransactionID:   req.TransactionID,
			AccountID:       req.AccountID,
			Amount:          req.Amount,
			TransactionType: sub_balance_manager.TransactionTypeDebit,
			Description:     req.Description,
			Metadata:        req.Metadata,
		}

		// Use locking mode from config
		if h.config.SubBalance.LockingMode == "optimistic" {
			result, err = h.usecases.SubBalanceManager.ProcessDebitTransactionOptimistic(ctx, debitReq)
		} else {
			result, err = h.usecases.SubBalanceManager.ProcessDebitTransaction(ctx, debitReq)
		}
	case transaction.TransactionTypeCredit:
		// Process credit transaction
		creditReq := sub_balance_manager.ProcessTransactionRequest{
			TransactionID:   req.TransactionID,
			AccountID:       req.AccountID,
			Amount:          req.Amount,
			TransactionType: sub_balance_manager.TransactionTypeCredit,
			Description:     req.Description,
			Metadata:        req.Metadata,
		}
		result, err = h.usecases.SubBalanceManager.ProcessCreditTransaction(ctx, creditReq)
	default:
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid transaction type"})
		return
	}

	if err != nil {
		h.logger.Error("Failed to process transaction", zap.Error(err))
		h.writeJSONResponse(ctx, map[string]string{"error": "Failed to process transaction"})
		return
	}

	h.writeJSONResponse(ctx, result)
}

// GetSubBalanceInfo handles sub-balance info retrieval
func (h *FastHTTPHandler) GetSubBalanceInfo(ctx *fasthttp.RequestCtx) {
	accountID := h.extractAccountID(ctx)
	if accountID == "" {
		h.writeJSONResponse(ctx, map[string]string{"error": "Invalid account ID"})
		return
	}

	info, err := h.usecases.SubBalanceManager.GetSubBalanceInfo(ctx, accountID)
	if err != nil {
		h.logger.Error("Failed to get sub balance info", zap.Error(err))
		h.writeJSONResponse(ctx, map[string]string{"error": "Failed to get sub balance info"})
		return
	}

	h.writeJSONResponse(ctx, info)
}

// Helper methods

// writeJSONResponse writes JSON response with optimized performance
func (h *FastHTTPHandler) writeJSONResponse(ctx *fasthttp.RequestCtx, data interface{}) {
	ctx.SetContentType("application/json")

	// Use fasthttp's optimized JSON encoding
	if err := json.NewEncoder(ctx).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.WriteString(`{"error":"Internal server error"}`)
	}
}

// extractAccountID extracts account ID from URL path
func (h *FastHTTPHandler) extractAccountID(ctx *fasthttp.RequestCtx) string {
	path := string(ctx.Path())
	parts := strings.Split(path, "/")

	// Look for account ID in the path
	for i, part := range parts {
		if part == "account" && i+1 < len(parts) {
			return parts[i+1]
		}
		if strings.HasPrefix(part, "account") && strings.Contains(part, "-") {
			// Extract UUID from account path
			accountPart := strings.TrimPrefix(part, "account")
			accountPart = strings.TrimPrefix(accountPart, "/")
			if len(accountPart) > 0 && accountPart[0] != '/' {
				return accountPart
			}
		}
	}

	return ""
}

// extractTransactionID extracts transaction ID from URL path
func (h *FastHTTPHandler) extractTransactionID(ctx *fasthttp.RequestCtx) string {
	path := string(ctx.Path())
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if part == "transaction" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}
