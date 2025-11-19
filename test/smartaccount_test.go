package test

import (
	"testing"

	"github.com/compose-network/dome/internal/smartaccount"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBridge(t *testing.T) {
	// TODO: Implement bridge test
}

func TestCreateSmartAccount(t *testing.T) {
	ctx := t.Context()
	MultiChainValidatorAddr := common.HexToAddress("0x5e729b0d9d35fa3bd7ace526437151ec9e1d5929")

	// Build the 21-byte ValidationId = 0x01 || validator address
	var rootValidator [21]byte
	rootValidator[0] = 0x01 // type prefix: 0x01 for validator
	copy(rootValidator[1:], MultiChainValidatorAddr.Bytes())

	// Pack validator data: owner address for the validator
	owner := TestAccountA.GetAddress()
	validatorData, err := abi.Arguments{
		{Type: abi.Type{T: abi.AddressTy}},
	}.Pack(owner)
	require.NoError(t, err, "Failed to pack validator data")

	zeroAddress := common.Address{}
	emptyBytes := []byte{}

	// Create initData for the smart account
	// Using real test account addresses
	initData := &smartaccount.InitData{
		RootValidator: rootValidator, // Validator ID
		Hook:          zeroAddress,   // Zero address for hook
		ValidatorData: validatorData, // Packed owner address
		HookData:      emptyBytes,    // Empty hook data
		InitConfig:    [][]byte{},    // Empty init config
	}

	// Create the smart account
	smartAcc, err := smartaccount.CreateSmartAccount(ctx, TestAccountA, initData)
	require.NoError(t, err, "Failed to create smart account")
	require.NotNil(t, smartAcc, "Smart account should not be nil")

	// Verify the smart account has a valid address
	address := smartAcc.GetAddress()
	require.NotEqual(t, common.Address{}, address, "Smart account address should not be zero")
	require.NotNil(t, address, "Smart account address should not be nil")

	t.Logf("Successfully created smart account at address: %s", address.Hex())
}
