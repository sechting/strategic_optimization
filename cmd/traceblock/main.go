package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MatchRecord struct {
	Block    uint64
	TxHash   string
	Contract string
	Type     string
	Selector string
	GasUsed  string
}

// Adjust this if your node runs on a different host/port.
const rpcURL = "http://localhost:8545"

// ---- Logging ----
var traceBlockLogger *log.Logger

func initTraceBlockErrorLogger() {
	const logDir = "logs"
	const logFile = "traceBlock.error.log"

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		// can't log this without spamming stderr; silently give up
		return
	}

	path := filepath.Join(logDir, logFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}

	// We intentionally do NOT close f here: logger keeps it open
	traceBlockLogger = log.New(f, "", log.LstdFlags)
}

func logTraceBlockError(err error) {
	if err == nil || traceBlockLogger == nil {
		return
	}
	traceBlockLogger.Println(err.Error())
}

// ---- Main ----
func main() {
	startB := flag.Int("start", 912760, "Starting block number to start scraping for ERC20 creations")
	endB := flag.Int("end", 912761, "Ending block number for scraping")
	cores := flag.Int("cores", 8, "Maximum number of concurrent goroutines for scraping")
	flag.Parse()

	/* initialize the logger */
	initProcessErrorLogger()
	initTraceBlockErrorLogger()

	/*  configuration */
	const addrFile = "data/scrape_addresses.json"
	const csvPath = "data/erc20calls.csv"
	numWorkers := *cores

	// both start and end are included in crawl
	startBlock := uint64(*startB)
	endBlock := uint64(*endB)
	totalBlocks := (endBlock - startBlock) + 1
	width := len(strconv.FormatUint(totalBlocks, 10))

	fmt.Printf("tracing blocks from %d to %d (%d blocks) using %d workers\n", startBlock, endBlock, totalBlocks, numWorkers)

	targetContracts, err := loadTargetContracts(addrFile)
	if err != nil {
		log.Fatalf("failed to load target contracts, stopped!: %v\n", err)
		os.Exit(1)
	}

	// normalize addresses
	targets := make(map[string]struct{}, len(targetContracts))
	for _, addr := range targetContracts {
		targets[strings.ToLower(addr)] = struct{}{}
	}

	// Set up channels & worker pool
	jobs := make(chan uint64, 100)
	results := make(chan []MatchRecord, 100)
	progress := make(chan uint64, 100)

	var wg sync.WaitGroup

	// monitor progress because this takes potentially a long time to finish and
	// seeing something helps to not panic
	go func() {
		completed := uint64(0)
		for range progress {
			completed++
			pct := float64(completed) * 100.0 / float64(totalBlocks)
			fmt.Printf("\rprogress: %*d/%d (%.2f%%)", width, completed, totalBlocks, pct)
		}
		if completed > 0 {
			pct := float64(completed) * 100.0 / float64(totalBlocks)
			fmt.Printf("\rprogress: %*d/%d (%.2f%%)\n", width, completed, totalBlocks, pct)
		} else {
			fmt.Println()
		}
	}()

	// Start CSV writer goroutine
	writerDone := make(chan struct{})
	go csvWriter(results, csvPath, writerDone)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go blockWorker(i, jobs, results, progress, targets, &wg)
	}

	// Feed jobs
	for b := startBlock; b <= endBlock; b++ {
		jobs <- b
	}
	close(jobs)
	wg.Wait()
	close(results)
	close(progress)
	<-writerDone
}

// ---- Parallelization ----
func blockWorker(id int, jobs <-chan uint64, results chan<- []MatchRecord, progress chan<- uint64, targets map[string]struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	for block := range jobs {
		matches, err := traceBlock(block, targets)

		// declare work done to main
		progress <- block

		// now check if we need to send back matches
		if err != nil || len(matches) == 0 {
			// errors are already logged, so we can move on if something went wrong
			continue
		}
		results <- matches
	}
}

// ---- All the actual logic ----
func loadTargetContracts(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read addresses file: %w", err)
	}

	var addrs []string
	if err := json.Unmarshal(data, &addrs); err != nil {
		return nil, fmt.Errorf("unmarshal addresses JSON: %w", err)
	}

	return addrs, nil
}

// traceBlock sends one debug_traceBlockByNumber request for the given block
// and filters out all function calls to desired contracts. results will be saved in csv file
// format:
// block, txhash, contract, type, selector, gasUsed
func traceBlock(block uint64, targets map[string]struct{}) ([]MatchRecord, error) {
	// Build the JSON-RPC request struct using our helper.
	reqObj := prepareCall(block)

	// Marshal to JSON.
	bodyBytes, err := json.Marshal(reqObj)
	if err != nil {
		wrapped := fmt.Errorf("block %d: marshal request: %w", block, err)
		logTraceBlockError(wrapped)
		return nil, wrapped
	}

	// HTTP request with context/timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(bodyBytes))
	if err != nil {
		wrapped := fmt.Errorf("block %d: create HTTP request: %w", block, err)
		logTraceBlockError(wrapped)
		return nil, wrapped
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		wrapped := fmt.Errorf("block %d: HTTP request failed: %w", block, err)
		logTraceBlockError(wrapped)
		return nil, wrapped
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		wrapped := fmt.Errorf("block %d: RPC HTTP status not OK: %s", block, resp.Status)
		logTraceBlockError(wrapped)
		return nil, wrapped
	}

	// Use our helper to decode into []TxTrace.
	traces, err := processRequest(resp.Body)
	if err != nil {
		wrapped := fmt.Errorf("block %d: processRequest: %w", block, err)
		logTraceBlockError(wrapped)
		return nil, wrapped
	}

	// Collect matches for this block
	var matches []MatchRecord

	for _, tx := range traces {
		if tx.Error != "" {
			// tracer-level error for this tx -> log and continue
			wrapped := fmt.Errorf("block %d tx %s: tracer error: %s", block, tx.TxHash, tx.Error)
			logTraceBlockError(wrapped)
			continue
		}

		walkCallFrames(true, tx.Result, block, tx.TxHash, targets, func(b uint64, txh string, f CallFrame) { handleMatch(&matches, b, txh, f) })
	}

	return matches, nil
}

// walkCallFrames walks the call tree starting at root, and calls handler
// for every frame whose `To` is in targets (map of normalized addresses).
func walkCallFrames(
	isRoot bool,
	root CallFrame,
	blockNumber uint64,
	txHash string,
	targets map[string]struct{},
	handler func(block uint64, txHash string, f CallFrame),
) {
	if isRoot {
		root.Type = "TXCALL"
	}
	toNorm := strings.ToLower(root.To)
	if _, ok := targets[toNorm]; ok {
		handler(blockNumber, txHash, root)
	}

	for _, child := range root.Calls {
		walkCallFrames(false, child, blockNumber, txHash, targets, handler)
	}
}

// handleMatch appends one MatchRecord into the slice for every matched frame.
func handleMatch(matches *[]MatchRecord, block uint64, txHash string, f CallFrame) {
	// skip CREATE/CREATE2 frames
	switch strings.ToUpper(f.Type) {
	case "CREATE", "CREATE2":
		return
	}
	selector := ""
	if len(f.Input) >= 10 {
		selector = f.Input[:10]
	}

	rec := MatchRecord{
		Block:    block,
		TxHash:   txHash,
		Contract: f.To,
		Type:     f.Type,    // high-level type: CALL / CREATE / ...
		Selector: selector,  // 4-byte function selector
		GasUsed:  f.GasUsed, // still hex, you can later convert to decimal if you want
	}

	if (rec.Selector == "0x") || (rec.Selector == "") {
		return
	}

	*matches = append(*matches, rec)
}

func appendMatchesToCSV(path string, records []MatchRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Check if file exists to decide whether to write header.
	_, err := os.Stat(path)
	fileExists := err == nil

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open csv file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)

	// Write header if file is new
	if !fileExists {
		header := []string{"block", "txhash", "contract", "type", "selector", "gasUsed"}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("write csv header: %w", err)
		}
	}

	for _, r := range records {
		row := []string{
			strconv.FormatUint(r.Block, 10),
			r.TxHash,
			r.Contract,
			r.Type,
			r.Selector,
			r.GasUsed,
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}

	return nil
}

func csvWriter(results <-chan []MatchRecord, path string, done chan<- struct{}) {
	defer close(done)

	for batch := range results {
		if len(batch) == 0 {
			continue
		}
		if err := appendMatchesToCSV(path, batch); err != nil {
			// log, but don't crash the whole thing
			wrapped := fmt.Errorf("appendMatchesToCSV: %w", err)
			logTraceBlockError(wrapped)
		}
	}
}
