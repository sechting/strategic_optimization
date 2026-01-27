package main

import (
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type SeqInfo struct {
	Offset    int      `json:"offset"`
	Contracts []string `json:"contracts"`
}

// SaveMap saves the normalized init sequences to a file
func SaveMap(Map map[string][]string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // pretty print

	return encoder.Encode(Map)
}

func SaveSeqInfoMap(m map[string]SeqInfo, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // pretty print

	return encoder.Encode(m)
}

func LoadMap(filename string) (map[string][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var m map[string][]string
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}

	return m, nil
}

func hashName(name string) string {
	hash := sha1.Sum([]byte(name))
	return hex.EncodeToString(hash[:])
}

func SaveInstructions(insts []Instruction, dir string, name string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, name+".json")

	// Skip if file already exists
	if _, err := os.Stat(filePath); err == nil {
		// File exists — skip
		return nil
	} else if !os.IsNotExist(err) {
		// Unexpected error accessing file
		return fmt.Errorf("error checking if file exists: %v", err)
	}

	data, err := json.MarshalIndent(insts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal instructions: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %s: %v", name, err)
	}

	return nil
}

func main() {
	csvPath := flag.String("file", "data/erc20scrape.csv", "path to the csv file")
	flag.Parse()

	if *csvPath == "" {
		log.Fatalf("use -file path/to/csv_file\n")
	}

	f, err := os.Open(*csvPath)
	if err != nil {
		log.Fatalf("Error opening file: %v\n", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	if _, err := reader.Read(); err != nil {
		log.Fatalf("Error reading CSV headers: %v\n", err)
	}

	initMap := make(map[string][]string)
	dispatcherMap := make(map[string][]string)
	dispatcherMapArgs := make(map[string]SeqInfo)
	initDispatcherMap := make(map[string][]string)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error reading CSV record: %v\n", err)
		}

		address := record[2]
		bytecode := record[3]

		seq, err := DisassembleBytecode(bytecode)
		if err != nil {
			//log.Printf("Skipping contract %s due to error: %v\n", address, err)
			continue // skip this broken record and continue with the next
		}

		// Extract the initialization sequence
		init, pc, err := ExtractInit(seq)
		if err != nil {
			//log.Printf("Skipping contract %s due to error: %v\n", address, err)
			continue // skip this broken record and continue with the next
		}

		// Extract the dispatcher sequence
		dispatcher, _, err := ExtractDispatcher(seq, pc)
		if err != nil {
			//log.Printf("Skipping contract %s due to error: %v\n", address, err)
			continue // skip this broken record and continue with the next
		}

		// save the init and dispatcher sequences
		// if we found a ridiculous long init sequence, we skip it (we did something wrong or it's really exotic)
		if len(init) > 50 {
			//log.Printf("Skipping contract %s due to long init sequence: %d\n", address, len(init))
			continue
		}

		normalizedI := NormalizeInstructions(init, false)
		normalizedIA := NormalizeInstructions(init, true)
		initMap[normalizedI] = append(initMap[normalizedI], address)
		err = SaveInstructions(init, "data/init", hashName(normalizedI))

		if err != nil {
			log.Printf("Failed to save init instructions for contract %s: %v\n", address, err)
		}

		normalizedD := NormalizeInstructions(dispatcher, false)
		dispatcherMap[normalizedD] = append(dispatcherMap[normalizedD], address)
		err = SaveInstructions(dispatcher, "data/dispatcher", hashName(normalizedD))
		normalizedDA := NormalizeInstructions(dispatcher, true)
		// we have a more unique setup for the dispatcher with args - we want to know the offset
		info, ok := dispatcherMapArgs[normalizedDA]
		if !ok {
			info = SeqInfo{Offset: GetInstrPC(seq, pc), Contracts: []string{}}
		}
		info.Contracts = append(info.Contracts, address)
		dispatcherMapArgs[normalizedDA] = info
		// for the actual analysis we need to combine init+dispatcher
		initDispatcherMap[normalizedIA+normalizedDA] = append(initDispatcherMap[normalizedIA+normalizedDA], address)

		normalizedB := NormalizeInstructions(seq, false)
		err = SaveInstructions(seq, "data/contracts", hashName(normalizedB))

		if err != nil {
			log.Printf("Failed to save dispatcher instructions for contract %s: %v\n", address, err)
		}
	}
	//fmt.Printf("Initmap: %s\n", initMap)

	err = SaveMap(initMap, "data/init_sequences.json")
	if err != nil {
		log.Fatalf("Failed to save init map: %v", err)
	}
	err = SaveMap(dispatcherMap, "data/dispatcher_sequences.json")
	if err != nil {
		log.Fatalf("Failed to save dispatcher map: %v", err)
	}
	err = SaveSeqInfoMap(dispatcherMapArgs, "data/dispatcher_sequences_args.json")
	if err != nil {
		log.Fatalf("Failed to save dispatcher with args map: %v", err)
	}
	err = SaveMap(initDispatcherMap, "data/init_dispatcher_sequences.json")
	if err != nil {
		log.Fatalf("Failed to save init+dispatcher map: %v", err)
	}
	fmt.Println("Successfully saved init and dispatcher sequences to json files.")

}
