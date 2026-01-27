package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
)

func createCSV(fn string) (*os.File, *csv.Writer, error) {
	file, err := os.Create(fn)
	if err != nil {
		return nil, nil, err
	}

	writer := csv.NewWriter(file)

	headers := []string{"block", "txHash", "contractAddress", "bytecode", "type"}
	if err := writer.Write(headers); err != nil {
		file.Close()
		return nil, nil, err
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		file.Close()
		return nil, nil, err
	}

	return file, writer, nil
}
func checkERC777Selectors(code []byte) bool {

	erc777Selectors := [][]byte{
		{0x9d, 0x8a, 0x62, 0xf8}, // send(address,uint256,bytes)
		{0x9d, 0xc2, 0x9f, 0xac}, // burn(uint256,bytes)
		{0xce, 0xdb, 0x0e, 0xc8}, // operatorSend(address,address,uint256,bytes,bytes)
		{0x82, 0xaa, 0xac, 0x64}, // authorizeOperator(address)
		{0x6b, 0x20, 0xd4, 0x2b}, // revokeOperator(address)
		{0x18, 0xa2, 0xe6, 0xe9}, // isOperatorFor(address,address)
		{0x93, 0x30, 0xc5, 0xe8}, // defaultOperators()
	}

	for _, sel := range erc777Selectors {
		if !bytes.Contains(code, sel) {
			return false
		}
	}

	return true
}

func checkERC865Selectors(code []byte) bool {

	erc865Selectors := [][]byte{
		{0xb6, 0xb5, 0x5f, 0x25}, // transferPreSigned(address,address,uint256,uint256,uint256,bytes)
		{0x2e, 0x17, 0xde, 0x78}, // approvePreSigned(address,address,uint256,uint256,uint256,bytes)
	}

	for _, sel := range erc865Selectors {
		if !bytes.Contains(code, sel) {
			return false
		}
	}

	return true
}

func checkERC827Selectors(code []byte) bool {

	erc827Selectors := [][]byte{
		{0x2f, 0x54, 0xbf, 0x6e}, // transfer(address,uint256,bytes)
		{0x6e, 0xa7, 0xe6, 0xf3}, // transferFrom(address,address,uint256,bytes)
		{0x38, 0xba, 0xbd, 0x8b}, // approve(address,uint256,bytes)
	}

	for _, sel := range erc827Selectors {
		if !bytes.Contains(code, sel) {
			return false
		}
	}

	return true
}

func checkERC621Selectors(code []byte) bool {

	erc621Selectors := [][]byte{
		{0x83, 0x41, 0xbf, 0x7a}, // increaseSupply(uint256)
		{0x23, 0x62, 0x3a, 0x9f}, // decreaseSupply(uint256)
	}

	for _, sel := range erc621Selectors {
		if !bytes.Contains(code, sel) {
			return false
		}
	}

	return true
}

func checkERCSelectors(code []byte) (bool, string) {
	// Limit the search to the first 1024 bytes (or the whole code if it's shorter)
	dispatcherRegion := code

	if len(code) > 1024 {
		dispatcherRegion = code[:1024]
	}

	std := "erc20"

	selectors := [][]byte{
		{0x18, 0x16, 0x0d, 0xdd}, // totalSupply()
		{0x70, 0xa0, 0x82, 0x31}, // balanceOf(address)
		{0xa9, 0x05, 0x9c, 0xbb}, // transfer(address,uint256)
		{0x23, 0xb8, 0x72, 0xdd}, // transferFrom(address,address,uint256)
		{0x09, 0x5e, 0xa7, 0xb3}, // approve(address,uint256)
		{0xdd, 0x62, 0xed, 0x3e}, // allowance(address,address)
	}

	// Check that each selector is present in the dispatcher region.
	for _, sel := range selectors {
		if !bytes.Contains(dispatcherRegion, sel) {
			return false, ""
		}
	}

	if bytes.Contains(dispatcherRegion, []byte{0xf8, 0xa8, 0xfd, 0x6d}) { // transfer(address,uint256,bytes)
		std += "erc223"
	}

	if checkERC621Selectors(dispatcherRegion) {
		std += "erc621"
	}

	if bytes.Contains(dispatcherRegion, []byte{0x7a, 0xab, 0x5f, 0x8c}) { // transferAndCall(address,uint256,bytes)
		std += "erc677"
	}

	if checkERC777Selectors(dispatcherRegion) {
		std += "erc777"
	}

	if checkERC827Selectors(dispatcherRegion) {
		std += "erc827"
	}

	if checkERC865Selectors(dispatcherRegion) {
		std += "erc865"
	}

	return true, std
}

func collectData(client *ethclient.Client, start int, chunkSize int, dir string) {

	filename := fmt.Sprintf("%d.csv", start)
	filePath := filepath.Join(dir, filename)
	file, writer, err := createCSV(filePath)
	if err != nil {
		log.Fatalf("Error creating CSV file: %v", err)
	}
	defer file.Close()

	for currBlck := start; currBlck <= start+chunkSize; currBlck++ {
		currentBlock, err := client.BlockByNumber(context.Background(), big.NewInt(int64(currBlck)))
		if err != nil {
			log.Fatalf("Failed to get block: %v", err)
			continue
		}
		for _, transaction := range currentBlock.Transactions() {
			if transaction.To() == nil {
				thx := transaction.Hash()

				receipt, err := client.TransactionReceipt(context.Background(), thx)
				if err != nil {
					log.Fatal(err)
				}
				cAddr := receipt.ContractAddress.Hex()

				code, err := client.CodeAt(context.Background(), receipt.ContractAddress, nil)
				if err != nil {
					log.Fatal(err)
				}

				res, std := checkERCSelectors(code)

				if res != true {
					//fmt.Printf("we skip a non ERC20 contract %s\n", cAddr)
					continue
				}

				row := []string{fmt.Sprintf("%d", currBlck), thx.String(), cAddr, "0x" + hex.EncodeToString(code), std}
				if err := writer.Write(row); err != nil {
					log.Fatalf("Error writing row to CSV: %v", err)
				}

				writer.Flush()
				if err := writer.Error(); err != nil {
					log.Fatalf("Error flushing CSV writer: %v", err)
				}
			}

		}
	}
}

func main() {
	/* parse command line flags */
	startBlock := flag.Int("start", 912760, "Starting block number to start scraping for ERC20 creations")
	endBlock := flag.Int("end", 912761, "Ending block number for scraping")
	cores := flag.Int("cores", 8, "Maximum number of concurrent goroutines for scraping")
	chunk := flag.Int("chunk", 100000, "Number of blocks to process per goroutine")
	flag.Parse()
	maxConcurrent := *cores
	chunkSize := *chunk
	fmt.Printf("Scraping for ERC20 contracts starting at block: %d, until block: %d\n", *startBlock, *endBlock)

	/* connect to the node */
	ethNodeURL := "http://localhost:8545"
	client, err := ethclient.Dial(ethNodeURL)
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum node: %v", err)
	}
	defer client.Close()

	fmt.Printf("Connected to Ethereum node\n")

	/* set up directory for temporary csv files */
	dir := "tmp_csv_files"
	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Fatalf("failed to create directory: %v", err)
	}

	/* set up goroutines to collect data */
	var wg sync.WaitGroup

	blockNumber := *startBlock
	semaphore := make(chan struct{}, maxConcurrent)

	for current := *startBlock; current <= *endBlock; current += chunkSize {
		wg.Add(1)

		semaphore <- struct{}{}

		chunkS := current
		chunkE := current + chunkSize - 1
		if chunkE > *endBlock {
			chunkE = *endBlock
		}

		go func(cs, cSize int, client *ethclient.Client, wg *sync.WaitGroup, dir string) {
			defer wg.Done()

			collectData(client, cs, cSize, dir)

			<-semaphore
		}(chunkS, chunkE-chunkS, client, &wg, dir)
	}

	wg.Wait()
	fmt.Println("All workers have completed!")

	/* combine all csv tables to one */
	combinedCSV := "data/erc20scrape.csv"
	finalFile, err := os.Create(combinedCSV)
	if err != nil {
		log.Fatalf("Error creating combined CSV: %v", err)
	}
	defer finalFile.Close()

	writer := csv.NewWriter(finalFile)
	defer writer.Flush()

	for i := *startBlock; i <= *endBlock; i += chunkSize {
		filename := fmt.Sprintf("%d.csv", i)
		filePath := filepath.Join(dir, filename)
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatalf("Error opening file %s: %v", filename, err)
		}

		reader := csv.NewReader(file)
		records, err := reader.ReadAll()
		if err != nil {
			log.Fatalf("Error reading CSV file %s: %v", filename, err)
		}
		file.Close()

		startRow := 0
		if i > blockNumber && len(records) > 0 {
			startRow = 1
		}

		for j := startRow; j < len(records); j++ {
			if err := writer.Write(records[j]); err != nil {
				log.Fatalf("Error writing record to combined file: %v", err)
			}
		}
	}
	fmt.Printf("Combined CSV file created at %s\n", combinedCSV)
	os.RemoveAll(dir)
	//fmt.Printf("cleaned up temporary files\nERC20 scan Done!")
}
