package test

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/compose-network/rollup-probe/configs"
	"github.com/compose-network/rollup-probe/internal/logger"
	"github.com/compose-network/rollup-probe/internal/transactions"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingPong(t *testing.T) {
	ctx := t.Context()

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()
	pingPongAddress := configs.Values.L2.Contracts[configs.ContractNamePingPong].Address

	// construct calldata on rollup A
	calldataA, err := pingPongABI.Pack("ping",
		TestRollupB.ChainID(),         // otherChain
		TestAccountA.GetAddress(),     // pongSender
		TestAccountB.GetAddress(),     // pingReceiver
		sessionID,                     // sessionId
		[]byte("Hello from rollup A"), // data
	)
	require.NoError(t, err)
	require.NotNil(t, calldataA)

	// Create transaction details
	transactionADetails := transactions.TransactionDetails{
		To:        pingPongAddress,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataA,
	}
	// create transaction to be sent from accountA
	txA, signedTransactionA, err := transactions.CreateTransaction(ctx, transactionADetails, TestAccountA)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionA)
	// preparations for tx A done -------------------------------------------------------------

	// construct calldata on rollup B
	calldataB, err := pingPongABI.Pack("pong",
		TestRollupA.ChainID(),         // otherChain
		TestAccountB.GetAddress(),     // pingSender
		sessionID,                     // sessionId
		[]byte("Hello from rollup B"), // data
	)
	require.NoError(t, err)
	require.NotNil(t, calldataB)

	// Create transaction details
	transactionBDetails := transactions.TransactionDetails{
		To:        pingPongAddress,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataB,
	}
	// create transaction to be sent from accountB
	txB, signedTransactionB, err := transactions.CreateTransaction(ctx, transactionBDetails, TestAccountB)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionB)
	// preparations for tx B done -------------------------------------------------------------

	// create cross tx request msg
	crossTxRequestMsg, err := transactions.CreateCrossTxRequestMsg(ctx, TestAccountA, TestAccountB, signedTransactionA, signedTransactionB)
	require.NoError(t, err)
	require.NotNil(t, crossTxRequestMsg)

	// send cross tx request msg
	err = transactions.SendCrossTxRequestMsg(ctx, TestRollupA.RPCURL(), crossTxRequestMsg)
	require.NoError(t, err)

	// wait for 2 minutes before checking txs
	logger.Info("Waiting for 2 minutes before checking txs...")
	time.Sleep(2 * time.Minute)

	// check tx A
	tx, receipt, err := transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, receipt)
	// check tx is successful
	assert.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *tx.To(), pingPongAddress)
	assert.True(t, bytes.Equal(tx.Data(), calldataA))
	// check that receives back pong message
	// Find the pong event in the logs
	for _, log := range receipt.Logs {
		if len(log.Topics) > 0 && log.Topics[0] == pingPongABI.Events["PING"].ID {
			// Decode the event data
			eventData, err := pingPongABI.Unpack("PING", log.Data)
			require.NoError(t, err)
			assert.Equal(t, eventData[0], "PONG")
			break
		}
	}

	// check tx B
	tx, receipt, err = transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, receipt)
	// check tx is successful
	assert.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *tx.To(), pingPongAddress)
	assert.True(t, bytes.Equal(tx.Data(), calldataB))
	// check that receives back ping message
	// Find the ping event in the logs
	for _, log := range receipt.Logs {
		if len(log.Topics) > 0 && log.Topics[0] == pingPongABI.Events["PONG"].ID {
			// Decode the event data
			eventData, err := pingPongABI.Unpack("PONG", log.Data)
			require.NoError(t, err)
			assert.Equal(t, eventData[0], "PING")
			break
		}
	}
}
