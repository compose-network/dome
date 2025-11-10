package transactions

import (
	"context"
	"fmt"

	"github.com/compose-network/rollup-probe/internal/accounts"
	"github.com/compose-network/rollup-probe/internal/logger"
	"github.com/compose-network/rollup-probe/pkg/rollupv1"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/protobuf/proto"
)

const sendTxRPCMethod = "eth_sendXTransaction"

func CreateCrossTxRequestMsg(ctx context.Context, ac1 *accounts.Account, ac2 *accounts.Account, signedTx1 []byte, signedTx2 []byte) ([]byte, error) {
	xtRequest := &rollupv1.XTRequest{
		Transactions: []*rollupv1.TransactionRequest{
			{
				ChainId: ac1.GetRollup().ChainID().Bytes(),
				Transaction: [][]byte{
					signedTx1,
				},
			},
			{
				ChainId: ac2.GetRollup().ChainID().Bytes(),
				Transaction: [][]byte{
					signedTx2,
				},
			},
		},
	}

	spMsg := &rollupv1.Message{
		SenderId: "client",
		Payload: &rollupv1.Message_XtRequest{
			XtRequest: xtRequest,
		},
	}
	logger.Debug("Cross tx request msg created successfully: %v", spMsg)
	encodedPayload, err := proto.Marshal(spMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal XTRequest: %v", err)
	}
	logger.Debug("Cross tx request msg encoded successfully: %x", encodedPayload)
	return encodedPayload, nil
}

func SendCrossTxRequestMsg(ctx context.Context, rpcURL string, encodedPayload []byte) error {
	l1Client, err := rpc.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("could not connect to custom rpc: %v", err)
	}
	defer l1Client.Close()

	err = l1Client.CallContext(ctx, nil, sendTxRPCMethod, hexutil.Encode(encodedPayload))
	if err != nil {
		return fmt.Errorf("RPC call failed: %v", err)
	}

	logger.Info("Cross tx request msg sent successfully: %x", encodedPayload)
	return nil
}
