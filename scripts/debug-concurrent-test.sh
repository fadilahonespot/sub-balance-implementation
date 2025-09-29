#!/bin/bash

# Debug concurrent test script
BASE_URL="http://localhost:8080"
ACCOUNT_ID="7d2f3856-f666-4bd0-9baa-9868ba335908"
CONCURRENT_REQUESTS=10

echo "🔍 Debug Concurrent Test"
echo "========================"
echo "Testing $CONCURRENT_REQUESTS concurrent requests..."

# Function to make a single request
make_request() {
    local i=$1
    local txn_id="debug_concurrent_$i"
    
    echo "Starting request $i..."
    
    local response=$(curl -s -X POST "$BASE_URL/api/v1/transaction/execute" \
        -H "Content-Type: application/json" \
        -d "{
            \"transaction_id\": \"$txn_id\",
            \"account_id\": \"$ACCOUNT_ID\",
            \"transaction_type\": \"debit\",
            \"amount\": \"100\",
            \"description\": \"Debug concurrent test $i\",
            \"metadata\": {\"source\":\"debug_concurrent\",\"request_id\":\"$i\"}
        }")
    
    local status=$(echo "$response" | jq -r '.status // "error"')
    local error=$(echo "$response" | jq -r '.error // empty')
    
    if [ "$status" = "completed" ]; then
        echo "✅ Request $i: SUCCESS"
        echo "SUCCESS"
    else
        echo "❌ Request $i: FAILED - $error"
        echo "FAILED: $error"
    fi
}

# Start concurrent requests
echo "Starting $CONCURRENT_REQUESTS concurrent requests..."
start_time=$(date +%s%3N)

# Launch all requests in background
for i in $(seq 1 $CONCURRENT_REQUESTS); do
    make_request $i &
done

# Wait for all background jobs to complete
wait

end_time=$(date +%s%3N)
duration=$((end_time - start_time))

echo ""
echo "📊 Results:"
echo "==========="
echo "Total requests: $CONCURRENT_REQUESTS"
echo "Duration: ${duration}ms"
echo "Average per request: $((duration / CONCURRENT_REQUESTS))ms"

# Count results
success_count=$(grep -c "SUCCESS" <<< "$(jobs -p | xargs -I {} cat /dev/fd/{} 2>/dev/null)" || echo "0")
failed_count=$((CONCURRENT_REQUESTS - success_count))

echo "Successful: $success_count"
echo "Failed: $failed_count"
if [ "$CONCURRENT_REQUESTS" -gt 0 ]; then
    echo "Success rate: $((success_count * 100 / CONCURRENT_REQUESTS))%"
else
    echo "Success rate: 0%"
fi

echo ""
echo "🎯 Debug test completed!"
