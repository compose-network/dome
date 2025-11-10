package test

import (
	"bytes"
	"math/big"
	"sync"
	"testing"

	"github.com/compose-network/rollup-probe/configs"
	"github.com/compose-network/rollup-probe/internal/transactions"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	mintedAmount      = big.NewInt(9000000000000000000) // 9 tokens
	transferredAmount = big.NewInt(100000000000000000)  // 0.1 tokens
)

/*
TestMintTokensCrossRollup tests the minting of tokens on both chains and sends the txs as cross rollup tx
*/
func TestMintTokensCrossRollup(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address

	// get initial token balance for both accounts
	initialTokenBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	initialTokenBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)

	// construct contract call parameters for transaction from accountA
	calldataA, err := TokenABI.Pack("mint",
		TestAccountA.GetAddress(),
		mintedAmount,
	)
	require.NoError(t, err)
	require.NotNil(t, calldataA)

	// Create transaction details
	transactionADetails := transactions.TransactionDetails{
		To:        tokenAddress,
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

	// construct contract call parameters for transaction from accountB
	calldataB, err := TokenABI.Pack("mint",
		TestAccountB.GetAddress(),
		mintedAmount,
	)
	require.NoError(t, err)
	require.NotNil(t, calldataB)

	// Create transaction details
	transactionBDetails := transactions.TransactionDetails{
		To:        tokenAddress,
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

	// Check tx A and tx B in parallel
	type txResult struct {
		tx      *types.Transaction
		receipt *types.Receipt
		err     error
	}

	var wg sync.WaitGroup
	resultA := make(chan txResult, 1)
	resultB := make(chan txResult, 1)

	// Check tx A
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, receipt, err := transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
		resultA <- txResult{tx: tx, receipt: receipt, err: err}
	}()

	// Check tx B
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, receipt, err := transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
		resultB <- txResult{tx: tx, receipt: receipt, err: err}
	}()

	// Wait for both goroutines to complete
	wg.Wait()

	// Get results for tx A
	resA := <-resultA
	require.NoError(t, resA.err)
	require.NotNil(t, resA.tx)
	require.NotNil(t, resA.receipt)
	// check tx is successful
	assert.Equal(t, resA.receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *resA.tx.To(), tokenAddress)
	assert.True(t, bytes.Equal(resA.tx.Data(), calldataA))

	// Get results for tx B
	resB := <-resultB
	require.NoError(t, resB.err)
	require.NotNil(t, resB.tx)
	require.NotNil(t, resB.receipt)
	// check tx is successful
	assert.Equal(t, resB.receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *resB.tx.To(), tokenAddress)
	assert.True(t, bytes.Equal(resB.tx.Data(), calldataB))

	// check balances after txs
	tokenBalanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tokenBalanceAAfter)
	tokenBalanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tokenBalanceBAfter)
	assert.Equal(t, initialTokenBalanceA.Add(initialTokenBalanceA, mintedAmount), tokenBalanceAAfter)
	assert.Equal(t, initialTokenBalanceB.Add(initialTokenBalanceB, mintedAmount), tokenBalanceBAfter)
}

/*
TestSendCrossTxBridgeFromAToB sends tokens from chain A to chain B and sends the txs as cross rollup tx
  - create txA that is a send tokens bridge call on chain A
  - create txB that is a receive tokens bridge call on chain B
  - check if balances are updated correctly, both tx successfull and tx data not malformed
*/
func TestSendCrossTxBridgeFromAToB(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address
	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	// get initial token balance for both accounts
	initialTokenBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialTokenBalanceA)
	initialTokenBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialTokenBalanceB)

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	// construct contract call parameters for transaction from accountA
	calldataA, err := BridgeABI.Pack("send",
		TestRollupB.ChainID(), // otherChainId
		configs.Values.L2.Contracts[configs.ContractNameToken].Address, // token
		TestAccountA.GetAddress(),                                      // sender
		TestAccountB.GetAddress(),                                      // receiver
		transferredAmount,                                              // amount
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

	// send cross tx request msg to source chain (A)
	err = transactions.SendCrossTxRequestMsg(ctx, TestRollupA.RPCURL(), crossTxRequestMsg)
	require.NoError(t, err)

	// Check tx A and tx B in parallel
	type txResult struct {
		tx      *types.Transaction
		receipt *types.Receipt
		err     error
	}

	var wg sync.WaitGroup
	resultA := make(chan txResult, 1)
	resultB := make(chan txResult, 1)

	// Check tx A
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, receipt, err := transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
		resultA <- txResult{tx: tx, receipt: receipt, err: err}
	}()

	// Check tx B
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, receipt, err := transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
		resultB <- txResult{tx: tx, receipt: receipt, err: err}
	}()

	// Wait for both goroutines to complete
	wg.Wait()

	// Get results for tx A
	resA := <-resultA
	require.NoError(t, resA.err)
	require.NotNil(t, resA.tx)
	require.NotNil(t, resA.receipt)
	// check tx is successful
	assert.Equal(t, resA.receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *resA.tx.To(), bridgeAddr)
	assert.True(t, bytes.Equal(resA.tx.Data(), calldataA))

	// Get results for tx B
	resB := <-resultB
	require.NoError(t, resB.err)
	require.NotNil(t, resB.tx)
	require.NotNil(t, resB.receipt)
	// check tx is successful
	assert.Equal(t, resB.receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *resB.tx.To(), bridgeAddr)
	assert.True(t, bytes.Equal(resB.tx.Data(), calldataB))

	// check balances after txs
	tokenBalanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tokenBalanceAAfter)
	tokenBalanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tokenBalanceBAfter)
	assert.Equal(t, initialTokenBalanceA.Sub(initialTokenBalanceA, transferredAmount), tokenBalanceAAfter)
	assert.Equal(t, initialTokenBalanceB.Add(initialTokenBalanceB, transferredAmount), tokenBalanceBAfter)
}

/*
TestSendCrossTxBridgeFromBToA sends tokens from chain B to chain A and sends the txs as cross rollup tx
  - create txA that is a send tokens bridge call on chain B
  - create txB that is a receive tokens bridge call on chain A
  - check if balances are updated correctly, both tx successfull and tx data not malformed
*/
func TestSendCrossTxBridgeFromBToA(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address
	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	// get initial token balance for both accounts
	initialTokenBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialTokenBalanceB)
	initialTokenBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialTokenBalanceA)

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	// construct contract call parameters for transaction from accountB
	calldataB, err := BridgeABI.Pack("send",
		TestRollupA.ChainID(), // otherChainId
		configs.Values.L2.Contracts[configs.ContractNameToken].Address, // token
		TestAccountB.GetAddress(),                                      // sender
		TestAccountA.GetAddress(),                                      // receiver
		transferredAmount,                                              // amount
		sessionID,                                                      // sessionId
		bridgeAddr,                                                     // srcBridge
	)
	require.NoError(t, err)
	require.NotNil(t, calldataB)

	// Create transaction details
	transactionADetails := transactions.TransactionDetails{
		To:        bridgeAddr,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataB,
	}

	// create transaction to be sent from accountB
	txB, signedTransactionB, err := transactions.CreateTransaction(ctx, transactionADetails, TestAccountB)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionB)
	// preparations for tx B done -------------------------------------------------------------

	// construct contract call parameters for transaction from accountA
	calldataA, err := BridgeABI.Pack("receiveTokens",
		TestRollupB.ChainID(),     // ChainSrc
		TestAccountB.GetAddress(), // sender
		TestAccountA.GetAddress(), // receiver
		sessionID,                 // sessionId
		bridgeAddr,                // srcBridge
	)
	require.NoError(t, err)
	require.NotNil(t, calldataA)

	// Create transaction details
	transactionBDetails := transactions.TransactionDetails{
		To:        bridgeAddr,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldataA,
	}

	// create transaction to be sent from accountA
	txA, signedTransactionA, err := transactions.CreateTransaction(ctx, transactionBDetails, TestAccountA)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionA)
	// preparations for tx A done -------------------------------------------------------------

	// create cross tx request msg
	crossTxRequestMsg, err := transactions.CreateCrossTxRequestMsg(ctx, TestAccountB, TestAccountA, signedTransactionB, signedTransactionA)
	require.NoError(t, err)
	require.NotNil(t, crossTxRequestMsg)

	// send cross tx request msg to source chain (B)
	err = transactions.SendCrossTxRequestMsg(ctx, TestRollupB.RPCURL(), crossTxRequestMsg)
	require.NoError(t, err)

	// Check tx A and tx B in parallel
	type txResult struct {
		tx      *types.Transaction
		receipt *types.Receipt
		err     error
	}

	var wg sync.WaitGroup
	resultA := make(chan txResult, 1)
	resultB := make(chan txResult, 1)

	// Check tx A
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, receipt, err := transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
		resultA <- txResult{tx: tx, receipt: receipt, err: err}
	}()

	// Check tx B
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, receipt, err := transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
		resultB <- txResult{tx: tx, receipt: receipt, err: err}
	}()

	// Wait for both goroutines to complete
	wg.Wait()

	// Get results for tx B
	resB := <-resultB
	require.NoError(t, resB.err)
	require.NotNil(t, resB.tx)
	require.NotNil(t, resB.receipt)
	// check tx is successful
	assert.Equal(t, resB.receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *resB.tx.To(), bridgeAddr)
	assert.True(t, bytes.Equal(resB.tx.Data(), calldataB))

	// Get results for tx A
	resA := <-resultA
	require.NoError(t, resA.err)
	require.NotNil(t, resA.tx)
	require.NotNil(t, resA.receipt)
	// check tx is successful
	assert.Equal(t, resA.receipt.Status, types.ReceiptStatusSuccessful)
	// check that calldata and receiver are not malformed
	assert.Equal(t, *resA.tx.To(), bridgeAddr)
	assert.True(t, bytes.Equal(resA.tx.Data(), calldataA))

	// check balances after txs
	tokenBalanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tokenBalanceBAfter)
	tokenBalanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tokenBalanceAAfter)
	assert.Equal(t, initialTokenBalanceB.Sub(initialTokenBalanceB, transferredAmount), tokenBalanceBAfter)
	assert.Equal(t, initialTokenBalanceA.Add(initialTokenBalanceA, transferredAmount), tokenBalanceAAfter)
}

/*
TestSendOnAAndFailingSelfMoveBalanceOnB
  - create txA that is a send tokens bridge call on chain A
  - create txB that is a self move balance that fails on chain B
  - check if neither of the transactions are executed
*/
func TestSendOnAAndFailingSelfMoveBalanceOnB(t *testing.T) {
	ctx := t.Context()
	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address

	// get token balance for account A
	initialTokenBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialTokenBalanceA)
	// get eth balance for account B
	initialBalanceB, err := TestAccountB.GetBalance(ctx)
	require.NoError(t, err)
	require.NotNil(t, initialBalanceB)

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	// construct contract call parameters for transaction from accountA
	calldataA, err := BridgeABI.Pack("send",
		TestRollupB.ChainID(), // otherChainId
		configs.Values.L2.Contracts[configs.ContractNameToken].Address, // token
		TestAccountA.GetAddress(),                                      // sender
		TestAccountB.GetAddress(),                                      // receiver
		transferredAmount,                                              // amount
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
	txA, signedTransactionA, err := transactions.CreateTransaction(ctx, transactionADetails, TestAccountA)
	require.NoError(t, err)
	require.NotNil(t, signedTransactionA)
	// preparations for tx A done -------------------------------------------------------------

	// get balance of accountB
	balanceB, err := TestAccountB.GetBalance(ctx)
	require.NoError(t, err)
	require.NotNil(t, balanceB)

	// create txB details
	txBDetails := transactions.TransactionDetails{
		To:        TestAccountB.GetAddress(),
		Value:     balanceB.Add(balanceB, big.NewInt(100000)), // more than balanceB
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

	// neither tx should be sent to the chain
	_, _, err = transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction receipt not found after 10 retries for hash")

	_, _, err = transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction receipt not found after 10 retries for hash")

	// token balance on A should be the same as before
	tokenBalanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tokenBalanceAAfter)
	assert.Equal(t, initialTokenBalanceA, tokenBalanceAAfter)
	// eth balance on B should be the same as before
	balanceBAfter, err := TestAccountB.GetBalance(ctx)
	require.NoError(t, err)
	require.NotNil(t, balanceBAfter)
	assert.Equal(t, initialBalanceB, balanceBAfter)
}

/*
TestSendCrossTxBridgeWithOutOfGasOnB
  - create txA that is a send tokens bridge call on chain A
  - create txB that is a receive tokens on chain B with out of gas
  - check if neither of the transactions are executed
*/
func TestSendCrossTxBridgeWithOutOfGasOnB(t *testing.T) {
	ctx := t.Context()
	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address

	// get initial token balance for account A
	initialTokenBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialTokenBalanceA)
	// get initial token balance for account B
	initialTokenBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialTokenBalanceB)

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	// construct contract call parameters for transaction from accountA
	calldataA, err := BridgeABI.Pack("send",
		TestRollupB.ChainID(), // otherChainId
		configs.Values.L2.Contracts[configs.ContractNameToken].Address, // token
		TestAccountA.GetAddress(),                                      // sender
		TestAccountB.GetAddress(),                                      // receiver
		transferredAmount,                                              // amount
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
		Gas:       300000, // 318,316 gas is needed
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

	// neither tx should be sent to the chain
	_, _, err = transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction receipt not found after 10 retries for hash")

	_, _, err = transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction receipt not found after 10 retries for hash")

	// check balances after txs
	tokenBalanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	assert.Equal(t, initialTokenBalanceA, tokenBalanceAAfter)
	tokenBalanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	assert.Equal(t, initialTokenBalanceB, tokenBalanceBAfter)
}

/*
TestSelfMoveBalanceOnAandreceiveTokensOnB
- create txA that is a self move balance on chain A -> nothing to do with contract, should succeed on chainA
- create txB that is a receive tokens on chain B -> should fail on chainB because nobody called send on chain A
- check if neither of the transactions are executed
*/
func TestSelfMoveBalanceOnAandreceiveTokensOnB(t *testing.T) {
	ctx := t.Context()
	bridgeAddr := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	// get initial eth balance for account A
	initialBalanceA, err := TestAccountA.GetBalance(ctx)
	require.NoError(t, err)
	require.NotNil(t, initialBalanceA)

	// generate random session ID , will be used for both transactions
	sessionID := transactions.GenerateRandomSessionID()

	// construct contract call parameters for transaction from accountA
	transactionADetails := transactions.TransactionDetails{
		To:        TestAccountA.GetAddress(),
		Value:     big.NewInt(500000000000000000), // 0.5 eth
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      nil, // no data needed for self move balance
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

	// neither of txs should be processed
	_, _, err = transactions.GetTransactionDetails(ctx, txA.Hash(), TestRollupA)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction receipt not found after 10 retries for hash")
	_, _, err = transactions.GetTransactionDetails(ctx, txB.Hash(), TestRollupB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction receipt not found after 10 retries for hash")

	// check balances after txs
	balanceAAfter, err := TestAccountA.GetBalance(ctx)
	require.NoError(t, err)
	require.NotNil(t, balanceAAfter)
	assert.Equal(t, initialBalanceA, balanceAAfter)
}
