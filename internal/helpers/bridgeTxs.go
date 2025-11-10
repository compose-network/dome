package helpers

import (
	"context"
	"math/big"
	"testing"

	"github.com/compose-network/rollup-probe/internal/logger"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/compose-network/rollup-probe/configs"
	"github.com/compose-network/rollup-probe/internal/accounts"
	"github.com/compose-network/rollup-probe/internal/transactions"
)

/*
SendBridgeTx sends a bridge transaction from ac1 to ac2 with the given amount
*/
func SendBridgeTx(
	t *testing.T,
	ac1 *accounts.Account,
	ac2 *accounts.Account,
	amount *big.Int,
	tokenABI abi.ABI,
	bridgeABI abi.ABI,
) (*types.Transaction, *types.Transaction, error) {

	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	// construct contract call parameters for transaction from accountA
	calldataA, err := bridgeABI.Pack("send",
		ac2.GetRollup().ChainID(),                                      // otherChainId
		configs.Values.L2.Contracts[configs.ContractNameToken].Address, // token
		ac1.GetAddress(),                                               // sender
		ac2.GetAddress(),                                               // receiver
		amount,                                                         // amount
		sessionID,                                                      // sessionId
		bridgeAddr,                                                     // destBridge
	)
	require.NoError(t, err)
	require.NotNil(t, calldataA)

	// Create transaction details
	transactionADetails := transactions.TransactionDetails{
		To:        bridgeAddr,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataA,
	}

	// create transaction to be sent from accountA
	txA, signedTransactionA, err := transactions.CreateTransaction(context.Background(), transactionADetails, ac1)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionA)
	// preparations for tx A done -------------------------------------------------------------

	// construct contract call parameters for transaction from accountB
	calldataB, err := bridgeABI.Pack("receiveTokens",
		ac1.GetRollup().ChainID(), // ChainSrc
		ac2.GetAddress(),          // sender
		ac2.GetAddress(),          // receiver
		sessionID,                 // sessionId
		bridgeAddr,                // srcBridge
	)
	require.NoError(t, err)
	require.NotNil(t, calldataB)

	// Create transaction details
	transactionBDetails := transactions.TransactionDetails{
		To:        bridgeAddr,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataB,
	}

	// create transaction to be sent from accountB
	txB, signedTransactionB, err := transactions.CreateTransaction(context.Background(), transactionBDetails, ac2)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionB)
	// preparations for tx B done -------------------------------------------------------------

	// create cross tx request msg
	crossTxRequestMsg, err := transactions.CreateCrossTxRequestMsg(context.Background(), ac1, ac2, signedTransactionA, signedTransactionB)
	require.NoError(t, err)
	require.NotNil(t, crossTxRequestMsg)

	// send cross tx request msg to source chain (A)
	err = transactions.SendCrossTxRequestMsg(context.Background(), ac1.GetRollup().RPCURL(), crossTxRequestMsg)
	require.NoError(t, err)

	logger.Info("Bridge transaction A sent successfully: %s", txA.Hash())
	logger.Info("Bridge transaction B sent successfully: %s", txB.Hash())

	return txA, txB, err
}

/*
SendBridgeTxWithStartingNonce sends a bridge transaction from ac1 to ac2 with the given amount and starting nonce.
Can be used to send multiple bridge txs from same account with different nonces.
*/
func SendBridgeTxWithNonce(
	t *testing.T,
	ac1 *accounts.Account,
	ac1_nonce uint64,
	ac2 *accounts.Account,
	ac2_nonce uint64,
	amount *big.Int,
	tokenABI abi.ABI,
	bridgeABI abi.ABI,

) (*types.Transaction, *types.Transaction, error) {

	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	// construct contract call parameters for transaction from accountA
	calldataA, err := bridgeABI.Pack("send",
		ac2.GetRollup().ChainID(),                                      // otherChainId
		configs.Values.L2.Contracts[configs.ContractNameToken].Address, // token
		ac1.GetAddress(),                                               // sender
		ac2.GetAddress(),                                               // receiver
		amount,                                                         // amount
		sessionID,                                                      // sessionId
		bridgeAddr,                                                     // destBridge
	)
	require.NoError(t, err)
	require.NotNil(t, calldataA)

	// Create transaction details
	transactionADetails := transactions.TransactionDetails{
		To:        bridgeAddr,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataA,
	}

	// create transaction to be sent from accountA
	txA, signedTransactionA, err := transactions.CreateTransactionWithNonce(context.Background(), transactionADetails, ac1, ac1_nonce)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionA)
	// preparations for tx A done -------------------------------------------------------------

	// construct contract call parameters for transaction from accountB
	calldataB, err := bridgeABI.Pack("receiveTokens",
		ac1.GetRollup().ChainID(), // ChainSrc
		ac2.GetAddress(),          // sender
		ac2.GetAddress(),          // receiver
		sessionID,                 // sessionId
		bridgeAddr,                // srcBridge
	)
	require.NoError(t, err)
	require.NotNil(t, calldataB)

	// Create transaction details
	transactionBDetails := transactions.TransactionDetails{
		To:        bridgeAddr,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataB,
	}

	// create transaction to be sent from accountB
	txB, signedTransactionB, err := transactions.CreateTransactionWithNonce(context.Background(), transactionBDetails, ac2, ac2_nonce)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionB)
	// preparations for tx B done -------------------------------------------------------------

	// create cross tx request msg
	crossTxRequestMsg, err := transactions.CreateCrossTxRequestMsg(context.Background(), ac1, ac2, signedTransactionA, signedTransactionB)
	require.NoError(t, err)
	require.NotNil(t, crossTxRequestMsg)

	// send cross tx request msg to source chain (A)
	err = transactions.SendCrossTxRequestMsg(context.Background(), ac1.GetRollup().RPCURL(), crossTxRequestMsg)
	require.NoError(t, err)

	logger.Info("Bridge transaction A sent successfully: %s", txA.Hash())
	logger.Info("Bridge transaction B sent successfully: %s", txB.Hash())

	return txA, txB, err
}
