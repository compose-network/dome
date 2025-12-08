package helpers

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"math/big"
	"strings"
)

func normalize(solType string) string {
	t := strings.TrimSpace(solType)
	// ethers allows "uint" / "int" as shorthand for 256-bit
	if t == "uint" {
		return "uint256"
	}
	if t == "int" {
		return "int256"
	}
	return t
}

func EncodeLikeEthers(typeStrs []string, values []interface{}) ([]byte, error) {
	if len(typeStrs) != len(values) {
		return nil, fmt.Errorf("types/values length mismatch")
	}

	// Build abi.Arguments from the provided type strings.
	args := make(abi.Arguments, len(typeStrs))
	for i, ts := range typeStrs {
		t, err := abi.NewType(normalize(ts), "", nil)
		if err != nil {
			return nil, fmt.Errorf("type %q: %w", ts, err)
		}
		args[i] = abi.Argument{Name: fmt.Sprintf("arg%d", i), Type: t}
	}

	// Pack encodes like Solidity's abi.encode(...)
	return args.Pack(values...)
}

func PackAccountGasLimits(verificationGasLimit, callGasLimit *big.Int) [32]byte {
	// packed = (verificationGasLimit << 128) | callGasLimit
	packed := new(big.Int).Lsh(verificationGasLimit, 128)
	packed.Add(packed, callGasLimit)

	var out [32]byte
	packed.FillBytes(out[:])
	return out
}

func PackGasFees(maxFeePerGas, maxPriorityFeePerGas *big.Int) [32]byte {
	// packed = (maxFeePerGas << 128) | maxPriorityFeePerGas
	packed := new(big.Int).Lsh(maxFeePerGas, 128)
	packed.Add(packed, maxPriorityFeePerGas)

	var out [32]byte
	packed.FillBytes(out[:])
	return out
}
