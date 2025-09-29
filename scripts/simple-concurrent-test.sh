#!/bin/bash

# Simple concurrent test
BASE_URL="http://localhost:8080"
ACCOUNT_ID="7d2f3856-f666-4bd0-9baa-9868ba335908"

echo "🔍 Simple Concurrent Test"
echo "========================="

# Test 1: Single request
echo "Test 1: Single request"
response1=$(curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
    -H "Content-Type: application/json" \
    -d "{
        \"transaction_id\": \"simple_test_1\",
        \"account_id\": \"$ACCOUNT_ID\",
        \"transaction_type\": \"debit\",
        \"amount\": \"100\",
        \"description\": \"Simple test 1\"
    }")

status1=$(echo "$response1" | jq -r '.status // "error"')
echo "Result 1: $status1"

# Test 2: 5 concurrent requests
echo ""
echo "Test 2: 5 concurrent requests"
start_time=$(date +%s%3N)

for i in $(seq 1 5); do
    curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
        -H "Content-Type: application/json" \
        -d "{
            \"transaction_id\": \"concurrent_test_$i\",
            \"account_id\": \"$ACCOUNT_ID\",
            \"transaction_type\": \"debit\",
            \"amount\": \"100\",
            \"description\": \"Concurrent test $i\"
        }" &
done

wait

end_time=$(date +%s%3N)
duration=$((end_time - start_time))

echo "Duration: ${duration}ms"
echo "Average per request: $((duration / 5))ms"

echo ""
echo "🎯 Simple test completed!"
