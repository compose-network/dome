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
	numOfTxs      = 5
	numOfAccounts = 25
	delay         = 100 * time.Millisecond
)

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

	// TODO: await for receipts
	logger.Info("Await 12 seconds for minting transaction to be included into block...")
	time.Sleep(12 * time.Second)

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

	for i := range numOfTxs {
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

func TestStressBridgeDifferentAccounts(t *testing.T) {
	ctx := t.Context()

	mintedAndTransferredAmount := big.NewInt(1000000000000000000) // 1 token
	//spam x nr of accounts on both rollups
	accountsOnRollupA := make([]*accounts.Account, numOfAccounts)
	accountsOnRollupB := make([]*accounts.Account, numOfAccounts)
	for i := range numOfAccounts {
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
	// mint 9 tokens to all accounts
	logger.Info("Minting 9 tokens to all accounts...")
	for _, acc := range accountsOnRollupA {
		tx, hash, err := helpers.SendMintTx(t, acc, mintedAndTransferredAmount, TokenABI)
		require.NoError(t, err)
		require.NotNil(t, tx)
		require.NotNil(t, hash)
		time.Sleep(delay)
	}
	// wait 10 sec until we send bridge txs
	logger.Info("Waiting 10 sec until we send bridge txs...")
	time.Sleep(10 * time.Second)

	var txs_A []*types.Transaction
	var txs_B []*types.Transaction

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
}
