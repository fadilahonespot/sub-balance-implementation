package sub_balance

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"sort"
	"strings"
	"sync"
	"time"

	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/account_balance_shard"
	"sub-balance-implementation/internal/domain/sub_balance_manager"
	"sub-balance-implementation/internal/domain/transaction"
	"sub-balance-implementation/internal/infra/postgres"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// usecase implements the sub_balance_manager.Usecase interface
type usecase struct {
	db                      *gorm.DB
	accountRepo             account.Repository
	accountBalanceShardRepo account_balance_shard.Repository
	transactionRepo         transaction.Repository
	advisoryLockManager     *postgres.AdvisoryLockManager
	logger                  *zap.Logger
}

// convertMetadataToString converts map[string]interface{} to JSON string
func (u *usecase) convertMetadataToString(metadata map[string]interface{}) string {
	if len(metadata) == 0 {
		return ""
	}

	jsonBytes, err := json.Marshal(metadata)
	if err != nil {
		u.logger.Warn("Failed to marshal metadata to JSON", zap.Error(err))
		return ""
	}

	return string(jsonBytes)
}

// generateHash generates a CRC32 hash from input string
func (u *usecase) generateHash(input string) uint32 {
	return crc32.ChecksumIEEE([]byte(input))
}

// NewUsecase creates a new sub balance manager usecase
func NewUsecase(
	db *gorm.DB,
	accountRepo account.Repository,
	accountBalanceShardRepo account_balance_shard.Repository,
	transactionRepo transaction.Repository,
	advisoryLockManager *postgres.AdvisoryLockManager,
	logger *zap.Logger,
) sub_balance_manager.Usecase {
	return &usecase{
		db:                      db,
		accountRepo:             accountRepo,
		accountBalanceShardRepo: accountBalanceShardRepo,
		transactionRepo:         transactionRepo,
		advisoryLockManager:     advisoryLockManager,
		logger:                  logger,
	}
}

// InitializeSubBalance initializes sub balance for an account
func (u *usecase) InitializeSubBalance(ctx context.Context, accountID string, shardCount int) error {
	u.logger.Info("Initializing sub balance",
		zap.String("account_id", accountID),
		zap.Int("shard_count", shardCount),
	)

	// Get account
	acc, err := u.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Check if already using sub balance
	if acc.UseSubBalance {
		return fmt.Errorf("account %s already uses sub balance", accountID)
	}

	// Validate shard count
	if shardCount <= 0 || shardCount > 10 {
		return fmt.Errorf("invalid shard count: %d, must be between 1 and 10", shardCount)
	}

	// Acquire account-level lock for initialization (required for creating multiple shards)
	lockInfo, err := u.advisoryLockManager.AcquireAdvisoryLock(ctx, accountID, postgres.LockTypeAccount, 0) // Use account-level locking for initialization
	if err != nil {
		return fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	if !lockInfo.IsAcquired {
		return fmt.Errorf("failed to acquire advisory lock for account %s", accountID)
	}

	// Start database transaction
	return u.executeInTransaction(ctx, func(txCtx context.Context) error {
		// Update account configuration
		if err := u.accountRepo.UpdateSubBalanceConfig(txCtx, accountID, true, shardCount); err != nil {
			return fmt.Errorf("failed to update account sub balance config: %w", err)
		}

		// Create shards
		shards := make([]*account_balance_shard.AccountBalanceShard, shardCount)
		for i := 0; i < shardCount; i++ {
			shard := &account_balance_shard.AccountBalanceShard{
				ID:              uuid.New().String(),
				ParentAccountID: accountID,
				ShardIndex:      i,
				ShardHash:       fmt.Sprintf("shard_%d", i),
				CreditAmount:    decimal.Zero,
				DebitAmount:     decimal.Zero,
				TotalBalance:    decimal.Zero,
				ReserveBalance:  decimal.Zero,
				UnsettledAmount: decimal.Zero,
			}

			if err := u.accountBalanceShardRepo.Create(txCtx, shard); err != nil {
				return fmt.Errorf("failed to create shard %d: %w", i, err)
			}

			shards[i] = shard
		}

		u.logger.Info("Sub balance initialized successfully",
			zap.String("account_id", accountID),
			zap.Int("shard_count", len(shards)),
		)

		return nil
	})
}

// GetSubBalanceInfo retrieves sub balance information for an account
func (u *usecase) GetSubBalanceInfo(ctx context.Context, accountID string) (*sub_balance_manager.SubBalanceInfo, error) {
	u.logger.Info("Getting sub balance info", zap.String("account_id", accountID))

	// Get account
	acc, err := u.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	info := &sub_balance_manager.SubBalanceInfo{
		AccountID:     accountID,
		ShardCount:    acc.SubBalanceShardCount,
		TotalBalance:  decimal.Zero,
		UseSubBalance: acc.UseSubBalance,
		Shards:        make([]*sub_balance_manager.ShardInfo, 0),
		CreatedAt:     acc.CreatedOn,
	}

	if !acc.UseSubBalance {
		return info, nil
	}

	// Get shards
	shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	// Calculate total balance and create shard info
	for _, shard := range shards {
		info.TotalBalance = info.TotalBalance.Add(shard.TotalBalance)

		shardInfo := &sub_balance_manager.ShardInfo{
			ShardID:        shard.ID,
			ShardIndex:     shard.ShardIndex,
			ShardHash:      shard.ShardHash,
			TotalBalance:   shard.TotalBalance,
			CreditAmount:   shard.CreditAmount,
			DebitAmount:    shard.DebitAmount,
			ReserveBalance: shard.ReserveBalance,
		}

		// Calculate utilization percentage
		if info.TotalBalance.GreaterThan(decimal.Zero) {
			utilization := shard.TotalBalance.Div(info.TotalBalance).Mul(decimal.NewFromInt(100))
			shardInfo.Utilization, _ = utilization.Float64()
		}

		info.Shards = append(info.Shards, shardInfo)
	}

	return info, nil
}

// SelectShardForDebit selects the best shard for a debit operation using PRE-BALANCE TRANSFER strategy
func (u *usecase) SelectShardForDebit(ctx context.Context, accountID string, amount decimal.Decimal) (*sub_balance_manager.ShardSelectionResult, error) {
	u.logger.Info("Selecting shard for debit with PRE-BALANCE TRANSFER strategy",
		zap.String("account_id", accountID),
		zap.String("amount", amount.String()),
	)

	// Step 1: Find shard with sufficient balance OR prepare for pre-transfer
	result, err := u.selectShardWithPreBalanceTransfer(ctx, accountID, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to select shard with pre-balance transfer: %w", err)
	}

	u.logger.Info("Shard selection with pre-balance transfer completed",
		zap.String("account_id", accountID),
		zap.String("selected_shard_id", result.SelectedShardID),
		zap.String("strategy", string(result.Strategy)),
		zap.String("reason", result.Reason),
	)

	return result, nil
}

// selectShardWithPreBalanceTransfer implements PRE-BALANCE TRANSFER strategy
func (u *usecase) selectShardWithPreBalanceTransfer(ctx context.Context, accountID string, amount decimal.Decimal) (*sub_balance_manager.ShardSelectionResult, error) {
	// Get all shards for the account (read-only, no locks)
	shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", accountID)
	}

	// Sort shards by balance (highest first) to find best candidates
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].TotalBalance.GreaterThan(shards[j].TotalBalance)
	})

	// Strategy 1: Find shard with sufficient balance (no transfer needed)
	for _, shard := range shards {
		if shard.TotalBalance.GreaterThanOrEqual(amount) {
			u.logger.Info("Found shard with sufficient balance, no transfer needed",
				zap.String("shard_id", shard.ID),
				zap.String("balance", shard.TotalBalance.String()),
				zap.String("amount", amount.String()),
			)

			return &sub_balance_manager.ShardSelectionResult{
				SelectedShardID: shard.ID,
				ShardIndex:      shard.ShardIndex,
				ShardHash:       shard.ShardHash,
				CurrentBalance:  shard.TotalBalance,
				Strategy:        sub_balance_manager.StrategyHighestBalance,
				Reason:          "Selected shard with sufficient balance, no pre-transfer needed",
			}, nil
		}
	}

	// Strategy 2: Pre-transfer balance to make one shard sufficient
	u.logger.Info("No shard has sufficient balance, implementing pre-balance transfer",
		zap.String("account_id", accountID),
		zap.String("amount", amount.String()),
	)

	// Select target shard (highest balance shard)
	targetShard := shards[0]
	remainingNeeded := amount.Sub(targetShard.TotalBalance)

	u.logger.Info("Pre-transfer planning",
		zap.String("target_shard_id", targetShard.ID),
		zap.String("target_current_balance", targetShard.TotalBalance.String()),
		zap.String("remaining_needed", remainingNeeded.String()),
	)

	// Calculate total available balance across all shards
	totalAvailable := decimal.Zero
	for _, shard := range shards {
		totalAvailable = totalAvailable.Add(shard.TotalBalance)
	}

	if totalAvailable.LessThan(amount) {
		return nil, fmt.Errorf("insufficient total balance: available=%s, required=%s",
			totalAvailable.String(), amount.String())
	}

	// Execute pre-balance transfer (lock-free, optimistic approach)
	err = u.executePreBalanceTransfer(ctx, shards, targetShard.ID, remainingNeeded)
	if err != nil {
		return nil, fmt.Errorf("pre-balance transfer failed: %w", err)
	}

	// Return target shard with updated balance
	newTargetBalance := amount // After pre-transfer, target shard will have exactly the needed amount

	return &sub_balance_manager.ShardSelectionResult{
		SelectedShardID: targetShard.ID,
		ShardIndex:      targetShard.ShardIndex,
		ShardHash:       targetShard.ShardHash,
		CurrentBalance:  newTargetBalance,
		Strategy:        sub_balance_manager.StrategyPreBalanceTransfer,
		Reason:          fmt.Sprintf("Pre-transferred %s to target shard for sufficient balance", remainingNeeded.String()),
	}, nil
}

// executePreBalanceTransfer performs lock-free pre-balance transfer between shards
func (u *usecase) executePreBalanceTransfer(ctx context.Context, shards []*account_balance_shard.AccountBalanceShard, targetShardID string, amount decimal.Decimal) error {
	u.logger.Info("Executing pre-balance transfer",
		zap.String("target_shard_id", targetShardID),
		zap.String("amount", amount.String()),
	)

	// Use optimistic approach with retry mechanism
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := u.attemptPreBalanceTransfer(ctx, shards, targetShardID, amount)
		if err == nil {
			u.logger.Info("Pre-balance transfer completed successfully",
				zap.String("target_shard_id", targetShardID),
				zap.Int("attempt", attempt),
			)
			return nil
		}

		u.logger.Warn("Pre-balance transfer attempt failed, retrying",
			zap.String("target_shard_id", targetShardID),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
			zap.Error(err),
		)

		if attempt < maxRetries {
			// Brief delay before retry
			time.Sleep(time.Duration(attempt) * 10 * time.Millisecond)
		}
	}

	return fmt.Errorf("pre-balance transfer failed after %d attempts", maxRetries)
}

// attemptPreBalanceTransfer attempts a single pre-balance transfer
func (u *usecase) attemptPreBalanceTransfer(ctx context.Context, shards []*account_balance_shard.AccountBalanceShard, targetShardID string, amount decimal.Decimal) error {
	return u.executeInTransaction(ctx, func(txCtx context.Context) error {
		// Get fresh shard data
		freshShards, err := u.accountBalanceShardRepo.GetByParentAccountID(txCtx, shards[0].ParentAccountID)
		if err != nil {
			return fmt.Errorf("failed to get fresh shard data: %w", err)
		}

		// Sort by balance (highest first)
		sort.Slice(freshShards, func(i, j int) bool {
			return freshShards[i].TotalBalance.GreaterThan(freshShards[j].TotalBalance)
		})

		// Find target shard
		var targetShard *account_balance_shard.AccountBalanceShard
		for _, shard := range freshShards {
			if shard.ID == targetShardID {
				targetShard = shard
				break
			}
		}

		if targetShard == nil {
			return fmt.Errorf("target shard not found: %s", targetShardID)
		}

		remainingAmount := amount
		var shardsToUpdate []*account_balance_shard.AccountBalanceShard

		// Transfer from other shards to target
		for _, shard := range freshShards {
			if shard.ID == targetShardID {
				continue // Skip target shard
			}

			if remainingAmount.LessThanOrEqual(decimal.Zero) {
				break
			}

			transferAmount := remainingAmount
			if shard.TotalBalance.LessThan(transferAmount) {
				transferAmount = shard.TotalBalance
			}

			if transferAmount.GreaterThan(decimal.Zero) {
				// Update source shard
				shard.TotalBalance = shard.TotalBalance.Sub(transferAmount)
				shardsToUpdate = append(shardsToUpdate, shard)
				remainingAmount = remainingAmount.Sub(transferAmount)

				u.logger.Info("Pre-transfer from source shard",
					zap.String("source_shard_id", shard.ID),
					zap.String("transfer_amount", transferAmount.String()),
					zap.String("remaining_balance", shard.TotalBalance.String()),
				)
			}
		}

		// Update target shard
		targetShard.TotalBalance = targetShard.TotalBalance.Add(amount.Sub(remainingAmount))
		shardsToUpdate = append(shardsToUpdate, targetShard)

		u.logger.Info("Pre-transfer to target shard",
			zap.String("target_shard_id", targetShard.ID),
			zap.String("added_amount", amount.Sub(remainingAmount).String()),
			zap.String("new_balance", targetShard.TotalBalance.String()),
		)

		// Save all changes
		if len(shardsToUpdate) > 0 {
			err = u.accountBalanceShardRepo.UpdateBalances(txCtx, shardsToUpdate)
			if err != nil {
				return fmt.Errorf("failed to update shard balances: %w", err)
			}
		}

		return nil
	})
}

// selectShardForDebitSequential provides fallback sequential shard selection
func (u *usecase) selectShardForDebitSequential(ctx context.Context, accountID string, amount decimal.Decimal) (*sub_balance_manager.ShardSelectionResult, error) {
	u.logger.Info("Using sequential shard selection as fallback",
		zap.String("account_id", accountID),
		zap.String("amount", amount.String()),
	)

	// Get shards
	shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", accountID)
	}

	// Sort shards by balance (descending) for optimal selection
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].TotalBalance.GreaterThan(shards[j].TotalBalance)
	})

	// Try to acquire lock on shards with sufficient balance (sorted by balance)
	availableShards := make([]*account_balance_shard.AccountBalanceShard, 0)
	for _, shard := range shards {
		if shard.TotalBalance.GreaterThanOrEqual(amount) {
			// Try to acquire lock with very short timeout (non-blocking)
			lockInfo, err := u.advisoryLockManager.AcquireShardAdvisoryLock(ctx, shard.ID, 10) // 10ms timeout for fast fallback
			if err == nil && lockInfo.IsAcquired {
				// SUCCESS! Lock acquired, this shard is available
				return &sub_balance_manager.ShardSelectionResult{
					SelectedShardID: shard.ID,
					ShardIndex:      shard.ShardIndex,
					ShardHash:       shard.ShardHash,
					CurrentBalance:  shard.TotalBalance,
					Strategy:        sub_balance_manager.StrategyBalanceBased,
					Reason:          fmt.Sprintf("Selected available shard %d with balance %s (sequential fallback)", shard.ShardIndex, shard.TotalBalance.String()),
				}, nil
			}
			// Lock failed, add to available list for fallback
			availableShards = append(availableShards, shard)
		}
	}

	// If no immediate lock available, use consistent hashing for load balancing
	if len(availableShards) > 0 {
		hashInput := fmt.Sprintf("%s_%d", accountID, time.Now().UnixNano())
		hash := u.generateHash(hashInput)
		shardIndex := hash % uint32(len(availableShards))
		selectedShard := availableShards[shardIndex]

		return &sub_balance_manager.ShardSelectionResult{
			SelectedShardID: selectedShard.ID,
			ShardIndex:      selectedShard.ShardIndex,
			ShardHash:       selectedShard.ShardHash,
			CurrentBalance:  selectedShard.TotalBalance,
			Strategy:        sub_balance_manager.StrategyConsistentHashing,
			Reason:          fmt.Sprintf("Selected shard %d using consistent hashing (will attempt lock)", selectedShard.ShardIndex),
		}, nil
	}

	return nil, fmt.Errorf("insufficient balance across all shards for account %s", accountID)
}

// SelectShardForCredit selects the best shard for a credit operation
func (u *usecase) SelectShardForCredit(ctx context.Context, accountID string, amount decimal.Decimal) (*sub_balance_manager.ShardSelectionResult, error) {
	u.logger.Info("Selecting shard for credit",
		zap.String("account_id", accountID),
		zap.String("amount", amount.String()),
	)

	// Get shards
	shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", accountID)
	}

	// Sort shards by balance (lowest first) for credit operations (load balancing)
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].TotalBalance.LessThan(shards[j].TotalBalance)
	})

	// Select shard with lowest balance for load balancing
	selectedShard := shards[0]

	return &sub_balance_manager.ShardSelectionResult{
		SelectedShardID: selectedShard.ID,
		ShardIndex:      selectedShard.ShardIndex,
		ShardHash:       selectedShard.ShardHash,
		CurrentBalance:  selectedShard.TotalBalance,
		Strategy:        sub_balance_manager.StrategyLoadBalancing,
		Reason:          "Selected shard with lowest balance for load balancing",
	}, nil
}

// SelectShardsForCrossShard selects multiple shards for cross-shard transactions
func (u *usecase) SelectShardsForCrossShard(ctx context.Context, accountID string, amount decimal.Decimal) ([]*sub_balance_manager.ShardSelectionResult, error) {
	u.logger.Info("Selecting shards for cross-shard transaction",
		zap.String("account_id", accountID),
		zap.String("amount", amount.String()),
	)

	// Get shards
	shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", accountID)
	}

	// Sort shards by balance (highest first)
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].TotalBalance.GreaterThan(shards[j].TotalBalance)
	})

	// Select shards that can contribute to the transaction
	results := make([]*sub_balance_manager.ShardSelectionResult, 0)
	remainingAmount := amount

	for _, shard := range shards {
		if remainingAmount.LessThanOrEqual(decimal.Zero) {
			break
		}

		contribution := shard.TotalBalance
		if contribution.GreaterThan(remainingAmount) {
			contribution = remainingAmount
		}

		if contribution.GreaterThan(decimal.Zero) {
			results = append(results, &sub_balance_manager.ShardSelectionResult{
				SelectedShardID: shard.ID,
				ShardIndex:      shard.ShardIndex,
				ShardHash:       shard.ShardHash,
				CurrentBalance:  shard.TotalBalance,
				Strategy:        sub_balance_manager.StrategyBalanceBased,
				Reason:          fmt.Sprintf("Contributing %s to cross-shard transaction", contribution.String()),
			})

			remainingAmount = remainingAmount.Sub(contribution)
		}
	}

	if remainingAmount.GreaterThan(decimal.Zero) {
		return nil, fmt.Errorf("insufficient balance across all shards for cross-shard transaction")
	}

	return results, nil
}

// AcquireAdvisoryLock acquires an advisory lock for an account
func (u *usecase) AcquireAdvisoryLock(ctx context.Context, accountID string, timeout time.Duration) (*sub_balance_manager.AdvisoryLockInfo, error) {
	u.logger.Info("Acquiring advisory lock",
		zap.String("account_id", accountID),
		zap.Duration("timeout", timeout),
	)

	lockInfo, err := u.advisoryLockManager.AcquireAdvisoryLock(ctx, accountID, postgres.LockTypeAccount, 0) // Use adaptive timeout
	if err != nil {
		return nil, fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	return &sub_balance_manager.AdvisoryLockInfo{
		LockKey:    lockInfo.LockKey,
		AccountID:  lockInfo.AccountID,
		LockType:   lockInfo.LockType,
		AcquiredAt: lockInfo.AcquiredAt,
		Timeout:    lockInfo.Timeout,
		IsAcquired: lockInfo.IsAcquired,
	}, nil
}

// ReleaseAdvisoryLock releases an advisory lock
func (u *usecase) ReleaseAdvisoryLock(ctx context.Context, lockKey string) error {
	u.logger.Info("Releasing advisory lock", zap.String("lock_key", lockKey))

	return u.advisoryLockManager.ReleaseAdvisoryLock(ctx, lockKey)
}

// IsAdvisoryLockAcquired checks if an advisory lock is acquired
func (u *usecase) IsAdvisoryLockAcquired(ctx context.Context, lockKey string) (bool, error) {
	return u.advisoryLockManager.IsAdvisoryLockAcquired(ctx, lockKey)
}

// ProcessDebitTransaction processes a debit transaction with TRUE shard-level locking
func (u *usecase) ProcessDebitTransaction(ctx context.Context, req sub_balance_manager.ProcessTransactionRequest) (*sub_balance_manager.TransactionResult, error) {
	u.logger.Info("Processing debit transaction with TRUE shard-level locking",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
		zap.String("amount", req.Amount.String()),
	)

	// Try single shard first (NO account-level lock)
	shardSelection, err := u.SelectShardForDebit(ctx, req.AccountID, req.Amount)
	if err != nil {
		// If single shard fails, try cross-shard transaction
		u.logger.Info("Single shard debit failed, attempting cross-shard transaction",
			zap.String("account_id", req.AccountID),
			zap.String("amount", req.Amount.String()),
			zap.Error(err),
		)

		// Convert to cross-shard request
		crossShardReq := sub_balance_manager.ProcessCrossShardTransactionRequest{
			TransactionID: req.TransactionID,
			AccountID:     req.AccountID,
			Amount:        req.Amount,
			Description:   req.Description,
			Metadata:      req.Metadata,
		}

		// Process cross-shard transaction
		crossShardResult, crossShardErr := u.ProcessCrossShardTransaction(ctx, crossShardReq)
		if crossShardErr != nil {
			return nil, fmt.Errorf("both single shard and cross-shard debit failed: single_shard_error=%w, cross_shard_error=%w", err, crossShardErr)
		}

		// Convert cross-shard result to single transaction result
		return &sub_balance_manager.TransactionResult{
			TransactionID:   crossShardResult.TransactionID,
			AccountID:       crossShardResult.AccountID,
			Amount:          crossShardResult.TotalAmount,
			TransactionType: sub_balance_manager.TransactionTypeDebit,
			SelectedShardID: crossShardResult.ShardOperations[0].ShardID, // Use first shard as primary
			ShardIndex:      crossShardResult.ShardOperations[0].ShardIndex,
			PreviousBalance: crossShardResult.ShardOperations[0].PreviousBalance,
			NewBalance:      crossShardResult.ShardOperations[0].NewBalance,
			TotalBalance:    crossShardResult.TotalBalance,
			ProcessedAt:     crossShardResult.ProcessedAt,
			Status:          string(crossShardResult.Status),
		}, nil
	}

	// OPTIMIZED: Single shard lock only (no advisory lock needed after pre-balance transfer)
	u.logger.Info("Processing debit with SINGLE SHARD LOCK ONLY (pre-balance transfer completed)",
		zap.String("shard_id", shardSelection.SelectedShardID),
		zap.String("strategy", string(shardSelection.Strategy)),
		zap.String("reason", shardSelection.Reason),
	)

	// Start database transaction with SINGLE SHARD ROW LOCK ONLY
	var result *sub_balance_manager.TransactionResult
	err = u.executeInTransaction(ctx, func(txCtx context.Context) error {
		// Get shard for update (ROW LOCK ONLY - no advisory lock)
		shard, err := u.accountBalanceShardRepo.GetByIDForUpdate(txCtx, shardSelection.SelectedShardID)
		if err != nil {
			return fmt.Errorf("failed to get shard for update: %w", err)
		}

		// Validate sufficient balance (should be sufficient after pre-balance transfer)
		if shard.TotalBalance.LessThan(req.Amount) {
			return fmt.Errorf("insufficient balance in shard %s: %s (pre-balance transfer may have failed)",
				shard.ID, shard.TotalBalance.String())
		}

		// Update shard balance
		previousBalance := shard.TotalBalance
		shard.DebitAmount = shard.DebitAmount.Add(req.Amount)
		shard.TotalBalance = shard.CreditAmount.Sub(shard.DebitAmount)

		if err := u.accountBalanceShardRepo.UpdateBalance(txCtx, shard); err != nil {
			return fmt.Errorf("failed to update shard balance: %w", err)
		}

		// Create transaction record
		txnReq := transaction.CreateTransactionRequest{
			TransactionID:   req.TransactionID,
			ParentAccountID: req.AccountID,
			ShardID:         shard.ID,
			ShardIndex:      shard.ShardIndex,
			TransactionType: transaction.TransactionTypeDebit,
			Amount:          req.Amount,
			PreviousBalance: previousBalance,
			NewBalance:      shard.TotalBalance,
			Description:     req.Description,
			Metadata:        req.Metadata,
		}

		if err := u.transactionRepo.Create(txCtx, &transaction.Transaction{
			ID:              uuid.New().String(),
			TransactionID:   txnReq.TransactionID,
			ParentAccountID: txnReq.ParentAccountID,
			ShardID:         txnReq.ShardID,
			ShardIndex:      txnReq.ShardIndex,
			TransactionType: string(txnReq.TransactionType),
			Amount:          txnReq.Amount,
			PreviousBalance: txnReq.PreviousBalance,
			NewBalance:      txnReq.NewBalance,
			Description:     txnReq.Description,
			Status:          string(transaction.TransactionStatusCompleted),
			Metadata:        u.convertMetadataToString(txnReq.Metadata),
		}); err != nil {
			return fmt.Errorf("failed to create transaction record: %w", err)
		}

		// Get total balance
		totalBalance, err := u.accountBalanceShardRepo.CalculateTotalBalance(txCtx, req.AccountID)
		if err != nil {
			return fmt.Errorf("failed to calculate total balance: %w", err)
		}

		result = &sub_balance_manager.TransactionResult{
			TransactionID:   req.TransactionID,
			AccountID:       req.AccountID,
			Amount:          req.Amount,
			TransactionType: req.TransactionType,
			SelectedShardID: shard.ID,
			ShardIndex:      shard.ShardIndex,
			PreviousBalance: previousBalance,
			NewBalance:      shard.TotalBalance,
			TotalBalance:    totalBalance,
			ProcessedAt:     time.Now(),
			Status:          "completed",
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	u.logger.Info("Debit transaction processed successfully with SINGLE SHARD LOCK ONLY (pre-balance transfer strategy)",
		zap.String("transaction_id", req.TransactionID),
		zap.String("shard_id", result.SelectedShardID),
		zap.String("strategy", string(shardSelection.Strategy)),
		zap.String("reason", shardSelection.Reason),
	)

	return result, nil
}

// ProcessCreditTransaction processes a credit transaction
func (u *usecase) ProcessCreditTransaction(ctx context.Context, req sub_balance_manager.ProcessTransactionRequest) (*sub_balance_manager.TransactionResult, error) {
	u.logger.Info("Processing credit transaction",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
		zap.String("amount", req.Amount.String()),
	)

	// Acquire advisory lock with optimized timeout
	lockInfo, err := u.AcquireAdvisoryLock(ctx, req.AccountID, 0) // Use adaptive timeout
	if err != nil {
		return nil, fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	if !lockInfo.IsAcquired {
		return nil, fmt.Errorf("failed to acquire advisory lock for account %s", req.AccountID)
	}

	// Start database transaction
	var result *sub_balance_manager.TransactionResult
	err = u.executeInTransaction(ctx, func(txCtx context.Context) error {
		// Select shard for credit
		shardSelection, err := u.SelectShardForCredit(txCtx, req.AccountID, req.Amount)
		if err != nil {
			return fmt.Errorf("failed to select shard for credit: %w", err)
		}

		// Get shard for update
		shard, err := u.accountBalanceShardRepo.GetByIDForUpdate(txCtx, shardSelection.SelectedShardID)
		if err != nil {
			return fmt.Errorf("failed to get shard for update: %w", err)
		}

		// Update shard balance
		previousBalance := shard.TotalBalance
		shard.CreditAmount = shard.CreditAmount.Add(req.Amount)
		shard.TotalBalance = shard.CreditAmount.Sub(shard.DebitAmount)

		if err := u.accountBalanceShardRepo.UpdateBalance(txCtx, shard); err != nil {
			return fmt.Errorf("failed to update shard balance: %w", err)
		}

		// Create transaction record
		txnReq := transaction.CreateTransactionRequest{
			TransactionID:   req.TransactionID,
			ParentAccountID: req.AccountID,
			ShardID:         shard.ID,
			ShardIndex:      shard.ShardIndex,
			TransactionType: transaction.TransactionTypeCredit,
			Amount:          req.Amount,
			PreviousBalance: previousBalance,
			NewBalance:      shard.TotalBalance,
			Description:     req.Description,
			Metadata:        req.Metadata,
		}

		if err := u.transactionRepo.Create(txCtx, &transaction.Transaction{
			ID:              uuid.New().String(),
			TransactionID:   txnReq.TransactionID,
			ParentAccountID: txnReq.ParentAccountID,
			ShardID:         txnReq.ShardID,
			ShardIndex:      txnReq.ShardIndex,
			TransactionType: string(txnReq.TransactionType),
			Amount:          txnReq.Amount,
			PreviousBalance: txnReq.PreviousBalance,
			NewBalance:      txnReq.NewBalance,
			Description:     txnReq.Description,
			Status:          string(transaction.TransactionStatusCompleted),
			Metadata:        u.convertMetadataToString(txnReq.Metadata),
		}); err != nil {
			return fmt.Errorf("failed to create transaction record: %w", err)
		}

		// Get total balance
		totalBalance, err := u.accountBalanceShardRepo.CalculateTotalBalance(txCtx, req.AccountID)
		if err != nil {
			return fmt.Errorf("failed to calculate total balance: %w", err)
		}

		result = &sub_balance_manager.TransactionResult{
			TransactionID:   req.TransactionID,
			AccountID:       req.AccountID,
			Amount:          req.Amount,
			TransactionType: req.TransactionType,
			SelectedShardID: shard.ID,
			ShardIndex:      shard.ShardIndex,
			PreviousBalance: previousBalance,
			NewBalance:      shard.TotalBalance,
			TotalBalance:    totalBalance,
			ProcessedAt:     time.Now(),
			Status:          "completed",
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	u.logger.Info("Credit transaction processed successfully",
		zap.String("transaction_id", req.TransactionID),
		zap.String("shard_id", result.SelectedShardID),
	)

	return result, nil
}

// ProcessCrossShardTransaction processes a cross-shard transaction
func (u *usecase) ProcessCrossShardTransaction(ctx context.Context, req sub_balance_manager.ProcessCrossShardTransactionRequest) (*sub_balance_manager.CrossShardTransactionResult, error) {
	u.logger.Info("Processing cross-shard transaction",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
		zap.String("amount", req.Amount.String()),
	)

	// Acquire advisory lock with optimized timeout
	lockInfo, err := u.AcquireAdvisoryLock(ctx, req.AccountID, 0) // Use adaptive timeout
	if err != nil {
		return nil, fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	if !lockInfo.IsAcquired {
		return nil, fmt.Errorf("failed to acquire advisory lock for account %s", req.AccountID)
	}

	// Start database transaction
	var result *sub_balance_manager.CrossShardTransactionResult
	err = u.executeInTransaction(ctx, func(txCtx context.Context) error {
		// Select shards for cross-shard transaction
		shardSelections, err := u.SelectShardsForCrossShard(txCtx, req.AccountID, req.Amount)
		if err != nil {
			return fmt.Errorf("failed to select shards for cross-shard transaction: %w", err)
		}

		shardOperations := make([]sub_balance_manager.ShardOperationResult, 0)
		remainingAmount := req.Amount

		// Process each shard
		for _, selection := range shardSelections {
			if remainingAmount.LessThanOrEqual(decimal.Zero) {
				break
			}

			// Get shard for update
			shard, err := u.accountBalanceShardRepo.GetByIDForUpdate(txCtx, selection.SelectedShardID)
			if err != nil {
				return fmt.Errorf("failed to get shard for update: %w", err)
			}

			// Calculate contribution amount
			contribution := shard.TotalBalance
			if contribution.GreaterThan(remainingAmount) {
				contribution = remainingAmount
			}

			// Update shard balance
			previousBalance := shard.TotalBalance
			shard.DebitAmount = shard.DebitAmount.Add(contribution)
			shard.TotalBalance = shard.CreditAmount.Sub(shard.DebitAmount)

			if err := u.accountBalanceShardRepo.UpdateBalance(txCtx, shard); err != nil {
				return fmt.Errorf("failed to update shard balance: %w", err)
			}

			// Create transaction record
			txnReq := transaction.CreateTransactionRequest{
				TransactionID:   req.TransactionID,
				ParentAccountID: req.AccountID,
				ShardID:         shard.ID,
				ShardIndex:      shard.ShardIndex,
				TransactionType: transaction.TransactionTypeDebit,
				Amount:          contribution,
				PreviousBalance: previousBalance,
				NewBalance:      shard.TotalBalance,
				Description:     req.Description,
				Metadata:        req.Metadata,
			}

			if err := u.transactionRepo.Create(txCtx, &transaction.Transaction{
				ID:              uuid.New().String(),
				TransactionID:   txnReq.TransactionID,
				ParentAccountID: txnReq.ParentAccountID,
				ShardID:         txnReq.ShardID,
				ShardIndex:      txnReq.ShardIndex,
				TransactionType: string(txnReq.TransactionType),
				Amount:          txnReq.Amount,
				PreviousBalance: txnReq.PreviousBalance,
				NewBalance:      txnReq.NewBalance,
				Description:     txnReq.Description,
				Status:          string(transaction.TransactionStatusCompleted),
				Metadata:        u.convertMetadataToString(txnReq.Metadata),
			}); err != nil {
				return fmt.Errorf("failed to create transaction record: %w", err)
			}

			// Add to shard operations
			shardOperations = append(shardOperations, sub_balance_manager.ShardOperationResult{
				ShardID:         shard.ID,
				ShardIndex:      shard.ShardIndex,
				Amount:          contribution,
				Operation:       sub_balance_manager.TransactionTypeDebit,
				PreviousBalance: previousBalance,
				NewBalance:      shard.TotalBalance,
				Status:          "completed",
				ProcessedAt:     time.Now(),
			})

			remainingAmount = remainingAmount.Sub(contribution)
		}

		// Get total balance
		totalBalance, err := u.accountBalanceShardRepo.CalculateTotalBalance(txCtx, req.AccountID)
		if err != nil {
			return fmt.Errorf("failed to calculate total balance: %w", err)
		}

		result = &sub_balance_manager.CrossShardTransactionResult{
			TransactionID:   req.TransactionID,
			AccountID:       req.AccountID,
			TotalAmount:     req.Amount,
			ShardOperations: shardOperations,
			TotalBalance:    totalBalance,
			ProcessedAt:     time.Now(),
			Status:          sub_balance_manager.CrossShardStatusCompleted,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	u.logger.Info("Cross-shard transaction processed successfully",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
		zap.Int("shard_count", len(result.ShardOperations)),
	)

	return result, nil
}

// GetTotalBalance retrieves the total balance for an account
func (u *usecase) GetTotalBalance(ctx context.Context, accountID string) (decimal.Decimal, error) {
	u.logger.Info("Getting total balance", zap.String("account_id", accountID))

	totalBalance, err := u.accountBalanceShardRepo.CalculateTotalBalance(ctx, accountID)
	if err != nil {
		u.logger.Error("Failed to get total balance",
			zap.String("account_id", accountID),
			zap.Error(err),
		)
		return decimal.Zero, fmt.Errorf("failed to get total balance: %w", err)
	}

	return totalBalance, nil
}

// GetShardBalances retrieves shard balances for an account
func (u *usecase) GetShardBalances(ctx context.Context, accountID string) ([]*sub_balance_manager.ShardBalance, error) {
	u.logger.Info("Getting shard balances", zap.String("account_id", accountID))

	shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	balances := make([]*sub_balance_manager.ShardBalance, len(shards))
	for i, shard := range shards {
		balances[i] = &sub_balance_manager.ShardBalance{
			ShardID:         shard.ID,
			ShardIndex:      shard.ShardIndex,
			ShardHash:       shard.ShardHash,
			TotalBalance:    shard.TotalBalance,
			CreditAmount:    shard.CreditAmount,
			DebitAmount:     shard.DebitAmount,
			ReserveBalance:  shard.ReserveBalance,
			UnsettledAmount: shard.UnsettledAmount,
			LastUpdated:     shard.ModifiedOn,
		}
	}

	return balances, nil
}

// ValidateBalanceConsistency validates balance consistency for an account
func (u *usecase) ValidateBalanceConsistency(ctx context.Context, accountID string) (*sub_balance_manager.ConsistencyReport, error) {
	u.logger.Info("Validating balance consistency", zap.String("account_id", accountID))

	report, err := u.accountBalanceShardRepo.ValidateConsistency(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate consistency: %w", err)
	}

	// Convert to sub balance manager format
	consistencyReport := &sub_balance_manager.ConsistencyReport{
		AccountID:       report.ParentAccountID,
		IsConsistent:    report.IsConsistent,
		TotalBalance:    report.TotalBalance,
		ShardBalances:   make([]*sub_balance_manager.ShardBalance, len(report.ShardBalances)),
		Inconsistencies: make([]sub_balance_manager.Inconsistency, len(report.Inconsistencies)),
		ValidatedAt:     report.ValidatedAt,
	}

	for i, shardBalance := range report.ShardBalances {
		consistencyReport.ShardBalances[i] = &sub_balance_manager.ShardBalance{
			ShardID:         shardBalance.ShardID,
			ShardIndex:      shardBalance.ShardIndex,
			ShardHash:       shardBalance.ShardHash,
			TotalBalance:    shardBalance.TotalBalance,
			CreditAmount:    shardBalance.CreditAmount,
			DebitAmount:     shardBalance.DebitAmount,
			ReserveBalance:  shardBalance.ReserveBalance,
			UnsettledAmount: decimal.Zero, // Not available in account_balance_shard
			LastUpdated:     time.Now(),   // Not available in account_balance_shard
		}
	}

	for i, inconsistency := range report.Inconsistencies {
		consistencyReport.Inconsistencies[i] = sub_balance_manager.Inconsistency{
			ShardID:    inconsistency.ShardID,
			ShardIndex: inconsistency.ShardIndex,
			Issue:      inconsistency.Issue,
			Expected:   inconsistency.Expected,
			Actual:     inconsistency.Actual,
			Severity:   inconsistency.Severity,
		}
	}

	return consistencyReport, nil
}

// RebalanceShards rebalances shards for an account
func (u *usecase) RebalanceShards(ctx context.Context, accountID string) error {
	u.logger.Info("Rebalancing shards", zap.String("account_id", accountID))

	// Acquire advisory lock with optimized timeout
	lockInfo, err := u.AcquireAdvisoryLock(ctx, accountID, 0) // Use adaptive timeout
	if err != nil {
		return fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	if !lockInfo.IsAcquired {
		return fmt.Errorf("failed to acquire advisory lock for account %s", accountID)
	}

	// Start database transaction
	return u.executeInTransaction(ctx, func(txCtx context.Context) error {
		// Get all shards for update
		shards, err := u.accountBalanceShardRepo.GetByParentAccountIDForUpdate(txCtx, accountID)
		if err != nil {
			return fmt.Errorf("failed to get shards for rebalancing: %w", err)
		}

		if len(shards) < 2 {
			u.logger.Info("No rebalancing needed - insufficient shards",
				zap.String("account_id", accountID),
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
				zap.String("account_id", accountID),
			)
			return nil
		}

		// Update shards
		if err := u.accountBalanceShardRepo.UpdateBalances(txCtx, updates); err != nil {
			u.logger.Error("Failed to update shards during rebalancing",
				zap.String("account_id", accountID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to update shards during rebalancing: %w", err)
		}

		u.logger.Info("Shards rebalanced successfully",
			zap.String("account_id", accountID),
			zap.Int("updated_shards", len(updates)),
			zap.String("target_balance", targetBalance.String()),
		)

		return nil
	})
}

// OptimizeShardDistribution optimizes shard distribution for an account
func (u *usecase) OptimizeShardDistribution(ctx context.Context, accountID string) error {
	u.logger.Info("Optimizing shard distribution", zap.String("account_id", accountID))

	// For now, this is the same as rebalancing
	// In the future, this could include more sophisticated optimization algorithms
	return u.RebalanceShards(ctx, accountID)
}

// txKey is used as a context key for database transaction
type txKey string

const txKeyValue txKey = "tx"

// ParallelShardResult represents the result of parallel shard lock acquisition
type ParallelShardResult struct {
	Shard      *account_balance_shard.AccountBalanceShard
	LockInfo   *postgres.AdvisoryLockInfo
	Error      error
	Success    bool
	Index      int
	AcquiredAt time.Time
}

// ParallelShardSelectionConfig configures parallel shard selection behavior
type ParallelShardSelectionConfig struct {
	MaxConcurrency   int           // Maximum number of concurrent lock attempts
	LockTimeout      time.Duration // Timeout for each lock attempt
	EarlyReturn      bool          // Return immediately when first lock is acquired
	SortByBalance    bool          // Sort shards by balance before processing
	BalanceThreshold float64       // Minimum balance ratio to consider shard
}

// selectShardForDebitParallel performs parallel shard selection with concurrent lock acquisition
func (u *usecase) selectShardForDebitParallel(ctx context.Context, accountID string, amount decimal.Decimal, config ParallelShardSelectionConfig) (*sub_balance_manager.ShardSelectionResult, error) {
	u.logger.Info("Selecting shard for debit with parallel lock acquisition",
		zap.String("account_id", accountID),
		zap.String("amount", amount.String()),
		zap.Int("max_concurrency", config.MaxConcurrency),
		zap.Duration("lock_timeout", config.LockTimeout),
	)

	// Get shards
	shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", accountID)
	}

	// Filter shards with sufficient balance
	eligibleShards := make([]*account_balance_shard.AccountBalanceShard, 0)
	for _, shard := range shards {
		if shard.TotalBalance.GreaterThanOrEqual(amount) {
			eligibleShards = append(eligibleShards, shard)
		}
	}

	if len(eligibleShards) == 0 {
		return nil, fmt.Errorf("insufficient balance across all shards for account %s", accountID)
	}

	// Sort shards by balance if configured
	if config.SortByBalance {
		sort.Slice(eligibleShards, func(i, j int) bool {
			return eligibleShards[i].TotalBalance.GreaterThan(eligibleShards[j].TotalBalance)
		})
	}

	// Limit concurrency to prevent overwhelming the system
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 5 // Default concurrency
	}
	if len(eligibleShards) < config.MaxConcurrency {
		config.MaxConcurrency = len(eligibleShards)
	}

	// Channel to receive results
	resultChan := make(chan ParallelShardResult, config.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var selectedResult *ParallelShardResult
	var firstSuccess time.Time

	// Start goroutines for parallel lock acquisition
	for i := 0; i < config.MaxConcurrency; i++ {
		wg.Add(1)
		go func(shardIndex int) {
			defer wg.Done()

			shard := eligibleShards[shardIndex]

			// Try to acquire lock
			lockInfo, err := u.advisoryLockManager.AcquireShardAdvisoryLock(ctx, shard.ID, config.LockTimeout)
			acquiredAt := time.Now()

			result := ParallelShardResult{
				Shard:      shard,
				LockInfo:   lockInfo,
				Error:      err,
				Success:    err == nil && lockInfo != nil && lockInfo.IsAcquired,
				Index:      shardIndex,
				AcquiredAt: acquiredAt,
			}

			// If early return is enabled and this is the first success, store it
			if config.EarlyReturn && result.Success {
				mu.Lock()
				if selectedResult == nil || firstSuccess.IsZero() || acquiredAt.Before(firstSuccess) {
					selectedResult = &result
					firstSuccess = acquiredAt
				}
				mu.Unlock()
			}

			resultChan <- result
		}(i)
	}

	// Close result channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []ParallelShardResult
	var successfulResults []ParallelShardResult

	for result := range resultChan {
		results = append(results, result)
		if result.Success {
			successfulResults = append(successfulResults, result)

			// If early return is enabled and we have a selected result, use it
			if config.EarlyReturn && selectedResult != nil && result.Shard.ID == selectedResult.Shard.ID {
				return u.convertParallelResultToShardSelection(result), nil
			}
		}
	}

	// If no successful results, return error
	if len(successfulResults) == 0 {
		u.logger.Warn("No shards could be locked in parallel",
			zap.String("account_id", accountID),
			zap.Int("total_attempts", len(results)),
			zap.Duration("lock_timeout", config.LockTimeout),
		)
		return nil, fmt.Errorf("all shard lock attempts failed for account %s", accountID)
	}

	// Select the best successful result (first one by balance or fastest)
	var bestResult ParallelShardResult
	if config.SortByBalance {
		// Select the one with highest balance
		bestResult = successfulResults[0]
		for _, result := range successfulResults[1:] {
			if result.Shard.TotalBalance.GreaterThan(bestResult.Shard.TotalBalance) {
				bestResult = result
			}
		}
	} else {
		// Select the fastest one
		bestResult = successfulResults[0]
		for _, result := range successfulResults[1:] {
			if result.AcquiredAt.Before(bestResult.AcquiredAt) {
				bestResult = result
			}
		}
	}

	// Release locks for unsuccessful attempts
	for _, result := range results {
		if result.Success && result.Shard.ID != bestResult.Shard.ID {
			u.advisoryLockManager.ReleaseAdvisoryLock(ctx, result.Shard.ID)
		}
	}

	u.logger.Info("Parallel shard selection completed",
		zap.String("account_id", accountID),
		zap.String("selected_shard_id", bestResult.Shard.ID),
		zap.Int("successful_attempts", len(successfulResults)),
		zap.Int("total_attempts", len(results)),
		zap.Duration("acquisition_time", time.Since(bestResult.AcquiredAt)),
	)

	return u.convertParallelResultToShardSelection(bestResult), nil
}

// convertParallelResultToShardSelection converts ParallelShardResult to ShardSelectionResult
func (u *usecase) convertParallelResultToShardSelection(result ParallelShardResult) *sub_balance_manager.ShardSelectionResult {
	return &sub_balance_manager.ShardSelectionResult{
		SelectedShardID: result.Shard.ID,
		ShardIndex:      result.Shard.ShardIndex,
		ShardHash:       result.Shard.ShardHash,
		CurrentBalance:  result.Shard.TotalBalance,
		Strategy:        sub_balance_manager.StrategyLoadBalancing,
		Reason:          fmt.Sprintf("Selected shard %d via parallel acquisition with balance %s", result.Shard.ShardIndex, result.Shard.TotalBalance.String()),
	}
}

// getDefaultParallelConfig returns default configuration for parallel shard selection
func (u *usecase) getDefaultParallelConfig() ParallelShardSelectionConfig {
	return ParallelShardSelectionConfig{
		MaxConcurrency:   10,                    // Process up to 10 shards in parallel (increased)
		LockTimeout:      25 * time.Millisecond, // 25ms timeout per lock (reduced for speed)
		EarlyReturn:      true,                  // Return immediately when first lock is acquired
		SortByBalance:    true,                  // Sort by balance for optimal selection
		BalanceThreshold: 0.05,                  // Consider shards with at least 5% of amount (reduced)
	}
}

// getOptimizedParallelConfig returns optimized configuration for high TPS scenarios
func (u *usecase) getOptimizedParallelConfig() ParallelShardSelectionConfig {
	return ParallelShardSelectionConfig{
		MaxConcurrency:   15,                    // Process up to 15 shards in parallel (maximum)
		LockTimeout:      15 * time.Millisecond, // 15ms timeout per lock (very fast)
		EarlyReturn:      true,                  // Return immediately when first lock is acquired
		SortByBalance:    true,                  // Sort by balance for optimal selection
		BalanceThreshold: 0.02,                  // Consider shards with at least 2% of amount (very low threshold)
	}
}

// executeInTransaction executes a function within a database transaction
func (u *usecase) executeInTransaction(ctx context.Context, fn func(context.Context) error) error {
	// Use GORM's transaction support for proper database transaction
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKeyValue, tx)
		return fn(txCtx)
	})
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// ProcessDebitTransactionOptimistic processes debit transaction using optimistic locking
func (u *usecase) ProcessDebitTransactionOptimistic(ctx context.Context, req sub_balance_manager.ProcessTransactionRequest) (*sub_balance_manager.TransactionResult, error) {
	u.logger.Info("Processing debit transaction with OPTIMISTIC LOCKING",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
		zap.String("amount", req.Amount.String()),
	)

	// Step 1: Lock-free balance check
	shards, err := u.accountBalanceShardRepo.GetShardsWithBalanceInfo(ctx, req.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shards with balance info: %w", err)
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards found for account %s", req.AccountID)
	}

	// Step 2: Find shard with sufficient balance using round-robin distribution (lock-free)
	var selectedShard *account_balance_shard.AccountBalanceShard
	var eligibleShards []*account_balance_shard.AccountBalanceShard

	// Collect all shards with sufficient balance
	for _, shard := range shards {
		if shard.TotalBalance.GreaterThanOrEqual(req.Amount) {
			eligibleShards = append(eligibleShards, shard)
		}
	}

	// Use round-robin selection based on transaction ID hash for load distribution
	if len(eligibleShards) > 0 {
		// Use transaction ID to create deterministic but distributed selection
		hash := u.generateHash(req.TransactionID)
		selectedIndex := int(hash % uint32(len(eligibleShards)))
		selectedShard = eligibleShards[selectedIndex]

		u.logger.Debug("Shard selected using round-robin distribution",
			zap.String("transaction_id", req.TransactionID),
			zap.String("selected_shard_id", selectedShard.ID),
			zap.Int("shard_index", selectedShard.ShardIndex),
			zap.Int("eligible_shards_count", len(eligibleShards)),
			zap.String("selected_balance", selectedShard.TotalBalance.String()),
		)
	}

	// Step 3: If no single shard has sufficient balance, try cross-shard
	if selectedShard == nil {
		u.logger.Info("No single shard has sufficient balance, attempting cross-shard with optimistic locking",
			zap.String("account_id", req.AccountID),
			zap.String("amount", req.Amount.String()),
		)
		return u.processCrossShardDebitOptimistic(ctx, req, shards)
	}

	// Step 4: Process single shard debit with optimistic locking
	return u.processSingleShardDebitOptimistic(ctx, req, selectedShard)
}

// processSingleShardDebitOptimistic processes debit on single shard with optimistic locking
func (u *usecase) processSingleShardDebitOptimistic(ctx context.Context, req sub_balance_manager.ProcessTransactionRequest, shard *account_balance_shard.AccountBalanceShard) (*sub_balance_manager.TransactionResult, error) {
	maxRetries := 10 // Increased from 3 to 10 for better conflict resolution

	// Prepare updated shard data
	previousBalance := shard.TotalBalance
	newBalance := shard.TotalBalance.Sub(req.Amount)

	updatedShard := &account_balance_shard.AccountBalanceShard{
		ID:              shard.ID,
		ParentAccountID: shard.ParentAccountID,
		ShardIndex:      shard.ShardIndex,
		ShardHash:       shard.ShardHash,
		CreditAmount:    shard.CreditAmount,
		DebitAmount:     shard.DebitAmount.Add(req.Amount),
		TotalBalance:    newBalance,
		ReserveBalance:  shard.ReserveBalance,
		UnsettledAmount: shard.UnsettledAmount,
		Version:         shard.Version,
	}

	// Try optimistic locking with retry
	err := u.accountBalanceShardRepo.UpdateBalanceOptimisticWithRetry(ctx, updatedShard, shard.Version, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to update shard balance with optimistic locking: %w", err)
	}

	// Create transaction record
	transaction := &transaction.Transaction{
		ID:              uuid.New().String(),
		TransactionID:   req.TransactionID,
		ParentAccountID: req.AccountID,
		ShardID:         shard.ID,
		ShardIndex:      shard.ShardIndex,
		TransactionType: string(transaction.TransactionTypeDebit),
		Amount:          req.Amount,
		PreviousBalance: previousBalance,
		NewBalance:      newBalance,
		Description:     req.Description,
		Status:          string(transaction.TransactionStatusCompleted),
		Metadata:        u.convertMetadataToString(req.Metadata),
		Version:         1,
		RetryCount:      0, // Will be updated if retries occurred
	}

	if err := u.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction record: %w", err)
	}

	// Get total balance (lock-free)
	totalBalance, err := u.accountBalanceShardRepo.CalculateTotalBalance(ctx, req.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate total balance: %w", err)
	}

	result := &sub_balance_manager.TransactionResult{
		TransactionID:   req.TransactionID,
		AccountID:       req.AccountID,
		Amount:          req.Amount,
		TransactionType: req.TransactionType,
		SelectedShardID: shard.ID,
		ShardIndex:      shard.ShardIndex,
		PreviousBalance: previousBalance,
		NewBalance:      newBalance,
		TotalBalance:    totalBalance,
		ProcessedAt:     time.Now(),
	}

	u.logger.Info("Debit transaction processed successfully with OPTIMISTIC LOCKING",
		zap.String("transaction_id", req.TransactionID),
		zap.String("shard_id", shard.ID),
		zap.String("strategy", "optimistic_single_shard"),
		zap.String("reason", "Selected shard with sufficient balance, optimistic locking applied"),
	)

	return result, nil
}

// processCrossShardDebitOptimistic processes cross-shard debit with optimistic locking
func (u *usecase) processCrossShardDebitOptimistic(ctx context.Context, req sub_balance_manager.ProcessTransactionRequest, shards []*account_balance_shard.AccountBalanceShard) (*sub_balance_manager.TransactionResult, error) {
	// Calculate total available balance
	totalAvailableBalance := decimal.Zero
	for _, shard := range shards {
		totalAvailableBalance = totalAvailableBalance.Add(shard.TotalBalance)
	}

	if totalAvailableBalance.LessThan(req.Amount) {
		return nil, fmt.Errorf("insufficient total balance: available %s, required %s",
			totalAvailableBalance.String(), req.Amount.String())
	}

	// Find target shard (highest balance)
	var targetShard *account_balance_shard.AccountBalanceShard
	maxBalance := decimal.Zero
	for _, shard := range shards {
		if shard.TotalBalance.GreaterThan(maxBalance) {
			maxBalance = shard.TotalBalance
			targetShard = shard
		}
	}

	if targetShard == nil {
		return nil, fmt.Errorf("no target shard found")
	}

	// Transfer balance to target shard using optimistic locking
	remainingAmount := req.Amount.Sub(targetShard.TotalBalance)

	for _, shard := range shards {
		if shard.ID == targetShard.ID {
			continue // Skip target shard
		}

		if remainingAmount.LessThanOrEqual(decimal.Zero) {
			break // Enough balance transferred
		}

		transferAmount := remainingAmount
		if transferAmount.GreaterThan(shard.TotalBalance) {
			transferAmount = shard.TotalBalance
		}

		// Update source shard (debit)
		sourceUpdated := &account_balance_shard.AccountBalanceShard{
			ID:              shard.ID,
			ParentAccountID: shard.ParentAccountID,
			ShardIndex:      shard.ShardIndex,
			ShardHash:       shard.ShardHash,
			CreditAmount:    shard.CreditAmount,
			DebitAmount:     shard.DebitAmount,
			TotalBalance:    shard.TotalBalance.Sub(transferAmount),
			ReserveBalance:  shard.ReserveBalance,
			UnsettledAmount: shard.UnsettledAmount,
			Version:         shard.Version,
		}

		err := u.accountBalanceShardRepo.UpdateBalanceOptimisticWithRetry(ctx, sourceUpdated, shard.Version, 10)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer balance from shard %s: %w", shard.ID, err)
		}

		// Update target shard (credit)
		targetUpdated := &account_balance_shard.AccountBalanceShard{
			ID:              targetShard.ID,
			ParentAccountID: targetShard.ParentAccountID,
			ShardIndex:      targetShard.ShardIndex,
			ShardHash:       targetShard.ShardHash,
			CreditAmount:    targetShard.CreditAmount,
			DebitAmount:     targetShard.DebitAmount,
			TotalBalance:    targetShard.TotalBalance.Add(transferAmount),
			ReserveBalance:  targetShard.ReserveBalance,
			UnsettledAmount: targetShard.UnsettledAmount,
			Version:         targetShard.Version,
		}

		err = u.accountBalanceShardRepo.UpdateBalanceOptimisticWithRetry(ctx, targetUpdated, targetShard.Version, 10)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer balance to shard %s: %w", targetShard.ID, err)
		}

		// Update target shard for next iteration
		targetShard = targetUpdated
		remainingAmount = remainingAmount.Sub(transferAmount)
	}

	// Now process the debit on target shard
	return u.processSingleShardDebitOptimistic(ctx, req, targetShard)
}
