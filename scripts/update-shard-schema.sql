-- Update database shard schema to match application expectations
-- This script adds missing columns to the accounts table

-- Add missing columns to accounts table
ALTER TABLE accounts 
ADD COLUMN IF NOT EXISTS created_by VARCHAR(255) DEFAULT 'system',
ADD COLUMN IF NOT EXISTS modified_by VARCHAR(255) DEFAULT 'system',
ADD COLUMN IF NOT EXISTS wallet_type_id VARCHAR(255) DEFAULT '1',
ADD COLUMN IF NOT EXISTS wallet_type_name VARCHAR(255) DEFAULT '',
ADD COLUMN IF NOT EXISTS instance_type VARCHAR(255) DEFAULT 'individual',
ADD COLUMN IF NOT EXISTS wallet_status VARCHAR(50) DEFAULT 'active',
ADD COLUMN IF NOT EXISTS currency_id VARCHAR(255) DEFAULT '1',
ADD COLUMN IF NOT EXISTS owner_id VARCHAR(255) DEFAULT 'system',
ADD COLUMN IF NOT EXISTS minimum_balance DECIMAL(20,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS upper_limit DECIMAL(20,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS lower_limit DECIMAL(20,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS active BOOLEAN DEFAULT true,
ADD COLUMN IF NOT EXISTS hot_account BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS debit_hot_account BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS checksum VARCHAR(255) DEFAULT '';

-- Rename shard_count to sub_balance_shard_count to match application
ALTER TABLE accounts RENAME COLUMN shard_count TO sub_balance_shard_count;

-- Add missing indexes
CREATE INDEX IF NOT EXISTS idx_accounts_owner_id ON accounts(owner_id);
CREATE INDEX IF NOT EXISTS idx_accounts_hot_account ON accounts(hot_account);
CREATE INDEX IF NOT EXISTS idx_accounts_use_sub_balance ON accounts(use_sub_balance);
CREATE INDEX IF NOT EXISTS idx_accounts_created_on ON accounts(created_on);

-- Add comments
COMMENT ON TABLE accounts IS 'Main accounts table for storing account information';
COMMENT ON COLUMN accounts.hot_account IS 'Indicates if this is a high-traffic account';
COMMENT ON COLUMN accounts.use_sub_balance IS 'Indicates if this account uses sub-balance sharding';
COMMENT ON COLUMN accounts.sub_balance_shard_count IS 'Number of shards for sub-balance (default: 3)';
