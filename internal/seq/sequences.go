package seq

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strings"
)

type seqCount struct {
	Seq   string
	Count int
}

type SeqInfo struct {
	Offset    int      `json:"offset"`
	Contracts []string `json:"contracts"`
}

var nonOptionalFuncs = []string{
	"a9059cbb", // transfer(address,uint256)
	"095ea7b3", // approve(address,uint256)
	"23b872dd", // transferFrom(address,address,uint256)
	"70a08231", // balanceOf(address)
	"18160ddd", // totalSupply()
	"dd62ed3e", // allowance(address,address)
}

// there are some contracts, that do some custom shizzle in their init code and jump too much through the code
// since it is not standard sol compiler output we can just blacklist them for this analysis. It's just two
// weirdos anyway. fuck em!
var blockList = []string{
	"61000934156105d0565b610011610342565b6370a0823181146100bb576318160ddd81146100da5763a9059cbb81146100ef576323b872dd81146101185763095ea7b3811461014b5763dd62ed3e8114610174576340c10f19811461019d57635c975abb81146101c657638456cb5981146101d157633f4ba83a81146101e657635397e38981146101fb57632900e3778114610206576313548f9e81146102245763f44ff712811461022e57632b6efdb2811461023957",
	"6080604052366100175734151961001557600080fd5b005b34801561002357600080fd5b5061002e3415610149565b60003560e01c6370a082318114610086576318160ddd81146100a15763a9059cbb81146100ac576323b872dd81146100cc5763095ea7b381146100ec5763dd62ed3e8114610104576340c10f19811461013157",
}

func ReadSeqInfo(path string) map[string]SeqInfo {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read file: %v", err)
	}
	fmt.Printf("read %d bytes from %s\n", len(data), path)
	var seqInfos map[string]SeqInfo
	if err := json.Unmarshal(data, &seqInfos); err != nil {
		log.Fatalf("parse JSON: %v", err)
	}
	return seqInfos
}

func ReadSequences(path string) map[string][]string {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read file: %v", err)
	}
	fmt.Printf("read %d bytes from %s\n", len(data), path)
	var sequences map[string][]string
	if err := json.Unmarshal(data, &sequences); err != nil {
		log.Fatalf("parse JSON: %v", err)
	}
	return sequences
}

func MapSequence(sequences map[string][]string) []seqCount {
	// Build slice for sorting.
	list := make([]seqCount, 0, len(sequences))
	for seq, contracts := range sequences {
		list = append(list, seqCount{Seq: seq, Count: len(contracts)})
	}
	// Sort by occurrence (desc). Tie-break by sequence (asc).
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Seq < list[j].Seq
	})
	return list
}

func FilterSeqInfo(sequences map[string]SeqInfo) map[string]SeqInfo {
	out := make(map[string]SeqInfo)
	for seq, info := range sequences {
		ok := true
		if slices.Contains(blockList, seq) {
			ok = false
			//fmt.Printf("filtering out blacklisted sequence\n")
			continue
		}
		for _, fnc := range nonOptionalFuncs {
			if !strings.Contains(seq, "63"+fnc) {
				ok = false
				break
			}
		}
		if ok {
			out[seq] = info
		}
	}
	return out
}

func FilterSequence(sequences map[string][]string) map[string][]string {
	out := make(map[string][]string)
	for seq, contracts := range sequences {
		ok := true
		if slices.Contains(blockList, seq) {
			ok = false
			//fmt.Printf("filtering out blacklisted sequence\n")
			continue
		}
		for _, fnc := range nonOptionalFuncs {
			if !strings.Contains(seq, "63"+fnc) {
				ok = false
				break
			}
		}
		if ok {
			for _, item := range contracts {
				out[seq] = append(out[seq], item)
			}
		}
	}
	return out
}
