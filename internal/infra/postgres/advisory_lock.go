package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AdvisoryLockManager manages PostgreSQL advisory locks
type AdvisoryLockManager struct {
	db     *gorm.DB
	logger *zap.Logger
	config AdvisoryLockConfig
}

// AdvisoryLockConfig contains configuration for advisory locks
type AdvisoryLockConfig struct {
	DefaultTimeout time.Duration `json:"default_timeout"`
	MaxRetries     int           `json:"max_retries"`
	RetryDelay     time.Duration `json:"retry_delay"`
	FastTimeout    time.Duration `json:"fast_timeout"`
	SlowTimeout    time.Duration `json:"slow_timeout"`
	EnableRetry    bool          `json:"enable_retry"`
	EnableMetrics  bool          `json:"enable_metrics"`
}

// DefaultAdvisoryLockConfig returns default configuration
func DefaultAdvisoryLockConfig() AdvisoryLockConfig {
	return AdvisoryLockConfig{
		DefaultTimeout: 5 * time.Second, // Reduced from 15s
		MaxRetries:     3,               // Allow 3 retries
		RetryDelay:     50 * time.Millisecond,
		FastTimeout:    2 * time.Second,  // For simple operations
		SlowTimeout:    10 * time.Second, // For complex operations
		EnableRetry:    true,
		EnableMetrics:  true,
	}
}

// NewAdvisoryLockManager creates a new advisory lock manager
func NewAdvisoryLockManager(db *gorm.DB, logger *zap.Logger) *AdvisoryLockManager {
	return &AdvisoryLockManager{
		db:     db,
		logger: logger,
		config: DefaultAdvisoryLockConfig(),
	}
}

// NewAdvisoryLockManagerWithConfig creates a new advisory lock manager with custom config
func NewAdvisoryLockManagerWithConfig(db *gorm.DB, logger *zap.Logger, config AdvisoryLockConfig) *AdvisoryLockManager {
	return &AdvisoryLockManager{
		db:     db,
		logger: logger,
		config: config,
	}
}

// AdvisoryLockInfo contains information about an advisory lock
type AdvisoryLockInfo struct {
	LockKey    string        `json:"lock_key"`
	AccountID  string        `json:"account_id"`
	LockType   string        `json:"lock_type"`
	AcquiredAt time.Time     `json:"acquired_at"`
	Timeout    time.Duration `json:"timeout"`
	IsAcquired bool          `json:"is_acquired"`
}

// LockType represents the type of advisory lock
type LockType string

const (
	LockTypeAccount    LockType = "account"
	LockTypeShard      LockType = "shard"
	LockTypeCrossShard LockType = "cross_shard"
)

// AcquireAdvisoryLock acquires a PostgreSQL advisory lock with optimized settings
func (alm *AdvisoryLockManager) AcquireAdvisoryLock(ctx context.Context, accountID string, lockType LockType, timeout time.Duration) (*AdvisoryLockInfo, error) {
	// Use adaptive timeout if not specified
	if timeout == 0 {
		timeout = alm.getAdaptiveTimeout(lockType)
	}

	lockKey := alm.generateLockKey(accountID, lockType, "")
	lockID := alm.generateLockID(accountID, lockType, "")

	alm.logger.Info("Attempting to acquire advisory lock",
		zap.String("lock_key", lockKey),
		zap.Int64("lock_id", lockID),
		zap.String("account_id", accountID),
		zap.String("lock_type", string(lockType)),
		zap.Duration("timeout", timeout),
	)

	// Try to acquire lock with retry mechanism
	return alm.acquireLockWithRetry(ctx, lockKey, lockID, accountID, lockType, timeout)
}

// acquireLockWithRetry attempts to acquire lock with retry mechanism
func (alm *AdvisoryLockManager) acquireLockWithRetry(ctx context.Context, lockKey string, lockID int64, accountID string, lockType LockType, timeout time.Duration) (*AdvisoryLockInfo, error) {
	var lastErr error
	startTime := time.Now()

	for attempt := 0; attempt <= alm.config.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Try to acquire lock
		acquired, err := alm.tryAcquireLock(ctx, lockID)
		if err != nil {
			lastErr = err
			alm.logger.Warn("Failed to acquire advisory lock",
				zap.String("lock_key", lockKey),
				zap.Int64("lock_id", lockID),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)

			if attempt < alm.config.MaxRetries && alm.config.EnableRetry {
				time.Sleep(alm.config.RetryDelay)
				continue
			}
			return nil, fmt.Errorf("failed to acquire advisory lock after %d attempts: %w", attempt+1, err)
		}

		if acquired {
			duration := time.Since(startTime)
			alm.logger.Info("Advisory lock acquired successfully",
				zap.String("lock_key", lockKey),
				zap.Int64("lock_id", lockID),
				zap.String("account_id", accountID),
				zap.Int("attempts", attempt+1),
				zap.Duration("duration", duration),
			)

			// Record metrics if enabled
			if alm.config.EnableMetrics {
				alm.recordLockMetrics(lockType, duration, attempt+1)
			}

			return &AdvisoryLockInfo{
				LockKey:    lockKey,
				AccountID:  accountID,
				LockType:   string(lockType),
				AcquiredAt: time.Now(),
				Timeout:    timeout,
				IsAcquired: true,
			}, nil
		}

		// Lock not acquired, retry if enabled
		if attempt < alm.config.MaxRetries && alm.config.EnableRetry {
			alm.logger.Debug("Advisory lock not available, retrying",
				zap.String("lock_key", lockKey),
				zap.Int("attempt", attempt+1),
				zap.Duration("retry_delay", alm.config.RetryDelay),
			)
			time.Sleep(alm.config.RetryDelay)
			continue
		}

		// No more retries
		alm.logger.Warn("Advisory lock not acquired - already held by another transaction",
			zap.String("lock_key", lockKey),
			zap.Int64("lock_id", lockID),
			zap.String("account_id", accountID),
			zap.Int("attempts", attempt+1),
		)
		return &AdvisoryLockInfo{
			LockKey:    lockKey,
			AccountID:  accountID,
			LockType:   string(lockType),
			AcquiredAt: time.Now(),
			Timeout:    timeout,
			IsAcquired: false,
		}, nil
	}

	return nil, fmt.Errorf("failed to acquire advisory lock after %d attempts: %w", alm.config.MaxRetries+1, lastErr)
}

// tryAcquireLock attempts to acquire a single lock
func (alm *AdvisoryLockManager) tryAcquireLock(ctx context.Context, lockID int64) (bool, error) {
	var acquired bool
	err := alm.db.WithContext(ctx).Raw(
		"SELECT pg_try_advisory_xact_lock(?)",
		lockID,
	).Scan(&acquired).Error

	return acquired, err
}

// getAdaptiveTimeout returns appropriate timeout based on lock type
func (alm *AdvisoryLockManager) getAdaptiveTimeout(lockType LockType) time.Duration {
	switch lockType {
	case LockTypeAccount:
		return alm.config.DefaultTimeout
	case LockTypeShard:
		return alm.config.FastTimeout
	case LockTypeCrossShard:
		return alm.config.SlowTimeout
	default:
		return alm.config.DefaultTimeout
	}
}

// recordLockMetrics records lock acquisition metrics
func (alm *AdvisoryLockManager) recordLockMetrics(lockType LockType, duration time.Duration, attempts int) {
	// This could be extended to send metrics to monitoring system
	alm.logger.Debug("Lock metrics recorded",
		zap.String("lock_type", string(lockType)),
		zap.Duration("acquisition_time", duration),
		zap.Int("attempts", attempts),
	)
}

// AcquireFastAdvisoryLock acquires a lock with minimal timeout for simple operations
func (alm *AdvisoryLockManager) AcquireFastAdvisoryLock(ctx context.Context, accountID string, lockType LockType) (*AdvisoryLockInfo, error) {
	return alm.AcquireAdvisoryLock(ctx, accountID, lockType, alm.config.FastTimeout)
}

// AcquireSlowAdvisoryLock acquires a lock with extended timeout for complex operations
func (alm *AdvisoryLockManager) AcquireSlowAdvisoryLock(ctx context.Context, accountID string, lockType LockType) (*AdvisoryLockInfo, error) {
	return alm.AcquireAdvisoryLock(ctx, accountID, lockType, alm.config.SlowTimeout)
}

// UpdateConfig updates the advisory lock configuration
func (alm *AdvisoryLockManager) UpdateConfig(config AdvisoryLockConfig) {
	alm.config = config
	alm.logger.Info("Advisory lock configuration updated",
		zap.Duration("default_timeout", config.DefaultTimeout),
		zap.Int("max_retries", config.MaxRetries),
		zap.Duration("retry_delay", config.RetryDelay),
		zap.Bool("enable_retry", config.EnableRetry),
	)
}

// ReleaseAdvisoryLock releases a PostgreSQL advisory lock
// Note: Advisory locks are automatically released when the transaction ends
func (alm *AdvisoryLockManager) ReleaseAdvisoryLock(ctx context.Context, lockKey string) error {
	alm.logger.Info("Releasing advisory lock",
		zap.String("lock_key", lockKey),
	)

	// Advisory locks are automatically released when the transaction ends
	// This is just for logging purposes
	return nil
}

// IsAdvisoryLockAcquired checks if an advisory lock is currently acquired
func (alm *AdvisoryLockManager) IsAdvisoryLockAcquired(ctx context.Context, lockKey string) (bool, error) {
	// Parse lock key to extract components
	parts := strings.Split(lockKey, ":")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid lock key format: %s", lockKey)
	}

	lockTypeStr := parts[0]
	lockIDStr := parts[1]

	var lockID int64

	switch lockTypeStr {
	case "shard":
		lockID = alm.generateLockID("", LockTypeShard, lockIDStr)
	case "account":
		lockID = alm.generateLockID(lockIDStr, LockTypeAccount, "")
	case "cross_shard":
		lockID = alm.generateLockID(lockIDStr, LockTypeCrossShard, "")
	default:
		return false, fmt.Errorf("unsupported lock type in key: %s", lockTypeStr)
	}

	// Use pg_try_advisory_lock to check if lock is available (not currently held)
	// This is different from pg_try_advisory_xact_lock which tries to acquire the lock
	var isAvailable bool
	err := alm.db.WithContext(ctx).Raw(
		"SELECT pg_try_advisory_lock(?)",
		lockID,
	).Scan(&isAvailable).Error

	if err != nil {
		return false, fmt.Errorf("failed to check advisory lock status: %w", err)
	}

	// If we can acquire the lock, it means it's not currently held
	// So we release it immediately and return false (not acquired)
	if isAvailable {
		// Release the lock we just acquired for checking
		alm.db.WithContext(ctx).Exec("SELECT pg_advisory_unlock(?)", lockID)
		return false, nil
	}

	// If we can't acquire the lock, it means it's currently held
	return true, nil
}

// GetAdvisoryLockInfo gets information about an advisory lock
func (alm *AdvisoryLockManager) GetAdvisoryLockInfo(ctx context.Context, lockKey string) (*AdvisoryLockInfo, error) {
	// Parse lock key to extract components
	parts := strings.Split(lockKey, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid lock key format: %s", lockKey)
	}

	lockTypeStr := parts[0]
	lockIDStr := parts[1]

	isAcquired, err := alm.IsAdvisoryLockAcquired(ctx, lockKey)
	if err != nil {
		return nil, err
	}

	return &AdvisoryLockInfo{
		LockKey:    lockKey,
		AccountID:  lockIDStr, // For shard locks, this will be empty, for account locks this will be the account ID
		LockType:   lockTypeStr,
		AcquiredAt: time.Now(),
		Timeout:    0, // Advisory locks don't have explicit timeouts
		IsAcquired: isAcquired,
	}, nil
}

// AcquireShardAdvisoryLock acquires a PostgreSQL advisory lock for a specific shard
func (alm *AdvisoryLockManager) AcquireShardAdvisoryLock(ctx context.Context, shardID string, timeout time.Duration) (*AdvisoryLockInfo, error) {
	lockKey := alm.generateLockKey("", LockTypeShard, shardID)
	lockID := alm.generateLockID("", LockTypeShard, shardID)

	alm.logger.Info("Attempting to acquire shard advisory lock",
		zap.String("lock_key", lockKey),
		zap.Int64("lock_id", lockID),
		zap.String("shard_id", shardID),
		zap.Duration("timeout", timeout),
	)

	// Use retry mechanism for shard-level locking with exponential backoff
	var acquired bool
	var attempts int
	maxAttempts := 3
	retryDelay := 5 * time.Millisecond

	for attempts = 0; attempts < maxAttempts; attempts++ {
		err := alm.db.WithContext(ctx).Raw(
			"SELECT pg_try_advisory_xact_lock(?)",
			lockID,
		).Scan(&acquired).Error

		if err != nil {
			alm.logger.Error("Failed to acquire shard advisory lock",
				zap.String("lock_key", lockKey),
				zap.Int64("lock_id", lockID),
				zap.String("shard_id", shardID),
				zap.Int("attempt", attempts+1),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to acquire shard advisory lock: %w", err)
		}

		if acquired {
			break
		}

		// If not acquired and not the last attempt, wait and retry
		if attempts < maxAttempts-1 {
			alm.logger.Debug("Shard advisory lock not acquired, retrying",
				zap.String("lock_key", lockKey),
				zap.Int64("lock_id", lockID),
				zap.String("shard_id", shardID),
				zap.Int("attempt", attempts+1),
				zap.Duration("retry_delay", retryDelay),
			)
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	if !acquired {
		alm.logger.Warn("Shard advisory lock not acquired after all retries",
			zap.String("lock_key", lockKey),
			zap.Int64("lock_id", lockID),
			zap.String("shard_id", shardID),
			zap.Int("attempts", attempts),
		)
		return &AdvisoryLockInfo{
			LockKey:    lockKey,
			AccountID:  "", // No account ID for shard locks
			LockType:   string(LockTypeShard),
			AcquiredAt: time.Now(),
			Timeout:    timeout,
			IsAcquired: false,
		}, nil
	}

	alm.logger.Info("Shard advisory lock acquired successfully",
		zap.String("lock_key", lockKey),
		zap.Int64("lock_id", lockID),
		zap.String("shard_id", shardID),
	)

	return &AdvisoryLockInfo{
		LockKey:    lockKey,
		AccountID:  "", // No account ID for shard locks
		LockType:   string(LockTypeShard),
		AcquiredAt: time.Now(),
		Timeout:    timeout,
		IsAcquired: true,
	}, nil
}

// AcquireMultipleShardLocks acquires advisory locks for multiple shards (for cross-shard transactions)
func (alm *AdvisoryLockManager) AcquireMultipleShardLocks(ctx context.Context, shardIDs []string, timeout time.Duration) ([]*AdvisoryLockInfo, error) {
	if len(shardIDs) == 0 {
		return nil, fmt.Errorf("no shard IDs provided")
	}

	// Sort shard IDs to prevent deadlock
	sortedShardIDs := make([]string, len(shardIDs))
	copy(sortedShardIDs, shardIDs)
	// Simple sort - in production, you might want more sophisticated ordering
	for i := 0; i < len(sortedShardIDs); i++ {
		for j := i + 1; j < len(sortedShardIDs); j++ {
			if sortedShardIDs[i] > sortedShardIDs[j] {
				sortedShardIDs[i], sortedShardIDs[j] = sortedShardIDs[j], sortedShardIDs[i]
			}
		}
	}

	var lockInfos []*AdvisoryLockInfo
	for _, shardID := range sortedShardIDs {
		lockInfo, err := alm.AcquireShardAdvisoryLock(ctx, shardID, timeout)
		if err != nil {
			// Release already acquired locks
			for _, acquiredLock := range lockInfos {
				alm.ReleaseAdvisoryLock(ctx, acquiredLock.LockKey)
			}
			return nil, fmt.Errorf("failed to acquire lock for shard %s: %w", shardID, err)
		}

		if !lockInfo.IsAcquired {
			// Release already acquired locks
			for _, acquiredLock := range lockInfos {
				alm.ReleaseAdvisoryLock(ctx, acquiredLock.LockKey)
			}
			return nil, fmt.Errorf("failed to acquire lock for shard %s: lock not available", shardID)
		}

		lockInfos = append(lockInfos, lockInfo)
	}

	return lockInfos, nil
}

// generateLockKey generates a unique lock key for advisory locking
func (alm *AdvisoryLockManager) generateLockKey(accountID string, lockType LockType, shardID string) string {
	switch lockType {
	case LockTypeShard:
		if shardID == "" {
			panic("shard ID is required for shard lock type")
		}
		shardHash := alm.hashString(shardID)
		return fmt.Sprintf("shard:%d", shardHash)
	case LockTypeAccount:
		if accountID == "" {
			panic("account ID is required for account lock type")
		}
		accountHash := alm.hashString(accountID)
		return fmt.Sprintf("account:%d", accountHash)
	case LockTypeCrossShard:
		if accountID == "" {
			panic("account ID is required for cross-shard lock type")
		}
		accountHash := alm.hashString(accountID)
		return fmt.Sprintf("cross_shard:%d", accountHash)
	default:
		panic(fmt.Sprintf("unsupported lock type: %s", lockType))
	}
}

// generateLockID generates a unique numeric lock ID for PostgreSQL advisory locks
func (alm *AdvisoryLockManager) generateLockID(accountID string, lockType LockType, shardID string) int64 {
	var lockID int64

	switch lockType {
	case LockTypeShard:
		if shardID == "" {
			panic("shard ID is required for shard lock type")
		}
		// Use shard ID hash for shard locks
		lockID = alm.hashString(shardID)
	case LockTypeAccount:
		if accountID == "" {
			panic("account ID is required for account lock type")
		}
		// Use account ID hash for account locks
		lockID = alm.hashString(accountID)
	case LockTypeCrossShard:
		if accountID == "" {
			panic("account ID is required for cross-shard lock type")
		}
		// Use account ID hash + lock type hash for cross-shard locks
		accountHash := alm.hashString(accountID)
		lockTypeHash := alm.hashString(string(lockType))
		lockID = accountHash + lockTypeHash
	default:
		panic(fmt.Sprintf("unsupported lock type: %s", lockType))
	}

	// Ensure positive value and within int64 range
	if lockID < 0 {
		lockID = -lockID
	}

	// Use modulo to ensure it fits within a reasonable range for advisory locks
	// PostgreSQL advisory locks work best with smaller numbers
	lockID = lockID % 1000000000 // Keep it under 1 billion

	return lockID
}

// hashString converts a string to a numeric hash for advisory lock
func (alm *AdvisoryLockManager) hashString(s string) int64 {
	hash := int64(0)
	for _, c := range s {
		hash = hash*31 + int64(c)
	}

	// Ensure positive value
	if hash < 0 {
		hash = -hash
	}

	return hash
}

// GetAdvisoryLockStats returns statistics about advisory locks
func (alm *AdvisoryLockManager) GetAdvisoryLockStats(ctx context.Context) (map[string]interface{}, error) {
	var stats struct {
		TotalLocks   int64 `json:"total_locks"`
		ActiveLocks  int64 `json:"active_locks"`
		LockWaitTime int64 `json:"lock_wait_time"`
		LockHoldTime int64 `json:"lock_hold_time"`
	}

	// Get advisory lock statistics from PostgreSQL
	err := alm.db.WithContext(ctx).Raw(`
		SELECT 
			COUNT(*) as total_locks,
			COUNT(*) FILTER (WHERE granted = true) as active_locks,
			AVG(EXTRACT(EPOCH FROM (now() - query_start))) as lock_wait_time,
			AVG(EXTRACT(EPOCH FROM (now() - query_start))) as lock_hold_time
		FROM pg_locks 
		WHERE locktype = 'advisory'
	`).Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get advisory lock stats: %w", err)
	}

	return map[string]interface{}{
		"total_locks":    stats.TotalLocks,
		"active_locks":   stats.ActiveLocks,
		"lock_wait_time": stats.LockWaitTime,
		"lock_hold_time": stats.LockHoldTime,
	}, nil
}

// WaitForAdvisoryLock waits for an advisory lock to be available
func (alm *AdvisoryLockManager) WaitForAdvisoryLock(ctx context.Context, accountID string, lockType LockType, timeout time.Duration) (*AdvisoryLockInfo, error) {
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			lockInfo, err := alm.AcquireAdvisoryLock(ctx, accountID, lockType, timeout)
			if err != nil {
				return nil, err
			}

			if lockInfo.IsAcquired {
				alm.logger.Info("Advisory lock acquired after waiting",
					zap.String("lock_key", lockInfo.LockKey),
					zap.Duration("wait_time", time.Since(start)),
				)
				return lockInfo, nil
			}

			if time.Since(start) > timeout {
				return nil, fmt.Errorf("timeout waiting for advisory lock: %s", lockInfo.LockKey)
			}
		}
	}
}
