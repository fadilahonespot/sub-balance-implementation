-- Ultra Performance Indexes for High TPS
-- Migration: 005_ultra_performance_indexes.up.sql

-- Ultra-optimized index for account_balance_shard updates (covers most common queries)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_balance_shard_ultra_performance 
ON account_balance_shard(parent_account_id, shard_index, id) 
INCLUDE (total_balance, credit_amount, debit_amount, modified_on);

-- Ultra-optimized index for transaction inserts (covers all insert fields)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_ultra_performance 
ON transactions(parent_account_id, shard_id, created_on) 
INCLUDE (transaction_id, transaction_type, amount, status);

-- Partial index for active shards only (faster updates)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_balance_shard_active_only 
ON account_balance_shard(parent_account_id, shard_index) 
WHERE total_balance > 0 OR credit_amount > 0 OR debit_amount > 0;

-- Index for balance calculations (optimized for SUM queries)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_balance_shard_balance_only 
ON account_balance_shard(parent_account_id) 
INCLUDE (total_balance);

-- Optimized index for shard selection queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_balance_shard_selection 
ON account_balance_shard(parent_account_id, total_balance DESC, shard_index) 
INCLUDE (id, shard_hash);

-- Index for transaction lookups by transaction_id (faster than primary key for some queries)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_transaction_id_lookup 
ON transactions(transaction_id, created_on) 
INCLUDE (id, parent_account_id, shard_id, status);

-- Partial index for completed transactions only
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_completed_only 
ON transactions(parent_account_id, created_on DESC) 
WHERE status = 'completed';

-- Ultra-fast index for shard balance updates
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_balance_shard_update_optimized 
ON account_balance_shard(id) 
INCLUDE (parent_account_id, total_balance, modified_on);

-- Index for account validation queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_validation 
ON accounts(id, status) 
INCLUDE (name, email);

-- Statistics update for query planner optimization
ANALYZE account_balance_shard;
ANALYZE transactions;
ANALYZE accounts;

