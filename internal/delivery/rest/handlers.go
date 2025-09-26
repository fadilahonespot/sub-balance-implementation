package rest

import (
	"net/http"
	"strconv"
	"time"

	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/sub_balance_manager"
	"sub-balance-implementation/internal/domain/transaction"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
		"service":   "sub-balance-implementation",
	})
}

// Metrics handles metrics requests
func (h *Handler) Metrics(c echo.Context) error {
	// Get database stats
	sqlDB, err := h.db.DB()
	if err != nil {
		h.logger.Error("Failed to get database connection", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get database connection",
		})
	}
	dbStats := sqlDB.Stats()

	return c.JSON(http.StatusOK, map[string]interface{}{
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
			"max_idle_closed":      dbStats.MaxIdleClosed,
			"max_idle_time_closed": dbStats.MaxIdleTimeClosed,
			"max_lifetime_closed":  dbStats.MaxLifetimeClosed,
		},
	})
}

// CreateAccount handles account creation requests
func (h *Handler) CreateAccount(c echo.Context) error {
	var req account.CreateAccountRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Validate request
	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		h.logger.Error("Validation failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Validation failed: " + err.Error(),
		})
	}

	// Create account
	acc, err := h.usecases.Account.CreateAccount(c.Request().Context(), req)
	if err != nil {
		h.logger.Error("Failed to create account", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create account: " + err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, acc)
}

// GetAccount handles account retrieval requests
func (h *Handler) GetAccount(c echo.Context) error {
	accountID := c.Param("id")

	// Get account
	acc, err := h.usecases.Account.GetAccount(c.Request().Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to get account", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Account not found: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, acc)
}

// UpdateAccount handles account update requests
func (h *Handler) UpdateAccount(c echo.Context) error {
	accountID := c.Param("id")
	h.logger.Info("UpdateAccount endpoint called", zap.String("account_id", accountID))

	var req account.UpdateAccountRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Validate request
	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		h.logger.Error("Validation failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Validation failed: " + err.Error(),
		})
	}

	// Update account
	acc, err := h.usecases.Account.UpdateAccount(c.Request().Context(), accountID, req)
	if err != nil {
		h.logger.Error("Failed to update account", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update account: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, acc)
}

// DeleteAccount handles account deletion requests
func (h *Handler) DeleteAccount(c echo.Context) error {
	accountID := c.Param("id")
	h.logger.Info("DeleteAccount endpoint called", zap.String("account_id", accountID))

	// Delete account
	err := h.usecases.Account.DeleteAccount(c.Request().Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to delete account", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to delete account: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":    "Account deleted successfully",
		"account_id": accountID,
	})
}

// ListAccounts handles account listing requests
func (h *Handler) ListAccounts(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	// Set default values
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	h.logger.Info("ListAccounts endpoint called",
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	// List accounts
	accounts, err := h.usecases.Account.ListAccounts(c.Request().Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list accounts", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list accounts: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"accounts": accounts,
		"limit":    limit,
		"offset":   offset,
		"count":    len(accounts),
	})
}

// GetAccountBalance handles account balance retrieval requests
func (h *Handler) GetAccountBalance(c echo.Context) error {
	accountID := c.Param("id")

	// Get account balance
	balance, err := h.usecases.Account.GetAccountBalance(c.Request().Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to get account balance", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Failed to get account balance: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, balance)
}

// MigrateToSubBalance handles sub balance migration requests
func (h *Handler) MigrateToSubBalance(c echo.Context) error {
	accountID := c.Param("id")
	h.logger.Info("MigrateToSubBalance endpoint called", zap.String("account_id", accountID))

	var req struct {
		ShardCount int `json:"shard_count"`
	}
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Set default shard count if not provided
	shardCount := req.ShardCount
	if shardCount <= 0 {
		shardCount = 3
	}

	// Migrate to sub balance
	err := h.usecases.Account.MigrateToSubBalance(c.Request().Context(), accountID, shardCount)
	if err != nil {
		h.logger.Error("Failed to migrate to sub balance", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to migrate to sub balance: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Account migrated to sub balance successfully",
		"account_id":  accountID,
		"shard_count": shardCount,
	})
}

// ProcessTransactionWithBP handles transaction processing requests
func (h *Handler) ProcessTransactionWithBP(c echo.Context) error {
	var req transaction.ProcessTransactionWithBPRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Validate request
	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		h.logger.Error("Validation failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Validation failed: " + err.Error(),
		})
	}

	// Manual validation for decimal.Decimal amount
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		h.logger.Error("Invalid amount", zap.String("amount", req.Amount.String()))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Amount must be greater than 0",
		})
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
		result, err = h.usecases.SubBalanceManager.ProcessDebitTransaction(c.Request().Context(), debitReq)
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
		result, err = h.usecases.SubBalanceManager.ProcessCreditTransaction(c.Request().Context(), creditReq)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid transaction type",
		})
	}

	if err != nil {
		h.logger.Error("Failed to process transaction", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to process transaction: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// GetTransaction handles transaction retrieval requests
func (h *Handler) GetTransaction(c echo.Context) error {
	transactionID := c.Param("id")
	h.logger.Info("GetTransaction endpoint called", zap.String("transaction_id", transactionID))

	// Get transaction
	txn, err := h.usecases.Transaction.GetTransaction(c.Request().Context(), transactionID)
	if err != nil {
		h.logger.Error("Failed to get transaction", zap.String("transaction_id", transactionID), zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Transaction not found: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, txn)
}

// GetTransactionHistory handles transaction history requests
func (h *Handler) GetTransactionHistory(c echo.Context) error {
	accountID := c.Param("accountId")
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	// Set default values
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	h.logger.Info("GetTransactionHistory endpoint called",
		zap.String("account_id", accountID),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	// Get transaction history
	req := transaction.GetTransactionHistoryRequest{
		Limit:  limit,
		Offset: offset,
	}
	transactions, err := h.usecases.Transaction.GetTransactionHistory(c.Request().Context(), accountID, req)
	if err != nil {
		h.logger.Error("Failed to get transaction history", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get transaction history: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"account_id":   accountID,
		"transactions": transactions,
		"limit":        limit,
		"offset":       offset,
		"count":        len(transactions),
	})
}

// GetTransactionSummary handles transaction summary requests
func (h *Handler) GetTransactionSummary(c echo.Context) error {
	accountID := c.Param("accountId")
	period := c.QueryParam("period") // daily, weekly, monthly, yearly

	// Set default period if not provided
	if period == "" {
		period = "monthly"
	}

	h.logger.Info("GetTransactionSummary endpoint called",
		zap.String("account_id", accountID),
		zap.String("period", period),
	)

	// Get transaction summary
	req := transaction.GetTransactionSummaryRequest{
		// For now, we'll use empty dates to get all-time summary
		// In a real implementation, you might want to parse period and set appropriate dates
	}
	summary, err := h.usecases.Transaction.GetTransactionSummary(c.Request().Context(), accountID, req)
	if err != nil {
		h.logger.Error("Failed to get transaction summary", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get transaction summary: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, summary)
}

// GetSubBalanceInfo handles sub balance info requests
func (h *Handler) GetSubBalanceInfo(c echo.Context) error {
	accountID := c.Param("accountId")

	// Get sub balance info
	info, err := h.usecases.SubBalanceManager.GetSubBalanceInfo(c.Request().Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to get sub balance info", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Failed to get sub balance info: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, info)
}

// InitializeSubBalance handles sub balance initialization requests
func (h *Handler) InitializeSubBalance(c echo.Context) error {
	accountID := c.Param("accountId")

	var req sub_balance_manager.InitializeSubBalanceRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Set default shard count if not provided
	shardCount := req.ShardCount
	if shardCount == 0 {
		shardCount = 3
	}

	// Initialize sub balance
	err := h.usecases.SubBalanceManager.InitializeSubBalance(c.Request().Context(), accountID, shardCount)
	if err != nil {
		h.logger.Error("Failed to initialize sub balance", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to initialize sub balance: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Sub balance initialized successfully",
		"account_id":  accountID,
		"shard_count": shardCount,
	})
}

// RebalanceShards handles shard rebalancing requests
func (h *Handler) RebalanceShards(c echo.Context) error {
	accountID := c.Param("accountId")

	// Rebalance shards
	err := h.usecases.SubBalanceManager.RebalanceShards(c.Request().Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to rebalance shards", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to rebalance shards: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":    "Shards rebalanced successfully",
		"account_id": accountID,
	})
}

// ValidateConsistency handles consistency validation requests
func (h *Handler) ValidateConsistency(c echo.Context) error {
	accountID := c.Param("accountId")

	// Validate consistency
	report, err := h.usecases.SubBalanceManager.ValidateBalanceConsistency(c.Request().Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to validate consistency", zap.String("account_id", accountID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to validate consistency: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, report)
}

// GetShardInfo handles shard info requests
func (h *Handler) GetShardInfo(c echo.Context) error {
	accountID := c.Param("accountId")
	shardID := c.Param("shardId")
	h.logger.Info("GetShardInfo endpoint called",
		zap.String("account_id", accountID),
		zap.String("shard_id", shardID),
	)

	// Get all shard balances and find the specific shard
	shardBalances, err := h.usecases.SubBalanceManager.GetShardBalances(c.Request().Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to get shard balances",
			zap.String("account_id", accountID),
			zap.Error(err),
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get shard balances: " + err.Error(),
		})
	}

	// Find the specific shard
	var targetShard *sub_balance_manager.ShardBalance
	for _, shard := range shardBalances {
		if shard.ShardID == shardID {
			targetShard = shard
			break
		}
	}

	if targetShard == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Shard not found",
		})
	}

	return c.JSON(http.StatusOK, targetShard)
}
