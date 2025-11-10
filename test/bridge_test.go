package test

import (
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compose-network/rollup-probe/configs"
	"github.com/compose-network/rollup-probe/internal/transactions"
)

/*
	TestMain is the main function for the bridge tests

It sets up the test environment and runs the tests
*/
func TestMain(m *testing.M) {
	// Setup (beforeAll equivalent)
	setup()

	// Run all tests
	code := m.Run()

	// Exit with the same code as the tests
	os.Exit(code)
}

/*
		TestSelfMoveBalanceOnAandreceiveTokensOnB
		- create txA that is a self move balance on chain A -> nothing to do with contract, should succeed on chainA
		- create txB that is a receive tokens on chain B -> should fail on chainB because nobody called send on chain A
	    - check if neither of the transactions are executed
*/
func Dummy(t *testing.T) {
	ctx := t.Context()

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	// construct contract call parameters for transaction from accountA
	transactionADetails := transactions.TransactionDetails{
		To:        TestAccountA.GetAddress(),
		Value:     big.NewInt(10000),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      nil,
	}

	// create transaction to be sent from accountA
	txA, signedTransactionA, err := transactions.CreateTransaction(ctx, transactionADetails, TestAccountA)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionA)
	// preparations for tx A done -------------------------------------------------------------

	// construct contract call parameters for transaction from accountB
	calldataB, err := BridgeABI.Pack("receiveTokens",
		TestRollupA.ChainID(),     // ChainSrc
		TestAccountA.GetAddress(), // sender
		TestAccountB.GetAddress(), // receiver
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

	// both tx should not be sent to the chain
	_, _, err = transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get transaction by hash")

	_, _, err = transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get transaction by hash")
}
