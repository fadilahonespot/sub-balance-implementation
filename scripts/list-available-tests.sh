#!/bin/bash

# List Available Test Scripts
# Shows all available test scripts with descriptions

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}📋 Available TRUE Shard-Level Locking Test Scripts${NC}"
echo "=============================================================="
echo ""

echo -e "${CYAN}🧪 Individual Test Scripts:${NC}"
echo "----------------------------------------"
echo -e "${GREEN}1. test-single-account-shard-locking.sh${NC}"
echo "   - Tests single account with 10 shards"
echo "   - TPS scenarios: 10, 20, 30, 50, 100, 200, 300"
echo "   - Focus: Shard distribution and load balancing"
echo ""

echo -e "${GREEN}2. test-multi-account-shard-locking.sh${NC}"
echo "   - Tests 5 accounts with 10 shards each (50 total shards)"
echo "   - TPS scenarios: 10, 20, 30, 50, 100, 200, 300"
echo "   - Focus: True parallelism across multiple accounts"
echo ""

echo -e "${GREEN}3. test-balance-integrity-comprehensive.sh${NC}"
echo "   - Tests balance integrity across multiple scenarios"
echo "   - Scenarios: Single account, Multi-account, Cross-shard"
echo "   - Focus: Balance consistency and shard integrity"
echo ""

echo -e "${CYAN}🚀 Test Suite Scripts:${NC}"
echo "----------------------------------------"
echo -e "${GREEN}4. run-all-tests.sh${NC}"
echo "   - Runs single account + multi-account tests"
echo "   - Comprehensive performance testing"
echo ""

echo -e "${GREEN}5. run-complete-test-suite.sh${NC}"
echo "   - Runs ALL tests: balance integrity + performance"
echo "   - Complete test coverage"
echo ""

echo -e "${CYAN}📊 Test Features:${NC}"
echo "----------------------------------------"
echo -e "${YELLOW}✅ TRUE Shard-Level Locking${NC}"
echo "   - Each shard can be locked independently"
echo "   - No account-level lock contention"
echo "   - Maximum parallelism"
echo ""

echo -e "${YELLOW}✅ Consistent Hashing${NC}"
echo "   - Even distribution of load across shards"
echo "   - Deterministic shard selection"
echo "   - Load balancing"
echo ""

echo -e "${YELLOW}✅ Balance Integrity${NC}"
echo "   - Cross-shard balance consistency"
echo "   - Account-level balance validation"
echo "   - Shard-level balance validation"
echo ""

echo -e "${YELLOW}✅ Performance Testing${NC}"
echo "   - Multiple TPS scenarios"
echo "   - Single and multi-account testing"
echo "   - Detailed performance analysis"
echo ""

echo -e "${CYAN}📈 Test Scenarios:${NC}"
echo "----------------------------------------"
echo -e "${BLUE}Single Account Tests:${NC}"
echo "   - 10 TPS: 100 transactions over 10s"
echo "   - 20 TPS: 200 transactions over 10s"
echo "   - 30 TPS: 300 transactions over 10s"
echo "   - 50 TPS: 500 transactions over 10s"
echo "   - 100 TPS: 1000 transactions over 10s"
echo "   - 200 TPS: 2000 transactions over 10s"
echo "   - 300 TPS: 3000 transactions over 10s"
echo ""

echo -e "${BLUE}Multi-Account Tests:${NC}"
echo "   - 10 TPS: 100 transactions over 10s"
echo "   - 20 TPS: 200 transactions over 10s"
echo "   - 30 TPS: 300 transactions over 10s"
echo "   - 50 TPS: 500 transactions over 10s"
echo "   - 100 TPS: 1000 transactions over 10s"
echo "   - 200 TPS: 2000 transactions over 10s"
echo "   - 300 TPS: 3000 transactions over 10s"
echo ""

echo -e "${BLUE}Balance Integrity Tests:${NC}"
echo "   - Single Account: 1 account, 10 shards, 100 debits"
echo "   - Multi-Account: 5 accounts, 10 shards each, 50 debits per account"
echo "   - Cross-Shard: 1 account, 10 shards, 200 debits"
echo ""

echo -e "${CYAN}📄 Report Generation:${NC}"
echo "----------------------------------------"
echo "All tests generate detailed reports in the 'reports/' directory:"
echo "   - Performance metrics"
echo "   - Error analysis"
echo "   - Shard distribution"
echo "   - Balance integrity"
echo "   - Comparison tables"
echo ""

echo -e "${CYAN}🚀 Quick Start:${NC}"
echo "----------------------------------------"
echo "1. Start the server:"
echo "   go run cmd/main.go"
echo ""
echo "2. Run complete test suite:"
echo "   ./scripts/run-complete-test-suite.sh"
echo ""
echo "3. Or run individual tests:"
echo "   ./scripts/test-single-account-shard-locking.sh"
echo "   ./scripts/test-multi-account-shard-locking.sh"
echo "   ./scripts/test-balance-integrity-comprehensive.sh"
echo ""

echo -e "${GREEN}🎉 All test scripts are ready to use!${NC}"
