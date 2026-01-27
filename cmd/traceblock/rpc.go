package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// ---- JSON-RPC base types ----

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

// ---- Trace types for debug_traceBlockByNumber + callTracer ----

// CallFrame represents one node in the call tree returned by callTracer.
type CallFrame struct {
	Type    string      `json:"type"` // CALL, DELEGATECALL, STATICCALL, CREATE, ...
	From    string      `json:"from"`
	To      string      `json:"to"`
	Gas     string      `json:"gas"`
	GasUsed string      `json:"gasUsed"`
	Input   string      `json:"input"`
	Output  string      `json:"output"`
	Value   string      `json:"value"`
	Error   string      `json:"error"`           // present if reverted
	Calls   []CallFrame `json:"calls,omitempty"` // nested internal calls
}

// TxTrace ties a tx hash to its top-level call frame (root of the call tree).
type TxTrace struct {
	TxHash string    `json:"txHash"`
	Result CallFrame `json:"result"`
	Error  string    `json:"error"` // tracer-level error, if any
}

// ----- Logging ----
var procErrLogger *log.Logger

func initProcessErrorLogger() {
	const logDir = "logs"
	const logFile = "processRequest.error.log"

	_ = os.MkdirAll(logDir, 0o755)

	path := filepath.Join(logDir, logFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// optional: fallback to stderr or just skip logging
		return
	}

	procErrLogger = log.New(f, "", log.LstdFlags)
}

func logProcessError(err error) {
	if err == nil || procErrLogger == nil {
		return
	}
	procErrLogger.Println(err.Error())
}

// ---- Helpers for requests ----

// prepareCall builds the rpcRequest for debug_traceBlockByNumber using callTracer.
// blockHex must be a hex string with 0x prefix, e.g. "0xa0b0c0".
func prepareCall(block uint64) rpcRequest {

	blockHex := fmt.Sprintf("0x%x", block)

	params := []interface{}{
		blockHex,
		map[string]interface{}{
			"tracer":  "callTracer",
			"timeout": "60s",
		},
	}

	return rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "debug_traceBlockByNumber",
		Params:  params,
	}
}

// processRequest reads the HTTP response body from the node and unwraps it into
// a slice of TxTrace (one per transaction in the traced block).
func processRequest(body io.Reader) ([]TxTrace, error) {
	var resp rpcResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		logProcessError(fmt.Errorf("decode RPC response: %w", err))
		return nil, fmt.Errorf("decode RPC response: %w", err)
	}

	if resp.Error != nil {
		err := fmt.Errorf("RPC error: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
		logProcessError(err)
		return nil, err
	}

	var traces []TxTrace
	if err := json.Unmarshal(resp.Result, &traces); err != nil {
		logProcessError(fmt.Errorf("unmarshal traces: %w", err))
		return nil, fmt.Errorf("unmarshal traces: %w", err)
	}

	return traces, nil
}
