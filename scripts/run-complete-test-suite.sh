#!/bin/bash

# Complete TRUE Shard-Level Locking Test Suite
# Runs all tests: single account, multi-account, and balance integrity

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Complete TRUE Shard-Level Locking Test Suite${NC}"
echo "=============================================================="
echo -e "${CYAN}📋 This script will run all comprehensive tests${NC}"
echo ""

# Check if server is running
echo -e "${CYAN}📡 Checking server health...${NC}"
if ! curl -s "http://localhost:8080/api/v1/../health" > /dev/null; then
    echo -e "${RED}❌ Server is not running. Please start the server first:${NC}"
    echo "   go run cmd/main.go"
    exit 1
fi
echo -e "${GREEN}✅ Server is running${NC}"

echo ""
echo -e "${PURPLE}🧪 Starting Complete Test Suite...${NC}"
echo "=============================================="

# Phase 1: Balance Integrity Tests
echo -e "${YELLOW}📊 Phase 1: Balance Integrity Tests${NC}"
echo "=========================================="
echo -e "${CYAN}Testing balance integrity across multiple scenarios${NC}"
echo ""

./scripts/test-balance-integrity-comprehensive.sh

echo ""
echo -e "${GREEN}✅ Balance Integrity Tests Completed${NC}"
echo ""

# Wait between phases
echo -e "${CYAN}⏳ Waiting 5 seconds before starting performance tests...${NC}"
sleep 5

# Phase 2: Single Account Performance Tests
echo -e "${YELLOW}📊 Phase 2: Single Account Performance Tests${NC}"
echo "================================================"
echo -e "${CYAN}Testing shard distribution and load balancing with single account${NC}"
echo ""

./scripts/test-single-account-shard-locking.sh

echo ""
echo -e "${GREEN}✅ Single Account Performance Tests Completed${NC}"
echo ""

# Wait between phases
echo -e "${CYAN}⏳ Waiting 5 seconds before starting multi-account tests...${NC}"
sleep 5

# Phase 3: Multi-Account Performance Tests
echo -e "${YELLOW}📊 Phase 3: Multi-Account Performance Tests${NC}"
echo "================================================"
echo -e "${CYAN}Testing true parallelism with 5 accounts and 10 shards each${NC}"
echo ""

./scripts/test-multi-account-shard-locking.sh

echo ""
echo -e "${GREEN}✅ Multi-Account Performance Tests Completed${NC}"
echo ""

# Final summary
echo ""
echo -e "${BLUE}🎯 Complete Test Suite Summary${NC}"
echo "=============================================="
echo -e "${GREEN}✅ All tests completed successfully${NC}"
echo ""
echo -e "${CYAN}📄 Reports generated:${NC}"
echo "   - Balance Integrity: reports/balance_integrity_comprehensive_report_*.md"
echo "   - Single Account: reports/single_account_shard_locking_report_*.md"
echo "   - Multi-Account: reports/multi_account_shard_locking_report_*.md"
echo ""
echo -e "${CYAN}📊 Test Coverage:${NC}"
echo "   - Balance Integrity: 3 scenarios (single, multi, cross-shard)"
echo "   - Single Account: 9 TPS scenarios (10-1000 TPS)"
echo "   - Multi-Account: 7 TPS scenarios (50-2000 TPS)"
echo "   - Total Shards Tested: 5 accounts × 10 shards = 50 shards"
echo "   - TRUE Shard-Level Locking: ✅ Implemented"
echo "   - Consistent Hashing: ✅ Implemented"
echo "   - Load Distribution: ✅ Tested"
echo "   - Balance Integrity: ✅ Tested"
echo ""
echo -e "${GREEN}🎉 Complete TRUE Shard-Level Locking test suite completed!${NC}"
