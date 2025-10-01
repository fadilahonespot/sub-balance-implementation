-- Add version field for optimistic locking
-- Migration: 006_add_version_optimistic_locking.up.sql

-- Add version column to account_balance_shard table for optimistic locking
ALTER TABLE account_balance_shard ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;

-- Create index for version-based queries
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_version 
ON account_balance_shard(parent_account_id, version);

-- Add version column to transactions table for audit trail
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;

-- Create index for transaction version queries
CREATE INDEX IF NOT EXISTS idx_transactions_version 
ON transactions(parent_account_id, version, created_on);

-- Add retry_count column to track optimistic locking retries
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0;

-- Create index for retry tracking
CREATE INDEX IF NOT EXISTS idx_transactions_retry_count 
ON transactions(retry_count, created_on);
