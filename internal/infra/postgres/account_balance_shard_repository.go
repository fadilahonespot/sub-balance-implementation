package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sub-balance-implementation/internal/domain/account_balance_shard"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// accountBalanceShardRepository implements the account_balance_shard.Repository interface
type accountBalanceShardRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewAccountBalanceShardRepository creates a new account balance shard repository
func NewAccountBalanceShardRepository(db *gorm.DB, logger *zap.Logger) account_balance_shard.Repository {
	return &accountBalanceShardRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new account balance shard
func (r *accountBalanceShardRepository) Create(ctx context.Context, shard *account_balance_shard.AccountBalanceShard) error {
	if shard.ID == "" {
		shard.ID = uuid.New().String()
	}

	now := time.Now()
	shard.CreatedOn = now
	shard.ModifiedOn = now

	if err := r.db.WithContext(ctx).Create(shard).Error; err != nil {
		r.logger.Error("Failed to create account balance shard",
			zap.String("shard_id", shard.ID),
			zap.String("parent_account_id", shard.ParentAccountID),
			zap.Int("shard_index", shard.ShardIndex),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create account balance shard: %w", err)
	}

	r.logger.Info("Account balance shard created successfully",
		zap.String("shard_id", shard.ID),
		zap.String("parent_account_id", shard.ParentAccountID),
		zap.Int("shard_index", shard.ShardIndex),
	)

	return nil
}

// GetByID retrieves an account balance shard by ID
func (r *accountBalanceShardRepository) GetByID(ctx context.Context, id string) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("account balance shard not found: %s", id)
		}
		r.logger.Error("Failed to get account balance shard by ID",
			zap.String("shard_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account balance shard: %w", err)
	}

	return &shard, nil
}

// GetByIDForUpdate retrieves an account balance shard by ID with row lock
func (r *accountBalanceShardRepository) GetByIDForUpdate(ctx context.Context, id string) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("account balance shard not found: %s", id)
		}
		r.logger.Error("Failed to get account balance shard by ID for update",
			zap.String("shard_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account balance shard for update: %w", err)
	}

	return &shard, nil
}

// GetByParentAccountID retrieves all shards for a parent account
func (r *accountBalanceShardRepository) GetByParentAccountID(ctx context.Context, parentAccountID string) ([]*account_balance_shard.AccountBalanceShard, error) {
	var shards []*account_balance_shard.AccountBalanceShard

	// Optimized query with specific column selection and index hint
	if err := r.db.WithContext(ctx).
		Select("id, parent_account_id, shard_index, shard_hash, credit_amount, debit_amount, total_balance, reserve_balance, unsettled_amount, created_on, modified_on, checksum").
		Where("parent_account_id = ?", parentAccountID).
		Order("shard_index ASC").
		Find(&shards).Error; err != nil {
		r.logger.Error("Failed to get account balance shards by parent account ID",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account balance shards: %w", err)
	}

	return shards, nil
}

// GetByParentAccountIDForUpdate retrieves all shards for a parent account with row locks
func (r *accountBalanceShardRepository) GetByParentAccountIDForUpdate(ctx context.Context, parentAccountID string) ([]*account_balance_shard.AccountBalanceShard, error) {
	var shards []*account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("parent_account_id = ?", parentAccountID).
		Order("shard_index ASC").
		Find(&shards).Error; err != nil {
		r.logger.Error("Failed to get account balance shards by parent account ID for update",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account balance shards for update: %w", err)
	}

	return shards, nil
}

// GetByShardHash retrieves a shard by parent account ID and shard hash
func (r *accountBalanceShardRepository) GetByShardHash(ctx context.Context, parentAccountID, shardHash string) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Where("parent_account_id = ? AND shard_hash = ?", parentAccountID, shardHash).
		First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("account balance shard not found with hash: %s", shardHash)
		}
		r.logger.Error("Failed to get account balance shard by hash",
			zap.String("parent_account_id", parentAccountID),
			zap.String("shard_hash", shardHash),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account balance shard by hash: %w", err)
	}

	return &shard, nil
}

// GetByShardHashForUpdate retrieves a shard by parent account ID and shard hash with row lock
func (r *accountBalanceShardRepository) GetByShardHashForUpdate(ctx context.Context, parentAccountID, shardHash string) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("parent_account_id = ? AND shard_hash = ?", parentAccountID, shardHash).
		First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("account balance shard not found with hash: %s", shardHash)
		}
		r.logger.Error("Failed to get account balance shard by hash for update",
			zap.String("parent_account_id", parentAccountID),
			zap.String("shard_hash", shardHash),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account balance shard by hash for update: %w", err)
	}

	return &shard, nil
}

// UpdateBalance updates the balance of a shard with optimized SQL
func (r *accountBalanceShardRepository) UpdateBalance(ctx context.Context, shard *account_balance_shard.AccountBalanceShard) error {
	now := time.Now()
	shard.ModifiedOn = now

	// OPTIMIZED: Use raw SQL with specific fields for faster updates
	query := `UPDATE account_balance_shard SET 
		credit_amount = ?, 
		debit_amount = ?, 
		total_balance = ?, 
		reserve_balance = ?, 
		unsettled_amount = ?, 
		modified_on = ?, 
		checksum = ? 
		WHERE id = ?`

	if err := r.db.WithContext(ctx).Exec(query,
		shard.CreditAmount, shard.DebitAmount, shard.TotalBalance,
		shard.ReserveBalance, shard.UnsettledAmount, shard.ModifiedOn,
		shard.Checksum, shard.ID,
	).Error; err != nil {
		r.logger.Error("Failed to update account balance shard",
			zap.String("shard_id", shard.ID),
			zap.String("parent_account_id", shard.ParentAccountID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update account balance shard: %w", err)
	}

	// Reduce logging frequency for high TPS
	r.logger.Debug("Account balance shard updated successfully",
		zap.String("shard_id", shard.ID),
		zap.String("parent_account_id", shard.ParentAccountID),
		zap.String("total_balance", shard.TotalBalance.String()),
	)

	return nil
}

// UpdateBalances updates multiple shards in a single transaction
func (r *accountBalanceShardRepository) UpdateBalances(ctx context.Context, shards []*account_balance_shard.AccountBalanceShard) error {
	if len(shards) == 0 {
		return nil
	}

	now := time.Now()
	for _, shard := range shards {
		shard.ModifiedOn = now
	}

	if err := r.db.WithContext(ctx).Save(shards).Error; err != nil {
		r.logger.Error("Failed to update account balance shards",
			zap.Int("shard_count", len(shards)),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update account balance shards: %w", err)
	}

	r.logger.Info("Account balance shards updated successfully",
		zap.Int("shard_count", len(shards)),
	)

	return nil
}

// Delete deletes an account balance shard by ID
func (r *accountBalanceShardRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&account_balance_shard.AccountBalanceShard{})
	if result.Error != nil {
		r.logger.Error("Failed to delete account balance shard",
			zap.String("shard_id", id),
			zap.Error(result.Error),
		)
		return fmt.Errorf("failed to delete account balance shard: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account balance shard not found: %s", id)
	}

	r.logger.Info("Account balance shard deleted successfully",
		zap.String("shard_id", id),
	)

	return nil
}

// DeleteByParentAccountID deletes all shards for a parent account
func (r *accountBalanceShardRepository) DeleteByParentAccountID(ctx context.Context, parentAccountID string) error {
	result := r.db.WithContext(ctx).Where("parent_account_id = ?", parentAccountID).Delete(&account_balance_shard.AccountBalanceShard{})
	if result.Error != nil {
		r.logger.Error("Failed to delete account balance shards by parent account ID",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(result.Error),
		)
		return fmt.Errorf("failed to delete account balance shards: %w", result.Error)
	}

	r.logger.Info("Account balance shards deleted successfully",
		zap.String("parent_account_id", parentAccountID),
		zap.Int64("deleted_count", result.RowsAffected),
	)

	return nil
}

// GetShardWithLowestBalance retrieves the shard with the lowest balance for a parent account
func (r *accountBalanceShardRepository) GetShardWithLowestBalance(ctx context.Context, parentAccountID string) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Where("parent_account_id = ?", parentAccountID).
		Order("total_balance ASC").
		First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no shards found for account: %s", parentAccountID)
		}
		r.logger.Error("Failed to get shard with lowest balance",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get shard with lowest balance: %w", err)
	}

	return &shard, nil
}

// GetShardWithHighestBalance retrieves the shard with the highest balance for a parent account
func (r *accountBalanceShardRepository) GetShardWithHighestBalance(ctx context.Context, parentAccountID string) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Where("parent_account_id = ?", parentAccountID).
		Order("total_balance DESC").
		First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no shards found for account: %s", parentAccountID)
		}
		r.logger.Error("Failed to get shard with highest balance",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get shard with highest balance: %w", err)
	}

	return &shard, nil
}

// GetShardByIndex retrieves a shard by parent account ID and shard index
func (r *accountBalanceShardRepository) GetShardByIndex(ctx context.Context, parentAccountID string, shardIndex int) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Where("parent_account_id = ? AND shard_index = ?", parentAccountID, shardIndex).
		First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("shard not found with index: %d", shardIndex)
		}
		r.logger.Error("Failed to get shard by index",
			zap.String("parent_account_id", parentAccountID),
			zap.Int("shard_index", shardIndex),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get shard by index: %w", err)
	}

	return &shard, nil
}

// GetShardByIndexForUpdate retrieves a shard by parent account ID and shard index with row lock
func (r *accountBalanceShardRepository) GetShardByIndexForUpdate(ctx context.Context, parentAccountID string, shardIndex int) (*account_balance_shard.AccountBalanceShard, error) {
	var shard account_balance_shard.AccountBalanceShard

	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("parent_account_id = ? AND shard_index = ?", parentAccountID, shardIndex).
		First(&shard).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("shard not found with index: %d", shardIndex)
		}
		r.logger.Error("Failed to get shard by index for update",
			zap.String("parent_account_id", parentAccountID),
			zap.Int("shard_index", shardIndex),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get shard by index for update: %w", err)
	}

	return &shard, nil
}

// CalculateTotalBalance calculates the total balance across all shards for a parent account with optimized query
func (r *accountBalanceShardRepository) CalculateTotalBalance(ctx context.Context, parentAccountID string) (decimal.Decimal, error) {
	var totalBalance decimal.Decimal

	// OPTIMIZED: Use raw SQL with optimized index for faster SUM queries
	query := `SELECT COALESCE(SUM(total_balance), 0) FROM account_balance_shard WHERE parent_account_id = $1`

	if err := r.db.WithContext(ctx).Raw(query, parentAccountID).Scan(&totalBalance).Error; err != nil {
		r.logger.Error("Failed to calculate total balance",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return decimal.Zero, fmt.Errorf("failed to calculate total balance: %w", err)
	}

	// Reduce logging frequency for high TPS
	r.logger.Debug("Total balance calculated",
		zap.String("parent_account_id", parentAccountID),
		zap.String("total_balance", totalBalance.String()),
	)

	return totalBalance, nil
}

// ValidateConsistency validates the consistency of shard balances
func (r *accountBalanceShardRepository) ValidateConsistency(ctx context.Context, parentAccountID string) (*account_balance_shard.ConsistencyReport, error) {
	shards, err := r.GetByParentAccountID(ctx, parentAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards for consistency check: %w", err)
	}

	totalBalance, err := r.CalculateTotalBalance(ctx, parentAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate total balance: %w", err)
	}

	report := &account_balance_shard.ConsistencyReport{
		ParentAccountID: parentAccountID,
		IsConsistent:    true,
		TotalBalance:    totalBalance,
		ShardBalances:   make([]account_balance_shard.ShardBalance, len(shards)),
		Inconsistencies: make([]account_balance_shard.Inconsistency, 0),
		ValidatedAt:     time.Now(),
	}

	// Check each shard for consistency
	for i, shard := range shards {
		report.ShardBalances[i] = account_balance_shard.ShardBalance{
			ShardID:        shard.ID,
			ShardIndex:     shard.ShardIndex,
			ShardHash:      shard.ShardHash,
			TotalBalance:   shard.TotalBalance,
			CreditAmount:   shard.CreditAmount,
			DebitAmount:    shard.DebitAmount,
			ReserveBalance: shard.ReserveBalance,
		}

		// Check if total_balance = credit_amount - debit_amount
		expectedBalance := shard.CreditAmount.Sub(shard.DebitAmount)
		if !shard.TotalBalance.Equal(expectedBalance) {
			report.IsConsistent = false
			report.Inconsistencies = append(report.Inconsistencies, account_balance_shard.Inconsistency{
				ShardID:    shard.ID,
				ShardIndex: shard.ShardIndex,
				Issue:      "Total balance does not match credit - debit",
				Expected:   expectedBalance,
				Actual:     shard.TotalBalance,
				Severity:   "high",
			})
		}

		// Check for negative balances
		if shard.TotalBalance.LessThan(decimal.Zero) {
			report.IsConsistent = false
			report.Inconsistencies = append(report.Inconsistencies, account_balance_shard.Inconsistency{
				ShardID:    shard.ID,
				ShardIndex: shard.ShardIndex,
				Issue:      "Negative balance detected",
				Expected:   decimal.Zero,
				Actual:     shard.TotalBalance,
				Severity:   "high",
			})
		}
	}

	return report, nil
}

// GetShardStats returns statistics about shards
func (r *accountBalanceShardRepository) GetShardStats(ctx context.Context) (map[string]interface{}, error) {
	var stats struct {
		TotalShards       int64           `json:"total_shards"`
		TotalBalance      decimal.Decimal `json:"total_balance"`
		AverageBalance    decimal.Decimal `json:"average_balance"`
		MaxBalance        decimal.Decimal `json:"max_balance"`
		MinBalance        decimal.Decimal `json:"min_balance"`
		NegativeShards    int64           `json:"negative_shards"`
		ZeroBalanceShards int64           `json:"zero_balance_shards"`
	}

	// Get total shards count
	if err := r.db.WithContext(ctx).Model(&account_balance_shard.AccountBalanceShard{}).Count(&stats.TotalShards).Error; err != nil {
		return nil, fmt.Errorf("failed to get total shards count: %w", err)
	}

	// Get total balance
	if err := r.db.WithContext(ctx).
		Model(&account_balance_shard.AccountBalanceShard{}).
		Select("COALESCE(SUM(total_balance), 0)").
		Scan(&stats.TotalBalance).Error; err != nil {
		return nil, fmt.Errorf("failed to get total balance: %w", err)
	}

	// Get average balance
	if err := r.db.WithContext(ctx).
		Model(&account_balance_shard.AccountBalanceShard{}).
		Select("COALESCE(AVG(total_balance), 0)").
		Scan(&stats.AverageBalance).Error; err != nil {
		return nil, fmt.Errorf("failed to get average balance: %w", err)
	}

	// Get max balance
	if err := r.db.WithContext(ctx).
		Model(&account_balance_shard.AccountBalanceShard{}).
		Select("COALESCE(MAX(total_balance), 0)").
		Scan(&stats.MaxBalance).Error; err != nil {
		return nil, fmt.Errorf("failed to get max balance: %w", err)
	}

	// Get min balance
	if err := r.db.WithContext(ctx).
		Model(&account_balance_shard.AccountBalanceShard{}).
		Select("COALESCE(MIN(total_balance), 0)").
		Scan(&stats.MinBalance).Error; err != nil {
		return nil, fmt.Errorf("failed to get min balance: %w", err)
	}

	// Get negative shards count
	if err := r.db.WithContext(ctx).
		Model(&account_balance_shard.AccountBalanceShard{}).
		Where("total_balance < 0").
		Count(&stats.NegativeShards).Error; err != nil {
		return nil, fmt.Errorf("failed to get negative shards count: %w", err)
	}

	// Get zero balance shards count
	if err := r.db.WithContext(ctx).
		Model(&account_balance_shard.AccountBalanceShard{}).
		Where("total_balance = 0").
		Count(&stats.ZeroBalanceShards).Error; err != nil {
		return nil, fmt.Errorf("failed to get zero balance shards count: %w", err)
	}

	return map[string]interface{}{
		"total_shards":        stats.TotalShards,
		"total_balance":       stats.TotalBalance,
		"average_balance":     stats.AverageBalance,
		"max_balance":         stats.MaxBalance,
		"min_balance":         stats.MinBalance,
		"negative_shards":     stats.NegativeShards,
		"zero_balance_shards": stats.ZeroBalanceShards,
	}, nil
}

// UpdateBalanceOptimistic updates shard balance using optimistic locking
func (r *accountBalanceShardRepository) UpdateBalanceOptimistic(ctx context.Context, shard *account_balance_shard.AccountBalanceShard, expectedVersion int) error {
	now := time.Now()
	shard.ModifiedOn = now

	// OPTIMISTIC LOCKING: Update with version check (optimized with prepared statement)
	query := `UPDATE account_balance_shard SET 
		credit_amount = $1, 
		debit_amount = $2, 
		total_balance = $3, 
		reserve_balance = $4, 
		unsettled_amount = $5, 
		modified_on = $6, 
		checksum = $7,
		version = version + 1
		WHERE id = $8 AND version = $9`

	// Use prepared statement for better performance
	sqlDB, err := r.db.WithContext(ctx).DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	stmt, err := sqlDB.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.ExecContext(ctx,
		shard.CreditAmount, shard.DebitAmount, shard.TotalBalance,
		shard.ReserveBalance, shard.UnsettledAmount, shard.ModifiedOn,
		shard.Checksum, shard.ID, expectedVersion,
	)

	if err != nil {
		r.logger.Error("Failed to update account balance shard with optimistic locking",
			zap.String("shard_id", shard.ID),
			zap.String("parent_account_id", shard.ParentAccountID),
			zap.Int("expected_version", expectedVersion),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update account balance shard: %w", err)
	}

	// Check if any rows were affected (optimistic locking check)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("optimistic locking conflict: shard %s version mismatch (expected: %d)",
			shard.ID, expectedVersion)
	}

	// Increment version in memory
	shard.Version = expectedVersion + 1

	r.logger.Debug("Account balance shard updated successfully with optimistic locking",
		zap.String("shard_id", shard.ID),
		zap.String("parent_account_id", shard.ParentAccountID),
		zap.Int("new_version", shard.Version),
		zap.String("total_balance", shard.TotalBalance.String()),
	)

	return nil
}

// UpdateBalanceOptimisticWithRetry updates shard balance with retry mechanism
func (r *accountBalanceShardRepository) UpdateBalanceOptimisticWithRetry(ctx context.Context, shard *account_balance_shard.AccountBalanceShard, expectedVersion int, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Get fresh shard data before each retry
		if attempt > 0 {
			freshShard, err := r.GetByID(ctx, shard.ID)
			if err != nil {
				return fmt.Errorf("failed to get fresh shard data for retry: %w", err)
			}

			// Update shard data with fresh values
			shard.Version = freshShard.Version
			shard.TotalBalance = freshShard.TotalBalance
			shard.CreditAmount = freshShard.CreditAmount
			shard.DebitAmount = freshShard.DebitAmount
			expectedVersion = freshShard.Version

			// Exponential backoff: 5ms, 10ms, 20ms, 40ms, 80ms, etc.
			delayMs := 5 * (1 << (attempt - 1)) // 5, 10, 20, 40, 80, 160, 320...
			if delayMs > 100 {                  // Cap at 100ms
				delayMs = 100
			}
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}

		err := r.UpdateBalanceOptimistic(ctx, shard, expectedVersion)
		if err == nil {
			if attempt > 0 {
				r.logger.Info("Optimistic locking retry successful",
					zap.String("shard_id", shard.ID),
					zap.Int("attempt", attempt+1),
					zap.Int("final_version", shard.Version),
				)
			}
			return nil
		}

		lastErr = err

		// Check if it's an optimistic locking conflict
		if !strings.Contains(err.Error(), "optimistic locking conflict") {
			return err // Non-retryable error
		}

		r.logger.Debug("Optimistic locking conflict, retrying",
			zap.String("shard_id", shard.ID),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", maxRetries),
			zap.Error(err),
		)
	}

	return fmt.Errorf("optimistic locking failed after %d retries: %w", maxRetries, lastErr)
}

// GetShardsWithBalanceInfo gets shards with optimized balance info (lock-free)
func (r *accountBalanceShardRepository) GetShardsWithBalanceInfo(ctx context.Context, parentAccountID string) ([]*account_balance_shard.AccountBalanceShard, error) {
	var shards []*account_balance_shard.AccountBalanceShard

	// OPTIMIZED: Lock-free query with only necessary fields
	query := `SELECT id, parent_account_id, shard_index, shard_hash, 
		credit_amount, debit_amount, total_balance, reserve_balance, 
		unsettled_amount, version, modified_on
		FROM account_balance_shard 
		WHERE parent_account_id = $1 
		ORDER BY shard_index ASC`

	if err := r.db.WithContext(ctx).Raw(query, parentAccountID).Scan(&shards).Error; err != nil {
		r.logger.Error("Failed to get shards with balance info",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get shards with balance info: %w", err)
	}

	r.logger.Debug("Shards with balance info retrieved successfully",
		zap.String("parent_account_id", parentAccountID),
		zap.Int("shard_count", len(shards)),
	)

	return shards, nil
}
