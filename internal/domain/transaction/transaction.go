package transaction

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Transaction represents a financial transaction
type Transaction struct {
	ID              string          `json:"id" gorm:"primaryKey;column:id"`
	TransactionID   string          `json:"transaction_id" gorm:"column:transaction_id;index"`
	ParentAccountID string          `json:"parent_account_id" gorm:"column:parent_account_id;index"`
	ShardID         string          `json:"shard_id" gorm:"column:shard_id;index"`
	ShardIndex      int             `json:"shard_index" gorm:"column:shard_index"`
	TransactionType string          `json:"transaction_type" gorm:"column:transaction_type"`
	Amount          decimal.Decimal `json:"amount" gorm:"column:amount;type:decimal(20,2)"`
	PreviousBalance decimal.Decimal `json:"previous_balance" gorm:"column:previous_balance;type:decimal(20,2)"`
	NewBalance      decimal.Decimal `json:"new_balance" gorm:"column:new_balance;type:decimal(20,2)"`
	Description     string          `json:"description" gorm:"column:description"`
	Status          string          `json:"status" gorm:"column:status"`
	CreatedOn       time.Time       `json:"created_on" gorm:"column:created_on;autoCreateTime"`
	ModifiedOn      time.Time       `json:"modified_on" gorm:"column:modified_on;autoUpdateTime"`
	Checksum        string          `json:"checksum" gorm:"column:checksum"`
	Metadata        string          `json:"metadata" gorm:"column:metadata;type:jsonb"`
}

// TableName overrides the default table name
func (Transaction) TableName() string {
	return "transactions"
}

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeDebit  TransactionType = "debit"
	TransactionTypeCredit TransactionType = "credit"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusCompleted  TransactionStatus = "completed"
	TransactionStatusFailed     TransactionStatus = "failed"
	TransactionStatusRolledBack TransactionStatus = "rolled_back"
)

// Repository interface for transaction operations
type Repository interface {
	Create(ctx context.Context, transaction *Transaction) error
	GetByID(ctx context.Context, id string) (*Transaction, error)
	GetByTransactionID(ctx context.Context, transactionID string) ([]*Transaction, error)
	GetByParentAccountID(ctx context.Context, parentAccountID string, limit, offset int) ([]*Transaction, error)
	GetByShardID(ctx context.Context, shardID string, limit, offset int) ([]*Transaction, error)
	Update(ctx context.Context, transaction *Transaction) error
	Delete(ctx context.Context, id string) error
	GetTransactionHistory(ctx context.Context, parentAccountID string, fromDate, toDate time.Time, limit, offset int) ([]*Transaction, error)
	GetTransactionSummary(ctx context.Context, parentAccountID string, fromDate, toDate time.Time) (*TransactionSummary, error)
	GetTransactionStats(ctx context.Context) (map[string]interface{}, error)
	GetTransactionsByStatus(ctx context.Context, status TransactionStatus, limit, offset int) ([]*Transaction, error)
	GetTransactionsByType(ctx context.Context, transactionType TransactionType, limit, offset int) ([]*Transaction, error)
}

// Usecase interface for transaction business logic
type Usecase interface {
	CreateTransaction(ctx context.Context, req CreateTransactionRequest) (*Transaction, error)
	GetTransaction(ctx context.Context, id string) (*Transaction, error)
	GetTransactionHistory(ctx context.Context, accountID string, req GetTransactionHistoryRequest) ([]*Transaction, error)
	GetTransactionSummary(ctx context.Context, accountID string, req GetTransactionSummaryRequest) (*TransactionSummary, error)
	UpdateTransactionStatus(ctx context.Context, id string, status TransactionStatus) error
	ProcessTransactionWithBP(ctx context.Context, req ProcessTransactionWithBPRequest) (*ProcessTransactionWithBPResponse, error)
}

// TransactionSummary contains summary information about transactions
type TransactionSummary struct {
	ParentAccountID     string          `json:"parent_account_id"`
	TotalTransactions   int64           `json:"total_transactions"`
	TotalDebitAmount    decimal.Decimal `json:"total_debit_amount"`
	TotalCreditAmount   decimal.Decimal `json:"total_credit_amount"`
	NetAmount           decimal.Decimal `json:"net_amount"`
	AverageAmount       decimal.Decimal `json:"average_amount"`
	LargestTransaction  decimal.Decimal `json:"largest_transaction"`
	SmallestTransaction decimal.Decimal `json:"smallest_transaction"`
	Period              string          `json:"period"`
	FromDate            time.Time       `json:"from_date"`
	ToDate              time.Time       `json:"to_date"`
}

// Request/Response DTOs
type CreateTransactionRequest struct {
	TransactionID   string                 `json:"transaction_id" validate:"required"`
	ParentAccountID string                 `json:"parent_account_id" validate:"required"`
	ShardID         string                 `json:"shard_id" validate:"required"`
	ShardIndex      int                    `json:"shard_index" validate:"required"`
	TransactionType TransactionType        `json:"transaction_type" validate:"required,oneof=debit credit"`
	Amount          decimal.Decimal        `json:"amount" validate:"required,gt=0"`
	PreviousBalance decimal.Decimal        `json:"previous_balance" validate:"required"`
	NewBalance      decimal.Decimal        `json:"new_balance" validate:"required"`
	Description     string                 `json:"description,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type GetTransactionHistoryRequest struct {
	FromDate *time.Time `json:"from_date,omitempty"`
	ToDate   *time.Time `json:"to_date,omitempty"`
	Limit    int        `json:"limit,omitempty"`
	Offset   int        `json:"offset,omitempty"`
}

type GetTransactionSummaryRequest struct {
	FromDate *time.Time `json:"from_date,omitempty"`
	ToDate   *time.Time `json:"to_date,omitempty"`
}

type ProcessTransactionWithBPRequest struct {
	TransactionID   string                 `json:"transaction_id" validate:"required"`
	AccountID       string                 `json:"account_id" validate:"required"`
	Amount          decimal.Decimal        `json:"amount" validate:"required"`
	TransactionType TransactionType        `json:"transaction_type" validate:"required,oneof=debit credit"`
	Description     string                 `json:"description,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type ProcessTransactionWithBPResponse struct {
	TransactionID   string            `json:"transaction_id"`
	AccountID       string            `json:"account_id"`
	Amount          decimal.Decimal   `json:"amount"`
	TransactionType TransactionType   `json:"transaction_type"`
	SelectedShardID string            `json:"selected_shard_id"`
	ShardIndex      int               `json:"shard_index"`
	PreviousBalance decimal.Decimal   `json:"previous_balance"`
	NewBalance      decimal.Decimal   `json:"new_balance"`
	TotalBalance    decimal.Decimal   `json:"total_balance"`
	ProcessedAt     time.Time         `json:"processed_at"`
	Status          TransactionStatus `json:"status"`
	UseSubBalance   bool              `json:"use_sub_balance"`
	ShardCount      int               `json:"shard_count"`
}

type TransactionResponse struct {
	ID              string                 `json:"id"`
	TransactionID   string                 `json:"transaction_id"`
	ParentAccountID string                 `json:"parent_account_id"`
	ShardID         string                 `json:"shard_id"`
	ShardIndex      int                    `json:"shard_index"`
	TransactionType string                 `json:"transaction_type"`
	Amount          decimal.Decimal        `json:"amount"`
	PreviousBalance decimal.Decimal        `json:"previous_balance"`
	NewBalance      decimal.Decimal        `json:"new_balance"`
	Description     string                 `json:"description"`
	Status          string                 `json:"status"`
	CreatedOn       time.Time              `json:"created_on"`
	ModifiedOn      time.Time              `json:"modified_on"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}
