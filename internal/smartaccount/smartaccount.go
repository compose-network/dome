package smartaccount

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/compose-network/dome/internal/accounts"
	"github.com/compose-network/dome/internal/logger"
	"github.com/compose-network/dome/internal/transactions"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type SmartAccount struct {
	address common.Address
}

type InitData struct {
	RootValidator [21]byte
	Hook          common.Address
	ValidatorData []byte
	HookData      []byte
	InitConfig    [][]byte
}

// GetAddress returns the smart account address
func (sa *SmartAccount) GetAddress() common.Address {
	return sa.address
}

func CreateSmartAccount(ctx context.Context, ac *accounts.Account, data *InitData) (*SmartAccount, error) {
	// Parse kernel factory ABI
	kernelABI, err := abi.JSON(strings.NewReader(kernelAbi))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel ABI: %w", err)
	}

	// Encode initialize function call
	// First, we need to construct the initialize function ABI
	// Note: _rootValidator is bytes (21 bytes: 1 byte type prefix + 20 byte validator address)
	initializeABI := `[{"type":"function","name":"initialize","inputs":[{"name":"_rootValidator","type":"bytes21"},{"name":"hook","type":"address"},{"name":"validatorData","type":"bytes"},{"name":"hookData","type":"bytes"},{"name":"initConfig","type":"bytes[]"}]}]`
	initializeABIParsed, err := abi.JSON(strings.NewReader(initializeABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse initialize ABI: %w", err)
	}

	// Pack the initialize function call
	initCalldata, err := initializeABIParsed.Pack("initialize",
		data.RootValidator,
		data.Hook,
		data.ValidatorData,
		data.HookData,
		data.InitConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pack initialize function: %w", err)
	}

	// Generate a random salt (bytes32)
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	var salt32 [32]byte
	copy(salt32[:], salt)

	// Pack the createAccount function call
	factoryAddress := common.HexToAddress(kernelFactoryAddress)
	createAccountCalldata, err := kernelABI.Pack("createAccount", initCalldata, salt32)
	if err != nil {
		return nil, fmt.Errorf("failed to pack createAccount function: %w", err)
	}

	// Connect to RPC
	client, err := ethclient.DialContext(ctx, ac.GetRollup().RPCURL())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}
	defer client.Close()

	// Verify the factory contract exists and has code
	factoryCode, err := client.CodeAt(ctx, factoryAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check factory contract code: %w", err)
	}
	if len(factoryCode) == 0 {
		return nil, fmt.Errorf("factory contract has no code at address %s - contract may not be deployed", factoryAddress.Hex())
	}

	// Check if account already exists by calling getAddress
	var predictedAddress common.Address
	contract := bind.NewBoundContract(factoryAddress, kernelABI, client, client, client)
	callOpts := &bind.CallOpts{Context: ctx}
	err = contract.Call(callOpts, &[]interface{}{&predictedAddress}, "getAddress", initCalldata, salt32)
	if err != nil {
		return nil, fmt.Errorf("failed to call getAddress: %w", err)
	}
	logger.Info("Predicted smart account address: %s", predictedAddress.Hex())

	// Check if the account already exists (has code)
	code, err := client.CodeAt(ctx, predictedAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check if account exists: %w", err)
	}
	if len(code) > 0 {
		return &SmartAccount{address: predictedAddress}, nil
	}

	// Create transaction
	transactionDetails := transactions.TransactionDetails{
		To:        factoryAddress,
		Value:     big.NewInt(0), // 1 ETH
		Gas:       9000000,       // Adjust gas limit as needed
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      createAccountCalldata,
	}

	tx, _, err := transactions.CreateTransaction(ctx, transactionDetails, ac)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Send transaction
	hash, err := transactions.SendTransaction(ctx, tx, ac.GetRollup().RPCURL())
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	// Wait for transaction receipt
	_, receipt, err := transactions.GetTransactionDetails(ctx, hash, ac.GetRollup())
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("createAccount transaction failed with status: %d", receipt.Status)
	}

	return &SmartAccount{address: predictedAddress}, nil
}
