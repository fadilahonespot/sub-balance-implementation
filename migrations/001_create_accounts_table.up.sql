-- Create accounts table
CREATE TABLE IF NOT EXISTS accounts (
    id VARCHAR(255) PRIMARY KEY,
    created_by VARCHAR(255) NOT NULL,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_by VARCHAR(255) NOT NULL,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    wallet_no VARCHAR(255) UNIQUE NOT NULL,
    wallet_type_id VARCHAR(255) NOT NULL,
    wallet_type_name VARCHAR(255),
    instance_type VARCHAR(255) NOT NULL,
    wallet_status VARCHAR(50) DEFAULT 'active',
    currency_id VARCHAR(255) NOT NULL,
    owner_id VARCHAR(255) NOT NULL,
    minimum_balance DECIMAL(20,2) DEFAULT 0,
    upper_limit DECIMAL(20,2) DEFAULT 0,
    lower_limit DECIMAL(20,2) DEFAULT 0,
    active BOOLEAN DEFAULT true,
    hot_account BOOLEAN DEFAULT false,
    debit_hot_account BOOLEAN DEFAULT false,
    use_sub_balance BOOLEAN DEFAULT false,
    sub_balance_shard_count INTEGER DEFAULT 3,
    checksum VARCHAR(255)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_accounts_wallet_no ON accounts(wallet_no);
CREATE INDEX IF NOT EXISTS idx_accounts_owner_id ON accounts(owner_id);
CREATE INDEX IF NOT EXISTS idx_accounts_hot_account ON accounts(hot_account);
CREATE INDEX IF NOT EXISTS idx_accounts_use_sub_balance ON accounts(use_sub_balance);
CREATE INDEX IF NOT EXISTS idx_accounts_created_on ON accounts(created_on);

-- Add comments
COMMENT ON TABLE accounts IS 'Main accounts table for storing account information';
COMMENT ON COLUMN accounts.hot_account IS 'Indicates if this is a high-traffic account';
COMMENT ON COLUMN accounts.use_sub_balance IS 'Indicates if this account uses sub-balance sharding';
COMMENT ON COLUMN accounts.sub_balance_shard_count IS 'Number of shards for sub-balance (default: 3)';
