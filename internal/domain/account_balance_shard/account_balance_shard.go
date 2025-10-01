package account_balance_shard

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// AccountBalanceShard represents a shard of account balance
type AccountBalanceShard struct {
	ID              string          `json:"id" gorm:"primaryKey;column:id"`
	ParentAccountID string          `json:"parent_account_id" gorm:"column:parent_account_id;index"`
	ShardIndex      int             `json:"shard_index" gorm:"column:shard_index"`
	ShardHash       string          `json:"shard_hash" gorm:"column:shard_hash;index"`
	CreditAmount    decimal.Decimal `json:"credit_amount" gorm:"column:credit_amount;type:decimal(20,2);default:0"`
	DebitAmount     decimal.Decimal `json:"debit_amount" gorm:"column:debit_amount;type:decimal(20,2);default:0"`
	TotalBalance    decimal.Decimal `json:"total_balance" gorm:"column:total_balance;type:decimal(20,2);default:0"`
	ReserveBalance  decimal.Decimal `json:"reserve_balance" gorm:"column:reserve_balance;type:decimal(20,2);default:0"`
	UnsettledAmount decimal.Decimal `json:"unsettled_amount" gorm:"column:unsettled_amount;type:decimal(20,2);default:0"`
	CreatedOn       time.Time       `json:"created_on" gorm:"column:created_on;autoCreateTime"`
	ModifiedOn      time.Time       `json:"modified_on" gorm:"column:modified_on;autoUpdateTime"`
	Checksum        string          `json:"checksum" gorm:"column:checksum"`
	Version         int             `json:"version" gorm:"column:version;default:1"`
}

// TableName overrides the default table name
func (AccountBalanceShard) TableName() string {
	return "account_balance_shard"
}

// Repository interface for account balance shard operations
type Repository interface {
	Create(ctx context.Context, shard *AccountBalanceShard) error
	GetByID(ctx context.Context, id string) (*AccountBalanceShard, error)
	GetByIDForUpdate(ctx context.Context, id string) (*AccountBalanceShard, error)
	GetByParentAccountID(ctx context.Context, parentAccountID string) ([]*AccountBalanceShard, error)
	GetByParentAccountIDForUpdate(ctx context.Context, parentAccountID string) ([]*AccountBalanceShard, error)
	GetByShardHash(ctx context.Context, parentAccountID, shardHash string) (*AccountBalanceShard, error)
	GetByShardHashForUpdate(ctx context.Context, parentAccountID, shardHash string) (*AccountBalanceShard, error)
	UpdateBalance(ctx context.Context, shard *AccountBalanceShard) error
	UpdateBalances(ctx context.Context, shards []*AccountBalanceShard) error
	Delete(ctx context.Context, id string) error
	DeleteByParentAccountID(ctx context.Context, parentAccountID string) error
	GetShardWithLowestBalance(ctx context.Context, parentAccountID string) (*AccountBalanceShard, error)
	GetShardWithHighestBalance(ctx context.Context, parentAccountID string) (*AccountBalanceShard, error)
	GetShardByIndex(ctx context.Context, parentAccountID string, shardIndex int) (*AccountBalanceShard, error)
	GetShardByIndexForUpdate(ctx context.Context, parentAccountID string, shardIndex int) (*AccountBalanceShard, error)
	CalculateTotalBalance(ctx context.Context, parentAccountID string) (decimal.Decimal, error)
	ValidateConsistency(ctx context.Context, parentAccountID string) (*ConsistencyReport, error)

	// Optimistic locking methods (NEW)
	UpdateBalanceOptimistic(ctx context.Context, shard *AccountBalanceShard, expectedVersion int) error
	UpdateBalanceOptimisticWithRetry(ctx context.Context, shard *AccountBalanceShard, expectedVersion int, maxRetries int) error
	GetShardsWithBalanceInfo(ctx context.Context, parentAccountID string) ([]*AccountBalanceShard, error)
}

// Usecase interface for account balance shard business logic
type Usecase interface {
	CreateShards(ctx context.Context, parentAccountID string, shardCount int) ([]*AccountBalanceShard, error)
	GetShards(ctx context.Context, parentAccountID string) ([]*AccountBalanceShard, error)
	UpdateShardBalance(ctx context.Context, shardID string, creditAmount, debitAmount decimal.Decimal) error
	SelectShardForDebit(ctx context.Context, parentAccountID string, amount decimal.Decimal) (*AccountBalanceShard, error)
	SelectShardForCredit(ctx context.Context, parentAccountID string, amount decimal.Decimal) (*AccountBalanceShard, error)
	GetTotalBalance(ctx context.Context, parentAccountID string) (decimal.Decimal, error)
	ValidateShardConsistency(ctx context.Context, parentAccountID string) (*ConsistencyReport, error)
	RebalanceShards(ctx context.Context, parentAccountID string) error
}

// ConsistencyReport represents the result of consistency validation
type ConsistencyReport struct {
	ParentAccountID string          `json:"parent_account_id"`
	IsConsistent    bool            `json:"is_consistent"`
	TotalBalance    decimal.Decimal `json:"total_balance"`
	ShardBalances   []ShardBalance  `json:"shard_balances"`
	Inconsistencies []Inconsistency `json:"inconsistencies,omitempty"`
	ValidatedAt     time.Time       `json:"validated_at"`
}

type ShardBalance struct {
	ShardID        string          `json:"shard_id"`
	ShardIndex     int             `json:"shard_index"`
	ShardHash      string          `json:"shard_hash"`
	TotalBalance   decimal.Decimal `json:"total_balance"`
	CreditAmount   decimal.Decimal `json:"credit_amount"`
	DebitAmount    decimal.Decimal `json:"debit_amount"`
	ReserveBalance decimal.Decimal `json:"reserve_balance"`
}

type Inconsistency struct {
	ShardID    string          `json:"shard_id"`
	ShardIndex int             `json:"shard_index"`
	Issue      string          `json:"issue"`
	Expected   decimal.Decimal `json:"expected,omitempty"`
	Actual     decimal.Decimal `json:"actual,omitempty"`
	Severity   string          `json:"severity"` // "low", "medium", "high"
}

// Request/Response DTOs
type CreateShardsRequest struct {
	ParentAccountID string `json:"parent_account_id" validate:"required"`
	ShardCount      int    `json:"shard_count" validate:"required,min=1,max=10"`
}

type UpdateShardBalanceRequest struct {
	ShardID      string          `json:"shard_id" validate:"required"`
	CreditAmount decimal.Decimal `json:"credit_amount"`
	DebitAmount  decimal.Decimal `json:"debit_amount"`
}

type SelectShardRequest struct {
	ParentAccountID string          `json:"parent_account_id" validate:"required"`
	Amount          decimal.Decimal `json:"amount" validate:"required,gt=0"`
	Operation       string          `json:"operation" validate:"required,oneof=debit credit"`
}

type ShardResponse struct {
	ShardID         string          `json:"shard_id"`
	ParentAccountID string          `json:"parent_account_id"`
	ShardIndex      int             `json:"shard_index"`
	ShardHash       string          `json:"shard_hash"`
	CreditAmount    decimal.Decimal `json:"credit_amount"`
	DebitAmount     decimal.Decimal `json:"debit_amount"`
	TotalBalance    decimal.Decimal `json:"total_balance"`
	ReserveBalance  decimal.Decimal `json:"reserve_balance"`
	UnsettledAmount decimal.Decimal `json:"unsettled_amount"`
	CreatedOn       time.Time       `json:"created_on"`
	ModifiedOn      time.Time       `json:"modified_on"`
}

type TotalBalanceResponse struct {
	ParentAccountID string          `json:"parent_account_id"`
	TotalBalance    decimal.Decimal `json:"total_balance"`
	ShardCount      int             `json:"shard_count"`
	Shards          []ShardResponse `json:"shards"`
}
