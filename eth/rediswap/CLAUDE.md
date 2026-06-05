# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build all components
go mod tidy
mkdir -p bin
go build -o bin/server ./cmd/server
go build -o bin/user ./cmd/user
go build -o bin/arbitrager ./cmd/arbitrager

# Run complete demo (NEW - recommended!)
./demo.sh

# Run quick test
./quick_test.sh

# Run the automated test (reproduces paper Example 1)
./test.sh

# Run server with auto-processing (NEW!)
./bin/server -port 8080 -pool-x 4 -pool-y 100 -auto-process -process-interval 10

# Run server without auto-processing (original mode)
./bin/server -port 8080 -pool-x 4 -pool-y 100

# Run user client in continuous mode (NEW!)
./bin/user -server http://localhost:8080 -direction "X->Y" -input 8 -output 25 -continuous -interval 3

# Run user client in single-shot mode
./bin/user -server http://localhost:8080 -direction "X->Y" -input 8 -output 25

# Run arbitrager client in continuous mode (NEW!)
./bin/arbitrager -server http://localhost:8080 -id arb1 -belief 4.0 -continuous -interval 5

# Run arbitrager client in single-shot mode
./bin/arbitrager -server http://localhost:8080 -id arb1 -belief 4.0

# Process block manually via curl (only needed if auto-process disabled)
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}'
```

## Architecture

This is a **distributed RediSwap implementation** based on the paper "RediSwap: MEV Redistribution Mechanism for CFMMs" (Zhang et al., 2025). It implements a second-price auction mechanism for MEV redistribution in AMMs.

### Data Flow

```
User → rediswap_sendSwap → Server (TransactionStore)
Arbitrager → rediswap_sendBelief → Server (BeliefStore)
Trigger → rediswap_processBlock → Auction Engine → Bundle Generation → Refunds
```

### Components

**Server** (`cmd/server`): HTTP server that receives transactions and beliefs, runs auctions when `rediswap_processBlock` is called or automatically via auto-processor worker. Manages the AMM pool state (constant product x*y=k) and coordinates the entire auction process.
- **NEW**: Supports `-auto-process` flag to enable automatic block processing at regular intervals
- **NEW**: Auto-processor worker runs in background goroutine, similar to rebate's simulation worker

**User Client** (`cmd/user`): Submits swap transactions with direction (X→Y or Y→X), input amount, and minimum output. Transactions are stored in `TransactionStore` until block processing.
- **NEW**: Supports `-continuous` flag to run continuously and send transactions at regular intervals
- **NEW**: Configurable interval via `-interval` flag (default: 3 seconds)
- **NEW**: Enhanced logging with transaction counts and status

**Arbitrager Client** (`cmd/arbitrager`): Submits price beliefs (external market price: 1X = belief*Y). Multiple arbitragers compete in auctions by submitting different beliefs.
- **NEW**: Supports `-continuous` flag to run continuously and update beliefs at regular intervals
- **NEW**: Configurable interval via `-interval` flag (default: 5 seconds)
- **NEW**: Enhanced logging with update counts and status

**API Layer** (`api/api.go`): JSON-RPC 2.0 handler with three methods:
- `rediswap_sendSwap`: Accepts user transactions
- `rediswap_sendBelief`: Accepts arbitrager beliefs
- `rediswap_processBlock`: Triggers auction execution

**Auction Engine** (`internal/auction/auction.go`): Core MEV calculation and auction logic:
- **Transaction MEV**: `Φ = Δx · q + Δy` where (Δx, Δy) is net pool impact at limit state, q is arbitrager's belief
- **Rebalancing MEV**: `φ = (x·q + y) - (x̂·q + ŷ)` where (x̂, ŷ) is no-arbitrage state
- **Second-price auction**: Winner pays second-highest bid

**Storage** (`internal/store/`): In-memory stores cleared after each block:
- `TransactionStore`: Pending swap transactions
- `BeliefStore`: Arbitrager beliefs (map[arbID]belief)

**Pool** (`internal/pool/pool.go`): Constant product AMM (x*y=k) with swap execution and price calculation. Uses `github.com/shopspring/decimal` for precise arithmetic.

### Key Design Points

- **Block-based processing**: Transactions and beliefs accumulate, then `processBlock` runs all auctions atomically
- **Auto-processing mode (NEW)**: Server can automatically process blocks at regular intervals without manual triggering
- **Independent auctions**: Each transaction gets its own second-price auction, plus one rebalancing auction for LP LVR
- **Sandwich bundles**: Winners get front-run → user tx → back-run structure (simplified, not actual execution)
- **Refunds**: Transaction auction payments go to users, rebalancing payments go to LP
- **No persistence**: All storage is in-memory, cleared after block processing
- **Continuous mode (NEW)**: Users and arbitragers can run continuously, similar to rebate's clients
- **Complete demo system (NEW)**: Like rebate, now has a full demo with all entities running independently
- **Comparison with rebate**: Similar distributed architecture (server + clients + JSON-RPC) but different MEV mechanism (auctions vs simulation)

## Expected Results

Running `./test.sh` should reproduce paper Example 1:
- Pool: (4, 100), k=400
- TX1 (X→Y, 8→25): winner=arb1, payment=0
- TX2 (X→Y, 30→12): winner=arb1, payment=18 → refund to user
- TX3 (Y→X, 20→10): winner=arb2, payment=0
- Rebalancing: winner=arb2, payment=36 → refund to LP

## Demo System (NEW!)

RediSwap now includes a complete demo system similar to rebate, with multiple entities running independently:

### Quick Start
```bash
# Run the complete demo with all components
./demo.sh
```

This starts:
- 1 Server with auto-processing (every 10 seconds)
- 2 Arbitragers continuously updating beliefs
- 2 Users continuously sending swap transactions

### Manual Setup (Multiple Terminals)
```bash
# Terminal 1: Server
./bin/server -port 8080 -pool-x 4 -pool-y 100 -auto-process -process-interval 10

# Terminal 2: Arbitrager 1
./bin/arbitrager -server http://localhost:8080 -id arb1 -belief 4.0 -continuous

# Terminal 3: Arbitrager 2
./bin/arbitrager -server http://localhost:8080 -id arb2 -belief 5.0 -continuous

# Terminal 4: User 1 (X->Y swaps)
./bin/user -server http://localhost:8080 -direction "X->Y" -input 8 -output 25 -continuous

# Terminal 5: User 2 (Y->X swaps)
./bin/user -server http://localhost:8080 -direction "Y->X" -input 30 -output 12 -continuous
```

### Monitoring
```bash
# Watch server logs
tail -f logs/server.log

# Watch client logs
tail -f logs/user1.log
tail -f logs/arb1.log
```

See [README_DEMO.md](./README_DEMO.md) for complete documentation.

## Important Notes

- The `Sqrt` function uses `math.Sqrt` because `decimal.Pow(0.5)` doesn't work correctly
- Transaction directions must be exactly "X->Y" or "Y->X" (case-sensitive)
- Server must be started before clients can connect
- `processBlock` clears all pending transactions and beliefs after execution
- **NEW**: Auto-processing mode eliminates need for manual `processBlock` calls
- **NEW**: Continuous mode allows clients to run indefinitely, similar to rebate
- **NEW**: Enhanced logging provides detailed auction results and bundle information
