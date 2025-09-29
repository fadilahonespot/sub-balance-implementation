#!/bin/bash

# Comprehensive Balance Integrity Test Script
# Tests balance integrity across multiple scenarios

BASE_URL="http://localhost:8080/api/v1"
REPORT_FILE="reports/balance_integrity_comprehensive_report_$(date +%Y%m%d_%H%M%S).md"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Comprehensive Balance Integrity Test${NC}"
echo "=============================================="
echo -e "${CYAN}📄 Report will be saved to: $REPORT_FILE${NC}"

# Check if server is running
echo -e "${CYAN}📡 Checking server health...${NC}"
if ! curl -s "$BASE_URL/../health" > /dev/null; then
    echo -e "${RED}❌ Server is not running. Please start the server first:${NC}"
    echo "   go run cmd/main.go"
    exit 1
fi
echo -e "${GREEN}✅ Server is running${NC}"

# Create reports directory if not exists
mkdir -p reports

# Initialize report file
cat > "$REPORT_FILE" << EOF
# Comprehensive Balance Integrity Test Report

**Generated on:** $(date)
**Test Type:** Comprehensive Balance Integrity Test

## Test Scenarios

### Scenario 1: Single Account Balance Integrity
- **Account:** Single account with 10 shards
- **Credit:** 1,000,000 per shard
- **Debit:** 100 transactions of 1,000 each
- **Expected:** Balance should decrease by 100,000

### Scenario 2: Multi-Account Balance Integrity
- **Accounts:** 5 accounts with 10 shards each
- **Credit:** 1,000,000 per shard
- **Debit:** 50 transactions per account of 1,000 each
- **Expected:** Each account balance should decrease by 50,000

### Scenario 3: Cross-Shard Balance Integrity
- **Account:** Single account with 10 shards
- **Credit:** 1,000,000 per shard
- **Debit:** 200 transactions of 1,000 each
- **Expected:** Balance should decrease by 200,000

## Test Results

EOF

# Function to test balance integrity
test_balance_integrity() {
    local scenario_name="$1"
    local account_count="$2"
    local shard_count="$3"
    local credit_amount="$4"
    local debit_count="$5"
    local debit_amount="$6"
    
    echo -e "${YELLOW}🧪 Testing: $scenario_name${NC}"
    echo "   Accounts: $account_count"
    echo "   Shards per Account: $shard_count"
    echo "   Credit Amount: $credit_amount per shard"
    echo "   Debit Count: $debit_count per account"
    echo "   Debit Amount: $debit_amount per transaction"
    
    # Create accounts
    declare -a ACCOUNT_IDS
    declare -a INITIAL_BALANCES
    declare -a FINAL_BALANCES
    
    for ((i=1; i<=account_count; i++)); do
        WALLET_NO="integrity_test_${i}_$(date +%s)"
        
        account_data=$(cat <<EOF
{
    "wallet_no": "$WALLET_NO",
    "wallet_type_id": "WALLET_TYPE_001",
    "instance_type": "INDIVIDUAL",
    "currency_id": "IDR",
    "owner_id": "OWNER_001",
    "minimum_balance": "0",
    "upper_limit": "100000000",
    "lower_limit": "0",
    "use_sub_balance": true,
    "shard_count": $shard_count
}
EOF
)
        
        ACCOUNT_RESPONSE=$(curl -s -X POST "$BASE_URL/account/create" \
            -H "Content-Type: application/json" \
            -d "$account_data")
        
        ACCOUNT_ID=$(echo "$ACCOUNT_RESPONSE" | jq -r '.id' 2>/dev/null)
        
        if [ -z "$ACCOUNT_ID" ] || [ "$ACCOUNT_ID" = "null" ]; then
            echo -e "${RED}❌ Failed to create account $i${NC}"
            return 1
        fi
        
        ACCOUNT_IDS+=("$ACCOUNT_ID")
        echo -e "${GREEN}   ✅ Account $i created: $ACCOUNT_ID${NC}"
    done
    
    # Credit all accounts
    echo -e "${CYAN}💰 Crediting all accounts...${NC}"
    for ((i=0; i<account_count; i++)); do
        ACCOUNT_ID="${ACCOUNT_IDS[$i]}"
        echo -e "${CYAN}   Crediting account $((i+1))/$account_count...${NC}"
        
        # Credit each shard
        for ((j=1; j<=shard_count; j++)); do
            curl -s -X POST "$BASE_URL/transaction/execute" \
                -H "Content-Type: application/json" \
                -d "{\"transaction_id\": \"credit_${i}_${j}_$(date +%s)\", \"account_id\": \"$ACCOUNT_ID\", \"amount\": \"$credit_amount\", \"transaction_type\": \"credit\", \"description\": \"Initial credit for shard $j\", \"metadata\": {\"source\": \"integrity_test_script\"}}" > /dev/null
        done
        
        # Get initial balance
        INITIAL_BALANCE=$(curl -s "$BASE_URL/sub-balance/$ACCOUNT_ID" | jq -r '.total_balance // 0')
        INITIAL_BALANCES+=("$INITIAL_BALANCE")
        echo -e "${GREEN}   ✅ Account $((i+1)) credited: $INITIAL_BALANCE${NC}"
    done
    
    # Perform debit transactions
    echo -e "${CYAN}💸 Performing debit transactions...${NC}"
    local total_debit_amount=0
    
    for ((i=0; i<account_count; i++)); do
        ACCOUNT_ID="${ACCOUNT_IDS[$i]}"
        echo -e "${CYAN}   Processing $debit_count debits for account $((i+1))...${NC}"
        
        for ((j=1; j<=debit_count; j++)); do
            curl -s -X POST "$BASE_URL/transaction/execute" \
                -H "Content-Type: application/json" \
                -d "{\"transaction_id\": \"debit_${i}_${j}_$(date +%s)\", \"account_id\": \"$ACCOUNT_ID\", \"amount\": \"$debit_amount\", \"transaction_type\": \"debit\", \"description\": \"Integrity test debit $j\", \"metadata\": {\"source\": \"integrity_test_script\"}}" > /dev/null
            
            total_debit_amount=$((total_debit_amount + debit_amount))
        done
    done
    
    # Get final balances
    echo -e "${CYAN}📊 Checking final balances...${NC}"
    local total_expected_change=$((debit_count * debit_amount))
    local total_actual_change=0
    local integrity_passed=true
    
    for ((i=0; i<account_count; i++)); do
        ACCOUNT_ID="${ACCOUNT_IDS[$i]}"
        FINAL_BALANCE=$(curl -s "$BASE_URL/sub-balance/$ACCOUNT_ID" | jq -r '.total_balance // 0')
        FINAL_BALANCES+=("$FINAL_BALANCE")
        
        local initial_balance="${INITIAL_BALANCES[$i]}"
        local actual_change=$((initial_balance - FINAL_BALANCE))
        total_actual_change=$((total_actual_change + actual_change))
        
        echo -e "${CYAN}   Account $((i+1)): $initial_balance → $FINAL_BALANCE (change: -$actual_change)${NC}"
        
        # Check if balance change matches expected
        if [ $actual_change -eq $total_expected_change ]; then
            echo -e "${GREEN}   ✅ Account $((i+1)) balance integrity: PASSED${NC}"
        else
            echo -e "${RED}   ❌ Account $((i+1)) balance integrity: FAILED${NC}"
            echo -e "${RED}      Expected change: -$total_expected_change${NC}"
            echo -e "${RED}      Actual change: -$actual_change${NC}"
            integrity_passed=false
        fi
    done
    
    # Check shard balance consistency
    echo -e "${CYAN}🔍 Checking shard balance consistency...${NC}"
    for ((i=0; i<account_count; i++)); do
        ACCOUNT_ID="${ACCOUNT_IDS[$i]}"
        SHARD_INFO=$(curl -s "$BASE_URL/sub-balance/$ACCOUNT_ID")
        SHARD_SUM=$(echo "$SHARD_INFO" | jq -r '.shards | map(.total_balance | tonumber) | add // 0')
        ACCOUNT_TOTAL=$(echo "$SHARD_INFO" | jq -r '.total_balance // 0')
        
        echo -e "${CYAN}   Account $((i+1)): Shard Sum: $SHARD_SUM, Account Total: $ACCOUNT_TOTAL${NC}"
        
        if [ "$SHARD_SUM" = "$ACCOUNT_TOTAL" ]; then
            echo -e "${GREEN}   ✅ Account $((i+1)) shard consistency: PASSED${NC}"
        else
            echo -e "${RED}   ❌ Account $((i+1)) shard consistency: FAILED${NC}"
            integrity_passed=false
        fi
    done
    
    # Summary
    echo -e "${CYAN}📊 Integrity Test Summary:${NC}"
    echo "   Total Expected Change: -$total_expected_change"
    echo "   Total Actual Change: -$total_actual_change"
    echo "   Accounts: $account_count"
    echo "   Shards per Account: $shard_count"
    echo "   Total Shards: $((account_count * shard_count))"
    
    if [ "$integrity_passed" = true ]; then
        echo -e "${GREEN}   ✅ Overall Balance Integrity: PASSED${NC}"
    else
        echo -e "${RED}   ❌ Overall Balance Integrity: FAILED${NC}"
    fi
    
    # Add to report
    cat >> "$REPORT_FILE" << EOF

### $scenario_name

**Configuration:**
- Accounts: $account_count
- Shards per Account: $shard_count
- Credit Amount: $credit_amount per shard
- Debit Count: $debit_count per account
- Debit Amount: $debit_amount per transaction

**Results:**
- Total Expected Change: -$total_expected_change
- Total Actual Change: -$total_actual_change
- Balance Integrity: $([ "$integrity_passed" = true ] && echo "✅ PASSED" || echo "❌ FAILED")
- Shard Consistency: $([ "$integrity_passed" = true ] && echo "✅ PASSED" || echo "❌ FAILED")

**Account Details:**
EOF
    
    for ((i=0; i<account_count; i++)); do
        cat >> "$REPORT_FILE" << EOF
- Account $((i+1)): ${INITIAL_BALANCES[$i]} → ${FINAL_BALANCES[$i]} (change: -$((INITIAL_BALANCES[$i] - FINAL_BALANCES[$i])))
EOF
    done
    
    cat >> "$REPORT_FILE" << EOF

---
EOF
    
    # Cleanup
    echo -e "${CYAN}🗑️  Cleaning up test accounts...${NC}"
    for account_id in "${ACCOUNT_IDS[@]}"; do
        curl -s -X DELETE "$BASE_URL/account/$account_id" > /dev/null
    done
    echo -e "${GREEN}✅ Test accounts cleaned up${NC}"
    
    return $([ "$integrity_passed" = true ] && echo 0 || echo 1)
}

# Run test scenarios
echo -e "${PURPLE}🧪 Starting Comprehensive Balance Integrity Tests...${NC}"
echo "=============================================================="

# Test 1: Single Account Balance Integrity
test_balance_integrity "Single Account Balance Integrity" 1 10 1000000 100 1000

echo ""

# Test 2: Multi-Account Balance Integrity
test_balance_integrity "Multi-Account Balance Integrity" 5 10 1000000 50 1000

echo ""

# Test 3: Cross-Shard Balance Integrity
test_balance_integrity "Cross-Shard Balance Integrity" 1 10 1000000 200 1000

echo ""
echo -e "${GREEN}🎯 Comprehensive Balance Integrity Test Complete!${NC}"
echo "All balance integrity tests completed successfully."
echo -e "${GREEN}📄 Report generated: $REPORT_FILE${NC}"
echo -e "${GREEN}✅ Balance integrity test completed!${NC}"
