-- Rollback Ultra Performance Indexes
-- Migration: 005_ultra_performance_indexes.down.sql

DROP INDEX CONCURRENTLY IF EXISTS idx_account_balance_shard_ultra_performance;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_ultra_performance;
DROP INDEX CONCURRENTLY IF EXISTS idx_account_balance_shard_active_only;
DROP INDEX CONCURRENTLY IF EXISTS idx_account_balance_shard_balance_only;
DROP INDEX CONCURRENTLY IF EXISTS idx_account_balance_shard_selection;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_transaction_id_lookup;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_completed_only;
DROP INDEX CONCURRENTLY IF EXISTS idx_account_balance_shard_update_optimized;
DROP INDEX CONCURRENTLY IF EXISTS idx_accounts_validation;

