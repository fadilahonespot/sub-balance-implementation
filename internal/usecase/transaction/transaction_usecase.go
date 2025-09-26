package transaction

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sub-balance-implementation/internal/domain/transaction"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// usecase implements the transaction.Usecase interface
type usecase struct {
	repo   transaction.Repository
	logger *zap.Logger
}

// NewUsecase creates a new transaction usecase
func NewUsecase(repo transaction.Repository, logger *zap.Logger) transaction.Usecase {
	return &usecase{
		repo:   repo,
		logger: logger,
	}
}

// CreateTransaction creates a new transaction record
func (u *usecase) CreateTransaction(ctx context.Context, req transaction.CreateTransactionRequest) (*transaction.Transaction, error) {
	u.logger.Info("Creating transaction",
		zap.String("transaction_id", req.TransactionID),
		zap.String("parent_account_id", req.ParentAccountID),
		zap.String("shard_id", req.ShardID),
		zap.String("type", string(req.TransactionType)),
		zap.String("amount", req.Amount.String()),
	)

	// Convert metadata to JSON string
	var metadataJSON string
	if req.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	// Create transaction
	txn := &transaction.Transaction{
		ID:              uuid.New().String(),
		TransactionID:   req.TransactionID,
		ParentAccountID: req.ParentAccountID,
		ShardID:         req.ShardID,
		ShardIndex:      req.ShardIndex,
		TransactionType: string(req.TransactionType),
		Amount:          req.Amount,
		PreviousBalance: req.PreviousBalance,
		NewBalance:      req.NewBalance,
		Description:     req.Description,
		Status:          string(transaction.TransactionStatusPending),
		Metadata:        metadataJSON,
	}

	// Create transaction in database
	if err := u.repo.Create(ctx, txn); err != nil {
		u.logger.Error("Failed to create transaction",
			zap.String("transaction_id", req.TransactionID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	u.logger.Info("Transaction created successfully",
		zap.String("transaction_id", txn.ID),
		zap.String("business_transaction_id", req.TransactionID),
	)

	return txn, nil
}

// GetTransaction retrieves a transaction by ID
func (u *usecase) GetTransaction(ctx context.Context, id string) (*transaction.Transaction, error) {
	u.logger.Info("Getting transaction", zap.String("transaction_id", id))

	txn, err := u.repo.GetByID(ctx, id)
	if err != nil {
		u.logger.Error("Failed to get transaction",
			zap.String("transaction_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return txn, nil
}

// GetTransactionHistory retrieves transaction history for an account
func (u *usecase) GetTransactionHistory(ctx context.Context, accountID string, req transaction.GetTransactionHistoryRequest) ([]*transaction.Transaction, error) {
	u.logger.Info("Getting transaction history",
		zap.String("account_id", accountID),
		zap.Any("request", req),
	)

	// Set default date range if not provided
	fromDate := time.Now().AddDate(0, 0, -30) // Default: last 30 days
	toDate := time.Now()

	if req.FromDate != nil {
		fromDate = *req.FromDate
	}
	if req.ToDate != nil {
		toDate = *req.ToDate
	}

	// Set default pagination
	limit := 50
	offset := 0

	if req.Limit > 0 {
		limit = req.Limit
	}
	if req.Offset > 0 {
		offset = req.Offset
	}

	transactions, err := u.repo.GetTransactionHistory(ctx, accountID, fromDate, toDate, limit, offset)
	if err != nil {
		u.logger.Error("Failed to get transaction history",
			zap.String("account_id", accountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	u.logger.Info("Transaction history retrieved",
		zap.String("account_id", accountID),
		zap.Int("transaction_count", len(transactions)),
	)

	return transactions, nil
}

// GetTransactionSummary retrieves transaction summary for an account
func (u *usecase) GetTransactionSummary(ctx context.Context, accountID string, req transaction.GetTransactionSummaryRequest) (*transaction.TransactionSummary, error) {
	u.logger.Info("Getting transaction summary",
		zap.String("account_id", accountID),
		zap.Any("request", req),
	)

	// Set default date range if not provided
	fromDate := time.Now().AddDate(0, 0, -30) // Default: last 30 days
	toDate := time.Now()

	if req.FromDate != nil {
		fromDate = *req.FromDate
	}
	if req.ToDate != nil {
		toDate = *req.ToDate
	}

	summary, err := u.repo.GetTransactionSummary(ctx, accountID, fromDate, toDate)
	if err != nil {
		u.logger.Error("Failed to get transaction summary",
			zap.String("account_id", accountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transaction summary: %w", err)
	}

	u.logger.Info("Transaction summary retrieved",
		zap.String("account_id", accountID),
		zap.Int64("total_transactions", summary.TotalTransactions),
		zap.String("net_amount", summary.NetAmount.String()),
	)

	return summary, nil
}

// UpdateTransactionStatus updates the status of a transaction
func (u *usecase) UpdateTransactionStatus(ctx context.Context, id string, status transaction.TransactionStatus) error {
	u.logger.Info("Updating transaction status",
		zap.String("transaction_id", id),
		zap.String("status", string(status)),
	)

	// Get existing transaction
	txn, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// Update status
	txn.Status = string(status)

	// Update transaction
	if err := u.repo.Update(ctx, txn); err != nil {
		u.logger.Error("Failed to update transaction status",
			zap.String("transaction_id", id),
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	u.logger.Info("Transaction status updated successfully",
		zap.String("transaction_id", id),
		zap.String("status", string(status)),
	)

	return nil
}

// ProcessTransactionWithBP processes a transaction with business product support
func (u *usecase) ProcessTransactionWithBP(ctx context.Context, req transaction.ProcessTransactionWithBPRequest) (*transaction.ProcessTransactionWithBPResponse, error) {
	u.logger.Info("Processing transaction with BP",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
		zap.String("type", string(req.TransactionType)),
		zap.String("amount", req.Amount.String()),
	)

	// This is a placeholder implementation
	// In a real implementation, this would integrate with the SubBalanceManager
	// and handle the actual transaction processing logic

	response := &transaction.ProcessTransactionWithBPResponse{
		TransactionID:   req.TransactionID,
		AccountID:       req.AccountID,
		Amount:          req.Amount,
		TransactionType: req.TransactionType,
		SelectedShardID: "placeholder-shard-id",
		ShardIndex:      0,
		PreviousBalance: decimal.Zero,
		NewBalance:      req.Amount,
		TotalBalance:    req.Amount,
		ProcessedAt:     time.Now(),
		Status:          transaction.TransactionStatusCompleted,
		UseSubBalance:   true,
		ShardCount:      3,
	}

	u.logger.Info("Transaction with BP processed successfully",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
	)

	return response, nil
}

// GetTransactionsByStatus retrieves transactions by status
func (u *usecase) GetTransactionsByStatus(ctx context.Context, status transaction.TransactionStatus, limit, offset int) ([]*transaction.Transaction, error) {
	u.logger.Info("Getting transactions by status",
		zap.String("status", string(status)),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	transactions, err := u.repo.GetTransactionsByStatus(ctx, status, limit, offset)
	if err != nil {
		u.logger.Error("Failed to get transactions by status",
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transactions by status: %w", err)
	}

	return transactions, nil
}

// GetTransactionsByType retrieves transactions by type
func (u *usecase) GetTransactionsByType(ctx context.Context, transactionType transaction.TransactionType, limit, offset int) ([]*transaction.Transaction, error) {
	u.logger.Info("Getting transactions by type",
		zap.String("type", string(transactionType)),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	transactions, err := u.repo.GetTransactionsByType(ctx, transactionType, limit, offset)
	if err != nil {
		u.logger.Error("Failed to get transactions by type",
			zap.String("type", string(transactionType)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transactions by type: %w", err)
	}

	return transactions, nil
}

// GetTransactionStats retrieves transaction statistics
func (u *usecase) GetTransactionStats(ctx context.Context) (map[string]interface{}, error) {
	u.logger.Info("Getting transaction statistics")

	stats, err := u.repo.GetTransactionStats(ctx)
	if err != nil {
		u.logger.Error("Failed to get transaction statistics", zap.Error(err))
		return nil, fmt.Errorf("failed to get transaction statistics: %w", err)
	}

	return stats, nil
}

// ValidateTransaction validates a transaction request
func (u *usecase) ValidateTransaction(req transaction.CreateTransactionRequest) error {
	// Validate transaction ID
	if req.TransactionID == "" {
		return fmt.Errorf("transaction ID is required")
	}

	// Validate parent account ID
	if req.ParentAccountID == "" {
		return fmt.Errorf("parent account ID is required")
	}

	// Validate shard ID
	if req.ShardID == "" {
		return fmt.Errorf("shard ID is required")
	}

	// Validate amount
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("amount must be greater than zero")
	}

	// Validate transaction type
	if req.TransactionType != transaction.TransactionTypeDebit && req.TransactionType != transaction.TransactionTypeCredit {
		return fmt.Errorf("invalid transaction type: %s", req.TransactionType)
	}

	// Validate balances
	if req.PreviousBalance.LessThan(decimal.Zero) {
		return fmt.Errorf("previous balance cannot be negative")
	}

	if req.NewBalance.LessThan(decimal.Zero) {
		return fmt.Errorf("new balance cannot be negative")
	}

	// Validate balance consistency
	expectedNewBalance := req.PreviousBalance
	if req.TransactionType == transaction.TransactionTypeCredit {
		expectedNewBalance = expectedNewBalance.Add(req.Amount)
	} else {
		expectedNewBalance = expectedNewBalance.Sub(req.Amount)
	}

	if !req.NewBalance.Equal(expectedNewBalance) {
		return fmt.Errorf("new balance %s does not match expected balance %s", req.NewBalance.String(), expectedNewBalance.String())
	}

	return nil
}
