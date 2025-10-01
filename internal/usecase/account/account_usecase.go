package account

import (
	"context"
	"fmt"

	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/account_balance_shard"
	"sub-balance-implementation/pkg/utils"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// usecase implements the account.Usecase interface
type usecase struct {
	accountRepo             account.Repository
	accountBalanceShardRepo account_balance_shard.Repository
	hashUtils               *utils.HashUtils
	logger                  *zap.Logger
}

// NewUsecase creates a new account usecase
func NewUsecase(
	accountRepo account.Repository,
	accountBalanceShardRepo account_balance_shard.Repository,
	logger *zap.Logger,
) account.Usecase {
	return &usecase{
		accountRepo:             accountRepo,
		accountBalanceShardRepo: accountBalanceShardRepo,
		hashUtils:               utils.NewHashUtils(),
		logger:                  logger,
	}
}

// CreateAccount creates a new account
func (u *usecase) CreateAccount(ctx context.Context, req account.CreateAccountRequest) (*account.Account, error) {
	u.logger.Info("Creating new account",
		zap.String("wallet_no", req.WalletNo),
		zap.String("owner_id", req.OwnerId),
		zap.Bool("use_sub_balance", req.UseSubBalance),
	)

	// Check if wallet number already exists
	existingAccount, err := u.accountRepo.GetByWalletNo(ctx, req.WalletNo)
	if err == nil && existingAccount != nil {
		return nil, fmt.Errorf("account with wallet number %s already exists", req.WalletNo)
	}

	// Create new account
	newAccount := &account.Account{
		ID:                   uuid.New().String(),
		CreatedBy:            "system", // TODO: Get from context
		ModifiedBy:           "system", // TODO: Get from context
		WalletNo:             req.WalletNo,
		WalletTypeId:         req.WalletTypeId,
		WalletTypeName:       "", // Will be set later
		InstanceType:         req.InstanceType,
		WalletStatus:         "active",
		CurrencyId:           req.CurrencyId,
		OwnerId:              req.OwnerId,
		MinimumBalance:       req.MinimumBalance,
		UpperLimit:           req.UpperLimit,
		LowerLimit:           req.LowerLimit,
		Active:               true,
		UseSubBalance:        req.UseSubBalance,
		SubBalanceShardCount: req.ShardCount,
	}

	// Set default shard count if not provided
	if newAccount.SubBalanceShardCount == 0 {
		newAccount.SubBalanceShardCount = 3
	}

	// Create account in database
	if err := u.accountRepo.Create(ctx, newAccount); err != nil {
		u.logger.Error("Failed to create account",
			zap.String("wallet_no", req.WalletNo),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// Initialize sub balance shards if enabled
	if req.UseSubBalance {
		if err := u.initializeSubBalanceShards(ctx, newAccount.ID, newAccount.SubBalanceShardCount); err != nil {
			u.logger.Error("Failed to initialize sub balance shards",
				zap.String("account_id", newAccount.ID),
				zap.Int("shard_count", newAccount.SubBalanceShardCount),
				zap.Error(err),
			)
			// Rollback account creation
			u.accountRepo.Delete(ctx, newAccount.ID)
			return nil, fmt.Errorf("failed to initialize sub balance shards: %w", err)
		}
	}

	u.logger.Info("Account created successfully",
		zap.String("account_id", newAccount.ID),
		zap.String("wallet_no", newAccount.WalletNo),
		zap.Bool("use_sub_balance", newAccount.UseSubBalance),
	)

	return newAccount, nil
}

// GetAccount retrieves an account by ID
func (u *usecase) GetAccount(ctx context.Context, id string) (*account.Account, error) {
	u.logger.Info("Getting account", zap.String("account_id", id))

	acc, err := u.accountRepo.GetByID(ctx, id)
	if err != nil {
		u.logger.Error("Failed to get account",
			zap.String("account_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return acc, nil
}

// UpdateAccount updates an existing account
func (u *usecase) UpdateAccount(ctx context.Context, id string, req account.UpdateAccountRequest) (*account.Account, error) {
	u.logger.Info("Updating account", zap.String("account_id", id))

	// Get existing account
	acc, err := u.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Update fields if provided
	if req.WalletTypeName != nil {
		acc.WalletTypeName = *req.WalletTypeName
	}
	if req.InstanceType != nil {
		acc.InstanceType = *req.InstanceType
	}
	if req.WalletStatus != nil {
		acc.WalletStatus = *req.WalletStatus
	}
	if req.MinimumBalance != nil {
		acc.MinimumBalance = *req.MinimumBalance
	}
	if req.UpperLimit != nil {
		acc.UpperLimit = *req.UpperLimit
	}
	if req.LowerLimit != nil {
		acc.LowerLimit = *req.LowerLimit
	}
	if req.Active != nil {
		acc.Active = *req.Active
	}
	if req.UseSubBalance != nil {
		acc.UseSubBalance = *req.UseSubBalance
	}
	// Handle shard count change
	if req.ShardCount != nil && *req.ShardCount != acc.SubBalanceShardCount {
		oldShardCount := acc.SubBalanceShardCount
		newShardCount := *req.ShardCount

		u.logger.Info("Shard count changed",
			zap.String("account_id", id),
			zap.Int("old_count", oldShardCount),
			zap.Int("new_count", newShardCount),
		)

		// Handle shard count reduction
		if newShardCount < oldShardCount {
			if err := u.handleShardCountReduction(ctx, id, oldShardCount, newShardCount); err != nil {
				return nil, fmt.Errorf("failed to handle shard count reduction: %w", err)
			}
		}

		// Handle shard count increase
		if newShardCount > oldShardCount {
			if err := u.handleShardCountIncrease(ctx, id, oldShardCount, newShardCount); err != nil {
				return nil, fmt.Errorf("failed to handle shard count increase: %w", err)
			}
		}

		acc.SubBalanceShardCount = newShardCount
	}

	acc.ModifiedBy = "system" // TODO: Get from context

	// Update account in database
	if err := u.accountRepo.Update(ctx, acc); err != nil {
		u.logger.Error("Failed to update account",
			zap.String("account_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to update account: %w", err)
	}

	u.logger.Info("Account updated successfully", zap.String("account_id", id))

	return acc, nil
}

// DeleteAccount deletes an account
func (u *usecase) DeleteAccount(ctx context.Context, id string) error {
	u.logger.Info("Deleting account", zap.String("account_id", id))

	// Check if account exists
	acc, err := u.accountRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Delete sub balance shards if they exist
	if acc.UseSubBalance {
		if err := u.accountBalanceShardRepo.DeleteByParentAccountID(ctx, id); err != nil {
			u.logger.Error("Failed to delete sub balance shards",
				zap.String("account_id", id),
				zap.Error(err),
			)
			return fmt.Errorf("failed to delete sub balance shards: %w", err)
		}
	}

	// Delete account
	if err := u.accountRepo.Delete(ctx, id); err != nil {
		u.logger.Error("Failed to delete account",
			zap.String("account_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete account: %w", err)
	}

	u.logger.Info("Account deleted successfully", zap.String("account_id", id))

	return nil
}

// ListAccounts retrieves a list of accounts with pagination
func (u *usecase) ListAccounts(ctx context.Context, limit, offset int) ([]*account.Account, error) {
	u.logger.Info("Listing accounts",
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	accounts, err := u.accountRepo.List(ctx, limit, offset)
	if err != nil {
		u.logger.Error("Failed to list accounts",
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	return accounts, nil
}

// MigrateToSubBalance migrates an account to use sub balance
func (u *usecase) MigrateToSubBalance(ctx context.Context, accountID string, shardCount int) error {
	u.logger.Info("Migrating account to sub balance",
		zap.String("account_id", accountID),
		zap.Int("shard_count", shardCount),
	)

	// Get existing account
	acc, err := u.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Check if already using sub balance
	if acc.UseSubBalance {
		return fmt.Errorf("account %s already uses sub balance", accountID)
	}

	// Set default shard count if not provided
	if shardCount == 0 {
		shardCount = 3
	}

	// Update account configuration
	if err := u.accountRepo.UpdateSubBalanceConfig(ctx, accountID, true, shardCount); err != nil {
		u.logger.Error("Failed to update account sub balance config",
			zap.String("account_id", accountID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update account sub balance config: %w", err)
	}

	// Initialize sub balance shards
	if err := u.initializeSubBalanceShards(ctx, accountID, shardCount); err != nil {
		u.logger.Error("Failed to initialize sub balance shards",
			zap.String("account_id", accountID),
			zap.Int("shard_count", shardCount),
			zap.Error(err),
		)
		// Rollback account configuration
		u.accountRepo.UpdateSubBalanceConfig(ctx, accountID, false, 0)
		return fmt.Errorf("failed to initialize sub balance shards: %w", err)
	}

	u.logger.Info("Account migrated to sub balance successfully",
		zap.String("account_id", accountID),
		zap.Int("shard_count", shardCount),
	)

	return nil
}

// GetAccountBalance retrieves account balance information
func (u *usecase) GetAccountBalance(ctx context.Context, accountID string) (*account.AccountBalanceResponse, error) {
	u.logger.Info("Getting account balance", zap.String("account_id", accountID))

	// Get account
	acc, err := u.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	response := &account.AccountBalanceResponse{
		AccountID:     accountID,
		UseSubBalance: acc.UseSubBalance,
		ShardCount:    acc.SubBalanceShardCount,
	}

	if acc.UseSubBalance {
		// Get shard balances
		shards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get shard balances: %w", err)
		}

		// Calculate total balance
		totalBalance := decimal.Zero
		creditAmount := decimal.Zero
		debitAmount := decimal.Zero
		reserveBalance := decimal.Zero
		unsettledAmount := decimal.Zero

		response.Shards = make([]account.ShardBalance, len(shards))

		for i, shard := range shards {
			totalBalance = totalBalance.Add(shard.TotalBalance)
			creditAmount = creditAmount.Add(shard.CreditAmount)
			debitAmount = debitAmount.Add(shard.DebitAmount)
			reserveBalance = reserveBalance.Add(shard.ReserveBalance)
			unsettledAmount = unsettledAmount.Add(shard.UnsettledAmount)

			response.Shards[i] = account.ShardBalance{
				ShardID:        shard.ID,
				ShardIndex:     shard.ShardIndex,
				TotalBalance:   shard.TotalBalance,
				CreditAmount:   shard.CreditAmount,
				DebitAmount:    shard.DebitAmount,
				ReserveBalance: shard.ReserveBalance,
			}
		}

		response.TotalBalance = totalBalance
		response.CreditAmount = creditAmount
		response.DebitAmount = debitAmount
		response.ReserveBalance = reserveBalance
		response.UnsettledAmount = unsettledAmount
	} else {
		// For non-sub balance accounts, return zero balances
		response.TotalBalance = decimal.Zero
		response.CreditAmount = decimal.Zero
		response.DebitAmount = decimal.Zero
		response.ReserveBalance = decimal.Zero
		response.UnsettledAmount = decimal.Zero
	}

	return response, nil
}

// initializeSubBalanceShards initializes sub balance shards for an account
func (u *usecase) initializeSubBalanceShards(ctx context.Context, accountID string, shardCount int) error {
	u.logger.Info("Initializing sub balance shards",
		zap.String("account_id", accountID),
		zap.Int("shard_count", shardCount),
	)

	shards := make([]*account_balance_shard.AccountBalanceShard, shardCount)

	for i := 0; i < shardCount; i++ {
		shard := &account_balance_shard.AccountBalanceShard{
			ID:              uuid.New().String(),
			ParentAccountID: accountID,
			ShardIndex:      i,
			ShardHash:       u.hashUtils.GenerateConsistentShardHash(accountID, i, shardCount),
			CreditAmount:    decimal.Zero,
			DebitAmount:     decimal.Zero,
			TotalBalance:    decimal.Zero,
			ReserveBalance:  decimal.Zero,
			UnsettledAmount: decimal.Zero,
		}

		if err := u.accountBalanceShardRepo.Create(ctx, shard); err != nil {
			u.logger.Error("Failed to create shard",
				zap.String("account_id", accountID),
				zap.Int("shard_index", i),
				zap.Error(err),
			)
			return fmt.Errorf("failed to create shard %d: %w", i, err)
		}

		shards[i] = shard
	}

	u.logger.Info("Sub balance shards initialized successfully",
		zap.String("account_id", accountID),
		zap.Int("shard_count", len(shards)),
	)

	return nil
}

// GetShardIndexByAccountHash determines which shard an account should belong to using consistent hashing
func (u *usecase) GetShardIndexByAccountHash(accountID string, shardCount int) int {
	return u.hashUtils.GetShardIndexByAccountHash(accountID, shardCount)
}

// ValidateShardDistribution validates that accounts are evenly distributed across shards
func (u *usecase) ValidateShardDistribution(accountIDs []string, shardCount int) map[int]int {
	return u.hashUtils.ValidateShardDistribution(accountIDs, shardCount)
}

// GetOptimalShardCount calculates optimal shard count based on account count
func (u *usecase) GetOptimalShardCount(accountCount int) int {
	return u.hashUtils.GetOptimalShardCount(accountCount)
}

// handleShardCountReduction handles reducing the number of shards
func (u *usecase) handleShardCountReduction(ctx context.Context, accountID string, oldCount, newCount int) error {
	u.logger.Info("Handling shard count reduction",
		zap.String("account_id", accountID),
		zap.Int("old_count", oldCount),
		zap.Int("new_count", newCount),
	)

	// Get all existing shards
	existingShards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get existing shards: %w", err)
	}

	// Calculate total balance from shards that will be removed
	var totalBalanceToRedistribute decimal.Decimal
	var shardsToRemove []*account_balance_shard.AccountBalanceShard

	for _, shard := range existingShards {
		if shard.ShardIndex >= newCount {
			totalBalanceToRedistribute = totalBalanceToRedistribute.Add(shard.TotalBalance)
			shardsToRemove = append(shardsToRemove, shard)
		}
	}

	// If there's balance to redistribute, distribute it evenly among remaining shards
	if totalBalanceToRedistribute.GreaterThan(decimal.Zero) {
		balancePerShard := totalBalanceToRedistribute.Div(decimal.NewFromInt(int64(newCount)))

		for _, shard := range existingShards {
			if shard.ShardIndex < newCount {
				shard.TotalBalance = shard.TotalBalance.Add(balancePerShard)
				shard.CreditAmount = shard.CreditAmount.Add(balancePerShard)

				if err := u.accountBalanceShardRepo.UpdateBalance(ctx, shard); err != nil {
					return fmt.Errorf("failed to update shard %d: %w", shard.ShardIndex, err)
				}
			}
		}
	}

	// Delete shards that are no longer needed
	for _, shard := range shardsToRemove {
		if err := u.accountBalanceShardRepo.Delete(ctx, shard.ID); err != nil {
			return fmt.Errorf("failed to delete shard %d: %w", shard.ShardIndex, err)
		}
	}

	u.logger.Info("Shard count reduction completed",
		zap.String("account_id", accountID),
		zap.String("redistributed_balance", totalBalanceToRedistribute.String()),
		zap.Int("removed_shards", len(shardsToRemove)),
	)

	return nil
}

// handleShardCountIncrease handles increasing the number of shards
func (u *usecase) handleShardCountIncrease(ctx context.Context, accountID string, oldCount, newCount int) error {
	u.logger.Info("Handling shard count increase",
		zap.String("account_id", accountID),
		zap.Int("old_count", oldCount),
		zap.Int("new_count", newCount),
	)

	// Get existing shards
	existingShards, err := u.accountBalanceShardRepo.GetByParentAccountID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get existing shards: %w", err)
	}

	// Calculate total balance to redistribute
	var totalBalance decimal.Decimal
	for _, shard := range existingShards {
		totalBalance = totalBalance.Add(shard.TotalBalance)
	}

	// Create new shards
	for i := oldCount; i < newCount; i++ {
		newShard := &account_balance_shard.AccountBalanceShard{
			ID:              uuid.New().String(),
			ParentAccountID: accountID,
			ShardIndex:      i,
			ShardHash:       u.hashUtils.GenerateConsistentShardHash(accountID, i, newCount),
			CreditAmount:    decimal.Zero,
			DebitAmount:     decimal.Zero,
			TotalBalance:    decimal.Zero,
			ReserveBalance:  decimal.Zero,
			UnsettledAmount: decimal.Zero,
		}

		if err := u.accountBalanceShardRepo.Create(ctx, newShard); err != nil {
			return fmt.Errorf("failed to create new shard %d: %w", i, err)
		}
	}

	// Redistribute balance evenly across all shards
	if totalBalance.GreaterThan(decimal.Zero) {
		balancePerShard := totalBalance.Div(decimal.NewFromInt(int64(newCount)))

		// Update existing shards
		for _, shard := range existingShards {
			shard.TotalBalance = balancePerShard
			shard.CreditAmount = balancePerShard
			shard.DebitAmount = decimal.Zero

			if err := u.accountBalanceShardRepo.UpdateBalance(ctx, shard); err != nil {
				return fmt.Errorf("failed to update existing shard %d: %w", shard.ShardIndex, err)
			}
		}

		// Update new shards
		for i := oldCount; i < newCount; i++ {
			newShard := &account_balance_shard.AccountBalanceShard{
				ID:              uuid.New().String(),
				ParentAccountID: accountID,
				ShardIndex:      i,
				ShardHash:       u.hashUtils.GenerateConsistentShardHash(accountID, i, newCount),
				CreditAmount:    balancePerShard,
				DebitAmount:     decimal.Zero,
				TotalBalance:    balancePerShard,
				ReserveBalance:  decimal.Zero,
				UnsettledAmount: decimal.Zero,
			}

			if err := u.accountBalanceShardRepo.UpdateBalance(ctx, newShard); err != nil {
				return fmt.Errorf("failed to update new shard %d: %w", i, err)
			}
		}
	}

	u.logger.Info("Shard count increase completed",
		zap.String("account_id", accountID),
		zap.Int("new_shards_created", newCount-oldCount),
	)

	return nil
}
