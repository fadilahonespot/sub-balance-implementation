package sub_balance

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/account_balance_shard"
	"sub-balance-implementation/internal/domain/sub_balance_manager"
	"sub-balance-implementation/internal/domain/transaction"
	"sub-balance-implementation/internal/infra/postgres"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// usecase implements the sub_balance_manager.Usecase interface
type usecase struct {
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

// NewUsecase creates a new sub balance manager usecase
func NewUsecase(
	accountRepo account.Repository,
	accountBalanceShardRepo account_balance_shard.Repository,
	transactionRepo transaction.Repository,
	advisoryLockManager *postgres.AdvisoryLockManager,
	logger *zap.Logger,
) sub_balance_manager.Usecase {
	return &usecase{
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
		HotAccount:    acc.HotAccount,
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

// SelectShardForDebit selects the best shard for a debit operation
func (u *usecase) SelectShardForDebit(ctx context.Context, accountID string, amount decimal.Decimal) (*sub_balance_manager.ShardSelectionResult, error) {
	u.logger.Info("Selecting shard for debit",
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

	// Sort shards by balance (highest first) for debit operations
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].TotalBalance.GreaterThan(shards[j].TotalBalance)
	})

	// Find shard with sufficient balance
	for _, shard := range shards {
		if shard.TotalBalance.GreaterThanOrEqual(amount) {
			return &sub_balance_manager.ShardSelectionResult{
				SelectedShardID: shard.ID,
				ShardIndex:      shard.ShardIndex,
				ShardHash:       shard.ShardHash,
				CurrentBalance:  shard.TotalBalance,
				Strategy:        sub_balance_manager.StrategyBalanceBased,
				Reason:          "Selected shard with highest available balance",
			}, nil
		}
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

// ProcessDebitTransaction processes a debit transaction with shard-level locking
func (u *usecase) ProcessDebitTransaction(ctx context.Context, req sub_balance_manager.ProcessTransactionRequest) (*sub_balance_manager.TransactionResult, error) {
	u.logger.Info("Processing debit transaction",
		zap.String("transaction_id", req.TransactionID),
		zap.String("account_id", req.AccountID),
		zap.String("amount", req.Amount.String()),
	)

	// Start database transaction with shard-level locking
	var result *sub_balance_manager.TransactionResult
	err := u.executeInTransaction(ctx, func(txCtx context.Context) error {
		// Try single shard first
		shardSelection, err := u.SelectShardForDebit(txCtx, req.AccountID, req.Amount)
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
			crossShardResult, crossShardErr := u.ProcessCrossShardTransaction(txCtx, crossShardReq)
			if crossShardErr != nil {
				return fmt.Errorf("both single shard and cross-shard debit failed: single_shard_error=%w, cross_shard_error=%w", err, crossShardErr)
			}

			// Convert cross-shard result to single transaction result
			result = &sub_balance_manager.TransactionResult{
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
			}
			return nil
		}

		// Acquire shard-level lock for the selected shard
		shardLockInfo, err := u.advisoryLockManager.AcquireShardAdvisoryLock(txCtx, shardSelection.SelectedShardID, 0)
		if err != nil {
			return fmt.Errorf("failed to acquire shard lock: %w", err)
		}
		if !shardLockInfo.IsAcquired {
			return fmt.Errorf("failed to acquire shard lock for shard %s", shardSelection.SelectedShardID)
		}

		// Get shard for update
		shard, err := u.accountBalanceShardRepo.GetByIDForUpdate(txCtx, shardSelection.SelectedShardID)
		if err != nil {
			return fmt.Errorf("failed to get shard for update: %w", err)
		}

		// Validate sufficient balance
		if shard.TotalBalance.LessThan(req.Amount) {
			return fmt.Errorf("insufficient balance in shard %s: %s", shard.ID, shard.TotalBalance.String())
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

	u.logger.Info("Debit transaction processed successfully",
		zap.String("transaction_id", req.TransactionID),
		zap.String("shard_id", result.SelectedShardID),
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

// executeInTransaction executes a function within a database transaction
func (u *usecase) executeInTransaction(ctx context.Context, fn func(context.Context) error) error {
	// This is a placeholder implementation
	// In a real implementation, this would use GORM's transaction support
	return fn(ctx)
}
