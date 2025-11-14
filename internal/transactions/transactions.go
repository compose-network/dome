package transactions

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/compose-network/dome/internal/accounts"
	"github.com/compose-network/dome/internal/logger"
	"github.com/compose-network/dome/internal/rollup"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type TransactionDetails struct {
	To        common.Address
	Value     *big.Int
	Data      []byte
	GasTipCap *big.Int
	GasFeeCap *big.Int
	Gas       uint64
}

func CreateTransaction(ctx context.Context, tx TransactionDetails, ac *accounts.Account) (*types.Transaction, []byte, error) {
	nonce, err := ac.GetNonce(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get nonce: %w", err)
	}
	logger.Info("Creating transaction on %s with nonce: %d", ac.GetRollup().Name(), nonce)

	privateKey := ac.GetPrivateKey()
	if privateKey == nil {
		return nil, nil, fmt.Errorf("private key is nil")
	}
	logger.Info("Private key loaded successfully on %s for account: %s", ac.GetRollup().Name(), ac.GetAddress())

	txData := &types.DynamicFeeTx{
		ChainID:    ac.GetRollup().ChainID(),
		Nonce:      nonce,
		To:         &tx.To,
		Value:      tx.Value,
		Gas:        tx.Gas,
		GasTipCap:  tx.GasTipCap,
		GasFeeCap:  tx.GasFeeCap,
		AccessList: nil,
		Data:       tx.Data,
	}

	transaction := types.NewTx(txData)
	signedTransaction, err := types.SignTx(transaction, types.NewLondonSigner(ac.GetRollup().ChainID()), privateKey)
	if err != nil {
		logger.Error("failed to sign transaction: %w", err)
		return nil, nil, fmt.Errorf("failed to sign transaction: %w", err)
	}
	logger.Info("Transaction signed successfully: %s", signedTransaction.Hash())

	marshaledTx, err := signedTransaction.MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}
	return signedTransaction, marshaledTx, nil
}

func CreateTransactionWithNonce(ctx context.Context, tx TransactionDetails, ac *accounts.Account, nonce uint64) (*types.Transaction, []byte, error) {
	logger.Info("Creating transaction with nonce: %d", nonce)

	privateKey := ac.GetPrivateKey()
	if privateKey == nil {
		return nil, nil, fmt.Errorf("private key is nil")
	}
	logger.Info("Private key loaded successfully on %s for account: %s", ac.GetRollup().Name(), ac.GetAddress())

	txData := &types.DynamicFeeTx{
		ChainID:    ac.GetRollup().ChainID(),
		Nonce:      nonce, // use the nonce provided
		To:         &tx.To,
		Value:      tx.Value,
		Gas:        tx.Gas,
		GasTipCap:  tx.GasTipCap,
		GasFeeCap:  tx.GasFeeCap,
		AccessList: nil,
		Data:       tx.Data,
	}

	transaction := types.NewTx(txData)
	signedTransaction, err := types.SignTx(transaction, types.NewLondonSigner(ac.GetRollup().ChainID()), privateKey)
	if err != nil {
		logger.Error("failed to sign transaction: %w", err)
		return nil, nil, fmt.Errorf("failed to sign transaction: %w", err)
	}
	logger.Info("Transaction signed successfully: %s", signedTransaction.Hash())

	marshaledTx, err := signedTransaction.MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}
	return signedTransaction, marshaledTx, nil
}

func SendTransaction(ctx context.Context, tx *types.Transaction, rpcURL string) (common.Hash, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to connect to RPC URL %s: %w", rpcURL, err)
	}
	defer client.Close()

	err = client.SendTransaction(ctx, tx)
	if err != nil {
		logger.Error("failed to send transaction: %v", err)
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}
	logger.Info("Transaction sent successfully: %s", tx.Hash())
	return tx.Hash(), nil
}

// generateRandomSessionID returns a random big.Int in the range [0, 2^63-1]
func GenerateRandomSessionID() *big.Int {
	max := new(big.Int).Lsh(big.NewInt(1), 63)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		logger.Fatal("failed to generate random session ID: %v", err)
	}
	return n
}

// GetTransactionDetails retrieves transaction details from the blockchain using the transaction hash and RPC URL
// It will wait and retry every 600 milliseconds if the transaction is pending until it's confirmed or fails
func GetTransactionDetails(ctx context.Context, txHash common.Hash, rollup *rollup.Rollup) (*types.Transaction, *types.Receipt, error) {
	// Create Ethereum client
	client, err := ethclient.DialContext(ctx, rollup.RPCURL())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to RPC URL %s: %w", rollup.RPCURL(), err)
	}
	defer client.Close()

	logger.Info("Fetching transaction details on %s for hash: %s", rollup.Name(), txHash.Hex())

	// Start timer before polling for transaction status
	startTime := time.Now()

	// Retry counter for "not found" errors
	maxRetries := 10
	retryCount := 0
	retryInterval := 600 * time.Millisecond

	// Poll for transaction status every 500 milliseconds until confirmed or failed
	for {
		// Get transaction by hash
		tx, isPending, err := client.TransactionByHash(ctx, txHash)
		if err != nil {
			// if transaction did not reach the RPC yet, we retry every 500 milliseconds until it does, max 10 retries
			if errors.Is(err, ethereum.NotFound) {
				retryCount++
				if retryCount > maxRetries {
					return nil, nil, fmt.Errorf("transaction receipt not found after %d retries for hash %s", maxRetries, txHash.Hex())
				}
				logger.Debug("Transaction %s did not reach the RPC yet, waiting %s before retry... (retry %d/%d)", txHash.Hex(), retryInterval, retryCount, maxRetries)
				select {
				case <-ctx.Done():
					return nil, nil, fmt.Errorf("context cancelled while waiting for transaction %s", txHash.Hex())
				case <-time.After(retryInterval):
					continue // Retry
				}
			}
			return nil, nil, fmt.Errorf("failed to get transaction by hash %s: %w", txHash.Hex(), err)
		}

		if isPending {
			logger.Debug("Transaction %s is still pending, waiting %s before retry...", txHash.Hex(), retryInterval)

			// Wait 500 ms before retrying
			select {
			case <-ctx.Done():
				return nil, nil, fmt.Errorf("context cancelled while waiting for transaction %s", txHash.Hex())
			case <-time.After(retryInterval):
				continue // Retry
			}
		}

		// Transaction is no longer pending, get the receipt
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get transaction receipt for hash %s: %w", txHash.Hex(), err)
		}

		duration := time.Since(startTime)
		logger.Info("Successfully retrieved transaction details on %s for hash: %s)", rollup.Name(), txHash.Hex())
		logger.Info("Transaction took %s to be processed", duration)
		return tx, receipt, nil
	}
}

/*
DistributeEth distributes ETH to the given recipients. Used for distributing ETH from one account to multiple accounts.
*/
func DistributeEth(ctx context.Context, sponsor *accounts.Account, recipients []*accounts.Account, amount *big.Int) error {
	nonce, err := sponsor.GetNonce(ctx)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	for _, recipient := range recipients {

		transactionBDetails := TransactionDetails{
			To:        recipient.GetAddress(),
			Value:     amount,
			Gas:       25000,
			GasTipCap: big.NewInt(1000000),
			GasFeeCap: big.NewInt(2000000),
			Data:      nil,
		}

		tx, _, err := CreateTransactionWithNonce(ctx, transactionBDetails, sponsor, nonce)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}
		_, err = SendTransaction(ctx, tx, sponsor.GetRollup().RPCURL())
		if err != nil {
			return fmt.Errorf("failed to send transaction: %w", err)
		}

		// check if transaction is successful
		_, receipt, err := GetTransactionDetails(ctx, tx.Hash(), sponsor.GetRollup())
		if err != nil {
			return fmt.Errorf("failed to get transaction receipt: %w", err)
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			return fmt.Errorf("transaction failed: %s", tx.Hash().Hex())
		}
		// increment nonce for next transaction
		nonce++

	}
	return nil
}
