#!/bin/bash

# RediSwap Test Script - Reproduces Paper Example 1

set -e

echo "==================================================="
echo "RediSwap Distributed Test - Example 1"
echo "==================================================="
echo ""

# Kill any existing server
pkill -f "bin/server" 2>/dev/null || true
sleep 1

# Start server
echo "Starting server..."
./bin/server -port 8081 > /tmp/rediswap_server.log 2>&1 &
SERVER_PID=$!
sleep 2

# Register arbitragers
echo "Registering arbitragers..."
./bin/arbitrager -server http://localhost:8081 -id arb1 -belief 4.0 2>&1 | grep "Belief registered"
./bin/arbitrager -server http://localhost:8081 -id arb2 -belief 1.0 2>&1 | grep "Belief registered"

# Send transactions
echo "Sending transactions..."
./bin/user -server http://localhost:8081 -direction "X->Y" -input 8 -output 25 2>&1 | grep "Swap submitted"
./bin/user -server http://localhost:8081 -direction "X->Y" -input 30 -output 12 2>&1 | grep "Swap submitted"
./bin/user -server http://localhost:8081 -direction "Y->X" -input 20 -output 10 2>&1 | grep "Swap submitted"

# Process block
echo ""
echo "Processing block..."
echo ""
curl -s -X POST http://localhost:8081 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}' | python3 -m json.tool

echo ""
echo ""
echo "==================================================="
echo "Expected Results (from paper):"
echo "  TX1: winner=arb1, payment=0"
echo "  TX2: winner=arb1, payment=18"
echo "  TX3: winner=arb2, payment=0"
echo "  Rebalancing: winner=arb2, payment=36"
echo "==================================================="

# Cleanup
kill $SERVER_PID 2>/dev/null
echo ""
echo "Test complete!"
