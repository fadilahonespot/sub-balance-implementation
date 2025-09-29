package postgres

import (
	"context"
	"fmt"
	"time"

	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/infra"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// accountRepository implements the account.Repository interface
type accountRepository struct {
	db          *gorm.DB
	shardRouter *infra.ShardRouter
	logger      *zap.Logger
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *gorm.DB, logger *zap.Logger) account.Repository {
	return &accountRepository{
		db:     db,
		logger: logger,
	}
}

// NewAccountRepositoryWithSharding creates a new account repository with shard router
func NewAccountRepositoryWithSharding(db *gorm.DB, shardRouter *infra.ShardRouter, logger *zap.Logger) account.Repository {
	return &accountRepository{
		db:          db,
		shardRouter: shardRouter,
		logger:      logger,
	}
}

// Create creates a new account
func (r *accountRepository) Create(ctx context.Context, acc *account.Account) error {
	if acc.ID == "" {
		acc.ID = uuid.New().String()
	}

	now := time.Now()
	acc.CreatedOn = now
	acc.ModifiedOn = now

	// Use shard router if available, otherwise use primary database
	var db *gorm.DB
	if r.shardRouter != nil {
		r.logger.Info("Using shard router for account creation",
			zap.String("account_id", acc.ID),
		)

		// Get GORM shard connection based on account ID
		shardDB, shardID, err := r.shardRouter.GetGormShardConnection(acc.ID)
		if err != nil {
			r.logger.Error("Failed to get GORM shard connection",
				zap.String("account_id", acc.ID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to get GORM shard connection: %w", err)
		}

		db = shardDB
		r.logger.Info("Account will be created in shard",
			zap.String("account_id", acc.ID),
			zap.Int("shard_id", shardID),
		)
	} else {
		r.logger.Info("Using primary database for account creation",
			zap.String("account_id", acc.ID),
		)
		db = r.db
	}

	if err := db.WithContext(ctx).Create(acc).Error; err != nil {
		r.logger.Error("Failed to create account",
			zap.String("account_id", acc.ID),
			zap.String("wallet_no", acc.WalletNo),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create account: %w", err)
	}

	r.logger.Info("Account created successfully",
		zap.String("account_id", acc.ID),
		zap.String("wallet_no", acc.WalletNo),
	)

	return nil
}

// GetByID retrieves an account by ID
func (r *accountRepository) GetByID(ctx context.Context, id string) (*account.Account, error) {
	var acc account.Account

	// Use shard router if available
	var db *gorm.DB
	if r.shardRouter != nil {
		r.logger.Info("Using shard router for GetByID",
			zap.String("account_id", id),
		)

		shardDB, shardID, err := r.shardRouter.GetGormShardConnection(id)
		if err != nil {
			r.logger.Error("Failed to get GORM shard connection for GetByID",
				zap.String("account_id", id),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to get GORM shard connection: %w", err)
		}
		db = shardDB
		r.logger.Info("Getting account from shard",
			zap.String("account_id", id),
			zap.Int("shard_id", shardID),
		)
	} else {
		r.logger.Info("Using primary database for GetByID",
			zap.String("account_id", id),
		)
		db = r.db
	}

	if err := db.WithContext(ctx).Where("id = ?", id).First(&acc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("account not found: %s", id)
		}
		r.logger.Error("Failed to get account by ID",
			zap.String("account_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &acc, nil
}

// GetByWalletNo retrieves an account by wallet number
func (r *accountRepository) GetByWalletNo(ctx context.Context, walletNo string) (*account.Account, error) {
	var acc account.Account

	if err := r.db.WithContext(ctx).Where("wallet_no = ?", walletNo).First(&acc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("account not found with wallet_no: %s", walletNo)
		}
		r.logger.Error("Failed to get account by wallet number",
			zap.String("wallet_no", walletNo),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &acc, nil
}

// Update updates an existing account
func (r *accountRepository) Update(ctx context.Context, acc *account.Account) error {
	acc.ModifiedOn = time.Now()

	if err := r.db.WithContext(ctx).Save(acc).Error; err != nil {
		r.logger.Error("Failed to update account",
			zap.String("account_id", acc.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update account: %w", err)
	}

	r.logger.Info("Account updated successfully",
		zap.String("account_id", acc.ID),
	)

	return nil
}

// Delete deletes an account by ID
func (r *accountRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&account.Account{})
	if result.Error != nil {
		r.logger.Error("Failed to delete account",
			zap.String("account_id", id),
			zap.Error(result.Error),
		)
		return fmt.Errorf("failed to delete account: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: %s", id)
	}

	r.logger.Info("Account deleted successfully",
		zap.String("account_id", id),
	)

	return nil
}

// List retrieves a list of accounts with pagination
func (r *accountRepository) List(ctx context.Context, limit, offset int) ([]*account.Account, error) {
	var accounts []*account.Account

	query := r.db.WithContext(ctx).Model(&account.Account{})

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&accounts).Error; err != nil {
		r.logger.Error("Failed to list accounts",
			zap.Int("limit", limit),
			zap.Int("offset", offset),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	return accounts, nil
}

// GetHotAccounts retrieves all hot accounts
func (r *accountRepository) GetHotAccounts(ctx context.Context) ([]*account.Account, error) {
	var accounts []*account.Account

	if err := r.db.WithContext(ctx).Where("hot_account = ?", true).Find(&accounts).Error; err != nil {
		r.logger.Error("Failed to get hot accounts",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get hot accounts: %w", err)
	}

	r.logger.Info("Retrieved hot accounts",
		zap.Int("count", len(accounts)),
	)

	return accounts, nil
}

// GetAccountsWithSubBalance retrieves accounts that use sub balance
func (r *accountRepository) GetAccountsWithSubBalance(ctx context.Context) ([]*account.Account, error) {
	var accounts []*account.Account

	if err := r.db.WithContext(ctx).Where("use_sub_balance = ?", true).Find(&accounts).Error; err != nil {
		r.logger.Error("Failed to get accounts with sub balance",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get accounts with sub balance: %w", err)
	}

	return accounts, nil
}

// UpdateSubBalanceConfig updates sub balance configuration for an account
func (r *accountRepository) UpdateSubBalanceConfig(ctx context.Context, accountID string, useSubBalance bool, shardCount int) error {
	updates := map[string]interface{}{
		"use_sub_balance":         useSubBalance,
		"sub_balance_shard_count": shardCount,
		"modified_on":             time.Now(),
	}

	result := r.db.WithContext(ctx).Model(&account.Account{}).Where("id = ?", accountID).Updates(updates)
	if result.Error != nil {
		r.logger.Error("Failed to update sub balance config",
			zap.String("account_id", accountID),
			zap.Bool("use_sub_balance", useSubBalance),
			zap.Int("shard_count", shardCount),
			zap.Error(result.Error),
		)
		return fmt.Errorf("failed to update sub balance config: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("account not found: %s", accountID)
	}

	r.logger.Info("Sub balance config updated successfully",
		zap.String("account_id", accountID),
		zap.Bool("use_sub_balance", useSubBalance),
		zap.Int("shard_count", shardCount),
	)

	return nil
}

// GetAccountStats returns statistics about accounts
func (r *accountRepository) GetAccountStats(ctx context.Context) (map[string]interface{}, error) {
	var stats struct {
		TotalAccounts      int64 `json:"total_accounts"`
		ActiveAccounts     int64 `json:"active_accounts"`
		HotAccounts        int64 `json:"hot_accounts"`
		SubBalanceAccounts int64 `json:"sub_balance_accounts"`
		DebitHotAccounts   int64 `json:"debit_hot_accounts"`
	}

	// Get total accounts
	if err := r.db.WithContext(ctx).Model(&account.Account{}).Count(&stats.TotalAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get total accounts count: %w", err)
	}

	// Get active accounts
	if err := r.db.WithContext(ctx).Model(&account.Account{}).Where("active = ?", true).Count(&stats.ActiveAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get active accounts count: %w", err)
	}

	// Get hot accounts
	if err := r.db.WithContext(ctx).Model(&account.Account{}).Where("hot_account = ?", true).Count(&stats.HotAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get hot accounts count: %w", err)
	}

	// Get sub balance accounts
	if err := r.db.WithContext(ctx).Model(&account.Account{}).Where("use_sub_balance = ?", true).Count(&stats.SubBalanceAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get sub balance accounts count: %w", err)
	}

	// Get debit hot accounts
	if err := r.db.WithContext(ctx).Model(&account.Account{}).Where("debit_hot_account = ?", true).Count(&stats.DebitHotAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get debit hot accounts count: %w", err)
	}

	return map[string]interface{}{
		"total_accounts":       stats.TotalAccounts,
		"active_accounts":      stats.ActiveAccounts,
		"hot_accounts":         stats.HotAccounts,
		"sub_balance_accounts": stats.SubBalanceAccounts,
		"debit_hot_accounts":   stats.DebitHotAccounts,
	}, nil
}

// SearchAccounts searches accounts by various criteria
func (r *accountRepository) SearchAccounts(ctx context.Context, criteria account.SearchCriteria) ([]*account.Account, error) {
	var accounts []*account.Account

	query := r.db.WithContext(ctx).Model(&account.Account{})

	if criteria.WalletNo != "" {
		query = query.Where("wallet_no ILIKE ?", "%"+criteria.WalletNo+"%")
	}

	if criteria.OwnerID != "" {
		query = query.Where("owner_id = ?", criteria.OwnerID)
	}

	if criteria.WalletTypeID != "" {
		query = query.Where("wallet_type_id = ?", criteria.WalletTypeID)
	}

	if criteria.InstanceType != "" {
		query = query.Where("instance_type = ?", criteria.InstanceType)
	}

	if criteria.Active != nil {
		query = query.Where("active = ?", *criteria.Active)
	}

	if criteria.HotAccount != nil {
		query = query.Where("hot_account = ?", *criteria.HotAccount)
	}

	if criteria.UseSubBalance != nil {
		query = query.Where("use_sub_balance = ?", *criteria.UseSubBalance)
	}

	if criteria.Limit > 0 {
		query = query.Limit(criteria.Limit)
	}

	if criteria.Offset > 0 {
		query = query.Offset(criteria.Offset)
	}

	if err := query.Find(&accounts).Error; err != nil {
		r.logger.Error("Failed to search accounts",
			zap.Any("criteria", criteria),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to search accounts: %w", err)
	}

	return accounts, nil
}
