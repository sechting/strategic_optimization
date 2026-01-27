package main

import (
	"errors"
	"fmt"
)

func isDispatcher(instrs []Instruction, i int) (bool, int) {
	/* the pattern for the dispatcher is always as follows: we push the selector via push4,
	then optionally do some stack manipulation (DUP/SWAP), then do a comparison (EQ/GT/LT),
	and finally push the jump address and a conditional jump (JUMPI) to the function code.
	*/
	if instrs[i].Opcode != "63" { // PUSH4
		return false, -1
	} // is PUSH4 ??
	if i+4 >= len(instrs) {
		return false, -1
	} // enough instructions ??

	j := i + 1

	// Allow optional DUP1-DUP16 (0x80-0x8f) or SWAP1-SWAP16 (0x90-0x9f)
	for j < len(instrs) {
		op := instrs[j].Opcode
		if (op >= "80" && op <= "8f") || (op >= "90" && op <= "9f") {
			j++
			continue
		}
		break
	}

	if j+3 >= len(instrs) {
		return false, -1
	} // ran out of instructions

	cmp := instrs[j].Opcode // should be EQ, GT or LT
	if cmp != "10" && cmp != "11" && cmp != "14" {
		return false, -1
	}

	return instrs[j+1].Opcode == "61" && instrs[j+2].Opcode == "57", j + 2 // PUSH2, JUMPI
}

func ExtractInit(instructions []Instruction) ([]Instruction, int, error) {
	var initSeq []Instruction
	foundDispatcher := false
	dispatcherIndex := -1

	for i := 0; i < len(instructions)-3; i++ {
		inst := instructions[i]

		inDispatch, _ := isDispatcher(instructions, i)
		if inDispatch {
			foundDispatcher = true
			dispatcherIndex = i
			break
		}
		initSeq = append(initSeq, inst)
	}

	if !foundDispatcher {
		//log.Println("Warning: dispatcher start pattern not found; returning full instruction list")
		return instructions, -1, errors.New("dispatcher start not detected")
	}

	return initSeq, dispatcherIndex, nil
}

func ExtractDispatcher(instrs []Instruction, dispatcherIndex int) ([]Instruction, int, error) {
	if dispatcherIndex < 0 || dispatcherIndex >= len(instrs) {
		return nil, -1, fmt.Errorf("invalid dispatcher index")
	}

	var dispatcherSeq []Instruction
	i := dispatcherIndex
	gap := 0
	const MaxGap = 6

	for i < len(instrs) {
		isMatch, endIdx := isDispatcher(instrs, i)
		if !isMatch {
			gap++
			if gap > MaxGap {
				break
			}
			i++
			continue
		}

		// Add the dispatcher instructions to the sequence
		j := i - gap
		for j <= endIdx && j < len(instrs) {
			dispatcherSeq = append(dispatcherSeq, instrs[j])
			j++
		}
		i = endIdx + 1
		gap = 0
	}

	if len(dispatcherSeq) == 0 {
		return nil, -1, fmt.Errorf("no dispatcher pattern found at index %d", dispatcherIndex)
	}

	return dispatcherSeq, i, nil
}
