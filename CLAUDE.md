# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Go-based E2E test framework for blockchain cross-rollup transactions. The project tests cross-chain transaction functionality between multiple rollup networks using Ethereum-compatible RPC interfaces and a custom cross-transaction protocol.

## Development Commands

### Building and Testing
```bash
make build          # Build test binary (bin/dome)
make format         # Format code with go fmt
make lint           # Run golangci-lint
make deps           # Download and tidy dependencies
make clean          # Clean build artifacts (removes bin/)
make docker-build   # Build Docker image (dome:latest)
```

### Running Tests

Tests are compiled into a binary (`bin/dome`) and all test targets automatically build this binary if needed. Tests require configuration in `configs/config.yaml` (see Configuration Setup below).

```bash
# Run all tests (automatically builds binary first)
make test

# Run tests with specific log levels
make test-info                           # INFO log level, all tests
make test-info TEST_NAME=TestBridge      # INFO log level, specific test
make test-debug                          # DEBUG log level, all tests
make test-debug TEST_NAME=TestBridge     # DEBUG log level, specific test

# Run specific test suites
make test-bridge                         # Run all bridge tests
make smoke-test                          # Run smoke tests only

# Run the test binary directly
./bin/dome -test.v -test.run=TestSendCrossTxBridge
LOG_LEVEL=INFO ./bin/dome -test.v
```

Log levels are controlled via the `LOG_LEVEL` environment variable (DEBUG, INFO).

### Configuration Setup

Configuration is managed through a single embedded YAML file at `configs/config.yaml`. This file is embedded into the binary at compile time using `//go:embed`.

**Structure:**
```yaml
l2:
  chain-configs:
    rollup-a:
      pk: 0x...        # Private key for funded account on rollup-a
      id: 77777        # Chain ID for rollup-a
      rpc-url: http://localhost:18545
    rollup-b:
      pk: 0x...        # Private key for funded account on rollup-b
      id: 88888        # Chain ID for rollup-b
      rpc-url: http://localhost:28545
  contracts:
    bridge:
      address: 0x...   # Bridge contract address (deployed on both rollups)
      abi: ''          # Bridge contract ABI JSON
    token:
      address: 0x...   # Token contract address (deployed on both rollups)
      abi: ''          # Token contract ABI JSON
    ping-pong:
      address: 0x...   # PingPong contract address (deployed on both rollups)
      abi: ''          # PingPong contract ABI JSON
```

**Setup steps:**
1. If `configs/config.yaml` doesn't exist, `make build` will automatically copy it from `configs/config.example.yaml`
2. Edit `configs/config.yaml` and replace placeholder values with:
   - Private keys for funded accounts (one per rollup)
   - RPC URLs for each rollup
   - Chain IDs for each rollup
   - Contract addresses (bridge, token, ping-pong) deployed on both rollups
   - Contract ABIs as JSON strings
3. Rebuild the binary with `make build` to embed the updated config

**Security**: Never commit actual private keys. `config.yaml` is gitignored.

**CI/CD**: The build process automatically creates `config.yaml` from the example file if it doesn't exist, allowing builds to succeed in CI pipelines with placeholder values.

**Validation**: Config validation happens at package init time. The binary will panic on startup if:
- Both `rollup-a` and `rollup-b` configs are not present
- Any field (`pk`, `id`, `rpc-url`) is missing or zero-valued
- All three contracts (`bridge`, `ping-pong`, `token`) are not present
- Any contract address or ABI is empty

## Architecture

### Directory Structure

```
dome/
├── bin/              # Compiled test binary (bin/dome)
├── build/            # Build artifacts
│   └── Dockerfile    # Multi-stage Docker build
├── configs/          # Configuration management
│   ├── config.go     # Config structs, validation, and embed logic
│   ├── config.yaml   # Main config file (gitignored, embedded at compile time)
│   └── config.example.yaml  # Template for config.yaml
├── internal/         # Core framework (private packages)
│   ├── accounts/     # Account management for blockchain interactions
│   ├── logger/       # Centralized logging with DEBUG/INFO levels
│   ├── rollup/       # Rollup configuration and connection
│   └── transactions/ # Transaction creation and cross-chain logic
├── pkg/              # Public packages
│   └── rollupv1/     # Protobuf definitions for cross-rollup protocol
└── test/             # Test files
    ├── config.go     # Test setup and shared test variables
    └── *_test.go     # Test implementations
```

### Core Components

**configs/**: Configuration management with compile-time embedding
- Single YAML file defines both rollup configs with embedded private keys
- Uses `//go:embed` directive to embed config.yaml into the binary
- `configs.Values` global variable provides access to parsed config
- Chain configs accessed via: `configs.Values.L2.ChainConfigs[configs.ChainNameRollupA]`
- Validation runs at init time, panics on invalid config

**internal/accounts/**: Account management for blockchain interactions
- `Account` struct holds private key, address, rollup reference, and ethclient
- `NewRollupAccount(privateKeyHex, rollup)` creates accounts from private key strings
- Accounts are tied to specific rollups and handle nonce/balance queries via ethclient

**internal/rollup/**: Rollup configuration
- `Rollup` struct is a simple holder for RPC URL and chain ID
- `New(rpcURL, chainID)` constructor for creating rollup instances
- No longer loads from YAML - instantiated directly from configs package

**internal/transactions/**: Transaction creation and execution
- `transactions.go`: Standard Ethereum transaction creation (EIP-1559 dynamic fee)
- `cross_tx.go`: Cross-rollup transaction handling using protobuf messages
- `CreateTransaction()` creates and signs transactions with account's nonce
- `SendTransaction()` sends signed transactions to RPC endpoints
- `GetTransactionDetails()` polls for transaction confirmation with 5-second retry intervals

**internal/logger/**: Centralized logging with configurable levels (DEBUG/INFO)

**pkg/rollupv1/**: Protobuf definitions for cross-rollup messaging protocol
- `XTRequest` message contains transactions for multiple chains
- `Message` wrapper for cross-transaction requests
- Custom RPC method: `eth_sendXTransaction` sends cross-rollup transaction bundles

### Cross-Rollup Transaction Flow

1. Create and sign separate transactions for each rollup (RollupA and RollupB)
2. Marshal both signed transactions into an `XTRequest` protobuf message
3. Wrap the XTRequest in a `Message` envelope with sender ID
4. Encode the message using protobuf
5. Send via custom RPC method `eth_sendXTransaction` to one of the rollup nodes
6. The rollup network coordinates execution across both chains

### Test Structure

**test/config.go**:
- Shared test setup with global variables for rollups and accounts
- Loads config from `configs.Values` global
- Parses contract ABIs for Bridge, Token, and PingPong contracts
- `setup()` function initializes rollups, accounts, and ABIs

**Test Files**:
- `bridge_test.go`: Cross-rollup token bridge tests (mint, transfer, receive)
- `smoke_test.go`: Basic smoke tests for quick validation
- `ping_pong_test.go`: Cross-chain message passing tests
- `stress_test.go`: Load and stress testing
- `uncorelatedTx_test.go`: Independent transaction tests

## Key Technical Details

### Configuration Embedding
- Config is embedded at compile time using `//go:embed config.yaml`
- The binary is self-contained and doesn't need external config files at runtime
- Rebuild the binary after changing config.yaml to pick up new values

### Transaction Types
- All transactions use EIP-1559 dynamic fee structure (`DynamicFeeTx`)
- Nonces are managed via `PendingNonceAt()` to handle concurrent transactions
- Gas parameters (GasTipCap, GasFeeCap, Gas) must be specified for each transaction

### Protobuf Message Format
Cross-rollup transactions use a custom protobuf format where each `TransactionRequest` contains:
- `ChainId`: Target chain ID as bytes
- `Transaction`: Array of signed transaction bytes (supports batching)

### RPC Methods
- Standard Ethereum JSON-RPC for single-chain operations
- Custom `eth_sendXTransaction` for cross-rollup atomic transactions

### Docker Deployment
The project includes a production-ready Dockerfile with:
- Multi-stage build for minimal image size (scratch-based final image)
- Automatic config.yaml creation from example during build
- Non-root user (domeuser:domegroup) for security
- Embedded CA certificates for HTTPS RPC calls
- Multi-platform support (linux/amd64, linux/arm64)

Build the image with:
```bash
make docker-build                          # Build dome:latest
make docker-build DOCKER_TAG=v1.0.0        # Build with custom tag
```

Run tests in container:
```bash
docker run --rm dome:latest -test.v -test.run=TestSendCrossTxBridge
docker run --rm -e LOG_LEVEL=DEBUG dome:latest -test.v
```

## Module Path
`github.com/compose-network/dome`

## Go Version
1.25
