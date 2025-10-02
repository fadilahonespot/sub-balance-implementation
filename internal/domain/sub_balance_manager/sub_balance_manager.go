package sub_balance_manager

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// SubBalanceManager handles sub balance operations and routing
type SubBalanceManager struct {
	ShardCount int
}

// ShardSelectionStrategy defines how shards are selected for operations
type ShardSelectionStrategy string

const (
	StrategyBalanceBased       ShardSelectionStrategy = "balance_based"
	StrategyLoadBalancing      ShardSelectionStrategy = "load_balancing"
	StrategyRoundRobin         ShardSelectionStrategy = "round_robin"
	StrategyHashBased          ShardSelectionStrategy = "hash_based"
	StrategyConsistentHashing  ShardSelectionStrategy = "consistent_hashing"
	StrategyHighestBalance     ShardSelectionStrategy = "highest_balance"
	StrategyPreBalanceTransfer ShardSelectionStrategy = "pre_balance_transfer"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeDebit  TransactionType = "debit"
	TransactionTypeCredit TransactionType = "credit"
)

// ShardSelectionResult contains the result of shard selection
type ShardSelectionResult struct {
	SelectedShardID string                 `json:"selected_shard_id"`
	ShardIndex      int                    `json:"shard_index"`
	ShardHash       string                 `json:"shard_hash"`
	CurrentBalance  decimal.Decimal        `json:"current_balance"`
	Strategy        ShardSelectionStrategy `json:"strategy"`
	Reason          string                 `json:"reason"`
}

// CrossShardTransaction represents a transaction that spans multiple shards
type CrossShardTransaction struct {
	TransactionID   string                      `json:"transaction_id"`
	ParentAccountID string                      `json:"parent_account_id"`
	TotalAmount     decimal.Decimal             `json:"total_amount"`
	ShardOperations []ShardOperation            `json:"shard_operations"`
	CreatedAt       time.Time                   `json:"created_at"`
	Status          CrossShardTransactionStatus `json:"status"`
}

type ShardOperation struct {
	ShardID        string          `json:"shard_id"`
	ShardIndex     int             `json:"shard_index"`
	Amount         decimal.Decimal `json:"amount"`
	Operation      TransactionType `json:"operation"`
	CurrentBalance decimal.Decimal `json:"current_balance"`
}

type CrossShardTransactionStatus string

const (
	CrossShardStatusPending    CrossShardTransactionStatus = "pending"
	CrossShardStatusProcessing CrossShardTransactionStatus = "processing"
	CrossShardStatusCompleted  CrossShardTransactionStatus = "completed"
	CrossShardStatusFailed     CrossShardTransactionStatus = "failed"
	CrossShardStatusRolledBack CrossShardTransactionStatus = "rolled_back"
)

// AdvisoryLockInfo contains information about advisory locks
type AdvisoryLockInfo struct {
	LockKey    string        `json:"lock_key"`
	AccountID  string        `json:"account_id"`
	LockType   string        `json:"lock_type"`
	AcquiredAt time.Time     `json:"acquired_at"`
	Timeout    time.Duration `json:"timeout"`
	IsAcquired bool          `json:"is_acquired"`
}

// Usecase interface for sub balance manager business logic
type Usecase interface {
	// Shard Management
	InitializeSubBalance(ctx context.Context, accountID string, shardCount int) error
	GetSubBalanceInfo(ctx context.Context, accountID string) (*SubBalanceInfo, error)

	// Shard Selection
	SelectShardForDebit(ctx context.Context, accountID string, amount decimal.Decimal, transactionID string) (*ShardSelectionResult, error)
	SelectShardForCredit(ctx context.Context, accountID string, amount decimal.Decimal) (*ShardSelectionResult, error)
	SelectShardsForCrossShard(ctx context.Context, accountID string, amount decimal.Decimal) ([]*ShardSelectionResult, error)

	// Advisory Lock Management
	AcquireAdvisoryLock(ctx context.Context, accountID string, timeout time.Duration) (*AdvisoryLockInfo, error)
	ReleaseAdvisoryLock(ctx context.Context, lockKey string) error
	IsAdvisoryLockAcquired(ctx context.Context, lockKey string) (bool, error)

	// Transaction Processing
	ProcessDebitTransaction(ctx context.Context, req ProcessTransactionRequest) (*TransactionResult, error)
	ProcessDebitTransactionOptimistic(ctx context.Context, req ProcessTransactionRequest) (*TransactionResult, error) // NEW: Optimistic locking version
	ProcessCreditTransaction(ctx context.Context, req ProcessTransactionRequest) (*TransactionResult, error)
	ProcessCrossShardTransaction(ctx context.Context, req ProcessCrossShardTransactionRequest) (*CrossShardTransactionResult, error)

	// Balance Operations
	GetTotalBalance(ctx context.Context, accountID string) (decimal.Decimal, error)
	GetShardBalances(ctx context.Context, accountID string) ([]*ShardBalance, error)
	ValidateBalanceConsistency(ctx context.Context, accountID string) (*ConsistencyReport, error)

	// Rebalancing
	RebalanceShards(ctx context.Context, accountID string) error
	OptimizeShardDistribution(ctx context.Context, accountID string) error
}

// SubBalanceInfo contains information about sub balance configuration
type SubBalanceInfo struct {
	AccountID      string          `json:"account_id"`
	ShardCount     int             `json:"shard_count"`
	TotalBalance   decimal.Decimal `json:"total_balance"`
	UseSubBalance  bool            `json:"use_sub_balance"`
	Shards         []*ShardInfo    `json:"shards"`
	LastRebalanced *time.Time      `json:"last_rebalanced,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type ShardInfo struct {
	ShardID        string          `json:"shard_id"`
	ShardIndex     int             `json:"shard_index"`
	ShardHash      string          `json:"shard_hash"`
	TotalBalance   decimal.Decimal `json:"total_balance"`
	CreditAmount   decimal.Decimal `json:"credit_amount"`
	DebitAmount    decimal.Decimal `json:"debit_amount"`
	ReserveBalance decimal.Decimal `json:"reserve_balance"`
	Utilization    float64         `json:"utilization"` // Percentage of total balance
}

type ShardBalance struct {
	ShardID         string          `json:"shard_id"`
	ShardIndex      int             `json:"shard_index"`
	ShardHash       string          `json:"shard_hash"`
	TotalBalance    decimal.Decimal `json:"total_balance"`
	CreditAmount    decimal.Decimal `json:"credit_amount"`
	DebitAmount     decimal.Decimal `json:"debit_amount"`
	ReserveBalance  decimal.Decimal `json:"reserve_balance"`
	UnsettledAmount decimal.Decimal `json:"unsettled_amount"`
	LastUpdated     time.Time       `json:"last_updated"`
}

type ConsistencyReport struct {
	AccountID       string          `json:"account_id"`
	IsConsistent    bool            `json:"is_consistent"`
	TotalBalance    decimal.Decimal `json:"total_balance"`
	ShardBalances   []*ShardBalance `json:"shard_balances"`
	Inconsistencies []Inconsistency `json:"inconsistencies,omitempty"`
	ValidatedAt     time.Time       `json:"validated_at"`
}

type Inconsistency struct {
	ShardID    string          `json:"shard_id"`
	ShardIndex int             `json:"shard_index"`
	Issue      string          `json:"issue"`
	Expected   decimal.Decimal `json:"expected,omitempty"`
	Actual     decimal.Decimal `json:"actual,omitempty"`
	Severity   string          `json:"severity"`
}

// Request/Response DTOs
type ProcessTransactionRequest struct {
	TransactionID   string                 `json:"transaction_id" validate:"required"`
	AccountID       string                 `json:"account_id" validate:"required"`
	Amount          decimal.Decimal        `json:"amount" validate:"required,gt=0"`
	TransactionType TransactionType        `json:"transaction_type" validate:"required,oneof=debit credit"`
	Description     string                 `json:"description,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type ProcessCrossShardTransactionRequest struct {
	TransactionID string                 `json:"transaction_id" validate:"required"`
	AccountID     string                 `json:"account_id" validate:"required"`
	Amount        decimal.Decimal        `json:"amount" validate:"required,gt=0"`
	Description   string                 `json:"description,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type TransactionResult struct {
	TransactionID   string          `json:"transaction_id"`
	AccountID       string          `json:"account_id"`
	Amount          decimal.Decimal `json:"amount"`
	TransactionType TransactionType `json:"transaction_type"`
	SelectedShardID string          `json:"selected_shard_id"`
	ShardIndex      int             `json:"shard_index"`
	PreviousBalance decimal.Decimal `json:"previous_balance"`
	NewBalance      decimal.Decimal `json:"new_balance"`
	TotalBalance    decimal.Decimal `json:"total_balance"`
	ProcessedAt     time.Time       `json:"processed_at"`
	Status          string          `json:"status"`
}

type CrossShardTransactionResult struct {
	TransactionID   string                      `json:"transaction_id"`
	AccountID       string                      `json:"account_id"`
	TotalAmount     decimal.Decimal             `json:"total_amount"`
	ShardOperations []ShardOperationResult      `json:"shard_operations"`
	TotalBalance    decimal.Decimal             `json:"total_balance"`
	ProcessedAt     time.Time                   `json:"processed_at"`
	Status          CrossShardTransactionStatus `json:"status"`
	AdvisoryLockKey string                      `json:"advisory_lock_key"`
}

type ShardOperationResult struct {
	ShardID         string          `json:"shard_id"`
	ShardIndex      int             `json:"shard_index"`
	Amount          decimal.Decimal `json:"amount"`
	Operation       TransactionType `json:"operation"`
	PreviousBalance decimal.Decimal `json:"previous_balance"`
	NewBalance      decimal.Decimal `json:"new_balance"`
	Status          string          `json:"status"`
	ProcessedAt     time.Time       `json:"processed_at"`
}

type InitializeSubBalanceRequest struct {
	AccountID  string `json:"account_id" validate:"required"`
	ShardCount int    `json:"shard_count" validate:"required,min=1,max=10"`
}

type RebalanceRequest struct {
	AccountID string `json:"account_id" validate:"required"`
	Force     bool   `json:"force,omitempty"`
}
