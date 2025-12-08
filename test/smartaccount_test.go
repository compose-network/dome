package test

import (
	"fmt"
	"math/big"

	"testing"

	"github.com/compose-network/dome/internal/smartaccount"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/stretchr/testify/require"

	"github.com/compose-network/dome/internal/helpers"

	"github.com/compose-network/dome/internal/transactions"
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
	owner := TestAccountB.GetAddress()
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
	smartAcc, err := smartaccount.CreateSmartAccount(ctx, TestAccountB, initData)
	require.NoError(t, err, "Failed to create smart account")
	require.NotNil(t, smartAcc, "Smart account should not be nil")

	// Verify the smart account has a valid address
	address := smartAcc.GetAddress()
	require.NotEqual(t, common.Address{}, address, "Smart account address should not be zero")
	require.NotNil(t, address, "Smart account address should not be nil")
}

func TestUserOps(t *testing.T) {
	type Call struct {
		To    common.Address
		Value *big.Int
		Data  []byte
	}

	type Calls []Call

	smartAccAddA := common.HexToAddress("0x127a4E99CBcE4dC3b2DEFC6B489247F84a3AcF16")
	//smartAccAddB := common.HexToAddress("0x9CE8051ee07F73Cfb604Fba419c508b61C1Afd96")
	randomAddress := common.HexToAddress("0x2600428acb2f2b01201bf6db3ead3649418b522b")
	entryPointAddr := common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")

	// dataA, err := TokenABI.Pack("send",
	// 	TestAccountA.GetAddress(),
	// 	smartAccAddA,
	// 	big.NewInt(1000000000000000000),
	// )
	// require.NoError(t, err)
	// require.NotNil(t, dataA)

	firstCall := Call{
		To:    randomAddress,
		Value: big.NewInt(0),
		Data:  []byte{},
	}
	typeStrs := []string{"address", "uint256", "bytes"}
	values := []interface{}{
		randomAddress,
		big.NewInt(500000000000000000),
		firstCall.Data,
	}
	encodedFirstCall, err := helpers.EncodeLikeEthers(typeStrs, values)
	require.NoError(t, err)
	require.NotNil(t, encodedFirstCall)

	verificationGasLimit := big.NewInt(150_000) // 150k
	callGasLimit := big.NewInt(500_000)         // 500k

	accountGasLimitsBytes := helpers.PackAccountGasLimits(
		verificationGasLimit,
		callGasLimit,
	)

	gwei := big.NewInt(1_000_000_000)
	maxFeePerGas := new(big.Int).Mul(big.NewInt(30), gwei)        // 30 gwei
	maxPriorityFeePerGas := new(big.Int).Mul(big.NewInt(2), gwei) // 2 gwei

	gasFeesBytes := helpers.PackGasFees(
		maxFeePerGas,
		maxPriorityFeePerGas,
	)

	preVerificationGas := big.NewInt(200_000_000)

	userOps := smartaccount.UserOps{
		UserOps: []smartaccount.UserOp{
			{
				Sender:             smartAccAddA, // smart account address
				Nonce:              big.NewInt(0),
				InitCode:           []byte{},
				CallData:           encodedFirstCall,
				AccountGasLimits:   accountGasLimitsBytes,
				PreVerificationGas: preVerificationGas,
				GasFees:            gasFeesBytes,
				PaymasterAndData:   []byte{},
				Signature:          []byte{},
			},
		},
	}
	// PAYMASTER
	paymasterEndpoint := "https://paymaster.stage.ops.ssvlabsinternal.com/rpc/v1/rollupA"

	paymasterRequest := smartaccount.PaymasterRequest{
		Sender:               smartAccAddA,
		Nonce:                big.NewInt(0),
		InitCode:             []byte{},
		CallData:             []byte{},
		CallGasLimit:         big.NewInt(500000),
		VerificationGasLimit: big.NewInt(150000),
		PreVerificationGas:   big.NewInt(200000000),
		MaxFeePerGas:         big.NewInt(30000000000),
		MaxPriorityFeePerGas: big.NewInt(2000000000),
		EntryPoint:           common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032"),
		ChainID:              TestRollupA.ChainID(),
	}

	paymasterAndData, paymasterResult, err := smartaccount.GetPaymasterAndData(t.Context(), paymasterEndpoint, paymasterRequest)
	require.NoError(t, err)
	require.NotNil(t, paymasterAndData)
	require.NotNil(t, paymasterResult)
	userOps.UserOps[0].PaymasterAndData = paymasterAndData

	signature := smartaccount.SignUserOp2(&userOps.UserOps[0], entryPointAddr, TestRollupA.ChainID(), TestAccountA)
	require.NoError(t, err)
	require.NotNil(t, signature)
	userOps.UserOps[0].Signature = signature

	tx, hash, err := smartaccount.SendUserOps(t.Context(), TestAccountA, userOps.UserOps, smartAccAddA)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, hash)

	_, txReceipt, err := transactions.GetTransactionDetails(t.Context(), hash, TestRollupA)
	require.NoError(t, err)
	require.NotNil(t, txReceipt)

	fmt.Printf("txReceipt: %+v\n", txReceipt)
}

func TestPaymaster(t *testing.T) {
	smartAccAddA := common.HexToAddress("0x127a4E99CBcE4dC3b2DEFC6B489247F84a3AcF16")
	paymasterEndpoint := "https://paymaster.stage.ops.ssvlabsinternal.com/rpc/v1/rollupA"

	paymasterRequest := smartaccount.PaymasterRequest{
		Sender:               smartAccAddA,
		Nonce:                big.NewInt(0),
		InitCode:             []byte{},
		CallData:             []byte{},
		CallGasLimit:         big.NewInt(500000),
		VerificationGasLimit: big.NewInt(150000),
		PreVerificationGas:   big.NewInt(200000000),
		MaxFeePerGas:         big.NewInt(30000000000),
		MaxPriorityFeePerGas: big.NewInt(2000000000),
		EntryPoint:           common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032"),
		ChainID:              TestRollupA.ChainID(),
	}

	paymasterAndData, paymasterResult, err := smartaccount.GetPaymasterAndData(t.Context(), paymasterEndpoint, paymasterRequest)
	require.NoError(t, err)
	require.NotNil(t, paymasterAndData)
	require.NotNil(t, paymasterResult)

	fmt.Printf("paymasterAndData: %s\n", paymasterAndData)
	fmt.Printf("paymasterResult: %+v\n", paymasterResult)
}
