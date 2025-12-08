package smartaccount

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Input for the paymaster call.
type PaymasterRequest struct {
	Sender               common.Address
	Nonce                *big.Int
	InitCode             []byte
	CallData             []byte
	CallGasLimit         *big.Int
	VerificationGasLimit *big.Int
	PreVerificationGas   *big.Int
	MaxFeePerGas         *big.Int
	MaxPriorityFeePerGas *big.Int
	EntryPoint           common.Address // second param in curl
	ChainID              *big.Int       // third param in curl (e.g. 77777 => 0x12fd1)
}

type userOpParam struct {
	Sender               string `json:"sender"`
	Nonce                string `json:"nonce"`
	InitCode             string `json:"initCode"`
	CallData             string `json:"callData"`
	CallGasLimit         string `json:"callGasLimit"`
	VerificationGasLimit string `json:"verificationGasLimit"`
	PreVerificationGas   string `json:"preVerificationGas"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
}

type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type paymasterResult struct {
	PreVerificationGas            string `json:"preVerificationGas"`
	CallGasLimit                  string `json:"callGasLimit"`
	PaymasterVerificationGasLimit string `json:"paymasterVerificationGasLimit"`
	PaymasterPostOpGasLimit       string `json:"paymasterPostOpGasLimit"`
	VerificationGasLimit          string `json:"verificationGasLimit"`
	Paymaster                     string `json:"paymaster"`
	PaymasterData                 string `json:"paymasterData"`
}

type jsonRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int              `json:"id"`
	Result  *paymasterResult `json:"result,omitempty"`
	Error   *struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

// GetPaymasterAndData calls the paymaster RPC and returns the PaymasterAndData bytes
// = address(paymaster) || paymasterData.
func GetPaymasterAndData(ctx context.Context, endpoint string, req PaymasterRequest) ([]byte, *paymasterResult, error) {
	// Build userOp param
	u := userOpParam{
		Sender:               req.Sender.Hex(),
		Nonce:                bigToHex(req.Nonce),
		InitCode:             bytesToHex(req.InitCode),
		CallData:             bytesToHex(req.CallData),
		CallGasLimit:         bigToHex(req.CallGasLimit),
		VerificationGasLimit: bigToHex(req.VerificationGasLimit),
		PreVerificationGas:   bigToHex(req.PreVerificationGas),
		MaxFeePerGas:         bigToHex(req.MaxFeePerGas),
		MaxPriorityFeePerGas: bigToHex(req.MaxPriorityFeePerGas),
	}

	// Params: [ userOp, entryPoint, chainId ]
	params := []interface{}{
		u,
		req.EntryPoint.Hex(),
		bigToHex(req.ChainID),
	}

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pm_getPaymasterData",
		Params:  params,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal json-rpc request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("call paymaster: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read paymaster response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("non-200 status from paymaster: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal paymaster response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, nil, fmt.Errorf("paymaster error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if rpcResp.Result == nil {
		return nil, nil, fmt.Errorf("paymaster response missing result")
	}

	// Build PaymasterAndData = abi.encodePacked(paymaster, paymasterData)
	pmAddr := common.HexToAddress(rpcResp.Result.Paymaster) // 20 bytes
	pmDataBytes, err := hexStringToBytes(rpcResp.Result.PaymasterData)
	if err != nil {
		return nil, nil, fmt.Errorf("decode paymasterData: %w", err)
	}

	paymasterAndData := append(pmAddr.Bytes(), pmDataBytes...)

	return paymasterAndData, rpcResp.Result, nil
}

// --- helpers ---

func bigToHex(b *big.Int) string {
	if b == nil || b.Sign() == 0 {
		return "0x0"
	}
	return "0x" + b.Text(16)
}

func bytesToHex(b []byte) string {
	if len(b) == 0 {
		return "0x"
	}
	return "0x" + hex.EncodeToString(b)
}

func hexStringToBytes(s string) ([]byte, error) {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	if s == "" {
		return []byte{}, nil
	}
	return hex.DecodeString(s)
}
