package postgres

import (
	"fmt"
	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/account_balance_shard"
	"sub-balance-implementation/internal/domain/transaction"
	"sub-balance-implementation/internal/infra"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RepositoryFactory creates and manages all repository instances
type RepositoryFactory struct {
	db          *gorm.DB
	shardRouter *infra.ShardRouter
	logger      *zap.Logger

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

// NewRepositoryFactoryWithSharding creates a new repository factory with shard router
func NewRepositoryFactoryWithSharding(db *gorm.DB, shardRouter *infra.ShardRouter, logger *zap.Logger) *RepositoryFactory {
	logger.Info("Creating RepositoryFactoryWithSharding",
		zap.Bool("shard_router_nil", shardRouter == nil),
		zap.Int("shard_count", shardRouter.GetShardCount()),
		zap.String("shard_router_addr", fmt.Sprintf("%p", shardRouter)),
	)

	rf := &RepositoryFactory{
		db:          db,
		shardRouter: shardRouter,
		logger:      logger,
	}

	logger.Info("RepositoryFactoryWithSharding created",
		zap.Bool("rf_shard_router_nil", rf.shardRouter == nil),
		zap.String("rf_shard_router_addr", fmt.Sprintf("%p", rf.shardRouter)),
	)

	return rf
}

// GetAccountRepository returns the account repository
func (rf *RepositoryFactory) GetAccountRepository() account.Repository {
	if rf.accountRepo == nil {
		rf.logger.Info("GetAccountRepository called",
			zap.Bool("shard_router_nil", rf.shardRouter == nil),
			zap.String("shard_router_addr", fmt.Sprintf("%p", rf.shardRouter)),
			zap.String("rf_addr", fmt.Sprintf("%p", rf)),
		)
		if rf.shardRouter != nil {
			rf.logger.Info("Creating AccountRepository with shard router")
			rf.accountRepo = NewAccountRepositoryWithSharding(rf.db, rf.shardRouter, rf.logger)
		} else {
			rf.logger.Info("Creating AccountRepository without shard router")
			rf.accountRepo = NewAccountRepository(rf.db, rf.logger)
		}
	}
	return rf.accountRepo
}

// GetAccountBalanceShardRepository returns the account balance shard repository
func (rf *RepositoryFactory) GetAccountBalanceShardRepository() account_balance_shard.Repository {
	if rf.accountBalanceShardRepo == nil {
		rf.logger.Info("GetAccountBalanceShardRepository called",
			zap.Bool("shard_router_nil", rf.shardRouter == nil),
			zap.String("shard_router_addr", fmt.Sprintf("%p", rf.shardRouter)),
		)
		if rf.shardRouter != nil {
			rf.logger.Info("Creating AccountBalanceShardRepository with shard router")
			rf.accountBalanceShardRepo = NewAccountBalanceShardRepositoryWithSharding(rf.db, rf.shardRouter, rf.logger)
		} else {
			rf.logger.Info("Creating AccountBalanceShardRepository without shard router")
			rf.accountBalanceShardRepo = NewAccountBalanceShardRepository(rf.db, rf.logger)
		}
	}
	return rf.accountBalanceShardRepo
}

// GetTransactionRepository returns the transaction repository
func (rf *RepositoryFactory) GetTransactionRepository() transaction.Repository {
	if rf.transactionRepo == nil {
		rf.logger.Info("GetTransactionRepository called",
			zap.Bool("shard_router_nil", rf.shardRouter == nil),
			zap.String("shard_router_addr", fmt.Sprintf("%p", rf.shardRouter)),
		)
		if rf.shardRouter != nil {
			rf.logger.Info("Creating TransactionRepository with shard router")
			rf.transactionRepo = NewTransactionRepositoryWithSharding(rf.db, rf.shardRouter, rf.logger)
		} else {
			rf.logger.Info("Creating TransactionRepository without shard router")
			rf.transactionRepo = NewTransactionRepository(rf.db, rf.logger)
		}
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

// GetDB returns the database connection
func (rf *RepositoryFactory) GetDB() *gorm.DB {
	return rf.db
}

// HealthCheck performs health check on all repositories
func (rf *RepositoryFactory) HealthCheck() error {
	return HealthCheck(rf.db)
}

// GetConnectionStats returns connection statistics
func (rf *RepositoryFactory) GetConnectionStats() (map[string]interface{}, error) {
	return GetConnectionStats(rf.db)
}
