package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sechting/strategic_optimization/internal/cpu"
	"github.com/sechting/strategic_optimization/internal/io"
	"github.com/sechting/strategic_optimization/internal/seq"
)

func saveContractAddresses(sequences map[string][]string) {
	file, err := os.Create("data/scrape_addresses.json")
	if err != nil {
		log.Fatalf("create file: %v", err)
	}
	defer file.Close()
	addresses := make([]string, 0)
	for _, contracts := range sequences {
		for _, addr := range contracts {
			addresses = append(addresses, addr)
		}
	}
	data, err := json.MarshalIndent(addresses, "", "  ")
	if err != nil {
		log.Fatalf("marshal JSON: %v", err)
	}
	if _, err := file.Write(data); err != nil {
		log.Fatalf("write file: %v", err)
	}
}

func loadPocContract() (string, error) {
	path := filepath.FromSlash("contract_generation/build/final/runtime.bin")
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read file: %v", err)
		return "", err
	}
	return string(data), nil
}

func loadHighContract() (string, error) {
	path := filepath.FromSlash("contract_generation/build/high/runtime.bin")
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read file: %v", err)
		return "", err
	}
	return string(data), nil
}

func main() {

	// initialization
	path := filepath.FromSlash("data/init_dispatcher_sequences.json")
	selectors := []string{"a9059cbb", "70a08231"}

	// load sequences and filter them
	sequences := seq.ReadSequences(path)
	finalList := seq.FilterSequence(sequences)

	// get some stats how many contracts we still work with
	contractCount := 0
	for _, contracts := range sequences {
		contractCount += len(contracts)
	}
	fmt.Printf("total contracts: %d\n", contractCount)
	contractCount = 0
	for _, contracts := range finalList {
		contractCount += len(contracts)
	}
	fmt.Printf("total contracts after filtering: %d\n", contractCount)

	// save a list of all contract addresses we analyze right now so we can scrape their total usage
	// for later analysis of energy wastage and carbon footprint.
	saveContractAddresses(finalList)
	/*
		// for debugging purposes - calculate energy consumption for a single sequence that is problematic
		test := "73356e5e8637531d2f32b561dd44c309f2e7157c6630146080604052600436106103825760003560e01c806387788782116101dd578063c3535b521161010e578063d905777e116100ac578063ec0c7e2811610086578063ec0c7e2814610842578063ed27f7c914610855578063ef8b30f71461085d578063fc7b9c181461087057600080fd5b8063d905777e146107fc578063dd62ed3e1461080f578063df69b22a1461082257600080fd5b8063cdffacc6116100e8578063cdffacc614610796578063ce96cb77146107a9578063d4a22bde146107bc578063d505accf146107dc57600080fd5b8063c3535b5214610768578063c63d75b614610770578063c6e6f5921461078357600080fd5b8063a9059cbb1161017b578063adfca15e11610155578063adfca15e146106f5578063b3d7f6b914610715578063b460af9414610728578063ba0876521461074857600080fd5b8063a9059cbb146106ad578063aa290e6d146106cd578063aced1661146106ed57600080fd5b806395d89b41116101b757806395d89b411461067557806399530b061461067d5780639aa7df9414610685578063a457c2d71461068d57600080fd5b8063877887821461063257806388a8d6021461064d57806394bf804d1461065557600080fd5b80633644e515116102b75780635e04a4d61161025557806370a082311161022f57806370a08231146105d7578063748747e6146105ea5780637a0ed6271461060a5780637ecebe001461061f57600080fd5b80635e04a4d61461055f5780636a5f1aa2146105975780636e553f65146105b757600080fd5b8063440368a311610291578063440368a31461051a5780634cdad5061461052f5780635141eebb1461054257806352ef6b2c1461054a57600080fd5b80633644e515146104df57806339509351146104e7578063402d267d1461050757600080fd5b80631d3b7227116103245780632606a10b116102fe5780632606a10b146104735780632d6326921461049d5780632ecfe315146104a5578063313ce567146104c557600080fd5b80631d3b72271461041d57806323b872dd14610432578063258294101461045257600080fd5b80630952864e116103605780630952864e146103ca578063095ea7b3146103d25780630a28a4771461040257806318160ddd1461041557600080fd5b806301e1d1141461038757806306fdde03146103a257806307a2d13a146103b757"
		gas, cpuwin, cpuubu := core.CalculateCpuAndGasForCall(test, "a9059cbb")
		fmt.Printf("test sequence consumed: %d gas, %d cpu (Win), %d cpu (Ubu)\n", gas, cpuwin, cpuubu)

		os.Exit(0)


	*/
	//load poc contract and do the same function query and save the difference of gas and cpu time as well.
	poc, err := loadPocContract()
	if err != nil {
		log.Fatalf("couldn't load POC contract: %v", err)
	}

	highruns, err := loadHighContract()
	if err != nil {
		log.Fatalf("couldn't load High contract: %v", err)
	}
	// calculate energy consumption until jump to passed function for all sequences.
	//pocGas, pocCpuWin, pocCpuUbu := cpu.CalculateCpuAndGasForCall(pocToken, "a9059cbb")
	//fmt.Printf("POC token contract consumes: %05d gas, %06d cpu (Win), %06d cpu (Ubu)\n", pocGas, pocCpuWin, pocCpuUbu)
	for _, selector := range selectors {
		fmt.Printf("selector: %s\n", selector)
		csvFile := fmt.Sprintf("data/call_costs_%s.csv", selector)
		diffCSV := fmt.Sprintf("data/call_costs_diff_%s.csv", selector)
		functionCosts := make(map[string][]string)
		pocFunctionCostsGas := uint64(0)
		pocFunctionCostsCpuWin := uint64(0)
		pocFunctionCostsCpuUbu := uint64(0)

		// compare it to pocContract if possible
		if poc != "" {
			gas, cpuwin, cpuubu := cpu.CalculateCpuAndGasForCall(poc, selector, []int{}, 0)
			pocFunctionCostsGas = gas
			pocFunctionCostsCpuWin = cpuwin
			pocFunctionCostsCpuUbu = cpuubu
			fmt.Printf("POC contract %s consumes: %05d gas, %06d cpu (Win), %06d cpu (Ubu)\n", selector, gas, cpuwin, cpuubu)
		}

		if highruns != "" {
			gas, cpuwin, cpuubu := cpu.CalculateCpuAndGasForCall(highruns, selector, []int{}, 0)
			fmt.Printf("High contract %s consumes: %05d gas, %06d cpu (Win), %06d cpu (Ubu)\n", selector, gas, cpuwin, cpuubu)
			fmt.Printf("strategic optimization saved | %d | nanoseconds on the cpu! \n", cpuubu-pocFunctionCostsCpuUbu)
		}

		for seq, contracts := range finalList {
			gas, cpuwin, cpuubu := cpu.CalculateCpuAndGasForCall(seq, selector, []int{}, 0)
			//fmt.Printf("sequence %q consumed: %d gas, %d cpu (Win), %d cpu (Ubu)\n", seq, gas, cpuwin, cpuubu)
			key := fmt.Sprintf("%05d|%06d|%06d", gas, cpuwin, cpuubu)
			functionCosts[key] = append(functionCosts[key], contracts...)
		}
		/*
				// for printing the summary, use the sorted keys
				keys := make([]string, 0, len(functionCosts))
				for k := range functionCosts {
					keys = append(keys, k)
				}
				sort.Strings(keys)


					fmt.Printf("Total amount of transferCost groups: %d\n", len(functionCosts))
					for _, cost := range keys {
						fmt.Printf("%s has %d contracts\n", cost, len(functionCosts[cost]))
					}

			contractCount = 0
			for _, contracts := range functionCosts {
				contractCount += len(contracts)
			}
			fmt.Printf("total contracts counted: %d\n", contractCount)

				for cost, contracts := range functionCosts {
					fmt.Printf("Cost %s has %d contracts\n", cost, len(contracts))
				}
		*/
		err := io.WriteTransferCostsCSV(csvFile, functionCosts)
		if err != nil {
			log.Fatalf("write CSV: %v", err)
		}
		if poc != "" {
			// rewrite functionCosts to show difference to poc contract
			diffCosts := make(map[string][]string)
			for key, contracts := range functionCosts {
				parts := strings.Split(key, "|")
				if len(parts) != 3 {
					log.Printf("warning: unexpected key format %q", key)
					continue
				}

				gas, err1 := strconv.Atoi(parts[0])
				cpuWin, err2 := strconv.Atoi(parts[1])
				cpuUbu, err3 := strconv.Atoi(parts[2])
				if err1 != nil || err2 != nil || err3 != nil {
					log.Printf("warning: parse error for key %q", key)
					continue
				}
				diffGas := int64(gas) - int64(pocFunctionCostsGas)
				diffCpuWin := int64(cpuWin) - int64(pocFunctionCostsCpuWin)
				diffCpuUbu := int64(cpuUbu) - int64(pocFunctionCostsCpuUbu)
				diffKey := fmt.Sprintf("%+d|%+d|%+d", diffGas, diffCpuWin, diffCpuUbu)
				diffCosts[diffKey] = append(diffCosts[diffKey], contracts...)
			}
			err := io.WriteTransferCostsCSV(diffCSV, diffCosts)
			if err != nil {
				log.Fatalf("write diff CSV: %v", err)
			}
		}
	}
}
