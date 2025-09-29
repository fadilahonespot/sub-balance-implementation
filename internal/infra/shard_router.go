package infra

import (
	"database/sql"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ShardRouter handles database sharding logic
type ShardRouter struct {
	shards     map[int]*sql.DB
	gormShards map[int]*gorm.DB
	shardCount int
}

// NewShardRouter creates a new shard router
func NewShardRouter(shardConfigs map[string]string) (*ShardRouter, error) {
	router := &ShardRouter{
		shards:     make(map[int]*sql.DB),
		gormShards: make(map[int]*gorm.DB),
		shardCount: len(shardConfigs),
	}

	// Initialize database connections for each shard
	for shardID, dsn := range shardConfigs {
		id, err := strconv.Atoi(shardID)
		if err != nil {
			return nil, fmt.Errorf("invalid shard ID: %s", shardID)
		}

		// Create SQL connection
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to shard %d: %w", id, err)
		}

		// Test connection
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping shard %d: %w", id, err)
		}

		// Create GORM connection
		gormDB, err := gorm.Open(postgres.New(postgres.Config{
			Conn: db,
		}), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create GORM connection for shard %d: %w", id, err)
		}

		// Configure GORM connection pool
		sqlDB, err := gormDB.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get underlying DB for shard %d: %w", id, err)
		}
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)

		router.shards[id] = db
		router.gormShards[id] = gormDB
	}

	return router, nil
}

// GetShardForAccount determines which shard to use for an account
func (sr *ShardRouter) GetShardForAccount(accountID string) (int, error) {
	if sr.shardCount == 0 {
		return 0, fmt.Errorf("no shards available")
	}

	// Use consistent hashing for shard selection
	hash := crc32.ChecksumIEEE([]byte(accountID))
	shardID := int(hash) % sr.shardCount

	return shardID, nil
}

// GetShardDB returns the database connection for a specific shard
func (sr *ShardRouter) GetShardDB(shardID int) (*sql.DB, error) {
	db, exists := sr.shards[shardID]
	if !exists {
		return nil, fmt.Errorf("shard %d not found", shardID)
	}
	return db, nil
}

// GetShardDBForAccount returns the database connection for an account
func (sr *ShardRouter) GetShardDBForAccount(accountID string) (*sql.DB, int, error) {
	shardID, err := sr.GetShardForAccount(accountID)
	if err != nil {
		return nil, 0, err
	}

	db, err := sr.GetShardDB(shardID)
	if err != nil {
		return nil, 0, err
	}

	return db, shardID, nil
}

// GetAllShards returns all shard databases
func (sr *ShardRouter) GetAllShards() map[int]*sql.DB {
	return sr.shards
}

// GetShardCount returns the number of shards
func (sr *ShardRouter) GetShardCount() int {
	return sr.shardCount
}

// GetGormShardConnection returns the GORM database connection for a given account ID
func (sr *ShardRouter) GetGormShardConnection(accountID string) (*gorm.DB, int, error) {
	shardID, err := sr.GetShardForAccount(accountID)
	if err != nil {
		return nil, 0, err
	}

	db, ok := sr.gormShards[shardID]
	if !ok {
		return nil, 0, fmt.Errorf("GORM shard %d not found for account %s", shardID, accountID)
	}

	return db, shardID, nil
}

// Close closes all database connections
func (sr *ShardRouter) Close() error {
	var errors []string

	for shardID, db := range sr.shards {
		if err := db.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("shard %d: %v", shardID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing shards: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ShardInfo contains information about a shard
type ShardInfo struct {
	ID           int
	DB           *sql.DB
	AccountRange string
}

// GetShardInfo returns information about all shards
func (sr *ShardRouter) GetShardInfo() []ShardInfo {
	var info []ShardInfo

	for shardID, db := range sr.shards {
		// Calculate account range for this shard
		rangeStart := (shardID * 100) / sr.shardCount
		rangeEnd := ((shardID + 1) * 100) / sr.shardCount
		accountRange := fmt.Sprintf("%d%%-%d%%", rangeStart, rangeEnd)

		info = append(info, ShardInfo{
			ID:           shardID,
			DB:           db,
			AccountRange: accountRange,
		})
	}

	return info
}

// HealthCheck checks the health of all shards
func (sr *ShardRouter) HealthCheck() map[int]error {
	results := make(map[int]error)

	for shardID, db := range sr.shards {
		if err := db.Ping(); err != nil {
			results[shardID] = err
		} else {
			results[shardID] = nil
		}
	}

	return results
}

// GetShardStats returns statistics for all shards
func (sr *ShardRouter) GetShardStats() (map[int]ShardStats, error) {
	stats := make(map[int]ShardStats)

	for shardID, db := range sr.shards {
		stat, err := sr.getShardStat(db, shardID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stats for shard %d: %w", shardID, err)
		}
		stats[shardID] = stat
	}

	return stats, nil
}

// ShardStats contains statistics for a shard
type ShardStats struct {
	ShardID           int
	TotalAccounts     int
	TotalShards       int
	TotalBalance      int64
	ActiveConnections int
	DatabaseSize      string
}

func (sr *ShardRouter) getShardStat(db *sql.DB, shardID int) (ShardStats, error) {
	var stat ShardStats
	stat.ShardID = shardID

	// Get total accounts
	err := db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&stat.TotalAccounts)
	if err != nil {
		return stat, err
	}

	// Get total shards
	err = db.QueryRow("SELECT COUNT(*) FROM account_balance_shard").Scan(&stat.TotalShards)
	if err != nil {
		return stat, err
	}

	// Get total balance
	err = db.QueryRow("SELECT COALESCE(SUM(total_balance), 0) FROM account_balance_shard").Scan(&stat.TotalBalance)
	if err != nil {
		return stat, err
	}

	// Get active connections
	err = db.QueryRow("SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&stat.ActiveConnections)
	if err != nil {
		return stat, err
	}

	// Get database size
	var size string
	err = db.QueryRow("SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&size)
	if err != nil {
		return stat, err
	}
	stat.DatabaseSize = size

	return stat, nil
}
