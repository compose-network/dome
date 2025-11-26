package test

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/compose-network/dome/configs"
	"github.com/compose-network/dome/internal/accounts"
	"github.com/compose-network/dome/internal/helpers"
	"github.com/compose-network/dome/internal/logger"
	"github.com/compose-network/dome/internal/transactions"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

const (
	// for single account and multiple transactions tests
	numOfTxs = 25 // total number of txs
	// for multiple accounts and 1 transaction per account tests
	numOfAccounts = 25 // total number of accounts to be spawned
	// for multiple accounts and multiple transactions tests. Ex: 5 accounts will send 5 txs each with 100ms delay between them => 25 txs in total with 100ms delay between them.
	numOfTxsForMultipleAccounts = 5 // max number of txs to be sent in parallel for each account
	numOfAccountsForMultipleTxs = 5 // number of accounts to be spawned in parallel
	// general delay between cross-rollup txs
	delay = 100 * time.Millisecond // delay between txs
)

/*
TestStressBridgeSameAccount will build numOfTxs transactions with the same account and send them to the bridge with delay.
*/
func TestStressBridgeSameAccount(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address

	transferedAmount := big.NewInt(500000000000000000)                       // 0.5 tokens
	mintedAmount := new(big.Int).Mul(transferedAmount, big.NewInt(numOfTxs)) // enough to send all txs

	// mint tokens for sender account
	tx, hash, err := helpers.SendMintTx(t, TestAccountA, mintedAmount, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, hash)

	// get starting nonces for sender account
	startingNonceA, err := TestAccountA.GetNonce(ctx)
	require.NoError(t, err)
	startingNonceB, err := TestAccountB.GetNonce(ctx)
	require.NoError(t, err)

	// get initial balances
	initialBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	initialBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)

	var txs_A []*types.Transaction
	var txs_B []*types.Transaction

	for i := 0; i < numOfTxs; i++ {
		logger.Info("Creating set of txs with nonce %d and %d", startingNonceA+uint64(i), startingNonceB+uint64(i))
		txA, txB, err := helpers.SendBridgeTxWithNonce(t, TestAccountA, startingNonceA+uint64(i), TestAccountB, startingNonceB+uint64(i), transferedAmount, TokenABI, BridgeABI)
		txs_A = append(txs_A, txA)
		txs_B = append(txs_B, txB)
		require.NoError(t, err)
		require.NotNil(t, txA)
		require.NotNil(t, txB)
		time.Sleep(delay)
	}

	// check the txs
	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	for _, tx := range txs_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)

	}
	for _, tx := range txs_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	}

	// check balances after txs
	balanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceAAfter)
	balanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceBAfter)

	expectedSentAmount := new(big.Int).Mul(transferedAmount, big.NewInt(numOfTxs))
	expectedBalanceA := new(big.Int).Sub(initialBalanceA, expectedSentAmount)
	expectedBalanceB := new(big.Int).Add(initialBalanceB, expectedSentAmount)
	require.Equal(t, expectedBalanceA, balanceAAfter)
	require.Equal(t, expectedBalanceB, balanceBAfter)
}

/*
TestStressBridgeDifferentAccounts will spawn <numOfAccounts> accounts on both rollups and send 1 transaction from each with delay between them.
*/
func TestStressBridgeDifferentAccounts(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address
	bridgeAddress := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	mintedAndTransferredAmount := big.NewInt(1000000000000000000) // 1 token
	//spam x nr of accounts on both rollups
	accountsOnRollupA := make([]*accounts.Account, numOfAccounts)
	accountsOnRollupB := make([]*accounts.Account, numOfAccounts)
	for i := 0; i < numOfAccounts; i++ {
		pk, err := crypto.GenerateKey()
		require.NoError(t, err)
		pkHex := hex.EncodeToString(crypto.FromECDSA(pk))
		accountsOnRollupA[i], err = accounts.NewRollupAccount(pkHex, TestRollupA)
		require.NoError(t, err)
		accountsOnRollupB[i], err = accounts.NewRollupAccount(pkHex, TestRollupB)
		require.NoError(t, err)
	}

	//distribute 0.1 eth to all accounts for gass
	logger.Info("Distributing 0.1 eth to all accounts...")
	err := transactions.DistributeEth(ctx, TestAccountA, accountsOnRollupA, big.NewInt(100000000000000000))
	require.NoError(t, err)
	err = transactions.DistributeEth(ctx, TestAccountB, accountsOnRollupB, big.NewInt(100000000000000000))
	require.NoError(t, err)

	// mint tokens for A accounts
	logger.Info("Minting tokens to all accounts...")
	for _, acc := range accountsOnRollupA {
		tx, hash, err := helpers.SendMintTx(t, acc, mintedAndTransferredAmount, TokenABI)
		require.NoError(t, err)
		require.NotNil(t, tx)
		require.NotNil(t, hash)
	}

	// approve tokens for the bridge contract
	logger.Info("Approving tokens for the bridge contract...")
	for _, acc := range accountsOnRollupA {
		_, _, err := helpers.ApproveTokens(t, acc, bridgeAddress, TokenABI)
		require.NoError(t, err)
	}

	var txs_A []*types.Transaction
	var txs_B []*types.Transaction
	// send bridge txs from A to B with delay
	for i := range len(accountsOnRollupA) {
		txA, txB, err := helpers.SendBridgeTx(t, accountsOnRollupA[i], accountsOnRollupB[i], mintedAndTransferredAmount, TokenABI, BridgeABI)
		txs_A = append(txs_A, txA)
		txs_B = append(txs_B, txB)
		require.NoError(t, err)
		require.NotNil(t, txA)
		require.NotNil(t, txB)
		time.Sleep(delay)
	}

	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	for _, tx := range txs_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}

	for _, tx := range txs_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}

	// expected balances
	for _, acc := range accountsOnRollupA {
		balance, err := acc.GetTokensBalance(ctx, tokenAddress, TokenABI)
		require.NoError(t, err)
		require.Equal(t, 0, balance.Cmp(big.NewInt(0))) // on rollup A, all tokens should be sent to rollup B
	}
	for _, acc := range accountsOnRollupB {
		balance, err := acc.GetTokensBalance(ctx, tokenAddress, TokenABI)
		require.NoError(t, err)
		require.Equal(t, 0, balance.Cmp(mintedAndTransferredAmount)) // on rollup B, all tokens should be received from rollup A
	}
}

/*
TestStressMultipleAccountsAndMultipleTxs will spawn <numOfAccounts> accounts on both rollups and send <numOfTxs> transactions with <delay> between them.
The txs will be sent in parallel up to <maxNumOfTxsInParalel> txs at a time.
*/
func TestStressMultipleAccountsAndMultipleTxs(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address
	bridgeAddress := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	//spam x nr of accounts on both rollups
	accountsOnRollupA := make([]*accounts.Account, numOfAccountsForMultipleTxs)
	accountsOnRollupB := make([]*accounts.Account, numOfAccountsForMultipleTxs)
	for i := range numOfAccountsForMultipleTxs {
		pk, err := crypto.GenerateKey()
		require.NoError(t, err)
		pkHex := hex.EncodeToString(crypto.FromECDSA(pk))
		accountsOnRollupA[i], err = accounts.NewRollupAccount(pkHex, TestRollupA)
		require.NoError(t, err)
		accountsOnRollupB[i], err = accounts.NewRollupAccount(pkHex, TestRollupB)
		require.NoError(t, err)
	}

	//distribute 0.1 eth to all accounts
	logger.Info("Distributing 0.1 eth to all accounts...")
	err := transactions.DistributeEth(ctx, TestAccountA, accountsOnRollupA, big.NewInt(100000000000000000))
	require.NoError(t, err)
	err = transactions.DistributeEth(ctx, TestAccountB, accountsOnRollupB, big.NewInt(100000000000000000))
	require.NoError(t, err)

	// get needed mint amount
	transferredAmount := big.NewInt(1000000000000000000)                                         // 1 token
	mintedAmount := new(big.Int).Mul(transferredAmount, big.NewInt(numOfTxsForMultipleAccounts)) // enough to send all txs

	// mint tokens for all accounts
	logger.Info("Minting tokens for all accounts on rollup A...")
	for _, acc := range accountsOnRollupA {
		tx, hash, err := helpers.SendMintTx(t, acc, mintedAmount, TokenABI)
		require.NoError(t, err)
		require.NotNil(t, tx)
		require.NotNil(t, hash)
	}

	// approve tokens for the bridge contract
	logger.Info("Approving tokens for the bridge contract...")
	for _, acc := range accountsOnRollupA {
		_, _, err := helpers.ApproveTokens(t, acc, bridgeAddress, TokenABI)
		require.NoError(t, err)
	}

	// nonces
	var noncesA []uint64
	var noncesB []uint64
	for i := 0; i < numOfAccountsForMultipleTxs; i++ {
		// get nonce for both accounts
		nonceA, err := accountsOnRollupA[i].GetNonce(ctx)
		noncesA = append(noncesA, nonceA)
		require.NoError(t, err)
		nonceB, err := accountsOnRollupB[i].GetNonce(ctx)
		noncesB = append(noncesB, nonceB)
		require.NoError(t, err)
	}

	// send bridge txs
	var txs_A []*types.Transaction
	var txs_B []*types.Transaction

	// for each account on A
	for i := range accountsOnRollupA {
		// for each tx to be sent
		for j := 0; j < numOfTxsForMultipleAccounts; j++ {
			// build bridge txs with different nonces
			txA, txB, err := helpers.SendBridgeTxWithNonce(t, accountsOnRollupA[i], noncesA[i]+uint64(j), accountsOnRollupB[i], noncesB[i]+uint64(j), transferredAmount, TokenABI, BridgeABI)
			require.NoError(t, err)
			require.NotNil(t, txA)
			require.NotNil(t, txB)
			txs_A = append(txs_A, txA)
			txs_B = append(txs_B, txB)
			time.Sleep(delay)
		}
	}

	// wait 30s until we check the txs
	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	// check if all txs are successful
	for _, tx := range txs_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}
	for _, tx := range txs_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}

	// expected balances
	for _, acc := range accountsOnRollupA {
		balance, err := acc.GetTokensBalance(ctx, tokenAddress, TokenABI)
		require.NoError(t, err)
		require.Equal(t, 0, balance.Cmp(big.NewInt(0))) // on rollup A, all tokens should be sent to rollup B
	}
	for _, acc := range accountsOnRollupB {
		balance, err := acc.GetTokensBalance(ctx, tokenAddress, TokenABI)
		require.NoError(t, err)
		expected := new(big.Int).Mul(transferredAmount, big.NewInt(numOfTxsForMultipleAccounts))
		require.Equal(t, 0, balance.Cmp(expected)) // on rollup B, all tokens sent from A should be received
	}
}

/*
TestStressAtoBAndBtoA will use 1 account <numOfTxs> txs from A to B and B to A with delay between them.
*/
func TestStressAtoBAndBtoA(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address

	mintedAndTransferredAmount := big.NewInt(1000000000000000000) // 1 token

	// mint tokens for sender account
	tx, hash, err := helpers.SendMintTx(t, TestAccountA, mintedAndTransferredAmount, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, hash)

	// get initial balances
	initialBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialBalanceA)
	initialBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialBalanceB)

	// get nonces for both accounts
	nonceA, err := TestAccountA.GetNonce(ctx)
	require.NoError(t, err)
	nonceB, err := TestAccountB.GetNonce(ctx)
	require.NoError(t, err)

	// send bridge txs from A to B and B to A with increasing nonce.
	// Track legs per rollup so we query the correct chain later.
	var txs_AtoB_A []*types.Transaction
	var txs_AtoB_B []*types.Transaction
	var txs_BtoA_A []*types.Transaction
	var txs_BtoA_B []*types.Transaction

	// totalNumOfTxs is half of numOfTxs, rounded down (e.g., 25 -> 12)
	totalNumOfTxs := numOfTxs / 2
	for i := 0; i < totalNumOfTxs; i++ {
		// Interleave nonces per chain so we never reuse the same nonce
		// for different bridge directions on the same account:
		// - A nonces: nonceA + 2*i      (A->B) and nonceA + 2*i+1      (B->A leg)
		// - B nonces: nonceB + 2*i      (A->B leg) and nonceB + 2*i+1  (B->A)
		aNonceAtoB := nonceA + uint64(2*i)
		bNonceAtoB := nonceB + uint64(2*i)
		bNonceBtoA := nonceB + uint64(2*i+1)
		aNonceBtoA := nonceA + uint64(2*i+1)

		// Bridge from A to B
		txA, txB, err := helpers.SendBridgeTxWithNonce(t, TestAccountA, aNonceAtoB, TestAccountB, bNonceAtoB, mintedAndTransferredAmount, TokenABI, BridgeABI)
		txs_AtoB_A = append(txs_AtoB_A, txA)
		txs_AtoB_B = append(txs_AtoB_B, txB)
		require.NoError(t, err)
		require.NotNil(t, txA)
		require.NotNil(t, txB)
		time.Sleep(delay)

		// Bridge from B back to A
		txB, txA, err = helpers.SendBridgeTxWithNonce(t, TestAccountB, bNonceBtoA, TestAccountA, aNonceBtoA, mintedAndTransferredAmount, TokenABI, BridgeABI)
		txs_BtoA_B = append(txs_BtoA_B, txB)
		txs_BtoA_A = append(txs_BtoA_A, txA)
		require.NoError(t, err)
		require.NotNil(t, txA)
		require.NotNil(t, txB)
		time.Sleep(delay)
	}

	// wait 30s until we check the txs
	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	// A→B legs
	for _, tx := range txs_AtoB_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}
	for _, tx := range txs_AtoB_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}

	// B→A legs
	for _, tx := range txs_BtoA_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}
	for _, tx := range txs_BtoA_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}

	// expected balances
	balanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceAAfter)
	balanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceBAfter)
	// should be the same as initial balance because we transferred the same amount of tokens back and forth
	require.Equal(t, initialBalanceA, balanceAAfter)
	require.Equal(t, initialBalanceB, balanceBAfter)
}

/*
TestStressNormalTxsMixWithCrossRollupTxs will use 1 account and send a self move balance tx and a bridge tx alternatively with increasing nonce and with delay between them.
*/
func TestStressNormalTxsMixWithCrossRollupTxs(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address

	transferedAmount := big.NewInt(500000000000000000)                       // 0.5 tokens
	mintedAmount := new(big.Int).Mul(transferedAmount, big.NewInt(numOfTxs)) // enough to send all txs

	// mint tokens for sender account
	tx, hash, err := helpers.SendMintTx(t, TestAccountA, mintedAmount, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, hash)

	// get initial balances
	initialBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialBalanceA)
	initialBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, initialBalanceB)

	// get nonces for both accounts
	nonceA, err := TestAccountA.GetNonce(ctx)
	require.NoError(t, err)
	nonceB, err := TestAccountB.GetNonce(ctx)
	require.NoError(t, err)

	// send self move balance tx and bridge tx alternatively with increasing nonce and with delay between them
	var txs_selfMoveBalance []*types.Transaction
	var txs_bridgeTxA []*types.Transaction
	var txs_bridgeTxB []*types.Transaction

	selfMoveBalanceAmount := big.NewInt(100000000000000000) // 0.1 eth
	for i := 0; i < numOfTxs; i++ {
		// Interleave nonces so we never replace a bridge tx with a self-move tx:
		// self-move uses even offsets, bridge uses odd offsets.
		selfNonceA := nonceA + uint64(2*i)
		bridgeNonceA := nonceA + uint64(2*i+1)
		bridgeNonceB := nonceB + uint64(i)

		// Self-move balance tx on rollup A
		tx, hash, err := helpers.SendSelfMoveBalanceTxWithNonce(ctx, TestAccountA, selfNonceA, selfMoveBalanceAmount)
		require.NoError(t, err)
		require.NotNil(t, tx)
		require.NotNil(t, hash)
		txs_selfMoveBalance = append(txs_selfMoveBalance, tx)
		time.Sleep(delay)

		// Cross-rollup bridge tx (A -> B)
		txA, txB, err := helpers.SendBridgeTxWithNonce(t, TestAccountA, bridgeNonceA, TestAccountB, bridgeNonceB, transferedAmount, TokenABI, BridgeABI)
		require.NoError(t, err)
		require.NotNil(t, txA)
		require.NotNil(t, txB)
		txs_bridgeTxA = append(txs_bridgeTxA, txA)
		txs_bridgeTxB = append(txs_bridgeTxB, txB)
		time.Sleep(delay)
	}

	// wait 30s until we check the txs
	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	for _, tx := range txs_selfMoveBalance {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}
	for _, tx := range txs_bridgeTxA {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}
	for _, tx := range txs_bridgeTxB {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}

	// expected balances
	balanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	balanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceBAfter)
	require.Equal(t, new(big.Int).Sub(initialBalanceA, mintedAmount), balanceAAfter)
	require.Equal(t, new(big.Int).Add(initialBalanceB, mintedAmount), balanceBAfter)
}

/*
TestStressWithWrongNonce will send a total of numOfTxs from same account and half of them with the wrong nonce.
It will then check if the txs with the wrong nonce are not processed and the txs with the correct nonce are processed.
It will also check if the balances are correct.
*/
func TestStressFromSameAccountHalfWrongNonce(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address
	
	transferedAmount := big.NewInt(500000000000000000)                       // 0.5 tokens
	mintedAmount := new(big.Int).Mul(transferedAmount, big.NewInt(numOfTxs)) // enough to send all txs

	// mint tokens for sender account
	tx, hash, err := helpers.SendMintTx(t, TestAccountA, mintedAmount, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, hash)

	// get starting nonces for sender account
	startingNonceA, err := TestAccountA.GetNonce(ctx)
	require.NoError(t, err)
	startingNonceB, err := TestAccountB.GetNonce(ctx)
	require.NoError(t, err)

	// get initial balances
	initialBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	initialBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)

	var txs_A []*types.Transaction
	var txs_B []*types.Transaction
	var txs_wrongNonceA []*types.Transaction
	var txs_wrongNonceB []*types.Transaction

	// we use numOfTxs/2 because we send half of the txs with the wrong nonce and half with the correct nonce. Total number of txs will still be numOfTxs.
	for i := 0; i < numOfTxs/2; i++ {
		logger.Info("Creating set of txs with nonce %d and %d", startingNonceA+uint64(i), startingNonceB+uint64(i))
		txA, txB, err := helpers.SendBridgeTxWithNonce(t, TestAccountA, startingNonceA+uint64(i), TestAccountB, startingNonceB+uint64(i), transferedAmount, TokenABI, BridgeABI)
		txs_A = append(txs_A, txA)
		txs_B = append(txs_B, txB)
		require.NoError(t, err)
		require.NotNil(t, txA)
		require.NotNil(t, txB)
		txWrongNonceA, txWrongNonceB, err := helpers.SendBridgeTxWithNonce(t, TestAccountA, startingNonceA+uint64(i)-1, TestAccountB, startingNonceB+uint64(i)-1, transferedAmount, TokenABI, BridgeABI)
		txs_wrongNonceA = append(txs_wrongNonceA, txWrongNonceA)
		txs_wrongNonceB = append(txs_wrongNonceB, txWrongNonceB)
		require.NoError(t, err)
		require.NotNil(t, txWrongNonceA)
		require.NotNil(t, txWrongNonceB)
		time.Sleep(delay)
	}

	// check the txs
	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	for _, tx := range txs_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)

	}
	for _, tx := range txs_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	}
	// txs with wrong nonce should not be processed
	for _, tx := range txs_wrongNonceA {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.Nil(t, receipt)
		require.Error(t, err)
	}
	for _, tx := range txs_wrongNonceB {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.Nil(t, receipt)
		require.Error(t, err)
	}

	// check balances after txs
	balanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceAAfter)
	balanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceBAfter)
	// we use here again numOfTxs/2 because we sent only half of the txs with the correct nonce.
	expectedSentAmount := new(big.Int).Mul(transferedAmount, big.NewInt(numOfTxs/2))
	expectedBalanceA := new(big.Int).Sub(initialBalanceA, expectedSentAmount)
	expectedBalanceB := new(big.Int).Add(initialBalanceB, expectedSentAmount)
	require.Equal(t, expectedBalanceA, balanceAAfter)
	require.Equal(t, expectedBalanceB, balanceBAfter)
}

/*
TestStressFromSameAccountHalfOutOfGas will send a total of numOfTxs from same account and half of them with out of gas error.
It will then check if the txs with the wrong nonce are not processed and the txs with the correct nonce are processed.
It will also check if the balances are correct.
*/
func TestStressFromSameAccountHalfOutOfGas(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address
	
	transferedAmount := big.NewInt(500000000000000000)                       // 0.5 tokens
	mintedAmount := new(big.Int).Mul(transferedAmount, big.NewInt(numOfTxs)) // enough to send all txs

	// mint tokens for sender account
	tx, hash, err := helpers.SendMintTx(t, TestAccountA, mintedAmount, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, hash)

	// get starting nonces for sender account
	startingNonceA, err := TestAccountA.GetNonce(ctx)
	require.NoError(t, err)
	startingNonceB, err := TestAccountB.GetNonce(ctx)
	require.NoError(t, err)

	// get initial balances
	initialBalanceA, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	initialBalanceB, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)

	var txs_A []*types.Transaction
	var txs_B []*types.Transaction
	var txs_OutOfGasA []*types.Transaction
	var txs_OutOfGasB []*types.Transaction

	// we use numOfTxs/2 because we send half of the txs with the wrong nonce and half with the correct nonce. Total number of txs will still be numOfTxs.
	for i := 0; i < numOfTxs/2; i++ {
		logger.Info("Creating set of txs with nonce %d and %d", startingNonceA+uint64(i), startingNonceB+uint64(i))
		txA, txB, err := helpers.SendBridgeTxWithNonce(t, TestAccountA, startingNonceA+uint64(i), TestAccountB, startingNonceB+uint64(i), transferedAmount, TokenABI, BridgeABI)
		txs_A = append(txs_A, txA)
		txs_B = append(txs_B, txB)
		require.NoError(t, err)
		require.NotNil(t, txA)
		require.NotNil(t, txB)
		txOutOfGasA, txOutOfGasB, err := helpers.SendFailingBridgeTxOutOfGasWithNonce(t, TestAccountA, startingNonceA+uint64(i+1), TestAccountB, startingNonceB+uint64(i+1), transferedAmount, TokenABI, BridgeABI)
		txs_OutOfGasA = append(txs_OutOfGasA, txOutOfGasA)
		txs_OutOfGasB = append(txs_OutOfGasB, txOutOfGasB)
		require.NoError(t, err)
		require.NotNil(t, txOutOfGasA)
		require.NotNil(t, txOutOfGasB)
		time.Sleep(delay)
	}

	// check the txs
	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	for _, tx := range txs_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)

	}
	for _, tx := range txs_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
	}
	// txs with wrong nonce should not be processed
	for _, tx := range txs_OutOfGasA {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.Nil(t, receipt)
		require.Error(t, err)
	}
	for _, tx := range txs_OutOfGasB {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.Nil(t, receipt)
		require.Error(t, err)
	}

	// check balances after txs
	balanceAAfter, err := TestAccountA.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceAAfter)
	balanceBAfter, err := TestAccountB.GetTokensBalance(ctx, tokenAddress, TokenABI)
	require.NoError(t, err)
	require.NotNil(t, balanceBAfter)
	// we use here again numOfTxs/2 because we sent only half of the txs with the correct nonce.
	expectedSentAmount := new(big.Int).Mul(transferedAmount, big.NewInt(numOfTxs/2))
	expectedBalanceA := new(big.Int).Sub(initialBalanceA, expectedSentAmount)
	expectedBalanceB := new(big.Int).Add(initialBalanceB, expectedSentAmount)
	require.Equal(t, expectedBalanceA, balanceAAfter)
	require.Equal(t, expectedBalanceB, balanceBAfter)
}

/*
TestStressPartiallyFailingBridgeFromMultipleAccounts will send a total of numOfTxs from multiple accounts and half of them with out of gas error.
It will then check if the txs with the out of gas error are not processed and the txs with the correct nonces are processed.
It will also check if the balances are correct.
*/
func TestStressPartiallyFailingBridgeFromMultipleAccounts(t *testing.T) {
	ctx := t.Context()
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address
	bridgeAddress := configs.Values.L2.Contracts[configs.ContractNameBridge].Address

	//spam x nr of accounts on both rollups that will have tokens 
	accountsOnRollupA := make([]*accounts.Account, numOfAccountsForMultipleTxs)
	accountsOnRollupB := make([]*accounts.Account, numOfAccountsForMultipleTxs)
	for i := range numOfAccountsForMultipleTxs {
		pk, err := crypto.GenerateKey()
		require.NoError(t, err)
		pkHex := hex.EncodeToString(crypto.FromECDSA(pk))
		accountsOnRollupA[i], err = accounts.NewRollupAccount(pkHex, TestRollupA)
		require.NoError(t, err)
		accountsOnRollupB[i], err = accounts.NewRollupAccount(pkHex, TestRollupB)
		require.NoError(t, err)
	}

	// spam x nr of accounts on rollup A that will NOT have tokens 
	// intentions here are to mix transactions from this accounts with transactions from accounts on rollup A that will have tokens
	// in this way we have a mix of failing tx and successfull txs 
	accountsOnRollupAWithoutTokens := make([]*accounts.Account, numOfAccountsForMultipleTxs)
	accountsOnRollupBWithoutTokens := make([]*accounts.Account, numOfAccountsForMultipleTxs)
	for i := range numOfAccountsForMultipleTxs {
		pk, err := crypto.GenerateKey()
		require.NoError(t, err)
		pkHex := hex.EncodeToString(crypto.FromECDSA(pk))
		accountsOnRollupAWithoutTokens[i], err = accounts.NewRollupAccount(pkHex, TestRollupA)
		require.NoError(t, err)
		accountsOnRollupBWithoutTokens[i], err = accounts.NewRollupAccount(pkHex, TestRollupB)
		require.NoError(t, err)
	}

	//distribute 0.1 eth to all accounts for gas
	logger.Info("Distributing 0.1 eth to all accounts...")
	err := transactions.DistributeEth(ctx, TestAccountA, accountsOnRollupA, big.NewInt(100000000000000000))
	require.NoError(t, err)
	err = transactions.DistributeEth(ctx, TestAccountB, accountsOnRollupB, big.NewInt(100000000000000000))
	require.NoError(t, err)
	err = transactions.DistributeEth(ctx, TestAccountA, accountsOnRollupAWithoutTokens, big.NewInt(100000000000000000))
	require.NoError(t, err)
	err = transactions.DistributeEth(ctx, TestAccountB, accountsOnRollupBWithoutTokens, big.NewInt(100000000000000000))
	require.NoError(t, err)

	// get needed mint amount
	transferredAmount := big.NewInt(1000000000000000000)                                         // 1 token
	mintedAmount := new(big.Int).Mul(transferredAmount, big.NewInt(numOfTxsForMultipleAccounts)) // enough to send all txs

	// mint tokens for all accounts
	logger.Info("Minting tokens for all accounts on rollup A...")
	for _, acc := range accountsOnRollupA {
		tx, hash, err := helpers.SendMintTx(t, acc, mintedAmount, TokenABI)
		require.NoError(t, err)
		require.NotNil(t, tx)
		require.NotNil(t, hash)
	}

	// approve tokens for the bridge contract
	logger.Info("Approving tokens for the bridge contract...")
	for _, acc := range accountsOnRollupA {
		_, _, err := helpers.ApproveTokens(t, acc, bridgeAddress, TokenABI)
		require.NoError(t, err)
	}
	// need approval for this accounts too because we want to fail with insuficient funds error 
	for _, acc := range accountsOnRollupAWithoutTokens {
		_, _, err := helpers.ApproveTokens(t, acc, bridgeAddress, TokenABI)
		require.NoError(t, err)
	}

	// nonces
	var noncesA []uint64
	var noncesB []uint64
	var noncesAWithoutTokens []uint64
	var noncesBWithoutTokens []uint64
	for i := 0; i < numOfAccountsForMultipleTxs; i++ {
		// get nonce for both accounts
		nonceA, err := accountsOnRollupA[i].GetNonce(ctx)
		noncesA = append(noncesA, nonceA)
		require.NoError(t, err)
		nonceB, err := accountsOnRollupB[i].GetNonce(ctx)
		noncesB = append(noncesB, nonceB)
		require.NoError(t, err)
		nonceAWithoutTokens, err := accountsOnRollupAWithoutTokens[i].GetNonce(ctx)
		noncesAWithoutTokens = append(noncesAWithoutTokens, nonceAWithoutTokens)
		require.NoError(t, err)
		nonceBWithoutTokens, err := accountsOnRollupBWithoutTokens[i].GetNonce(ctx)
		noncesBWithoutTokens = append(noncesBWithoutTokens, nonceBWithoutTokens)
		require.NoError(t, err)
	}

	// send bridge txs
	var txs_A []*types.Transaction
	var txs_B []*types.Transaction
	var txs_AWithoutTokens []*types.Transaction
	var txs_BWithoutTokens []*types.Transaction
	// for each account on A
	for i := range accountsOnRollupA {
		// for each tx to be sent
		for j := 0; j < numOfTxsForMultipleAccounts; j++ {
			// build bridge txs with different nonces
			txA, txB, err := helpers.SendBridgeTxWithNonce(t, accountsOnRollupA[i], noncesA[i]+uint64(j), accountsOnRollupB[i], noncesB[i]+uint64(j), transferredAmount, TokenABI, BridgeABI)
			require.NoError(t, err)
			require.NotNil(t, txA)
			require.NotNil(t, txB)
			txs_A = append(txs_A, txA)
			txs_B = append(txs_B, txB)
			txAWithoutTokens, txBWithoutTokens, err := helpers.SendBridgeTxWithNonce(t, accountsOnRollupAWithoutTokens[i], noncesAWithoutTokens[i], accountsOnRollupBWithoutTokens[i], noncesBWithoutTokens[i], transferredAmount, TokenABI, BridgeABI)
			require.NoError(t, err)
			require.NotNil(t, txAWithoutTokens)
			require.NotNil(t, txBWithoutTokens)
			txs_AWithoutTokens = append(txs_AWithoutTokens, txAWithoutTokens)
			txs_BWithoutTokens = append(txs_BWithoutTokens, txBWithoutTokens)
			time.Sleep(delay)
		}
	}

	// wait 30s until we check the txs
	logger.Info("Waiting 30s until we check the txs...")
	time.Sleep(30 * time.Second)
	// check if all txs are successful
	for _, tx := range txs_A {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}
	for _, tx := range txs_B {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.NoError(t, err)
		require.NotNil(t, receipt)
		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "tx %s", tx.Hash().Hex())
	}

	// check if all txs with not enough funds are not processed
	for _, tx := range txs_AWithoutTokens {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupA)
		require.Nil(t, receipt)
		require.Error(t, err)
	}
	for _, tx := range txs_BWithoutTokens {
		_, receipt, err := transactions.GetTransactionDetails(ctx, tx.Hash(), TestRollupB)
		require.Nil(t, receipt)
		require.Error(t, err)
	}

	// expected balances
	for _, acc := range accountsOnRollupA {
		balance, err := acc.GetTokensBalance(ctx, tokenAddress, TokenABI)
		require.NoError(t, err)
		require.Equal(t, 0, balance.Cmp(big.NewInt(0))) // on rollup A, all tokens should be sent to rollup B
	}
	for _, acc := range accountsOnRollupB {
		balance, err := acc.GetTokensBalance(ctx, tokenAddress, TokenABI)
		require.NoError(t, err)
		expected := new(big.Int).Mul(transferredAmount, big.NewInt(numOfTxsForMultipleAccounts))
		require.Equal(t, 0, balance.Cmp(expected)) // on rollup B, all tokens sent from A should be received
	}
}