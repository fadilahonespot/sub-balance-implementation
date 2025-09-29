-- Shard 1: Account ID range 0-33%
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_no VARCHAR(50) UNIQUE NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    use_sub_balance BOOLEAN DEFAULT false,
    shard_count INTEGER DEFAULT 8,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Account balance shard table
CREATE TABLE IF NOT EXISTS account_balance_shard (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_index INTEGER NOT NULL,
    shard_hash VARCHAR(100) NOT NULL,
    credit_amount DECIMAL(20,2) DEFAULT 0,
    debit_amount DECIMAL(20,2) DEFAULT 0,
    total_balance DECIMAL(20,2) DEFAULT 0,
    reserve_balance DECIMAL(20,2) DEFAULT 0,
    unsettled_amount DECIMAL(20,2) DEFAULT 0,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    UNIQUE(parent_account_id, shard_index)
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id VARCHAR(100) UNIQUE NOT NULL,
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_id UUID REFERENCES account_balance_shard(id),
    shard_index INTEGER,
    transaction_type VARCHAR(20) NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    previous_balance DECIMAL(20,2),
    new_balance DECIMAL(20,2),
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    metadata JSONB
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_accounts_wallet_no ON accounts(wallet_no);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_parent ON account_balance_shard(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_index ON account_balance_shard(parent_account_id, shard_index);
CREATE INDEX IF NOT EXISTS idx_transactions_parent ON transactions(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_shard ON transactions(shard_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);

-- Insert shard metadata
INSERT INTO accounts (id, wallet_no, account_name, use_sub_balance, shard_count) 
VALUES ('00000000-0000-0000-0000-000000000001', 'shard_1_metadata', 'Shard 1 Metadata', false, 0)
ON CONFLICT (wallet_no) DO NOTHING;
