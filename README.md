# Dome

A Go-based E2E test framework for cross-rollup blockchain transactions. Tests cross-chain transaction functionality between multiple rollup networks using Ethereum-compatible RPC interfaces and a custom cross-transaction protocol.

## Features

- **Cross-Rollup Transactions**: Test atomic transaction execution across multiple rollups
- **Custom Protocol**: Uses protobuf-based messaging for cross-chain coordination
- **Embedded Configuration**: Config is embedded at compile time for self-contained binaries
- **Test Binary**: Compiles tests into a standalone executable for easy distribution
- **Docker Support**: Production-ready multi-stage Dockerfile for containerized deployments

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
- Create `bin/dome` test binary
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
# Use embedded config
./bin/dome -test.v -test.run=TestSendCrossTxBridge
LOG_LEVEL=INFO ./bin/dome -test.v

# Use external config file
CONFIG_PATH=./configs/config.yaml ./bin/dome -test.v
CONFIG_PATH=/path/to/custom-config.yaml LOG_LEVEL=DEBUG ./bin/dome -test.v
```

## Project Structure

```
dome/
├── bin/              # Compiled test binary (bin/dome)
├── build/            # Build artifacts
│   └── Dockerfile    # Multi-stage Docker build
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

Configuration supports both embedded and external loading for maximum flexibility:

**Embedded Config (Default)**:
- Uses Go's `//go:embed` directive to embed `configs/config.yaml` at compile time
- Makes the binary self-contained and portable
- The `make build` target auto-creates `config.yaml` from the example if missing
- Useful for CI/CD pipelines and quick testing
- Rebuild after config changes to embed new values

**External Config (Recommended for Production)**:
- Set `CONFIG_PATH` environment variable to load config from external file
- Example: `CONFIG_PATH=/app/config.yaml ./bin/dome -test.v`
- Ideal for Docker deployments with mounted volumes
- Allows config changes without rebuilding the binary
- Falls back to embedded config if external file fails to load

**Loading Priority**:
1. Check `CONFIG_PATH` environment variable
2. If set, try to load from that path
3. If not set or loading fails, use embedded config
4. Panic if neither source provides valid config

### Testing

Tests are compiled into a binary (`bin/dome`) rather than using `go test` directly. Benefits:

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
make stress-test    # Run stress tests only
make format         # Format code
make lint           # Run linter
make clean          # Clean build artifacts
make deps           # Download dependencies
make docker-build   # Build Docker image
```

## Docker Deployment

The project includes a production-ready multi-stage Dockerfile for containerized deployments.

### Features

- **Minimal Image Size**: Based on `scratch` with only the test binary
- **Security**: Non-root user (domeuser:domegroup)
- **Multi-Platform**: Supports linux/amd64 and linux/arm64
- **Self-Contained**: Automatically creates config from example during build
- **CA Certificates**: Embedded for HTTPS RPC connections

### Building

```bash
# Build with default tag (dome:latest)
make docker-build

# Build with custom tag
make docker-build DOCKER_TAG=v1.0.0

# Build with custom image name
make docker-build DOCKER_IMAGE=myregistry/dome DOCKER_TAG=dev
```

Or use Docker directly:
```bash
docker build -f build/Dockerfile -t dome:latest .
```

### Running Tests

The Docker image embeds placeholder config from `config.example.yaml` by default. To use your actual config, mount it as a volume and set the `CONFIG_PATH` environment variable:

```bash
# Run with custom config (recommended for real tests)
docker run --rm \
  -v $(pwd)/configs/config.yaml:/app/config.yaml \
  -e CONFIG_PATH=/app/config.yaml \
  dome:latest -test.v -test.run=TestSendCrossTxBridge

# Run with DEBUG logging and custom config
docker run --rm \
  -v $(pwd)/configs/config.yaml:/app/config.yaml \
  -e CONFIG_PATH=/app/config.yaml \
  -e LOG_LEVEL=DEBUG \
  dome:latest -test.v

# Run all tests with custom config
docker run --rm \
  -v $(pwd)/configs/config.yaml:/app/config.yaml \
  -e CONFIG_PATH=/app/config.yaml \
  dome:latest -test.v

# Run with embedded config (placeholder values only)
docker run --rm dome:latest -test.v
```

**Config Loading Priority:**
1. If `CONFIG_PATH` environment variable is set, load from that path
2. If the external config fails or isn't set, fall back to embedded config
3. This makes the image work in both development and production scenarios

## Dependencies

- **go-ethereum**: Ethereum client library for RPC and transaction handling
- **protobuf**: Cross-rollup message serialization
- **gopkg.in/yaml.v3**: Configuration parsing

## License

This project is licensed under the MIT License.
