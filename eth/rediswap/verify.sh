#!/bin/bash

# Final verification script to ensure all components work

echo "=========================================="
echo "RediSwap System Verification"
echo "=========================================="
echo ""

# Check if binaries exist
echo "1. Checking binaries..."
if [ -f "bin/server" ] && [ -f "bin/user" ] && [ -f "bin/arbitrager" ]; then
    echo "✓ All binaries present"
else
    echo "✗ Missing binaries, building..."
    go build -o bin/server ./cmd/server
    go build -o bin/user ./cmd/user
    go build -o bin/arbitrager ./cmd/arbitrager
    echo "✓ Build complete"
fi
echo ""

# Check scripts are executable
echo "2. Checking scripts..."
if [ -x "demo.sh" ] && [ -x "quick_test.sh" ] && [ -x "test.sh" ]; then
    echo "✓ All scripts are executable"
else
    echo "✗ Some scripts not executable, fixing..."
    chmod +x demo.sh quick_test.sh test.sh
    echo "✓ Permissions fixed"
fi
echo ""

# Check documentation
echo "3. Checking documentation..."
docs=("README_DEMO.md" "CLAUDE.md" "IMPROVEMENTS.md")
for doc in "${docs[@]}"; do
    if [ -f "$doc" ]; then
        echo "  ✓ $doc"
    else
        echo "  ✗ $doc missing"
    fi
done
echo ""

# Test compilation
echo "4. Testing compilation..."
if go build -o /tmp/test_server ./cmd/server 2>&1 | grep -q "error"; then
    echo "✗ Server compilation failed"
    exit 1
else
    echo "✓ Server compiles successfully"
fi

if go build -o /tmp/test_user ./cmd/user 2>&1 | grep -q "error"; then
    echo "✗ User compilation failed"
    exit 1
else
    echo "✓ User compiles successfully"
fi

if go build -o /tmp/test_arbitrager ./cmd/arbitrager 2>&1 | grep -q "error"; then
    echo "✗ Arbitrager compilation failed"
    exit 1
else
    echo "✓ Arbitrager compiles successfully"
fi

rm -f /tmp/test_*
echo ""

# Summary
echo "=========================================="
echo "System Status: READY"
echo "=========================================="
echo ""
echo "Available commands:"
echo "  ./demo.sh          - Run complete demo system"
echo "  ./quick_test.sh    - Quick 15-second test"
echo "  ./test.sh          - Original paper Example 1"
echo ""
echo "Manual execution:"
echo "  ./bin/server -auto-process -process-interval 10"
echo "  ./bin/arbitrager -id arb1 -belief 4.0 -continuous"
echo "  ./bin/user -direction 'X->Y' -input 8 -output 25 -continuous"
echo ""
echo "Documentation:"
echo "  README_DEMO.md     - Complete demo guide"
echo "  CLAUDE.md          - Developer reference"
echo "  IMPROVEMENTS.md    - Change summary"
echo ""
