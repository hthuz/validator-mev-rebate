# RediSwap - Distributed Architecture

A distributed implementation of RediSwap with separate server, user, and arbitrager components communicating via JSON-RPC.

## Architecture

```
┌─────────┐                    ┌──────────┐
│  User   │──rediswap_sendSwap─▶│          │
└─────────┘                    │          │
                               │  Server  │
┌─────────────┐                │          │
│ Arbitrager  │─sendBelief────▶│          │
└─────────────┘                └──────────┘
                                     │
                               ┌─────▼──────┐
                               │ Process    │
                               │ Block      │
                               └────────────┘
```

### Components

1. **Server** (`cmd/server`):
   - Receives swap transactions from users
   - Receives price beliefs from arbitragers
   - Runs auctions and generates bundles
   - Manages the AMM pool state

2. **User** (`cmd/user`):
   - Submits swap transactions via `rediswap_sendSwap`
   - Specifies direction, input amount, and minimum output

3. **Arbitrager** (`cmd/arbitrager`):
   - Submits price beliefs via `rediswap_sendBelief`
   - Competes in auctions for MEV opportunities

## Quick Start

### 1. Build all components

```bash
go mod tidy

# Build server
go build -o bin/server ./cmd/server

# Build user client
go build -o bin/user ./cmd/user

# Build arbitrager client
go build -o bin/arbitrager ./cmd/arbitrager
```

### 2. Start the server

```bash
./bin/server -port 8080 -pool-x 4 -pool-y 100
```

### 3. Register arbitragers

In separate terminals:

```bash
# Arbitrager 1 (belief = 4)
./bin/arbitrager -id arb1 -belief 4.0

# Arbitrager 2 (belief = 1)
./bin/arbitrager -id arb2 -belief 1.0
```

### 4. Send user transactions

In separate terminals:

```bash
# TX1: X→Y, input=8, output≥25
./bin/user -direction "X->Y" -input 8 -output 25

# TX2: X→Y, input=30, output≥12
./bin/user -direction "X->Y" -input 30 -output 12

# TX3: Y→X, input=20, output≥10
./bin/user -direction "Y->X" -input 20 -output 10
```

### 5. Process the block

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "rediswap_processBlock",
    "params": [],
    "id": 1
  }'
```

## JSON-RPC API

### rediswap_sendSwap

Submit a swap transaction.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_sendSwap",
  "params": [{
    "direction": "X->Y",
    "input": 8.0,
    "output": 25.0
  }],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tx_id": "TX1",
    "status": "pending",
    "direction": "X->Y"
  },
  "id": 1
}
```

### rediswap_sendBelief

Submit an arbitrager's price belief.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_sendBelief",
  "params": [{
    "arb_id": "arb1",
    "belief": 4.0
  }],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "arb_id": "arb1",
    "belief": 4.0,
    "status": "registered"
  },
  "id": 1
}
```

### rediswap_processBlock

Process all pending transactions and run auctions.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "rediswap_processBlock",
  "params": [],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "block_number": 0,
    "bundles": [...],
    "auctions": [...],
    "refunds": [...],
    "rebalancing_winner": "arb2",
    "rebalancing_payment": "36"
  },
  "id": 1
}
```

## Example: Reproduce Paper Example 1

```bash
# Terminal 1: Start server
./bin/server

# Terminal 2: Register arb1
./bin/arbitrager -id arb1 -belief 4.0

# Terminal 3: Register arb2
./bin/arbitrager -id arb2 -belief 1.0

# Terminal 4: Send TX1
./bin/user -direction "X->Y" -input 8 -output 25

# Terminal 5: Send TX2
./bin/user -direction "X->Y" -input 30 -output 12

# Terminal 6: Send TX3
./bin/user -direction "Y->X" -input 20 -output 10

# Terminal 7: Process block
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"rediswap_processBlock","params":[],"id":1}'
```

**Expected Result:**
- TX1: winner=arb1, payment=0
- TX2: winner=arb1, payment=18
- TX3: winner=arb2, payment=0
- Rebalancing: winner=arb2, payment=36

## Command Line Options

### Server
```bash
./bin/server [options]
  -port int       Server port (default: 8080)
  -pool-x float   Initial pool reserve X (default: 4)
  -pool-y float   Initial pool reserve Y (default: 100)
```

### User
```bash
./bin/user [options]
  -server string     Server URL (default: "http://localhost:8080")
  -direction string  Swap direction (default: "X->Y")
  -input float       Input amount (default: 8)
  -output float      Minimum output (default: 25)
  -continuous        Send transactions continuously
```

### Arbitrager
```bash
./bin/arbitrager [options]
  -server string  Server URL (default: "http://localhost:8080")
  -id string      Arbitrager ID (default: "arb1")
  -belief float   Price belief (default: 4.0)
  -continuous     Send beliefs continuously
```

## Project Structure

```
rediswap/
├── cmd/
│   ├── server/         # Server executable
│   ├── user/           # User client
│   └── arbitrager/     # Arbitrager client
├── api/                # JSON-RPC API handlers
├── internal/
│   ├── auction/        # Auction logic and MEV calculation
│   ├── pool/           # AMM pool implementation
│   └── store/          # In-memory transaction and belief stores
├── pkg/
│   └── types/          # Shared data structures
└── go.mod
```

## Comparison with rebate/

Similar architecture to the rebate MEV-Share implementation:
- **Server**: Central coordinator (like rebate's server)
- **User**: Transaction submitter (like rebate's user)
- **Arbitrager**: MEV searcher (like rebate's searcher)
- **JSON-RPC**: Communication protocol
- **In-memory stores**: Transaction and belief storage

Key differences:
- RediSwap uses second-price auctions instead of bundle simulation
- Direct refunds to users and LPs instead of hints/backruns
- Simpler MEV calculation based on price beliefs

## License

MIT
