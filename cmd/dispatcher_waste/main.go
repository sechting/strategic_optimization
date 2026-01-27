package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"

	"github.com/sechting/strategic_optimization/internal/cpu"
	"github.com/sechting/strategic_optimization/internal/io"
	"github.com/sechting/strategic_optimization/internal/seq"
)

func main() {
	/*
		pocGas, pocCpuwin, pocCpuubu := core.CalculateCpuAndGasForCall(
			"6306fdde0314611fb757508163071bc3c914611f21578163078dfbe714611d93578163095ea7b314611d44578163128fced11461023357816318160ddd14611ce65781631a2c66c414611c8d57816320e8c56514611884578163213cae631461175057816323b872dd14611580578163313ce567146115245781633644e515146114e957816338d52e0f1461147a5781633ba0b9a9146112a75781633f4ba83a146111975781634e71e0c8146110885781635255d185146110195781635c975abb14610fd757816370a0823114610f76578163769f8e5d14610d4e57816376d5de8514610cdf578163784367d614610c615781637ecebe0014610bff5781638456cb5914610b4d57816384b0196e14610a4b5781638da5cb5b146109f857816395d89b41146108d0578163a40bee50146107c8578163a9059cbb146106f4578163b8f82b2614610651578163c4f59f9b14610600578163cbe52ae31461054a578163d505accf146102fd57508063da88ecb41461022e578063dd62ed3e1461028a578063e30c397814610238578063ef5cfb8c14610233578063f8b2f9911461022e5763fa5a4f06146101df57",
			"a9059cbb",
			[]int{0xa9059cbb, 0xa9059cbb, 0xa9059cbb, 0xa9059cbb}, 45)
		fmt.Printf("POC token contract consumes: %05d gas, %06d cpu (Win), %06d cpu (Ubu)\n", pocGas, pocCpuwin, pocCpuubu)
		return

	*/

	// initialization
	path := filepath.FromSlash("data/dispatcher_sequences_args.json")
	selectors := []string{"70a08231", "a9059cbb"}

	// load sequences and filter them
	sequences := seq.ReadSeqInfo(path)
	finalList := seq.FilterSeqInfo(sequences)

	// calculate energy consumption until jump to passed function for all sequences.
	//pocGas, pocCpuWin, pocCpuUbu := cpu.CalculateCpuAndGasForCall(pocToken, "a9059cbb")
	//fmt.Printf("POC token contract consumes: %05d gas, %06d cpu (Win), %06d cpu (Ubu)\n", pocGas, pocCpuWin, pocCpuUbu)
	for _, selector := range selectors {
		csvFile := fmt.Sprintf("data/selector_waste_%s.csv", selector)
		functionCosts := make(map[string][]string)
		selec, err := strconv.ParseInt(selector, 16, 64)
		if err != nil {
			fmt.Printf("error parsing selector %s: %v\n", selector, err)
			return
		}
		// hardcoded POC token contract to calculate baseline consumption **TODO make this dynamic
		pocGas, pocCpuwin, pocCpuubu := cpu.CalculateCpuAndGasForCall(
			"6370a08231146100d2578063a9059cbb146100e557806323b872dd146100bf578063095ea7b31461008e57806306fdde031461070557806318160ddd1461071a578063313ce5671461073057806395d89b411461073f578063dd62ed3e14610747575b5f5ffd",
			selector,
			[]int{int(selec), int(selec), int(selec)}, 0)

		for seq, info := range finalList {
			gas, cpuwin, cpuubu := cpu.CalculateCpuAndGasForCall(seq, selector, []int{int(selec), int(selec), int(selec), int(selec), int(selec), int(selec)}, info.Offset)
			//fmt.Printf("sequence consumed: %d gas, %d cpu (Win), %d cpu (Ubu)\n", gas, cpuwin, cpuubu)
			key := fmt.Sprintf("%05d|%06d|%06d", int(gas)-int(pocGas), int(cpuwin)-int(pocCpuwin), int(cpuubu)-int(pocCpuubu))
			functionCosts[key] = append(functionCosts[key], info.Contracts...)
		}

		err = io.WriteTransferCostsCSV(csvFile, functionCosts)
		if err != nil {
			log.Fatalf("write CSV: %v", err)
		}
	}
}
