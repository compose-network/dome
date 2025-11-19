package smartaccount

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/compose-network/dome/internal/accounts"
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
	// Define ABI for packing a single UserOp tuple (without signature)
	userOpABI := `[{"type":"function","name":"packUserOp","inputs":[{"name":"op","type":"tuple","components":[{"name":"sender","type":"address"},{"name":"nonce","type":"uint256"},{"name":"initCode","type":"bytes"},{"name":"callData","type":"bytes"},{"name":"accountGasLimits","type":"bytes32"},{"name":"preVerificationGas","type":"uint256"},{"name":"gasFees","type":"bytes32"},{"name":"paymasterAndData","type":"bytes"},{"name":"signature","type":"bytes"}]}],"outputs":[{"name":"","type":"bytes"}]}]`

	packABI, err := abi.JSON(strings.NewReader(userOpABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse userOp ABI: %w", err)
	}

	// Pack the userOp tuple (with empty signature for signing)
	userOpTuple := []interface{}{
		userOp.Sender,
		userOp.Nonce,
		userOp.InitCode,
		userOp.CallData,
		userOp.AccountGasLimits,
		userOp.PreVerificationGas,
		userOp.GasFees,
		userOp.PaymasterAndData,
		[]byte{}, // Empty signature when signing
	}

	pack, err := packABI.Pack("packUserOp", userOpTuple)
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
	opsTuples := make([]interface{}, len(ops))
	for i, op := range ops {
		// Use empty signature if nil (though it should be set by this point)
		signature := op.Signature
		if signature == nil {
			signature = []byte{}
		}
		opsTuples[i] = []interface{}{
			op.Sender,
			op.Nonce,
			op.InitCode,
			op.CallData,
			op.AccountGasLimits,
			op.PreVerificationGas,
			op.GasFees,
			op.PaymasterAndData,
			signature,
		}
	}

	// Pack the handleOps function call
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
