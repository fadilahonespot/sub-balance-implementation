package utils

import (
	"fmt"
	"hash/crc32"
)

// HashUtils provides consistent hashing utilities for sub balance
type HashUtils struct{}

// NewHashUtils creates a new hash utils instance
func NewHashUtils() *HashUtils {
	return &HashUtils{}
}

// GenerateAccountHash generates a consistent hash for an account ID
func (hu *HashUtils) GenerateAccountHash(accountID string) uint32 {
	return crc32.ChecksumIEEE([]byte(accountID))
}

// GetShardIndexByAccountHash determines which shard an account should belong to
func (hu *HashUtils) GetShardIndexByAccountHash(accountID string, shardCount int) int {
	hash := hu.GenerateAccountHash(accountID)
	return int(hash % uint32(shardCount))
}

// GenerateShardHash generates a consistent hash for a shard
func (hu *HashUtils) GenerateShardHash(accountID string, shardIndex int) string {
	data := fmt.Sprintf("%s_%d", accountID, shardIndex)
	hash := crc32.ChecksumIEEE([]byte(data))
	return fmt.Sprintf("shard_%d_%x", shardIndex, hash)
}

// GenerateConsistentShardHash generates a hash that's consistent across shards
func (hu *HashUtils) GenerateConsistentShardHash(accountID string, shardIndex int, totalShards int) string {
	// Use account hash + shard index for consistent routing
	accountHash := hu.GenerateAccountHash(accountID)
	shardHash := uint32(shardIndex) + accountHash
	return fmt.Sprintf("shard_%d_%x", shardIndex, shardHash)
}

// ValidateShardDistribution validates that accounts are evenly distributed across shards
func (hu *HashUtils) ValidateShardDistribution(accountIDs []string, shardCount int) map[int]int {
	distribution := make(map[int]int)

	for _, accountID := range accountIDs {
		shardIndex := hu.GetShardIndexByAccountHash(accountID, shardCount)
		distribution[shardIndex]++
	}

	return distribution
}

// GetOptimalShardCount calculates optimal shard count based on account count
func (hu *HashUtils) GetOptimalShardCount(accountCount int) int {
	// Rule of thumb: 1 shard per 1000-5000 accounts
	if accountCount < 1000 {
		return 1
	} else if accountCount < 5000 {
		return 3
	} else if accountCount < 10000 {
		return 5
	} else if accountCount < 50000 {
		return 10
	} else {
		return 20
	}
}

// CalculateLoadBalanceScore calculates how well balanced the shards are
func (hu *HashUtils) CalculateLoadBalanceScore(distribution map[int]int) float64 {
	if len(distribution) == 0 {
		return 0.0
	}

	// Calculate average
	total := 0
	for _, count := range distribution {
		total += count
	}
	average := float64(total) / float64(len(distribution))

	// Calculate variance
	variance := 0.0
	for _, count := range distribution {
		diff := float64(count) - average
		variance += diff * diff
	}
	variance /= float64(len(distribution))

	// Calculate standard deviation
	stdDev := variance

	// Score: 1.0 = perfect balance, 0.0 = terrible balance
	score := 1.0 - (stdDev / average)
	if score < 0 {
		score = 0
	}

	return score
}
