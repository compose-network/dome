package smartaccount

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/compose-network/dome/internal/accounts"
	"github.com/compose-network/dome/internal/helpers"
	"github.com/compose-network/dome/internal/logger"
	"github.com/compose-network/dome/internal/transactions"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type UserOps struct {
	UserOps []UserOp
}

type UserOp struct {
	Sender             common.Address
	Nonce              *big.Int
	InitCode           []byte
	CallData           []byte
	AccountGasLimits   [32]byte
	PreVerificationGas *big.Int
	GasFees            [32]byte
	PaymasterAndData   []byte
	Signature          []byte
}

// SignUserOp signs a UserOp and returns the signature.
// The UserOp's Signature field should be empty/nil when calling this function.
// After calling, you can set userOp.Signature = signature to add it to the struct.
func SignUserOp(userOp *UserOp, ac *accounts.Account) ([]byte, error) {
	// Use abi.Arguments to pack the tuple components directly
	// This is equivalent to packing a tuple in Solidity
	args := abi.Arguments{
		{Name: "sender", Type: abi.Type{T: abi.AddressTy, Size: 20}},
		{Name: "nonce", Type: abi.Type{T: abi.UintTy, Size: 256}},
		{Name: "initCode", Type: abi.Type{T: abi.BytesTy}},
		{Name: "callData", Type: abi.Type{T: abi.BytesTy}},
		{Name: "accountGasLimits", Type: abi.Type{T: abi.FixedBytesTy, Size: 32}},
		{Name: "preVerificationGas", Type: abi.Type{T: abi.UintTy, Size: 256}},
		{Name: "gasFees", Type: abi.Type{T: abi.FixedBytesTy, Size: 32}},
		{Name: "paymasterAndData", Type: abi.Type{T: abi.BytesTy}},
		{Name: "signature", Type: abi.Type{T: abi.BytesTy}},
	}

	pack, err := args.Pack(
		userOp.Sender,
		userOp.Nonce,
		userOp.InitCode,
		userOp.CallData,
		userOp.AccountGasLimits,
		userOp.PreVerificationGas,
		userOp.GasFees,
		userOp.PaymasterAndData,
		[]byte{}, // Empty signature when signing
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pack user op: %w", err)
	}

	// Hash the packed data before signing (standard practice)
	hash := crypto.Keccak256Hash(pack)

	// Sign the hash
	signature, err := crypto.Sign(hash.Bytes(), ac.GetPrivateKey())
	if err != nil {
		return nil, fmt.Errorf("failed to sign user op: %w", err)
	}
	return signature, nil
}

// SendUserOps sends user operations to the entry point contract
func SendUserOps(ctx context.Context, ac *accounts.Account, ops []UserOp, beneficiary common.Address) (*types.Transaction, common.Hash, error) {
	// Parse entry point ABI
	epABI, err := abi.JSON(strings.NewReader(entryPointABI))
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to parse entry point ABI: %w", err)
	}

	// Convert UserOp structs to tuples for ABI encoding
	// For tuple[], go-ethereum expects a slice of structs matching the tuple structure
	type userOpTuple struct {
		Sender             common.Address
		Nonce              *big.Int
		InitCode           []byte
		CallData           []byte
		AccountGasLimits   [32]byte
		PreVerificationGas *big.Int
		GasFees            [32]byte
		PaymasterAndData   []byte
		Signature          []byte
	}

	opsTuples := make([]userOpTuple, len(ops))
	for i, op := range ops {
		// Use empty signature if nil (though it should be set by this point)
		signature := op.Signature
		if signature == nil {
			signature = []byte{}
		}
		opsTuples[i] = userOpTuple{
			Sender:             op.Sender,
			Nonce:              op.Nonce,
			InitCode:           op.InitCode,
			CallData:           op.CallData,
			AccountGasLimits:   op.AccountGasLimits,
			PreVerificationGas: op.PreVerificationGas,
			GasFees:            op.GasFees,
			PaymasterAndData:   op.PaymasterAndData,
			Signature:          signature,
		}
	}

	// Pack the handleOps function call
	// For tuple[], we need to pass the slice of structs
	handleOpsCalldata, err := epABI.Pack("handleOps", opsTuples, beneficiary)
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to pack handleOps: %w", err)
	}

	// Create transaction
	entryPointAddr := common.HexToAddress(entryPointAddress)
	transactionDetails := transactions.TransactionDetails{
		To:        entryPointAddr,
		Value:     big.NewInt(0),
		Gas:       2000000, // Adjust gas limit as needed
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      handleOpsCalldata,
	}

	tx, _, err := transactions.CreateTransaction(ctx, transactionDetails, ac)
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Send transaction
	hash, err := transactions.SendTransaction(ctx, tx, ac.GetRollup().RPCURL())
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}
	logger.Info("HandleOps transaction sent: %s", hash.Hex())

	return tx, hash, nil
}

func HashUserOp(userOp *UserOp, entryPoint common.Address, chainId *big.Int) common.Hash {
	// Hash dynamic fields
	initCodeHash := crypto.Keccak256Hash(userOp.InitCode)
	callDataHash := crypto.Keccak256Hash(userOp.CallData)
	paymasterHash := crypto.Keccak256Hash(userOp.PaymasterAndData)

	// Hash the userOp struct
	encoded, err := helpers.EncodeLikeEthers(
		[]string{
			"bytes32", "bytes32", "bytes32",
			"address", "uint256",
			"bytes32", "uint256", "bytes32",
		},
		[]interface{}{
			initCodeHash,
			callDataHash,
			paymasterHash,
			userOp.Sender,
			userOp.Nonce,
			userOp.AccountGasLimits,
			userOp.PreVerificationGas,
			userOp.GasFees,
		},
	)
	if err != nil {
		return common.Hash{}
	}
	userOpStruct := crypto.Keccak256Hash(encoded)

	// Final hash = EIP712
	encodedDomain, err := helpers.EncodeLikeEthers(
		[]string{"bytes32", "address", "uint256"},
		[]interface{}{
			crypto.Keccak256Hash([]byte("EntryPoint")),
			entryPoint,
			chainId,
		},
	)
	if err != nil {
		return common.Hash{}
	}
	domainSeparator := crypto.Keccak256Hash(encodedDomain)

	return crypto.Keccak256Hash(
		[]byte("\x19\x01"),
		domainSeparator.Bytes(),
		userOpStruct.Bytes(),
	)
}

func SignUserOp2(userOp *UserOp, entryPoint common.Address, chainId *big.Int, ac *accounts.Account) []byte {
	hash := HashUserOp(userOp, entryPoint, chainId)

	sig, err := crypto.Sign(hash.Bytes(), ac.GetPrivateKey())
	if err != nil {
		panic(err)
	}

	// Fix v: convert 0/1 → 27/28
	sig[64] += 27

	return sig
}
