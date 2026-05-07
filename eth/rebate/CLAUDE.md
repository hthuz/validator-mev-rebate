# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the server
go run ./cmd/server/main.go -port 8080

# Build
go build ./cmd/server
go build ./cmd/user

# Run all tests
go test ./...

# Run a single test
go test ./internal/sim/... -run TestName
go test ./pkg/utils/... -run TestName

# Run the example user client
go run ./cmd/user/main.go
```

## Architecture

This is a **MEV-Share compatible validator rebate node** written in Go. It accepts MEV bundles from searchers, simulates them, tracks metrics, and distributes hints to builders.

### Data Flow

```
User Request → API Validation → BundleStore → SimulationQueue →
SimulationWorker → Simulator → MetricsStore + HintBroadcast → Builders
```

### Layers

**API Layer** (`api/`): JSON-RPC handler (`MevShareAPI`) exposes `mev_sendBundle`, `mev_simBundle`, and `eth_cancelBundleByHash`. `root_handler.go` serves the embedded web UI and routes HTTP requests. Rate limiting is in `rate.go`.

**Processing Layer** (`internal/queue/`, `internal/sim/`): `SimulationQueue` is a priority queue ordered by target block. `SimulationWorker` dequeues bundles when their target block is reached, runs simulation, extracts hints, and records metrics. `BundleStore` is an in-memory store indexed by bundle hash and matching hash (for backrun lookups).

**Metrics Layer** (`internal/metrics/`): Aggregates MEV data at three levels — per-block, per-validator, per-searcher — plus global totals. Exposed via REST endpoints at `/metrics/*`.

**Types** (`pkg/types/`): Core structs — `SendMevBundleArgs`, `MevBundleBody`, `MevBundleInclusion`, `MevBundlePrivacy`, `SimMevBundleResponse`.

**Utilities** (`pkg/utils/`): Bundle hashing, matching hash calculation, transaction sender extraction, and bundle validation (version, block range, nesting depth).

### Key Design Points

- **Backrun support**: Bundles can reference other bundles via `matchingHash` in `MevBundleBody`, enabling MEV-Share privacy-preserving backruns.
- **All storage is in-memory**: No persistence layer exists.
- **Concurrency**: Queue uses `sync.Cond` for signaling workers; stores use `sync.Mutex`.
- **Mock simulator**: `internal/sim/mock_sim.go` implements the `SimBackend` interface for tests.
- **Entry point**: `cmd/server/main.go` wires all components together and starts background goroutines for simulation and block updates.
