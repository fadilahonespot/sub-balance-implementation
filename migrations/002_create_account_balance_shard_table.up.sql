-- Create account_balance_shard table
CREATE TABLE IF NOT EXISTS account_balance_shard (
    id VARCHAR(255) PRIMARY KEY,
    parent_account_id VARCHAR(255) NOT NULL,
    shard_index INTEGER NOT NULL,
    shard_hash VARCHAR(32) NOT NULL,
    credit_amount DECIMAL(20,2) DEFAULT 0,
    debit_amount DECIMAL(20,2) DEFAULT 0,
    total_balance DECIMAL(20,2) DEFAULT 0,
    reserve_balance DECIMAL(20,2) DEFAULT 0,
    unsettled_amount DECIMAL(20,2) DEFAULT 0,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_parent_account_id ON account_balance_shard(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_shard_hash ON account_balance_shard(shard_hash);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_shard_index ON account_balance_shard(parent_account_id, shard_index);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_total_balance ON account_balance_shard(parent_account_id, total_balance);

-- Create unique constraint for parent_account_id + shard_index
CREATE UNIQUE INDEX IF NOT EXISTS idx_account_balance_shard_unique ON account_balance_shard(parent_account_id, shard_index);

-- Add foreign key constraint
ALTER TABLE account_balance_shard 
ADD CONSTRAINT fk_account_balance_shard_parent_account 
FOREIGN KEY (parent_account_id) REFERENCES accounts(id) ON DELETE CASCADE;

-- Add comments
COMMENT ON TABLE account_balance_shard IS 'Shard table for account balances to reduce lock contention';
COMMENT ON COLUMN account_balance_shard.parent_account_id IS 'Reference to parent account';
COMMENT ON COLUMN account_balance_shard.shard_index IS 'Index of the shard (0-based)';
COMMENT ON COLUMN account_balance_shard.shard_hash IS 'Hash value for consistent shard routing';
COMMENT ON COLUMN account_balance_shard.total_balance IS 'Calculated as credit_amount - debit_amount';
COMMENT ON COLUMN account_balance_shard.reserve_balance IS 'Reserved amount for pending transactions';
COMMENT ON COLUMN account_balance_shard.unsettled_amount IS 'Amount pending settlement';
