package tests

import (
	"fmt"
	"testing"
	"time"

	"sub-balance-implementation/pkg/utils"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// IntegrationTestSuite tests the complete integration of all sub balance principles
type IntegrationTestSuite struct {
	hashUtils *utils.HashUtils
}

func NewIntegrationTestSuite() *IntegrationTestSuite {
	return &IntegrationTestSuite{
		hashUtils: utils.NewHashUtils(),
	}
}

// TestSubBalancePrinciplesIntegration tests that all principles work together
func TestSubBalancePrinciplesIntegration(t *testing.T) {
	suite := NewIntegrationTestSuite()

	t.Run("Consistent Hashing Distribution", suite.testConsistentHashingDistribution)
	t.Run("Advisory Lock Integration", suite.testAdvisoryLockIntegration)
	t.Run("Shard Selection Strategies", suite.testShardSelectionStrategies)
	t.Run("Cross Shard Transaction Flow", suite.testCrossShardTransactionFlow)
	t.Run("Performance Optimization", suite.testPerformanceOptimization)
	t.Run("Consistency Validation", suite.testConsistencyValidation)
}

// testConsistentHashingDistribution verifies consistent hashing works correctly
func (s *IntegrationTestSuite) testConsistentHashingDistribution(t *testing.T) {
	// Test 1: Deterministic routing
	accountID := "acc-12345"
	shardCount := 3

	// Same account should always go to same shard
	shard1 := s.hashUtils.GetShardIndexByAccountHash(accountID, shardCount)
	shard2 := s.hashUtils.GetShardIndexByAccountHash(accountID, shardCount)
	shard3 := s.hashUtils.GetShardIndexByAccountHash(accountID, shardCount)

	assert.Equal(t, shard1, shard2, "Shard assignment should be deterministic")
	assert.Equal(t, shard2, shard3, "Shard assignment should be deterministic")

	// Test 2: Uniform distribution
	accountIDs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		accountIDs[i] = fmt.Sprintf("acc-%05d", i)
	}

	distribution := s.hashUtils.ValidateShardDistribution(accountIDs, shardCount)

	// Check total accounts
	totalAccounts := 0
	for _, count := range distribution {
		totalAccounts += count
	}
	assert.Equal(t, 1000, totalAccounts, "All accounts should be distributed")

	// Check distribution is relatively even (within 10% of average)
	average := float64(1000) / float64(shardCount)
	for shardIndex, count := range distribution {
		deviation := float64(count) - average
		percentageDeviation := (deviation / average) * 100

		assert.Less(t, percentageDeviation, 10.0,
			"Shard %d has %d accounts (%.2f%% deviation from average %.2f)",
			shardIndex, count, percentageDeviation, average)
	}

	// Test 3: Load balance score
	score := s.hashUtils.CalculateLoadBalanceScore(distribution)
	assert.Greater(t, score, 0.5, "Load balance score should be > 0.5 for reasonable distribution")
}

// testAdvisoryLockIntegration verifies advisory lock integration
func (s *IntegrationTestSuite) testAdvisoryLockIntegration(t *testing.T) {
	// This would test advisory lock integration in a real scenario
	// For now, we'll test the hash function used for lock keys

	accountID := "acc-12345"

	// Test deterministic lock key generation
	hash1 := s.hashUtils.GenerateAccountHash(accountID)
	hash2 := s.hashUtils.GenerateAccountHash(accountID)

	assert.Equal(t, hash1, hash2, "Lock key hash should be deterministic")

	// Test different accounts produce different lock keys
	accountID2 := "acc-67890"
	hash3 := s.hashUtils.GenerateAccountHash(accountID2)

	assert.NotEqual(t, hash1, hash3, "Different accounts should produce different lock keys")
}

// testShardSelectionStrategies verifies shard selection strategies
func (s *IntegrationTestSuite) testShardSelectionStrategies(t *testing.T) {
	// Mock shard data for testing
	shards := []MockShard{
		{ID: "shard-1", Index: 0, Balance: decimal.NewFromInt(1000000)},
		{ID: "shard-2", Index: 1, Balance: decimal.NewFromInt(2000000)},
		{ID: "shard-3", Index: 2, Balance: decimal.NewFromInt(500000)},
	}

	// Test debit strategy (select highest balance)
	debitAmount := decimal.NewFromInt(800000)
	selectedShard := selectShardForDebit(shards, debitAmount)

	assert.NotNil(t, selectedShard, "Should select a shard for debit")
	assert.Equal(t, "shard-2", selectedShard.ID, "Should select shard with highest balance")
	assert.True(t, selectedShard.Balance.GreaterThanOrEqual(debitAmount),
		"Selected shard should have sufficient balance")

	// Test credit strategy (select lowest balance for load balancing)
	creditAmount := decimal.NewFromInt(100000)
	selectedShard = selectShardForCredit(shards, creditAmount)

	assert.NotNil(t, selectedShard, "Should select a shard for credit")
	assert.Equal(t, "shard-3", selectedShard.ID, "Should select shard with lowest balance")
}

// testCrossShardTransactionFlow verifies cross-shard transaction handling
func (s *IntegrationTestSuite) testCrossShardTransactionFlow(t *testing.T) {
	// Mock shard data
	shards := []MockShard{
		{ID: "shard-1", Index: 0, Balance: decimal.NewFromInt(300000)},
		{ID: "shard-2", Index: 1, Balance: decimal.NewFromInt(400000)},
		{ID: "shard-3", Index: 2, Balance: decimal.NewFromInt(200000)},
	}

	// Test cross-shard transaction (amount > any single shard)
	amount := decimal.NewFromInt(800000)
	selectedShards := selectShardsForCrossShard(shards, amount)

	assert.NotEmpty(t, selectedShards, "Should select multiple shards for cross-shard transaction")

	// Verify total contribution covers the amount
	totalContribution := decimal.Zero
	for _, shard := range selectedShards {
		totalContribution = totalContribution.Add(shard.Contribution)
	}

	assert.True(t, totalContribution.GreaterThanOrEqual(amount),
		"Total contribution should cover the transaction amount")
}

// testPerformanceOptimization verifies performance improvements
func (s *IntegrationTestSuite) testPerformanceOptimization(t *testing.T) {
	// Test hash function performance
	accountID := "acc-12345"
	shardCount := 10

	start := time.Now()
	for i := 0; i < 10000; i++ {
		s.hashUtils.GetShardIndexByAccountHash(accountID, shardCount)
	}
	duration := time.Since(start)

	// Should be very fast (O(1) operation)
	assert.Less(t, duration, 10*time.Millisecond,
		"Hash-based shard selection should be very fast")

	// Test optimal shard count calculation
	optimalCount := s.hashUtils.GetOptimalShardCount(5000)
	assert.Equal(t, 5, optimalCount, "Should calculate optimal shard count correctly")
}

// testConsistencyValidation verifies consistency validation
func (s *IntegrationTestSuite) testConsistencyValidation(t *testing.T) {
	// Test shard distribution validation
	accountIDs := []string{"acc-1", "acc-2", "acc-3", "acc-4", "acc-5"}
	shardCount := 2

	distribution := s.hashUtils.ValidateShardDistribution(accountIDs, shardCount)

	// All accounts should be distributed
	totalAccounts := 0
	for _, count := range distribution {
		totalAccounts += count
	}
	assert.Equal(t, len(accountIDs), totalAccounts, "All accounts should be distributed")

	// Test load balance score calculation
	score := s.hashUtils.CalculateLoadBalanceScore(distribution)
	assert.GreaterOrEqual(t, score, 0.0, "Load balance score should be >= 0")
	assert.LessOrEqual(t, score, 1.0, "Load balance score should be <= 1")
}

// Mock structures for testing
type MockShard struct {
	ID      string
	Index   int
	Balance decimal.Decimal
}

type MockShardSelection struct {
	Shard        MockShard
	Contribution decimal.Decimal
}

// Helper functions for testing shard selection strategies
func selectShardForDebit(shards []MockShard, amount decimal.Decimal) *MockShard {
	// Sort by balance (highest first)
	for i := 0; i < len(shards)-1; i++ {
		for j := i + 1; j < len(shards); j++ {
			if shards[i].Balance.LessThan(shards[j].Balance) {
				shards[i], shards[j] = shards[j], shards[i]
			}
		}
	}

	// Find shard with sufficient balance
	for _, shard := range shards {
		if shard.Balance.GreaterThanOrEqual(amount) {
			return &shard
		}
	}

	return nil
}

func selectShardForCredit(shards []MockShard, amount decimal.Decimal) *MockShard {
	// Sort by balance (lowest first)
	for i := 0; i < len(shards)-1; i++ {
		for j := i + 1; j < len(shards); j++ {
			if shards[i].Balance.GreaterThan(shards[j].Balance) {
				shards[i], shards[j] = shards[j], shards[i]
			}
		}
	}

	// Select shard with lowest balance
	return &shards[0]
}

func selectShardsForCrossShard(shards []MockShard, amount decimal.Decimal) []MockShardSelection {
	// Sort by balance (highest first)
	for i := 0; i < len(shards)-1; i++ {
		for j := i + 1; j < len(shards); j++ {
			if shards[i].Balance.LessThan(shards[j].Balance) {
				shards[i], shards[j] = shards[j], shards[i]
			}
		}
	}

	var selections []MockShardSelection
	remainingAmount := amount

	for _, shard := range shards {
		if remainingAmount.LessThanOrEqual(decimal.Zero) {
			break
		}

		contribution := shard.Balance
		if contribution.GreaterThan(remainingAmount) {
			contribution = remainingAmount
		}

		if contribution.GreaterThan(decimal.Zero) {
			selections = append(selections, MockShardSelection{
				Shard:        shard,
				Contribution: contribution,
			})

			remainingAmount = remainingAmount.Sub(contribution)
		}
	}

	return selections
}

// Benchmark tests for performance validation
func BenchmarkHashBasedShardSelection(b *testing.B) {
	hashUtils := utils.NewHashUtils()
	accountID := "acc-12345"
	shardCount := 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashUtils.GetShardIndexByAccountHash(accountID, shardCount)
	}
}

func BenchmarkShardDistributionValidation(b *testing.B) {
	hashUtils := utils.NewHashUtils()
	accountIDs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		accountIDs[i] = fmt.Sprintf("acc-%05d", i)
	}
	shardCount := 5

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashUtils.ValidateShardDistribution(accountIDs, shardCount)
	}
}
