#!/bin/bash

# RediSwap Test Script - Multiple Examples

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REDISWAP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REDISWAP_DIR"

run_example_1() {
  echo "==================================================="
  echo "Example 1: Paper Reference Case"
  echo "Pool: (4, 100), Two arbitragers, Mixed transactions"
  echo "==================================================="
  echo ""

  # Kill any existing server
  pkill -f "bin/server" 2>/dev/null || true
  sleep 1

  # Start server
  echo "Starting server..."
  ./bin/server -port 8081 -pool-x 4 -pool-y 100 > /tmp/rediswap_server.log 2>&1 &
  SERVER_PID=$!
  sleep 2

  # Register arbitragers
  echo "Registering arbitragers..."
  ./bin/arbitrager -server http://localhost:8081 -id arb1 -belief 4.0 2>&1 | grep "Belief accepted by server"
  ./bin/arbitrager -server http://localhost:8081 -id arb2 -belief 1.0 2>&1 | grep "Belief accepted by server"

  # Send transactions
  echo "Sending transactions..."
  ./bin/user -server http://localhost:8081 -direction "X->Y" -input 8 -output 25 2>&1 | grep "Swap accepted by server"
  ./bin/user -server http://localhost:8081 -direction "X->Y" -input 30 -output 12 2>&1 | grep "Swap accepted by server"
  ./bin/user -server http://localhost:8081 -direction "Y->X" -input 20 -output 10 2>&1 | grep "Swap accepted by server"

  # Process block
  echo ""
  echo "Processing block..."
  echo ""
  curl -s -X POST http://localhost:8081 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}' | python3 -m json.tool

  echo ""
  echo "Expected: TX1(arb1,0), TX2(arb1,18), TX3(arb2,0), Rebalancing(arb2,36)"
  echo ""

  # Cleanup
  kill $SERVER_PID 2>/dev/null
}

run_example_2() {
  echo "==================================================="
  echo "Example 2: Competitive Arbitrage"
  echo "Pool: (10, 200), Three arbitragers, High competition"
  echo "==================================================="
  echo ""

  # Kill any existing server
  pkill -f "bin/server" 2>/dev/null || true
  sleep 1

  # Start server with different pool
  echo "Starting server..."
  ./bin/server -port 8082 -pool-x 10 -pool-y 200 > /tmp/rediswap_server2.log 2>&1 &
  SERVER_PID=$!
  sleep 2

  # Register three arbitragers with competitive beliefs
  echo "Registering arbitragers..."
  ./bin/arbitrager -server http://localhost:8082 -id arb1 -belief 25.0 2>&1 | grep "Belief accepted by server"
  ./bin/arbitrager -server http://localhost:8082 -id arb2 -belief 20.0 2>&1 | grep "Belief accepted by server"
  ./bin/arbitrager -server http://localhost:8082 -id arb3 -belief 15.0 2>&1 | grep "Belief accepted by server"

  # Send transactions
  echo "Sending transactions..."
  ./bin/user -server http://localhost:8082 -direction "X->Y" -input 5 -output 80 2>&1 | grep "Swap accepted by server"
  ./bin/user -server http://localhost:8082 -direction "Y->X" -input 50 -output 2 2>&1 | grep "Swap accepted by server"

  # Process block
  echo ""
  echo "Processing block..."
  echo ""
  curl -s -X POST http://localhost:8082 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}' | python3 -m json.tool

  echo ""
  echo "Expected: Second-price auction with 3 bidders"
  echo ""

  # Cleanup
  kill $SERVER_PID 2>/dev/null
}

run_example_3() {
  echo "==================================================="
  echo "Example 3: Large Pool with Y->X Dominant"
  echo "Pool: (50, 1000), Reverse direction focus"
  echo "==================================================="
  echo ""

  # Kill any existing server
  pkill -f "bin/server" 2>/dev/null || true
  sleep 1

  # Start server with larger pool
  echo "Starting server..."
  ./bin/server -port 8083 -pool-x 50 -pool-y 1000 > /tmp/rediswap_server3.log 2>&1 &
  SERVER_PID=$!
  sleep 2

  # Register arbitragers
  echo "Registering arbitragers..."
  ./bin/arbitrager -server http://localhost:8083 -id arb1 -belief 22.0 2>&1 | grep "Belief accepted by server"
  ./bin/arbitrager -server http://localhost:8083 -id arb2 -belief 18.0 2>&1 | grep "Belief accepted by server"

  # Send transactions - mostly Y->X
  echo "Sending transactions..."
  ./bin/user -server http://localhost:8083 -direction "Y->X" -input 200 -output 8 2>&1 | grep "Swap accepted by server"
  ./bin/user -server http://localhost:8083 -direction "Y->X" -input 150 -output 5 2>&1 | grep "Swap accepted by server"
  ./bin/user -server http://localhost:8083 -direction "X->Y" -input 10 -output 150 2>&1 | grep "Swap accepted by server"

  # Process block
  echo ""
  echo "Processing block..."
  echo ""
  curl -s -X POST http://localhost:8083 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}' | python3 -m json.tool

  echo ""
  echo "Expected: Rebalancing MEV with Y->X dominant flow"
  echo ""

  # Cleanup
  kill $SERVER_PID 2>/dev/null
}

# Run all examples
echo ""
echo "######################################################"
echo "# RediSwap Distributed Test Suite"
echo "######################################################"
echo ""

run_example_1
# echo ""
# echo ""

# run_example_2
# echo ""
# echo ""

# run_example_3

