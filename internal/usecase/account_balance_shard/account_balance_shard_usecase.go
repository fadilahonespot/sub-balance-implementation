package account_balance_shard

import (
	"context"
	"fmt"
	"sort"

	"sub-balance-implementation/internal/domain/account_balance_shard"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// usecase implements the account_balance_shard.Usecase interface
type usecase struct {
	repo   account_balance_shard.Repository
	logger *zap.Logger
}

// NewUsecase creates a new account balance shard usecase
func NewUsecase(repo account_balance_shard.Repository, logger *zap.Logger) account_balance_shard.Usecase {
	return &usecase{
		repo:   repo,
		logger: logger,
	}
}

// CreateShards creates sub balance shards for a parent account
func (u *usecase) CreateShards(ctx context.Context, parentAccountID string, shardCount int) ([]*account_balance_shard.AccountBalanceShard, error) {
	u.logger.Info("Creating shards for account",
		zap.String("parent_account_id", parentAccountID),
		zap.Int("shard_count", shardCount),
	)

	// Validate shard count
	if shardCount <= 0 || shardCount > 10 {
		return nil, fmt.Errorf("invalid shard count: %d, must be between 1 and 10", shardCount)
	}

	// Check if shards already exist
	existingShards, err := u.repo.GetByParentAccountID(ctx, parentAccountID)
	if err == nil && len(existingShards) > 0 {
		return nil, fmt.Errorf("shards already exist for account %s", parentAccountID)
	}

	shards := make([]*account_balance_shard.AccountBalanceShard, shardCount)

	for i := 0; i < shardCount; i++ {
		shard := &account_balance_shard.AccountBalanceShard{
			ID:              uuid.New().String(),
			ParentAccountID: parentAccountID,
			ShardIndex:      i,
			ShardHash:       fmt.Sprintf("shard_%d", i),
			CreditAmount:    decimal.Zero,
			DebitAmount:     decimal.Zero,
			TotalBalance:    decimal.Zero,
			ReserveBalance:  decimal.Zero,
			UnsettledAmount: decimal.Zero,
		}

		if err := u.repo.Create(ctx, shard); err != nil {
			u.logger.Error("Failed to create shard",
				zap.String("parent_account_id", parentAccountID),
				zap.Int("shard_index", i),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to create shard %d: %w", i, err)
		}

		shards[i] = shard
	}

	u.logger.Info("Shards created successfully",
		zap.String("parent_account_id", parentAccountID),
		zap.Int("shard_count", len(shards)),
	)

	return shards, nil
}

// GetShards retrieves all shards for a parent account
func (u *usecase) GetShards(ctx context.Context, parentAccountID string) ([]*account_balance_shard.AccountBalanceShard, error) {
	u.logger.Info("Getting shards for account", zap.String("parent_account_id", parentAccountID))

	shards, err := u.repo.GetByParentAccountID(ctx, parentAccountID)
	if err != nil {
		u.logger.Error("Failed to get shards",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	return shards, nil
}

// UpdateShardBalance updates the balance of a specific shard
func (u *usecase) UpdateShardBalance(ctx context.Context, shardID string, creditAmount, debitAmount decimal.Decimal) error {
	u.logger.Info("Updating shard balance",
		zap.String("shard_id", shardID),
		zap.String("credit_amount", creditAmount.String()),
		zap.String("debit_amount", debitAmount.String()),
	)

	// Get shard for update
	shard, err := u.repo.GetByIDForUpdate(ctx, shardID)
	if err != nil {
		return fmt.Errorf("failed to get shard: %w", err)
	}

	// Update amounts
	shard.CreditAmount = shard.CreditAmount.Add(creditAmount)
	shard.DebitAmount = shard.DebitAmount.Add(debitAmount)
	shard.TotalBalance = shard.CreditAmount.Sub(shard.DebitAmount)

	// Validate balance
	if shard.TotalBalance.LessThan(decimal.Zero) {
		return fmt.Errorf("insufficient balance in shard %s: %s", shardID, shard.TotalBalance.String())
	}

	// Update shard
	if err := u.repo.UpdateBalance(ctx, shard); err != nil {
		u.logger.Error("Failed to update shard balance",
			zap.String("shard_id", shardID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update shard balance: %w", err)
	}

	u.logger.Info("Shard balance updated successfully",
		zap.String("shard_id", shardID),
		zap.String("new_total_balance", shard.TotalBalance.String()),
	)

	return nil
}

// SelectShardForDebit selects the best shard for a debit operation
func (u *usecase) SelectShardForDebit(ctx context.Context, parentAccountID string, amount decimal.Decimal) (*account_balance_shard.AccountBalanceShard, error) {
	u.logger.Info("Selecting shard for debit",
		zap.String("parent_account_id", parentAccountID),
		zap.String("amount", amount.String()),
	)

	// Get all shards
	shards, err := u.repo.GetByParentAccountID(ctx, parentAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", parentAccountID)
	}

	// Sort shards by balance (highest first) for debit operations
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].TotalBalance.GreaterThan(shards[j].TotalBalance)
	})

	// Find shard with sufficient balance
	for _, shard := range shards {
		if shard.TotalBalance.GreaterThanOrEqual(amount) {
			u.logger.Info("Selected shard for debit",
				zap.String("shard_id", shard.ID),
				zap.Int("shard_index", shard.ShardIndex),
				zap.String("available_balance", shard.TotalBalance.String()),
				zap.String("requested_amount", amount.String()),
			)
			return shard, nil
		}
	}

	return nil, fmt.Errorf("insufficient balance across all shards for account %s", parentAccountID)
}

// SelectShardForCredit selects the best shard for a credit operation
func (u *usecase) SelectShardForCredit(ctx context.Context, parentAccountID string, amount decimal.Decimal) (*account_balance_shard.AccountBalanceShard, error) {
	u.logger.Info("Selecting shard for credit",
		zap.String("parent_account_id", parentAccountID),
		zap.String("amount", amount.String()),
	)

	// Get all shards
	shards, err := u.repo.GetByParentAccountID(ctx, parentAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", parentAccountID)
	}

	// Sort shards by balance (lowest first) for credit operations (load balancing)
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].TotalBalance.LessThan(shards[j].TotalBalance)
	})

	// Select shard with lowest balance for load balancing
	selectedShard := shards[0]

	u.logger.Info("Selected shard for credit",
		zap.String("shard_id", selectedShard.ID),
		zap.Int("shard_index", selectedShard.ShardIndex),
		zap.String("current_balance", selectedShard.TotalBalance.String()),
		zap.String("credit_amount", amount.String()),
	)

	return selectedShard, nil
}

// GetTotalBalance calculates the total balance across all shards
func (u *usecase) GetTotalBalance(ctx context.Context, parentAccountID string) (decimal.Decimal, error) {
	u.logger.Info("Getting total balance", zap.String("parent_account_id", parentAccountID))

	totalBalance, err := u.repo.CalculateTotalBalance(ctx, parentAccountID)
	if err != nil {
		u.logger.Error("Failed to calculate total balance",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return decimal.Zero, fmt.Errorf("failed to calculate total balance: %w", err)
	}

	return totalBalance, nil
}

// ValidateShardConsistency validates the consistency of shard balances
func (u *usecase) ValidateShardConsistency(ctx context.Context, parentAccountID string) (*account_balance_shard.ConsistencyReport, error) {
	u.logger.Info("Validating shard consistency", zap.String("parent_account_id", parentAccountID))

	report, err := u.repo.ValidateConsistency(ctx, parentAccountID)
	if err != nil {
		u.logger.Error("Failed to validate shard consistency",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to validate shard consistency: %w", err)
	}

	u.logger.Info("Shard consistency validation completed",
		zap.String("parent_account_id", parentAccountID),
		zap.Bool("is_consistent", report.IsConsistent),
		zap.Int("inconsistencies_count", len(report.Inconsistencies)),
	)

	return report, nil
}

// RebalanceShards rebalances shards to optimize distribution
func (u *usecase) RebalanceShards(ctx context.Context, parentAccountID string) error {
	u.logger.Info("Rebalancing shards", zap.String("parent_account_id", parentAccountID))

	// Get all shards
	shards, err := u.repo.GetByParentAccountIDForUpdate(ctx, parentAccountID)
	if err != nil {
		return fmt.Errorf("failed to get shards for rebalancing: %w", err)
	}

	if len(shards) < 2 {
		u.logger.Info("No rebalancing needed - insufficient shards",
			zap.String("parent_account_id", parentAccountID),
			zap.Int("shard_count", len(shards)),
		)
		return nil
	}

	// Calculate total balance
	totalBalance := decimal.Zero
	for _, shard := range shards {
		totalBalance = totalBalance.Add(shard.TotalBalance)
	}

	// Calculate target balance per shard
	targetBalance := totalBalance.Div(decimal.NewFromInt(int64(len(shards))))

	// Calculate rebalancing amounts
	rebalancingNeeded := false
	updates := make([]*account_balance_shard.AccountBalanceShard, 0)

	for _, shard := range shards {
		balanceDiff := shard.TotalBalance.Sub(targetBalance)

		// Only rebalance if difference is significant (> 1% of target)
		threshold := targetBalance.Div(decimal.NewFromInt(100))
		if balanceDiff.Abs().GreaterThan(threshold) {
			rebalancingNeeded = true

			// Calculate new amounts to achieve target balance
			newTotalBalance := targetBalance
			newCreditAmount := shard.CreditAmount
			newDebitAmount := shard.DebitAmount

			if balanceDiff.GreaterThan(decimal.Zero) {
				// Shard has excess balance, reduce credit or increase debit
				newDebitAmount = newDebitAmount.Add(balanceDiff)
			} else {
				// Shard has insufficient balance, increase credit or reduce debit
				newCreditAmount = newCreditAmount.Add(balanceDiff.Abs())
			}

			// Update shard
			shard.CreditAmount = newCreditAmount
			shard.DebitAmount = newDebitAmount
			shard.TotalBalance = newTotalBalance

			updates = append(updates, shard)
		}
	}

	if !rebalancingNeeded {
		u.logger.Info("No rebalancing needed - shards are well balanced",
			zap.String("parent_account_id", parentAccountID),
		)
		return nil
	}

	// Update shards
	if err := u.repo.UpdateBalances(ctx, updates); err != nil {
		u.logger.Error("Failed to update shards during rebalancing",
			zap.String("parent_account_id", parentAccountID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update shards during rebalancing: %w", err)
	}

	u.logger.Info("Shards rebalanced successfully",
		zap.String("parent_account_id", parentAccountID),
		zap.Int("updated_shards", len(updates)),
		zap.String("target_balance", targetBalance.String()),
	)

	return nil
}
