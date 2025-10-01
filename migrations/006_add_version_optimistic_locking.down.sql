-- Remove version fields for optimistic locking
-- Migration: 006_add_version_optimistic_locking.down.sql

-- Drop indexes first
DROP INDEX IF EXISTS idx_transactions_retry_count;
DROP INDEX IF EXISTS idx_transactions_version;
DROP INDEX IF EXISTS idx_account_balance_shard_version;

-- Remove columns
ALTER TABLE transactions DROP COLUMN IF EXISTS retry_count;
ALTER TABLE transactions DROP COLUMN IF EXISTS version;
ALTER TABLE account_balance_shard DROP COLUMN IF EXISTS version;
