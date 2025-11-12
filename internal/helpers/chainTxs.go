package helpers

import (
	"context"
	"fmt"
	"github.com/compose-network/dome/internal/accounts"
	"github.com/compose-network/dome/internal/logger"
	"github.com/compose-network/dome/internal/transactions"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

func SendSelfMoveBalanceTx(ctx context.Context, ac *accounts.Account, amount *big.Int) (*types.Transaction, common.Hash, error) {
	txDetails := transactions.TransactionDetails{
		To:        ac.GetAddress(),
		Value:     amount,
		Gas:       25000,
		GasTipCap: big.NewInt(1000000),
		GasFeeCap: big.NewInt(2000000),
		Data:      nil,
	}

	tx, _, err := transactions.CreateTransaction(ctx, txDetails, ac)
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to create transaction: %w", err)
	}
	hash, err := transactions.SendTransaction(ctx, tx, ac.GetRollup().RPCURL())
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}
	logger.Info("Self move balance transaction sent successfully: %s", hash)
	return tx, hash, nil
}

func SendSelfMoveBalanceTxWithNonce(ctx context.Context, ac *accounts.Account, nonce uint64, amount *big.Int) (*types.Transaction, common.Hash, error) {
	txDetails := transactions.TransactionDetails{
		To:        ac.GetAddress(),
		Value:     amount,
		Gas:       25000,
		GasTipCap: big.NewInt(1000000),
		GasFeeCap: big.NewInt(2000000),
		Data:      nil,
	}

	tx, _, err := transactions.CreateTransactionWithNonce(ctx, txDetails, ac, nonce)
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to create transaction: %w", err)
	}
	hash, err := transactions.SendTransaction(ctx, tx, ac.GetRollup().RPCURL())
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}
	logger.Info("Self move balance transaction sent successfully: %s", hash)
	return tx, hash, nil
}
