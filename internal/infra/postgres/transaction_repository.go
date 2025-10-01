package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sub-balance-implementation/internal/domain/transaction"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// transactionRepository implements the transaction.Repository interface
type transactionRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(db *gorm.DB, logger *zap.Logger) transaction.Repository {
	return &transactionRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new transaction with optimized batch processing
func (r *transactionRepository) Create(ctx context.Context, txn *transaction.Transaction) error {
	if txn.ID == "" {
		txn.ID = uuid.New().String()
	}

	now := time.Now()
	txn.CreatedOn = now
	txn.ModifiedOn = now

	// Convert metadata to JSON string if provided
	if txn.Metadata != "" {
		// Validate JSON
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(txn.Metadata), &metadata); err != nil {
			return fmt.Errorf("invalid metadata JSON: %w", err)
		}
	}

	// OPTIMIZED: Use raw SQL with prepared statement for faster inserts
	query := `INSERT INTO transactions (id, transaction_id, parent_account_id, shard_id, shard_index, transaction_type, amount, previous_balance, new_balance, description, status, created_on, modified_on, checksum, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if err := r.db.WithContext(ctx).Exec(query,
		txn.ID, txn.TransactionID, txn.ParentAccountID, txn.ShardID, txn.ShardIndex,
		txn.TransactionType, txn.Amount, txn.PreviousBalance, txn.NewBalance,
		txn.Description, txn.Status, txn.CreatedOn, txn.ModifiedOn, txn.Checksum, txn.Metadata,
	).Error; err != nil {
		r.logger.Error("Failed to create transaction",
			zap.String("transaction_id", txn.TransactionID),
			zap.String("parent_account_id", txn.ParentAccountID),
			zap.String("shard_id", txn.ShardID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Reduce logging frequency for high TPS
	r.logger.Debug("Transaction created successfully",
		zap.String("transaction_id", txn.TransactionID),
		zap.String("parent_account_id", txn.ParentAccountID),
		zap.String("shard_id", txn.ShardID),
		zap.String("type", txn.TransactionType),
		zap.String("amount", txn.Amount.String()),
	)

	return nil
}

// GetByID retrieves a transaction by ID
func (r *transactionRepository) GetByID(ctx context.Context, id string) (*transaction.Transaction, error) {
	var txn transaction.Transaction

	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&txn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("transaction not found: %s", id)
		}
		r.logger.Error("Failed to get transaction by ID",
			zap.String("transaction_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return &txn, nil
}

// GetByTransactionID retrieves all transactions with the same transaction ID
func (r *transactionRepository) GetByTransactionID(ctx context.Context, transactionID string) ([]*transaction.Transaction, error) {
	var transactions []*transaction.Transaction

	if err := r.db.WithContext(ctx).
		Where("transaction_id = ?", transactionID).
		Order("created_on ASC").
		Find(&transactions).Error; err != nil {
		r.logger.Error("Failed to get transactions by transaction ID",
			zap.String("transaction_id", transactionID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	return transactions, nil
}

// GetByParentAccountID retrieves transactions for a parent account with pagination
func (r *transactionRepository) GetByParentAccountID(ctx context.Context, parentAccountID string, limit, offset int) ([]*transaction.Transaction, error) {
	var transactions []*transaction.Transaction

	query := r.db.WithContext(ctx).
		Where("parent_account_id = ?", parentAccountID).
		Order("created_on DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&transactions).Error; err != nil {
		r.logger.Error("Failed to get transactions by parent account ID",
			zap.String("parent_account_id", parentAccountID),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	return transactions, nil
}

// GetByShardID retrieves transactions for a shard with pagination
func (r *transactionRepository) GetByShardID(ctx context.Context, shardID string, limit, offset int) ([]*transaction.Transaction, error) {
	var transactions []*transaction.Transaction

	query := r.db.WithContext(ctx).
		Where("shard_id = ?", shardID).
		Order("created_on DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&transactions).Error; err != nil {
		r.logger.Error("Failed to get transactions by shard ID",
			zap.String("shard_id", shardID),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	return transactions, nil
}

// Update updates an existing transaction
func (r *transactionRepository) Update(ctx context.Context, txn *transaction.Transaction) error {
	txn.ModifiedOn = time.Now()

	// Convert metadata to JSON string if provided
	if txn.Metadata != "" {
		// Validate JSON
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(txn.Metadata), &metadata); err != nil {
			return fmt.Errorf("invalid metadata JSON: %w", err)
		}
	}

	if err := r.db.WithContext(ctx).Save(txn).Error; err != nil {
		r.logger.Error("Failed to update transaction",
			zap.String("transaction_id", txn.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	r.logger.Info("Transaction updated successfully",
		zap.String("transaction_id", txn.ID),
	)

	return nil
}

// Delete deletes a transaction by ID
func (r *transactionRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&transaction.Transaction{})
	if result.Error != nil {
		r.logger.Error("Failed to delete transaction",
			zap.String("transaction_id", id),
			zap.Error(result.Error),
		)
		return fmt.Errorf("failed to delete transaction: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("transaction not found: %s", id)
	}

	r.logger.Info("Transaction deleted successfully",
		zap.String("transaction_id", id),
	)

	return nil
}

// GetTransactionHistory retrieves transaction history with date range and pagination
func (r *transactionRepository) GetTransactionHistory(ctx context.Context, parentAccountID string, fromDate, toDate time.Time, limit, offset int) ([]*transaction.Transaction, error) {
	var transactions []*transaction.Transaction

	query := r.db.WithContext(ctx).
		Where("parent_account_id = ?", parentAccountID).
		Where("created_on >= ? AND created_on <= ?", fromDate, toDate).
		Order("created_on DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&transactions).Error; err != nil {
		r.logger.Error("Failed to get transaction history",
			zap.String("parent_account_id", parentAccountID),
			zap.Time("from_date", fromDate),
			zap.Time("to_date", toDate),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	return transactions, nil
}

// GetTransactionSummary retrieves transaction summary for a date range
func (r *transactionRepository) GetTransactionSummary(ctx context.Context, parentAccountID string, fromDate, toDate time.Time) (*transaction.TransactionSummary, error) {
	var summary transaction.TransactionSummary

	// Get total transactions count
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("parent_account_id = ? AND created_on >= ? AND created_on <= ?", parentAccountID, fromDate, toDate).
		Count(&summary.TotalTransactions).Error; err != nil {
		return nil, fmt.Errorf("failed to get total transactions count: %w", err)
	}

	// Get total debit amount
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("parent_account_id = ? AND transaction_type = 'debit' AND created_on >= ? AND created_on <= ?", parentAccountID, fromDate, toDate).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&summary.TotalDebitAmount).Error; err != nil {
		return nil, fmt.Errorf("failed to get total debit amount: %w", err)
	}

	// Get total credit amount
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("parent_account_id = ? AND transaction_type = 'credit' AND created_on >= ? AND created_on <= ?", parentAccountID, fromDate, toDate).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&summary.TotalCreditAmount).Error; err != nil {
		return nil, fmt.Errorf("failed to get total credit amount: %w", err)
	}

	// Calculate net amount
	summary.NetAmount = summary.TotalCreditAmount.Sub(summary.TotalDebitAmount)

	// Get average amount
	if summary.TotalTransactions > 0 {
		if err := r.db.WithContext(ctx).
			Model(&transaction.Transaction{}).
			Where("parent_account_id = ? AND created_on >= ? AND created_on <= ?", parentAccountID, fromDate, toDate).
			Select("COALESCE(AVG(amount), 0)").
			Scan(&summary.AverageAmount).Error; err != nil {
			return nil, fmt.Errorf("failed to get average amount: %w", err)
		}
	}

	// Get largest transaction
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("parent_account_id = ? AND created_on >= ? AND created_on <= ?", parentAccountID, fromDate, toDate).
		Select("COALESCE(MAX(amount), 0)").
		Scan(&summary.LargestTransaction).Error; err != nil {
		return nil, fmt.Errorf("failed to get largest transaction: %w", err)
	}

	// Get smallest transaction
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("parent_account_id = ? AND created_on >= ? AND created_on <= ?", parentAccountID, fromDate, toDate).
		Select("COALESCE(MIN(amount), 0)").
		Scan(&summary.SmallestTransaction).Error; err != nil {
		return nil, fmt.Errorf("failed to get smallest transaction: %w", err)
	}

	// Set period information
	summary.ParentAccountID = parentAccountID
	summary.FromDate = fromDate
	summary.ToDate = toDate
	summary.Period = fmt.Sprintf("%s to %s", fromDate.Format("2006-01-02"), toDate.Format("2006-01-02"))

	return &summary, nil
}

// GetTransactionStats returns statistics about transactions
func (r *transactionRepository) GetTransactionStats(ctx context.Context) (map[string]interface{}, error) {
	var stats struct {
		TotalTransactions     int64           `json:"total_transactions"`
		TotalDebitAmount      decimal.Decimal `json:"total_debit_amount"`
		TotalCreditAmount     decimal.Decimal `json:"total_credit_amount"`
		PendingTransactions   int64           `json:"pending_transactions"`
		CompletedTransactions int64           `json:"completed_transactions"`
		FailedTransactions    int64           `json:"failed_transactions"`
		AverageAmount         decimal.Decimal `json:"average_amount"`
		MaxAmount             decimal.Decimal `json:"max_amount"`
		MinAmount             decimal.Decimal `json:"min_amount"`
	}

	// Get total transactions count
	if err := r.db.WithContext(ctx).Model(&transaction.Transaction{}).Count(&stats.TotalTransactions).Error; err != nil {
		return nil, fmt.Errorf("failed to get total transactions count: %w", err)
	}

	// Get total debit amount
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("transaction_type = 'debit'").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&stats.TotalDebitAmount).Error; err != nil {
		return nil, fmt.Errorf("failed to get total debit amount: %w", err)
	}

	// Get total credit amount
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("transaction_type = 'credit'").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&stats.TotalCreditAmount).Error; err != nil {
		return nil, fmt.Errorf("failed to get total credit amount: %w", err)
	}

	// Get pending transactions count
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("status = 'pending'").
		Count(&stats.PendingTransactions).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending transactions count: %w", err)
	}

	// Get completed transactions count
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("status = 'completed'").
		Count(&stats.CompletedTransactions).Error; err != nil {
		return nil, fmt.Errorf("failed to get completed transactions count: %w", err)
	}

	// Get failed transactions count
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Where("status = 'failed'").
		Count(&stats.FailedTransactions).Error; err != nil {
		return nil, fmt.Errorf("failed to get failed transactions count: %w", err)
	}

	// Get average amount
	if stats.TotalTransactions > 0 {
		if err := r.db.WithContext(ctx).
			Model(&transaction.Transaction{}).
			Select("COALESCE(AVG(amount), 0)").
			Scan(&stats.AverageAmount).Error; err != nil {
			return nil, fmt.Errorf("failed to get average amount: %w", err)
		}
	}

	// Get max amount
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Select("COALESCE(MAX(amount), 0)").
		Scan(&stats.MaxAmount).Error; err != nil {
		return nil, fmt.Errorf("failed to get max amount: %w", err)
	}

	// Get min amount
	if err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Select("COALESCE(MIN(amount), 0)").
		Scan(&stats.MinAmount).Error; err != nil {
		return nil, fmt.Errorf("failed to get min amount: %w", err)
	}

	return map[string]interface{}{
		"total_transactions":     stats.TotalTransactions,
		"total_debit_amount":     stats.TotalDebitAmount,
		"total_credit_amount":    stats.TotalCreditAmount,
		"pending_transactions":   stats.PendingTransactions,
		"completed_transactions": stats.CompletedTransactions,
		"failed_transactions":    stats.FailedTransactions,
		"average_amount":         stats.AverageAmount,
		"max_amount":             stats.MaxAmount,
		"min_amount":             stats.MinAmount,
	}, nil
}

// GetTransactionsByStatus retrieves transactions by status
func (r *transactionRepository) GetTransactionsByStatus(ctx context.Context, status transaction.TransactionStatus, limit, offset int) ([]*transaction.Transaction, error) {
	var transactions []*transaction.Transaction

	query := r.db.WithContext(ctx).
		Where("status = ?", string(status)).
		Order("created_on DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&transactions).Error; err != nil {
		r.logger.Error("Failed to get transactions by status",
			zap.String("status", string(status)),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transactions by status: %w", err)
	}

	return transactions, nil
}

// GetTransactionsByType retrieves transactions by type
func (r *transactionRepository) GetTransactionsByType(ctx context.Context, transactionType transaction.TransactionType, limit, offset int) ([]*transaction.Transaction, error) {
	var transactions []*transaction.Transaction

	query := r.db.WithContext(ctx).
		Where("transaction_type = ?", string(transactionType)).
		Order("created_on DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&transactions).Error; err != nil {
		r.logger.Error("Failed to get transactions by type",
			zap.String("transaction_type", string(transactionType)),
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get transactions by type: %w", err)
	}

	return transactions, nil
}
