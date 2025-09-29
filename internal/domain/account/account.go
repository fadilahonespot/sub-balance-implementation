package account

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Account represents the main account entity
type Account struct {
	ID                   string          `json:"id" gorm:"primaryKey;column:id"`
	CreatedBy            string          `json:"created_by" gorm:"column:created_by"`
	CreatedOn            time.Time       `json:"created_on" gorm:"column:created_on"`
	ModifiedBy           string          `json:"modified_by" gorm:"column:modified_by"`
	ModifiedOn           time.Time       `json:"modified_on" gorm:"column:modified_on"`
	WalletNo             string          `json:"wallet_no" gorm:"column:wallet_no"`
	AccountName          string          `json:"account_name" gorm:"column:account_name"`
	WalletTypeId         string          `json:"wallet_type_id" gorm:"column:wallet_type_id"`
	WalletTypeName       string          `json:"wallet_type_name" gorm:"column:wallet_type_name"`
	InstanceType         string          `json:"instance_type" gorm:"column:instance_type"`
	WalletStatus         string          `json:"wallet_status" gorm:"column:wallet_status"`
	CurrencyId           string          `json:"currency_id" gorm:"column:currency_id"`
	OwnerId              string          `json:"owner_id" gorm:"column:owner_id"`
	MinimumBalance       decimal.Decimal `json:"minimum_balance" gorm:"column:minimum_balance;type:decimal(20,2)"`
	UpperLimit           decimal.Decimal `json:"upper_limit" gorm:"column:upper_limit;type:decimal(20,2)"`
	LowerLimit           decimal.Decimal `json:"lower_limit" gorm:"column:lower_limit;type:decimal(20,2)"`
	Active               bool            `json:"active" gorm:"column:active"`
	HotAccount           bool            `json:"hot_account" gorm:"column:hot_account"`
	DebitHotAccount      bool            `json:"debit_hot_account" gorm:"column:debit_hot_account"`
	UseSubBalance        bool            `json:"use_sub_balance" gorm:"column:use_sub_balance"`
	SubBalanceShardCount int             `json:"sub_balance_shard_count" gorm:"column:sub_balance_shard_count"`
	Checksum             string          `json:"checksum" gorm:"column:checksum"`
}

// TableName overrides the default table name
func (Account) TableName() string {
	return "accounts"
}

// Repository interface for account operations
type Repository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id string) (*Account, error)
	GetByWalletNo(ctx context.Context, walletNo string) (*Account, error)
	Update(ctx context.Context, account *Account) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*Account, error)
	GetHotAccounts(ctx context.Context) ([]*Account, error)
	GetAccountsWithSubBalance(ctx context.Context) ([]*Account, error)
	UpdateSubBalanceConfig(ctx context.Context, accountID string, useSubBalance bool, shardCount int) error
	GetAccountStats(ctx context.Context) (map[string]interface{}, error)
	SearchAccounts(ctx context.Context, criteria SearchCriteria) ([]*Account, error)
}

// Usecase interface for account business logic
type Usecase interface {
	CreateAccount(ctx context.Context, req CreateAccountRequest) (*Account, error)
	GetAccount(ctx context.Context, id string) (*Account, error)
	UpdateAccount(ctx context.Context, id string, req UpdateAccountRequest) (*Account, error)
	DeleteAccount(ctx context.Context, id string) error
	ListAccounts(ctx context.Context, limit, offset int) ([]*Account, error)
	MigrateToSubBalance(ctx context.Context, accountID string, shardCount int) error
	GetAccountBalance(ctx context.Context, accountID string) (*AccountBalanceResponse, error)
}

// Request/Response DTOs
type CreateAccountRequest struct {
	WalletNo       string          `json:"wallet_no" validate:"required"`
	WalletTypeId   string          `json:"wallet_type_id" validate:"required"`
	InstanceType   string          `json:"instance_type" validate:"required"`
	CurrencyId     string          `json:"currency_id" validate:"required"`
	OwnerId        string          `json:"owner_id" validate:"required"`
	MinimumBalance decimal.Decimal `json:"minimum_balance"`
	UpperLimit     decimal.Decimal `json:"upper_limit"`
	LowerLimit     decimal.Decimal `json:"lower_limit"`
	UseSubBalance  bool            `json:"use_sub_balance"`
	ShardCount     int             `json:"shard_count"`
}

type UpdateAccountRequest struct {
	WalletTypeName  *string          `json:"wallet_type_name,omitempty"`
	InstanceType    *string          `json:"instance_type,omitempty"`
	WalletStatus    *string          `json:"wallet_status,omitempty"`
	MinimumBalance  *decimal.Decimal `json:"minimum_balance,omitempty"`
	UpperLimit      *decimal.Decimal `json:"upper_limit,omitempty"`
	LowerLimit      *decimal.Decimal `json:"lower_limit,omitempty"`
	Active          *bool            `json:"active,omitempty"`
	HotAccount      *bool            `json:"hot_account,omitempty"`
	DebitHotAccount *bool            `json:"debit_hot_account,omitempty"`
	UseSubBalance   *bool            `json:"use_sub_balance,omitempty"`
	ShardCount      *int             `json:"shard_count,omitempty"`
}

type AccountBalanceResponse struct {
	AccountID       string          `json:"account_id"`
	TotalBalance    decimal.Decimal `json:"total_balance"`
	CreditAmount    decimal.Decimal `json:"credit_amount"`
	DebitAmount     decimal.Decimal `json:"debit_amount"`
	ReserveBalance  decimal.Decimal `json:"reserve_balance"`
	UnsettledAmount decimal.Decimal `json:"unsettled_amount"`
	Shards          []ShardBalance  `json:"shards,omitempty"`
	UseSubBalance   bool            `json:"use_sub_balance"`
	ShardCount      int             `json:"shard_count"`
}

type ShardBalance struct {
	ShardID        string          `json:"shard_id"`
	ShardIndex     int             `json:"shard_index"`
	TotalBalance   decimal.Decimal `json:"total_balance"`
	CreditAmount   decimal.Decimal `json:"credit_amount"`
	DebitAmount    decimal.Decimal `json:"debit_amount"`
	ReserveBalance decimal.Decimal `json:"reserve_balance"`
}

type SearchCriteria struct {
	WalletNo      string `json:"wallet_no,omitempty"`
	OwnerID       string `json:"owner_id,omitempty"`
	WalletTypeID  string `json:"wallet_type_id,omitempty"`
	InstanceType  string `json:"instance_type,omitempty"`
	Active        *bool  `json:"active,omitempty"`
	HotAccount    *bool  `json:"hot_account,omitempty"`
	UseSubBalance *bool  `json:"use_sub_balance,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Offset        int    `json:"offset,omitempty"`
}
