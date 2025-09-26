package postgres

import (
	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/account_balance_shard"
	"sub-balance-implementation/internal/domain/transaction"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RepositoryFactory creates and manages all repository instances
type RepositoryFactory struct {
	db     *gorm.DB
	logger *zap.Logger

	// Repositories
	accountRepo             account.Repository
	accountBalanceShardRepo account_balance_shard.Repository
	transactionRepo         transaction.Repository

	// Utilities
	advisoryLockManager *AdvisoryLockManager
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(db *gorm.DB, logger *zap.Logger) *RepositoryFactory {
	return &RepositoryFactory{
		db:     db,
		logger: logger,
	}
}

// GetAccountRepository returns the account repository
func (rf *RepositoryFactory) GetAccountRepository() account.Repository {
	if rf.accountRepo == nil {
		rf.accountRepo = NewAccountRepository(rf.db, rf.logger)
	}
	return rf.accountRepo
}

// GetAccountBalanceShardRepository returns the account balance shard repository
func (rf *RepositoryFactory) GetAccountBalanceShardRepository() account_balance_shard.Repository {
	if rf.accountBalanceShardRepo == nil {
		rf.accountBalanceShardRepo = NewAccountBalanceShardRepository(rf.db, rf.logger)
	}
	return rf.accountBalanceShardRepo
}

// GetTransactionRepository returns the transaction repository
func (rf *RepositoryFactory) GetTransactionRepository() transaction.Repository {
	if rf.transactionRepo == nil {
		rf.transactionRepo = NewTransactionRepository(rf.db, rf.logger)
	}
	return rf.transactionRepo
}

// GetAdvisoryLockManager returns the advisory lock manager
func (rf *RepositoryFactory) GetAdvisoryLockManager() *AdvisoryLockManager {
	if rf.advisoryLockManager == nil {
		rf.advisoryLockManager = NewAdvisoryLockManager(rf.db, rf.logger)
	}
	return rf.advisoryLockManager
}

// GetAllRepositories returns all repositories in a single struct
func (rf *RepositoryFactory) GetAllRepositories() *AllRepositories {
	return &AllRepositories{
		Account:             rf.GetAccountRepository(),
		AccountBalanceShard: rf.GetAccountBalanceShardRepository(),
		Transaction:         rf.GetTransactionRepository(),
		AdvisoryLock:        rf.GetAdvisoryLockManager(),
	}
}

// AllRepositories contains all repository instances
type AllRepositories struct {
	Account             account.Repository
	AccountBalanceShard account_balance_shard.Repository
	Transaction         transaction.Repository
	AdvisoryLock        *AdvisoryLockManager
}

// Close closes all database connections
func (rf *RepositoryFactory) Close() error {
	return CloseConnection(rf.db)
}

// HealthCheck performs health check on all repositories
func (rf *RepositoryFactory) HealthCheck() error {
	return HealthCheck(rf.db)
}

// GetConnectionStats returns connection statistics
func (rf *RepositoryFactory) GetConnectionStats() (map[string]interface{}, error) {
	return GetConnectionStats(rf.db)
}
