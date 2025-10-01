-- Add performance indexes for sub-balance system
-- Migration: 004_add_performance_indexes.up.sql

-- Index for account_balance_shard queries by parent_account_id and shard_index
-- This optimizes the slow query: SELECT * FROM account_balance_shard WHERE parent_account_id = ? ORDER BY shard_index ASC
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_parent_shard_index 
ON account_balance_shard(parent_account_id, shard_index);

-- Index for account_balance_shard FOR UPDATE queries
-- This optimizes the slow query: SELECT * FROM account_balance_shard WHERE id = ? FOR UPDATE
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_id_for_update 
ON account_balance_shard(id);

-- Index for total balance calculation
-- This optimizes: SELECT COALESCE(SUM(total_balance), 0) FROM account_balance_shard WHERE parent_account_id = ?
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_parent_total_balance 
ON account_balance_shard(parent_account_id, total_balance);

-- Index for transaction queries by parent_account_id and created_on
-- This optimizes transaction history queries
CREATE INDEX IF NOT EXISTS idx_transactions_parent_created 
ON transactions(parent_account_id, created_on);

-- Index for transaction queries by shard_id and created_on
-- This optimizes shard-specific transaction queries
CREATE INDEX IF NOT EXISTS idx_transactions_shard_created 
ON transactions(shard_id, created_on);

-- Index for transaction status queries
-- This optimizes status-based transaction filtering
CREATE INDEX IF NOT EXISTS idx_transactions_status_created 
ON transactions(status, created_on);

-- Composite index for account_balance_shard balance queries
-- This optimizes balance-related queries with multiple conditions
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_balance_composite 
ON account_balance_shard(parent_account_id, shard_index, total_balance, credit_amount, debit_amount);

-- Index for advisory lock optimization
-- This helps with lock-related queries
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_lock_optimization 
ON account_balance_shard(id, parent_account_id, shard_index);
