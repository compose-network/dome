package transactions

import (
	"context"
	"fmt"

	"github.com/compose-network/dome/internal/accounts"
	"github.com/compose-network/dome/internal/logger"
	composeproto "github.com/compose-network/specs/compose/proto"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/protobuf/proto"
)

const sendTxRPCMethod = "eth_sendXTransaction"

func CreateCrossTxRequestMsg(ctx context.Context, ac1 *accounts.Account, ac2 *accounts.Account, signedTx1 []byte, signedTx2 []byte) ([]byte, error) {
	xtRequest := &composeproto.XTRequest{
		TransactionRequests: []*composeproto.TransactionRequest{
			{
				ChainId:     ac1.GetRollup().ChainID().Uint64(),
				Transaction: [][]byte{signedTx1},
			},
			{
				ChainId:     ac2.GetRollup().ChainID().Uint64(),
				Transaction: [][]byte{signedTx2},
			},
		},
	}

	spMsg := &composeproto.Message{
		SenderId: "client",
		Payload: &composeproto.Message_XtRequest{
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
