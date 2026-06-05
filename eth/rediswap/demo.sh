#!/bin/bash

# RediSwap Complete Demo Script
# This script demonstrates the full RediSwap system with separate entities running independently

set -e

echo "========================================"
echo "RediSwap Complete Demo"
echo "========================================"
echo ""

# Build binaries
echo "Building binaries..."
go mod tidy
mkdir -p bin
go build -o bin/server ./cmd/server
go build -o bin/user ./cmd/user
go build -o bin/arbitrager ./cmd/arbitrager
echo "✓ Build complete"
echo ""

# Clean up function
cleanup() {
    echo ""
    echo "Shutting down all processes..."
    pkill -f "bin/server" || true
    pkill -f "bin/user" || true
    pkill -f "bin/arbitrager" || true
    sleep 1
    echo "✓ All processes stopped"
}

trap cleanup EXIT

echo "========================================"
echo "Starting RediSwap Demo System"
echo "========================================"
echo ""
echo "Components:"
echo "  - Server: Auto-processing blocks every 10 seconds"
echo "  - 2 Arbitragers: Continuously updating beliefs"
echo "  - 2 Users: Continuously sending swap transactions"
echo ""
echo "Press Ctrl+C to stop all processes"
echo ""
echo "========================================"
echo ""

# Start server with auto-processing
echo "Starting server (pool: x=4, y=100, auto-process every 10s)..."
./bin/server \
    -port 8080 \
    -pool-x 4 \
    -pool-y 100 \
    -auto-process \
    -process-interval 10 \
    > logs/server.log 2>&1 &
SERVER_PID=$!
sleep 2

# Check if server started successfully
if ! ps -p $SERVER_PID > /dev/null; then
    echo "✗ Failed to start server"
    cat logs/server.log
    exit 1
fi
echo "✓ Server started (PID: $SERVER_PID)"
echo ""

# Start arbitrager 1
echo "Starting Arbitrager 1 (arb1, belief=4.0)..."
./bin/arbitrager \
    -server http://localhost:8080 \
    -id arb1 \
    -belief 4.0 \
    -continuous \
    -interval 5 \
    > logs/arb1.log 2>&1 &
ARB1_PID=$!
echo "✓ Arbitrager 1 started (PID: $ARB1_PID)"

# Start arbitrager 2
echo "Starting Arbitrager 2 (arb2, belief=5.0)..."
./bin/arbitrager \
    -server http://localhost:8080 \
    -id arb2 \
    -belief 5.0 \
    -continuous \
    -interval 6 \
    > logs/arb2.log 2>&1 &
ARB2_PID=$!
echo "✓ Arbitrager 2 started (PID: $ARB2_PID)"
echo ""

# Wait a bit for arbitragers to register
sleep 3

# Start user 1 (X->Y swaps)
echo "Starting User 1 (X->Y swaps: 8X for min 25Y)..."
./bin/user \
    -server http://localhost:8080 \
    -direction "X->Y" \
    -input 8 \
    -output 25 \
    -continuous \
    -interval 4 \
    > logs/user1.log 2>&1 &
USER1_PID=$!
echo "✓ User 1 started (PID: $USER1_PID)"

# Start user 2 (Y->X swaps)
echo "Starting User 2 (Y->X swaps: 30Y for min 12X)..."
./bin/user \
    -server http://localhost:8080 \
    -direction "Y->X" \
    -input 30 \
    -output 12 \
    -continuous \
    -interval 7 \
    > logs/user2.log 2>&1 &
USER2_PID=$!
echo "✓ User 2 started (PID: $USER2_PID)"
echo ""

echo "========================================"
echo "All components running!"
echo "========================================"
echo ""
echo "Log files:"
echo "  - Server:      tail -f logs/server.log"
echo "  - Arbitrager 1: tail -f logs/arb1.log"
echo "  - Arbitrager 2: tail -f logs/arb2.log"
echo "  - User 1:      tail -f logs/user1.log"
echo "  - User 2:      tail -f logs/user2.log"
echo ""
echo "Monitoring server logs..."
echo ""

# Follow server logs
tail -f logs/server.log
