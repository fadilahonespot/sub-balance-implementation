-- PostgreSQL Performance Optimization for High TPS
-- Run these commands to optimize PostgreSQL for sub-balance system

-- 1. Increase shared_buffers (25% of RAM, adjust based on your system)
ALTER SYSTEM SET shared_buffers = '2GB';

-- 2. Increase effective_cache_size (75% of RAM)
ALTER SYSTEM SET effective_cache_size = '6GB';

-- 3. Optimize work_mem for sorting and hashing
ALTER SYSTEM SET work_mem = '64MB';

-- 4. Increase maintenance_work_mem for index operations
ALTER SYSTEM SET maintenance_work_mem = '512MB';

-- 5. Optimize checkpoint settings for high write throughput
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '64MB';
ALTER SYSTEM SET checkpoint_timeout = '15min';
ALTER SYSTEM SET max_wal_size = '4GB';
ALTER SYSTEM SET min_wal_size = '1GB';

-- 6. Optimize connection settings
ALTER SYSTEM SET max_connections = 500;
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';

-- 7. Optimize query planner
ALTER SYSTEM SET random_page_cost = 1.1;
ALTER SYSTEM SET effective_io_concurrency = 200;
ALTER SYSTEM SET seq_page_cost = 1.0;

-- 8. Optimize for high concurrency
ALTER SYSTEM SET max_worker_processes = 8;
ALTER SYSTEM SET max_parallel_workers_per_gather = 4;
ALTER SYSTEM SET max_parallel_workers = 8;
ALTER SYSTEM SET max_parallel_maintenance_workers = 4;

-- 9. Optimize logging for performance (reduce logging overhead)
ALTER SYSTEM SET log_min_duration_statement = 1000; -- Only log queries > 1 second
ALTER SYSTEM SET log_checkpoints = off;
ALTER SYSTEM SET log_connections = off;
ALTER SYSTEM SET log_disconnections = off;
ALTER SYSTEM SET log_lock_waits = on;

-- 10. Optimize autovacuum for high TPS
ALTER SYSTEM SET autovacuum_max_workers = 6;
ALTER SYSTEM SET autovacuum_naptime = '10s';
ALTER SYSTEM SET autovacuum_vacuum_threshold = 50;
ALTER SYSTEM SET autovacuum_analyze_threshold = 50;
ALTER SYSTEM SET autovacuum_vacuum_scale_factor = 0.1;
ALTER SYSTEM SET autovacuum_analyze_scale_factor = 0.05;

-- 11. Optimize lock settings
ALTER SYSTEM SET deadlock_timeout = '1s';
ALTER SYSTEM SET lock_timeout = '5s';

-- 12. Enable query statistics
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- 13. Create optimized indexes (run after migration)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_balance_shard_ultra_performance 
ON account_balance_shard(parent_account_id, shard_index, id) 
INCLUDE (total_balance, credit_amount, debit_amount, modified_on);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_ultra_performance 
ON transactions(parent_account_id, shard_id, created_on) 
INCLUDE (transaction_id, transaction_type, amount, status);

-- 14. Update statistics
ANALYZE account_balance_shard;
ANALYZE transactions;
ANALYZE accounts;

-- 15. Reload configuration
SELECT pg_reload_conf();

-- Show current settings
SHOW shared_buffers;
SHOW effective_cache_size;
SHOW work_mem;
SHOW max_connections;
SHOW max_worker_processes;

