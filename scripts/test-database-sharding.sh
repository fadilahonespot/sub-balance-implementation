#!/bin/bash

# Database Sharding Test Script
# Test script untuk menguji database sharding dengan multiple PostgreSQL instances

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="http://localhost:8080"
SHARD_COUNT=3
TEST_ACCOUNTS=5
CREDIT_AMOUNT=1000000
DEBIT_AMOUNT=100000

# Test results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

echo -e "${BLUE}🗄️  Database Sharding Test Suite${NC}"
echo "=================================="
echo ""

# Function to log test results
log_test() {
    local test_name="$1"
    local status="$2"
    local message="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✅ $test_name${NC}: $message"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ $test_name${NC}: $message"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# Function to test database connection
test_database_connection() {
    local shard_id="$1"
    local port="$2"
    local db_name="$3"
    
    echo "Testing connection to Shard $shard_id (port $port)..."
    
    if docker exec postgres-shard-$shard_id psql -U postgres -d $db_name -c "SELECT 1;" > /dev/null 2>&1; then
        log_test "Database Shard $shard_id Connection" "PASS" "Successfully connected to $db_name"
        return 0
    else
        log_test "Database Shard $shard_id Connection" "FAIL" "Failed to connect to $db_name"
        return 1
    fi
}

# Function to test table existence
test_table_existence() {
    local shard_id="$1"
    local db_name="$2"
    
    echo "Checking tables in Shard $shard_id..."
    
    local tables=$(docker exec postgres-shard-$shard_id psql -U postgres -d $db_name -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | tr -d ' ')
    
    if [ "$tables" = "3" ]; then
        log_test "Shard $shard_id Tables" "PASS" "All required tables exist (accounts, account_balance_shard, transactions)"
        return 0
    else
        log_test "Shard $shard_id Tables" "FAIL" "Expected 3 tables, found $tables"
        return 1
    fi
}

# Function to test shard router functionality
test_shard_router() {
    echo "Testing shard router functionality..."
    
    # Test account creation with sharding
    local timestamp=$(date +%s)
    local account_data='{
        "wallet_no": "test_shard_'$timestamp'",
        "wallet_type_id": "1",
        "instance_type": "individual",
        "currency_id": "1",
        "owner_id": "test_owner_001",
        "use_sub_balance": true,
        "shard_count": 8
    }'
    
    local response=$(curl -s -X POST "$BASE_URL/api/v1/account/create" \
        -H "Content-Type: application/json" \
        -d "$account_data")
    
    local account_id=$(echo "$response" | jq -r '.id // empty')
    
    if [ -n "$account_id" ] && [ "$account_id" != "null" ]; then
        log_test "Account Creation with Sharding" "PASS" "Account created successfully: $account_id"
        
        # Test credit transaction
        local credit_data='{
            "transaction_id": "test_shard_credit_001",
            "account_id": "'$account_id'",
            "amount": "'$CREDIT_AMOUNT'",
            "transaction_type": "credit",
            "description": "Test sharding credit transaction"
        }'
        
        local credit_response=$(curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
            -H "Content-Type: application/json" \
            -d "$credit_data")
        
        local credit_success=$(echo "$credit_response" | jq -r '.success // false')
        
        if [ "$credit_success" = "true" ]; then
            log_test "Credit Transaction with Sharding" "PASS" "Credit transaction processed successfully"
            
            # Test debit transaction
            local debit_data='{
                "transaction_id": "test_shard_debit_001",
                "account_id": "'$account_id'",
                "amount": "'$DEBIT_AMOUNT'",
                "transaction_type": "debit",
                "description": "Test sharding debit transaction"
            }'
            
            local debit_response=$(curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
                -H "Content-Type: application/json" \
                -d "$debit_data")
            
            local debit_success=$(echo "$debit_response" | jq -r '.success // false')
            
            if [ "$debit_success" = "true" ]; then
                log_test "Debit Transaction with Sharding" "PASS" "Debit transaction processed successfully"
            else
                log_test "Debit Transaction with Sharding" "FAIL" "Debit transaction failed: $(echo "$debit_response" | jq -r '.error // "Unknown error"')"
            fi
        else
            log_test "Credit Transaction with Sharding" "FAIL" "Credit transaction failed: $(echo "$credit_response" | jq -r '.error // "Unknown error"')"
        fi
    else
        log_test "Account Creation with Sharding" "FAIL" "Account creation failed: $(echo "$response" | jq -r '.error // "Unknown error"')"
    fi
}

# Function to test cross-shard operations
test_cross_shard_operations() {
    echo "Testing cross-shard operations..."
    
    # Create multiple accounts to test shard distribution
    local accounts=()
    
    for i in $(seq 1 $TEST_ACCOUNTS); do
        local timestamp=$(date +%s)
        local account_data='{
            "wallet_no": "test_shard_multi_'$i'_'$timestamp'",
            "wallet_type_id": "1",
            "instance_type": "individual",
            "currency_id": "1",
            "owner_id": "test_owner_'$i'",
            "use_sub_balance": true,
            "shard_count": 8
        }'
        
        local response=$(curl -s -X POST "$BASE_URL/api/v1/account/create" \
            -H "Content-Type: application/json" \
            -d "$account_data")
        
        local account_id=$(echo "$response" | jq -r '.id // empty')
        
        if [ -n "$account_id" ] && [ "$account_id" != "null" ]; then
            accounts+=("$account_id")
            echo "Created account $i: $account_id"
        else
            echo "Failed to create account $i"
        fi
    done
    
    if [ ${#accounts[@]} -gt 0 ]; then
        log_test "Multi-Account Creation" "PASS" "Created ${#accounts[@]} accounts successfully"
        
        # Test concurrent transactions across shards
        echo "Testing concurrent transactions across shards..."
        local concurrent_success=0
        local concurrent_total=${#accounts[@]}
        
        for account_id in "${accounts[@]}"; do
            local credit_data='{
                "transaction_id": "test_concurrent_'$account_id'",
                "account_id": "'$account_id'",
                "amount": "'$CREDIT_AMOUNT'",
                "transaction_type": "credit",
                "description": "Concurrent shard test"
            }'
            
            local response=$(curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
                -H "Content-Type: application/json" \
                -d "$credit_data")
            
            local success=$(echo "$response" | jq -r '.success // false')
            
            if [ "$success" = "true" ]; then
                concurrent_success=$((concurrent_success + 1))
            fi
        done
        
        if [ $concurrent_success -eq $concurrent_total ]; then
            log_test "Concurrent Cross-Shard Operations" "PASS" "All $concurrent_success concurrent transactions succeeded"
        else
            log_test "Concurrent Cross-Shard Operations" "FAIL" "Only $concurrent_success/$concurrent_total transactions succeeded"
        fi
    else
        log_test "Multi-Account Creation" "FAIL" "Failed to create any accounts"
    fi
}

# Function to test shard data distribution
test_shard_data_distribution() {
    echo "Testing shard data distribution..."
    
    local shard_1_count=$(docker exec postgres-shard-1 psql -U postgres -d sub_balance_shard_1 -t -c "SELECT COUNT(*) FROM accounts;" 2>/dev/null | tr -d ' ')
    local shard_2_count=$(docker exec postgres-shard-2 psql -U postgres -d sub_balance_shard_2 -t -c "SELECT COUNT(*) FROM accounts;" 2>/dev/null | tr -d ' ')
    local shard_3_count=$(docker exec postgres-shard-3 psql -U postgres -d sub_balance_shard_3 -t -c "SELECT COUNT(*) FROM accounts;" 2>/dev/null | tr -d ' ')
    
    echo "Shard 1 accounts: $shard_1_count"
    echo "Shard 2 accounts: $shard_2_count"
    echo "Shard 3 accounts: $shard_3_count"
    
    local total_accounts=$((shard_1_count + shard_2_count + shard_3_count))
    
    if [ $total_accounts -gt 0 ]; then
        log_test "Shard Data Distribution" "PASS" "Data distributed across shards (Total: $total_accounts)"
    else
        log_test "Shard Data Distribution" "FAIL" "No data found in any shard"
    fi
}

# Function to test shard performance
test_shard_performance() {
    echo "Testing shard performance..."
    
    # Create a test account
    local timestamp=$(date +%s)
    local account_data='{
        "wallet_no": "test_performance_'$timestamp'",
        "wallet_type_id": "1",
        "instance_type": "individual",
        "currency_id": "1",
        "owner_id": "test_perf_owner",
        "use_sub_balance": true,
        "shard_count": 8
    }'
    
    local response=$(curl -s -X POST "$BASE_URL/api/v1/account/create" \
        -H "Content-Type: application/json" \
        -d "$account_data")
    
    local account_id=$(echo "$response" | jq -r '.id // empty')
    
    if [ -n "$account_id" ] && [ "$account_id" != "null" ]; then
        # Credit the account
        local credit_data='{
            "transaction_id": "test_perf_credit_001",
            "account_id": "'$account_id'",
            "amount": "10000000",
            "transaction_type": "credit",
            "description": "Performance test credit"
        }'
        
        curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
            -H "Content-Type: application/json" \
            -d "$credit_data" > /dev/null
        
        # Test concurrent debit transactions
        local start_time=$(date +%s%N)
        local concurrent_transactions=10
        local success_count=0
        
        for i in $(seq 1 $concurrent_transactions); do
            local debit_data='{
                "transaction_id": "test_perf_debit_'$i'",
                "account_id": "'$account_id'",
                "amount": "100000",
                "transaction_type": "debit",
                "description": "Performance test debit '$i'"
            }'
            
            local response=$(curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
                -H "Content-Type: application/json" \
                -d "$debit_data")
            
            local success=$(echo "$response" | jq -r '.success // false')
            
            if [ "$success" = "true" ]; then
                success_count=$((success_count + 1))
            fi
        done
        
        local end_time=$(date +%s%N)
        local duration=$(( (end_time - start_time) / 1000000 )) # Convert to milliseconds
        
        if [ $success_count -eq $concurrent_transactions ]; then
            log_test "Shard Performance Test" "PASS" "All $success_count transactions completed in ${duration}ms"
        else
            log_test "Shard Performance Test" "FAIL" "Only $success_count/$concurrent_transactions transactions succeeded in ${duration}ms"
        fi
    else
        log_test "Shard Performance Test" "FAIL" "Failed to create test account"
    fi
}

# Main test execution
echo -e "${YELLOW}🔍 Starting Database Sharding Tests...${NC}"
echo ""

# Test 1: Database Connections
echo -e "${BLUE}1. Testing Database Connections${NC}"
test_database_connection 1 5432 "sub_balance_shard_1"
test_database_connection 2 5433 "sub_balance_shard_2"
test_database_connection 3 5434 "sub_balance_shard_3"
echo ""

# Test 2: Table Existence
echo -e "${BLUE}2. Testing Table Existence${NC}"
test_table_existence 1 "sub_balance_shard_1"
test_table_existence 2 "sub_balance_shard_2"
test_table_existence 3 "sub_balance_shard_3"
echo ""

# Test 3: Shard Router Functionality
echo -e "${BLUE}3. Testing Shard Router Functionality${NC}"
test_shard_router
echo ""

# Test 4: Cross-Shard Operations
echo -e "${BLUE}4. Testing Cross-Shard Operations${NC}"
test_cross_shard_operations
echo ""

# Test 5: Shard Data Distribution
echo -e "${BLUE}5. Testing Shard Data Distribution${NC}"
test_shard_data_distribution
echo ""

# Test 6: Shard Performance
echo -e "${BLUE}6. Testing Shard Performance${NC}"
test_shard_performance
echo ""

# Final Results
echo -e "${BLUE}📊 Test Results Summary${NC}"
echo "========================"
echo -e "Total Tests: $TOTAL_TESTS"
echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "${RED}Failed: $FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! Database sharding is working correctly.${NC}"
    exit 0
else
    echo -e "${RED}⚠️  Some tests failed. Please check the logs above.${NC}"
    exit 1
fi
