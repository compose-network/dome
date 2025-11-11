package test

import (
	"math/big"
	"testing"
	"time"

	"github.com/compose-network/dome/internal/logger"
	"github.com/compose-network/dome/internal/transactions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
	TestTxASuccessAndTxBFailure
	- create txA for rollupA and txB for rollupB
	- txA is a self tx with less amount than A's balance -> should succeed on chainA
	- txB is a self tx with more amount than B's balance -> should fail on chainB
    - check if neither of the transactions are executed on chainA and chainB
*/
//*********************************************************************************************************************
func TestTxASuccessAndTxBFailure(t *testing.T) {
	ctx := t.Context()

	// generate random session ID , will be used for both transactions
	//sessionID := transactions.GenerateRandomSessionID()

	// get both accounts balances
	balanceA, err := TestAccountA.GetBalance(ctx)
	require.NoError(t, err)
	balanceB, err := TestAccountB.GetBalance(ctx)
	require.NoError(t, err)

	// check if both balances are not zero
	assert.True(t, balanceA.Cmp(big.NewInt(0)) > 0, "balanceA should be greater than 0")
	assert.True(t, balanceB.Cmp(big.NewInt(0)) > 0, "balanceB should be greater than 0")

	// create txA details
	txADetails := transactions.TransactionDetails{
		To:        TestAccountA.GetAddress(),
		Value:     balanceA.Div(balanceA, big.NewInt(2)), // less than balanceA
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      nil,
	}

	txA, signedTransactionA, err := transactions.CreateTransaction(ctx, txADetails, TestAccountA)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionA)
	// preparations for tx A done -------------------------------------------------------------

	// create txB details
	txBDetails := transactions.TransactionDetails{
		To:        TestAccountB.GetAddress(),
		Value:     balanceB.Add(balanceB, big.NewInt(1000000000000000000)), // more than balanceB
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      nil,
	}

	txB, signedTransactionB, err := transactions.CreateTransaction(ctx, txBDetails, TestAccountB)
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

	// wait for 10 seconds before checking txs
	logger.Info("Waiting for 2 minutes before checking txs...")
	time.Sleep(2 * time.Minute)

	// both tx should not be sent to the chain
	_, _, err = transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get transaction by hash")

	_, _, err = transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get transaction by hash")
}
