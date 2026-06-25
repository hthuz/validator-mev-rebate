#!/bin/bash

# Quick test of the new auto-processing demo system

echo "Starting quick test of RediSwap demo system..."
echo ""

# Create logs directory
mkdir -p logs

# Start server with auto-processing (shorter interval for testing)
echo "1. Starting server with auto-processing (every 5 seconds)..."
./bin/server -port 8080 -pool-x 4 -pool-y 100 -auto-process -process-interval 5 > logs/test_server.log 2>&1 &
SERVER_PID=$!
sleep 2

if ! ps -p $SERVER_PID > /dev/null; then
    echo "✗ Server failed to start"
    cat logs/test_server.log
    exit 1
fi
echo "✓ Server started (PID: $SERVER_PID)"

# Start one arbitrager
echo "2. Starting arbitrager..."
./bin/arbitrager -server http://localhost:8080 -id arb1 -belief 4.0 -continuous -interval 2 > logs/test_arb.log 2>&1 &
ARB_PID=$!
sleep 1
echo "✓ Arbitrager started (PID: $ARB_PID)"

# Start one user
echo "3. Starting user..."
./bin/user -server http://localhost:8080 -direction "X->Y" -input 8 -output 25 -continuous -interval 2 > logs/test_user.log 2>&1 &
USER_PID=$!
sleep 1
echo "✓ User started (PID: $USER_PID)"

echo ""
echo "Test system running for 15 seconds..."
echo "Watching server logs:"
echo ""

# Watch for 15 seconds
timeout 15 tail -f logs/test_server.log || true

echo ""
echo ""
echo "Stopping all processes..."
kill $SERVER_PID $ARB_PID $USER_PID 2>/dev/null || true
sleep 1

echo ""
echo "Test complete! Check logs for details:"
echo "  - Server: logs/test_server.log"
echo "  - Arbitrager: logs/test_arb.log"
echo "  - User: logs/test_user.log"
echo ""

# Check if blocks were processed
if grep -q "Block processing complete" logs/test_server.log; then
    echo "✓ SUCCESS: Blocks were processed automatically!"
else
    echo "✗ FAILED: No blocks were processed"
    echo "Server log:"
    cat logs/test_server.log
    exit 1
fi
