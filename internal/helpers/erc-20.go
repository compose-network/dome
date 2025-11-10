package helpers

import (
	"context"
	"math/big"
	"testing"

	"github.com/compose-network/rollup-probe/internal/logger"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/compose-network/rollup-probe/configs"
	"github.com/compose-network/rollup-probe/internal/accounts"
	"github.com/compose-network/rollup-probe/internal/transactions"
)

/*
MintTokens mints tokens to the given account
*/
func SendMintTx(t *testing.T, ac *accounts.Account, amount *big.Int, tokenABI abi.ABI) (*types.Transaction, common.Hash, error) {
	tokenAddress := configs.Values.L2.Contracts[configs.ContractNameToken].Address

	calldata, err := tokenABI.Pack("mint",
		ac.GetAddress(),
		amount,
	)
	require.NoError(t, err)
	require.NotNil(t, calldata)

	transactionDetails := transactions.TransactionDetails{
		To:        tokenAddress,
		Value:     big.NewInt(0),
		Gas:       900000,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(20000000000),
		Data:      calldata,
	}

	tx, signedTransaction, err := transactions.CreateTransaction(context.Background(), transactionDetails, ac)
	require.NoError(t, err)
	require.NotNil(t, signedTransaction)
	hash, err := transactions.SendTransaction(context.Background(), tx, ac.GetRollup().RPCURL())
	logger.Info("Mint transaction sent successfully: %s", hash)
	require.NoError(t, err)
	return tx, hash, err
}
