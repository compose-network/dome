# Dome

A Go-based E2E test framework for cross-rollup blockchain transactions. Tests cross-chain transaction functionality between multiple rollup networks using Ethereum-compatible RPC interfaces and a custom cross-transaction protocol.

## Features

- **Cross-Rollup Transactions**: Test atomic transaction execution across multiple rollups
- **Custom Protocol**: Uses protobuf-based messaging for cross-chain coordination
- **Embedded Configuration**: Config is embedded at compile time for self-contained binaries
- **Test Binary**: Compiles tests into a standalone executable for easy distribution

## Quick Start

### 1. Install Dependencies

```bash
make deps
```

### 2. Build Test Binary

```bash
make build
```

This will:
- Create `bin/probe` test binary
- Auto-generate `configs/config.yaml` from `configs/config.example.yaml` if it doesn't exist
- Embed the config into the binary

### 3. Configure

Edit `configs/config.yaml` with your rollup details:

```yaml
l2:
  chain-configs:
    rollup-a:
      pk: 0000...  # Private key for funded account
      id: 77777    # Chain ID
      rpc-url: http://localhost:18545

    rollup-b:
      pk: 0000...  # Private key for funded account
      id: 88888    # Chain ID
      rpc-url: http://localhost:28545

  contracts:
    bridge:
      address: 0x...
      abi: '[...]'
    ping-pong:
      address: 0x...
      abi: '[...]'
    token:
      address: 0x...
      abi: '[...]'
```

**⚠️ Security Note:** Never commit actual private keys. `config.yaml` is gitignored.

After editing config, rebuild to embed changes:
```bash
make build
```

### 4. Run Tests

```bash
# Run all tests
make test

# Run with INFO logging
make test-info

# Run with DEBUG logging
make test-debug

# Run specific test
make test-info TEST_NAME=TestSendCrossTxBridge

# Run specific test suites
make test-bridge   # Bridge tests
make smoke-test    # Smoke tests
make stress-test   # Stress tests
```

Run the binary directly:
```bash
./bin/probe -test.v -test.run=TestSendCrossTxBridge
LOG_LEVEL=INFO ./bin/probe -test.v
```

## Project Structure

```
rollup-probe/
├── bin/              # Compiled test binary (bin/probe)
├── configs/          # Configuration management
│   ├── config.go                 # Config structs, validation, embed logic
│   ├── config.yaml               # Main config (gitignored, embedded at compile time)
│   └── config.example.yaml       # Template for config.yaml
├── internal/         # Core framework (private packages)
│   ├── accounts/     # Account management for blockchain interactions
│   ├── logger/       # Centralized logging (DEBUG/INFO levels)
│   ├── rollup/       # Rollup configuration and connection
│   └── transactions/ # Transaction creation and cross-chain logic
├── pkg/              # Public packages
│   └── rollupv1/     # Protobuf definitions for cross-rollup protocol
└── test/             # Test files
    ├── config.go     # Test setup and shared variables
    └── *_test.go     # Test implementations
```

## How It Works

### Cross-Rollup Transaction Flow

1. **Create Transactions**: Sign separate transactions for each rollup (RollupA, RollupB)
2. **Bundle**: Marshal both into an `XTRequest` protobuf message
3. **Send**: Submit via custom `eth_sendXTransaction` RPC method
4. **Coordinate**: Rollup network coordinates atomic execution across both chains

### Configuration

Configuration uses Go's `//go:embed` directive to embed `configs/config.yaml` at compile time. This makes the binary self-contained and portable.

- The `make build` target auto-creates `config.yaml` from the example if missing
- Useful for CI/CD pipelines where config may not exist
- Rebuild after config changes to embed new values

### Testing

Tests are compiled into a binary (`bin/probe`) rather than using `go test` directly. Benefits:

- ✅ Distributable without Go toolchain
- ✅ Faster startup (pre-compiled)
- ✅ Ideal for CI/CD and remote test environments

## Development Commands

```bash
make build          # Build test binary
make test           # Run all tests
make test-info      # Run with INFO logging
make test-debug     # Run with DEBUG logging
make test-bridge    # Run bridge tests only
make smoke-test     # Run smoke tests only
make format         # Format code
make lint           # Run linter
make clean          # Clean build artifacts
make deps           # Download dependencies
```

## Dependencies

- **go-ethereum**: Ethereum client library for RPC and transaction handling
- **protobuf**: Cross-rollup message serialization
- **gopkg.in/yaml.v3**: Configuration parsing

## License

This project is licensed under the MIT License.
