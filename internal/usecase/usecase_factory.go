package usecase

import (
	"sub-balance-implementation/internal/domain/account"
	"sub-balance-implementation/internal/domain/account_balance_shard"
	"sub-balance-implementation/internal/domain/sub_balance_manager"
	"sub-balance-implementation/internal/domain/transaction"
	"sub-balance-implementation/internal/infra/postgres"
	accountUsecase "sub-balance-implementation/internal/usecase/account"
	accountBalanceShardUsecase "sub-balance-implementation/internal/usecase/account_balance_shard"
	"sub-balance-implementation/internal/usecase/sub_balance"
	transactionUsecase "sub-balance-implementation/internal/usecase/transaction"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UsecaseFactory creates and manages all usecase instances
type UsecaseFactory struct {
	db     *gorm.DB
	repos  *postgres.AllRepositories
	logger *zap.Logger

	// Usecases
	accountUsecase             account.Usecase
	accountBalanceShardUsecase account_balance_shard.Usecase
	transactionUsecase         transaction.Usecase
	subBalanceManagerUsecase   sub_balance_manager.Usecase
}

// NewUsecaseFactory creates a new usecase factory
func NewUsecaseFactory(db *gorm.DB, repos *postgres.AllRepositories, logger *zap.Logger) *UsecaseFactory {
	return &UsecaseFactory{
		db:     db,
		repos:  repos,
		logger: logger,
	}
}

// GetAccountUsecase returns the account usecase
func (uf *UsecaseFactory) GetAccountUsecase() account.Usecase {
	if uf.accountUsecase == nil {
		uf.accountUsecase = accountUsecase.NewUsecase(
			uf.repos.Account,
			uf.repos.AccountBalanceShard,
			uf.logger,
		)
	}
	return uf.accountUsecase
}

// GetAccountBalanceShardUsecase returns the account balance shard usecase
func (uf *UsecaseFactory) GetAccountBalanceShardUsecase() account_balance_shard.Usecase {
	if uf.accountBalanceShardUsecase == nil {
		uf.accountBalanceShardUsecase = accountBalanceShardUsecase.NewUsecase(
			uf.repos.AccountBalanceShard,
			uf.logger,
		)
	}
	return uf.accountBalanceShardUsecase
}

// GetTransactionUsecase returns the transaction usecase
func (uf *UsecaseFactory) GetTransactionUsecase() transaction.Usecase {
	if uf.transactionUsecase == nil {
		uf.transactionUsecase = transactionUsecase.NewUsecase(
			uf.repos.Transaction,
			uf.logger,
		)
	}
	return uf.transactionUsecase
}

// GetSubBalanceManagerUsecase returns the sub balance manager usecase
func (uf *UsecaseFactory) GetSubBalanceManagerUsecase() sub_balance_manager.Usecase {
	if uf.subBalanceManagerUsecase == nil {
		uf.subBalanceManagerUsecase = sub_balance.NewUsecase(
			uf.db,
			uf.repos.Account,
			uf.repos.AccountBalanceShard,
			uf.repos.Transaction,
			uf.repos.AdvisoryLock,
			uf.logger,
		)
	}
	return uf.subBalanceManagerUsecase
}

// GetAllUsecases returns all usecases in a single struct
func (uf *UsecaseFactory) GetAllUsecases() *AllUsecases {
	return &AllUsecases{
		Account:             uf.GetAccountUsecase(),
		AccountBalanceShard: uf.GetAccountBalanceShardUsecase(),
		Transaction:         uf.GetTransactionUsecase(),
		SubBalanceManager:   uf.GetSubBalanceManagerUsecase(),
	}
}

// AllUsecases contains all usecase instances
type AllUsecases struct {
	Account             account.Usecase
	AccountBalanceShard account_balance_shard.Usecase
	Transaction         transaction.Usecase
	SubBalanceManager   sub_balance_manager.Usecase
}
