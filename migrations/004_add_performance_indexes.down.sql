-- Remove performance indexes for sub-balance system
-- Migration: 004_add_performance_indexes.down.sql

-- Drop all performance indexes
DROP INDEX IF EXISTS idx_account_balance_shard_parent_shard_index;
DROP INDEX IF EXISTS idx_account_balance_shard_id_for_update;
DROP INDEX IF EXISTS idx_account_balance_shard_parent_total_balance;
DROP INDEX IF EXISTS idx_transactions_parent_created;
DROP INDEX IF EXISTS idx_transactions_shard_created;
DROP INDEX IF EXISTS idx_transactions_status_created;
DROP INDEX IF EXISTS idx_account_balance_shard_balance_composite;
DROP INDEX IF EXISTS idx_account_balance_shard_lock_optimization;
