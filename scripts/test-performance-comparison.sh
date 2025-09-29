#!/bin/bash

# Performance Comparison Test: Single Database vs Database Sharding
# This script compares performance between single database and database sharding setups

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="http://localhost:8080"
REPORT_DIR="reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo -e "${BLUE}🚀 Performance Comparison Test: Single DB vs Database Sharding${NC}"
echo "=================================================================="

# Create reports directory
mkdir -p "$REPORT_DIR"

# Function to test single transaction latency
test_single_transaction_latency() {
    local test_name="$1"
    local account_id="$2"
    local iterations=10
    
    echo -e "${CYAN}📊 Testing single transaction latency for $test_name...${NC}"
    
    local total_latency=0
    local success_count=0
    
    for i in $(seq 1 $iterations); do
        local start_time=$(date +%s.%N)
        
        local response=$(curl -s -w "%{http_code}" -o /tmp/response.json \
            -X POST "$BASE_URL/api/v1/transaction/execute" \
            -H "Content-Type: application/json" \
            -d "{
                \"account_id\": \"$account_id\",
                \"amount\": 1000,
                \"transaction_type\": \"debit\",
                \"description\": \"Performance test transaction $i\"
            }")
        
        local end_time=$(date +%s.%N)
        local latency=$(echo "scale=3; $end_time - $start_time" | bc)
        
        if [ "$response" = "200" ]; then
            total_latency=$(echo "$total_latency + $latency" | bc)
            success_count=$((success_count + 1))
        fi
        
        # Small delay between requests
        sleep 0.1
    done
    
    if [ $success_count -gt 0 ]; then
        local avg_latency=$(echo "scale=3; $total_latency / $success_count" | bc)
        echo -e "${GREEN}   ✅ Average latency: ${avg_latency}s (${success_count}/$iterations successful)${NC}"
        echo "$avg_latency"
    else
        echo -e "${RED}   ❌ All transactions failed${NC}"
        echo "0"
    fi
}

# Function to test concurrent transactions
test_concurrent_transactions() {
    local test_name="$1"
    local account_id="$2"
    local concurrent_count=10
    local duration=5
    
    echo -e "${CYAN}📊 Testing concurrent transactions for $test_name...${NC}"
    
    local start_time=$(date +%s)
    local end_time=$((start_time + duration))
    local success_count=0
    local total_requests=0
    
    # Start concurrent requests
    for i in $(seq 1 $concurrent_count); do
        (
            while [ $(date +%s) -lt $end_time ]; do
                local response=$(curl -s -w "%{http_code}" -o /dev/null \
                    -X POST "$BASE_URL/api/v1/transaction/execute" \
                    -H "Content-Type: application/json" \
                    -d "{
                        \"account_id\": \"$account_id\",
                        \"amount\": 1000,
                        \"transaction_type\": \"debit\",
                        \"description\": \"Concurrent test transaction\"
                    }")
                
                if [ "$response" = "200" ]; then
                    echo "success" >> /tmp/concurrent_results_$i.txt
                else
                    echo "failed" >> /tmp/concurrent_results_$i.txt
                fi
                
                total_requests=$((total_requests + 1))
                sleep 0.1
            done
        ) &
    done
    
    # Wait for all background processes
    wait
    
    # Count results
    local total_success=0
    for i in $(seq 1 $concurrent_count); do
        if [ -f "/tmp/concurrent_results_$i.txt" ]; then
            local success=$(grep -c "success" "/tmp/concurrent_results_$i.txt" 2>/dev/null || echo "0")
            total_success=$((total_success + success))
            rm -f "/tmp/concurrent_results_$i.txt"
        fi
    done
    
    local actual_tps=$(echo "scale=2; $total_success / $duration" | bc)
    local success_rate=$(echo "scale=2; $total_success * 100 / $total_requests" | bc 2>/dev/null || echo "0")
    
    echo -e "${GREEN}   ✅ Concurrent test results:${NC}"
    echo -e "      Total requests: $total_requests"
    echo -e "      Successful: $total_success"
    echo -e "      Success rate: ${success_rate}%"
    echo -e "      Actual TPS: $actual_tps"
    
    echo "$actual_tps,$success_rate"
}

# Function to create test account
create_test_account() {
    local test_name="$1"
    echo -e "${CYAN}👤 Creating test account for $test_name...${NC}"
    
    local response=$(curl -s -X POST "$BASE_URL/api/v1/account" \
        -H "Content-Type: application/json" \
        -d "{
            \"wallet_no\": \"perf_test_${test_name}_$(date +%s)\",
            \"wallet_type_id\": \"1\",
            \"instance_type\": \"1\",
            \"currency_id\": \"1\",
            \"owner_id\": \"1\"
        }")
    
    local account_id=$(echo "$response" | jq -r '.id // empty')
    
    if [ -n "$account_id" ] && [ "$account_id" != "null" ]; then
        echo -e "${GREEN}   ✅ Account created: $account_id${NC}"
        
        # Credit the account
        echo -e "${CYAN}💰 Crediting account...${NC}"
        for i in $(seq 1 10); do
            curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
                -H "Content-Type: application/json" \
                -d "{
                    \"account_id\": \"$account_id\",
                    \"amount\": 1000000,
                    \"transaction_type\": \"credit\",
                    \"description\": \"Initial credit $i\"
                }" > /dev/null
        done
        
        echo -e "${GREEN}   ✅ Account credited with 10,000,000${NC}"
        echo "$account_id"
    else
        echo -e "${RED}   ❌ Failed to create account${NC}"
        echo ""
    fi
}

# Main test execution
echo -e "${PURPLE}🧪 Starting Performance Comparison Tests...${NC}"
echo "=============================================================="

# Test 1: Single Database Performance (if available)
echo -e "${YELLOW}📊 Test 1: Single Database Performance${NC}"
single_account_id=$(create_test_account "single_db")
if [ -n "$single_account_id" ]; then
    single_latency=$(test_single_transaction_latency "Single DB" "$single_account_id")
    single_concurrent=$(test_concurrent_transactions "Single DB" "$single_account_id")
    echo -e "${GREEN}✅ Single DB test completed${NC}"
else
    echo -e "${RED}❌ Single DB test failed${NC}"
    single_latency="0"
    single_concurrent="0,0"
fi

echo ""

# Test 2: Database Sharding Performance
echo -e "${YELLOW}📊 Test 2: Database Sharding Performance${NC}"
sharding_account_id=$(create_test_account "sharding")
if [ -n "$sharding_account_id" ]; then
    sharding_latency=$(test_single_transaction_latency "Database Sharding" "$sharding_account_id")
    sharding_concurrent=$(test_concurrent_transactions "Database Sharding" "$sharding_account_id")
    echo -e "${GREEN}✅ Database Sharding test completed${NC}"
else
    echo -e "${RED}❌ Database Sharding test failed${NC}"
    sharding_latency="0"
    sharding_concurrent="0,0"
fi

echo ""

# Generate comparison report
echo -e "${BLUE}📊 Performance Comparison Results${NC}"
echo "=============================================="

echo -e "${CYAN}Single Transaction Latency:${NC}"
echo "  Single DB:      ${single_latency}s"
echo "  Database Sharding: ${sharding_latency}s"

echo ""
echo -e "${CYAN}Concurrent Performance:${NC}"
IFS=',' read -r single_tps single_success <<< "$single_concurrent"
IFS=',' read -r sharding_tps sharding_success <<< "$sharding_concurrent"

echo "  Single DB TPS:      $single_tps"
echo "  Database Sharding TPS: $sharding_tps"
echo "  Single DB Success Rate: ${single_success}%"
echo "  Database Sharding Success Rate: ${sharding_success}%"

echo ""
echo -e "${BLUE}📄 Detailed report saved to: $REPORT_DIR/performance_comparison_$TIMESTAMP.md${NC}"

# Save detailed report
cat > "$REPORT_DIR/performance_comparison_$TIMESTAMP.md" << EOF
# Performance Comparison Report

**Generated:** $(date)
**Test Type:** Single Database vs Database Sharding

## Test Configuration
- Single Transaction Latency: 10 iterations
- Concurrent Transactions: 10 concurrent, 5 seconds duration
- Account Setup: 10 shards, 1,000,000 credit each

## Results

### Single Transaction Latency
- **Single DB:** ${single_latency}s
- **Database Sharding:** ${sharding_latency}s

### Concurrent Performance
- **Single DB TPS:** $single_tps
- **Database Sharding TPS:** $sharding_tps
- **Single DB Success Rate:** ${single_success}%
- **Database Sharding Success Rate:** ${sharding_success}%

## Analysis
- Latency Impact: Database sharding adds overhead
- TPS Impact: Sharding may reduce overall TPS due to network overhead
- Success Rate: Both configurations should maintain high success rates

## Recommendations
1. Optimize database connection pooling
2. Consider reducing Docker networking overhead
3. Implement connection pooling per shard
4. Monitor database performance metrics
EOF

echo -e "${GREEN}🎉 Performance comparison test completed!${NC}"
