package cpu

import (
	"fmt"
	"os"
	"strconv"
)

/*
	The data we use is taken from the paper:
	"The OpBench Ethereum opcode benchmark framework: Design, implementation, validation and experiments"
	The data is referring to a
	Intel i7 3.50 GHz
	6 Cores
	32 GB Ram
	running Ubuntu 18.04.3 LTS or Windows 10
	using the Go-Ethereum (geth) client
*/

/*
	OpInfo holds information about an operation's resource usage.

Name is only the name of the operation and is just for human readability.
Gas is the gas cost of the operation.
CPU is the CPU time in nanoseconds taken by the operation. split in _win and _ubu
*/
type OpInfo struct {
	Name   string
	Gas    uint64
	CpuWin uint64
	CpuUbu uint64
}

var opByHex = map[string]OpInfo{
	"01": {Name: "ADD", Gas: 3, CpuWin: 602, CpuUbu: 620},
	"02": {Name: "MUL", Gas: 5, CpuWin: 1184, CpuUbu: 1089},
	"03": {Name: "SUB", Gas: 3, CpuWin: 611, CpuUbu: 640},
	"04": {Name: "DIV", Gas: 5, CpuWin: 1236, CpuUbu: 1130},
	"05": {Name: "SDIV", Gas: 5, CpuWin: 1755, CpuUbu: 1154},
	"06": {Name: "MOD", Gas: 5, CpuWin: 720, CpuUbu: 736},
	"07": {Name: "SMOD", Gas: 5, CpuWin: 504, CpuUbu: 520},
	"08": {Name: "ADDMOD", Gas: 8, CpuWin: 1120, CpuUbu: 1330},
	"09": {Name: "MULMOD", Gas: 8, CpuWin: 1113, CpuUbu: 1752},
	"0a": {Name: "EXP", Gas: 11, CpuWin: 13856, CpuUbu: 14158}, // this is using uint64, assuming only 1 byte exponents
	"0b": {Name: "SIGNEXTEND", Gas: 5, CpuWin: 1216, CpuUbu: 1290},
	"10": {Name: "LT", Gas: 3, CpuWin: 575, CpuUbu: 590},
	"11": {Name: "GT", Gas: 3, CpuWin: 578, CpuUbu: 582},
	"12": {Name: "SLT", Gas: 3, CpuWin: 656, CpuUbu: 673},
	"13": {Name: "SGT", Gas: 3, CpuWin: 648, CpuUbu: 681},
	"14": {Name: "EQ", Gas: 3, CpuWin: 571, CpuUbu: 594},
	"15": {Name: "ISZERO", Gas: 3, CpuWin: 589, CpuUbu: 618},
	"16": {Name: "AND", Gas: 3, CpuWin: 643, CpuUbu: 664},
	"17": {Name: "OR", Gas: 3, CpuWin: 646, CpuUbu: 667},
	"18": {Name: "XOR", Gas: 3, CpuWin: 549, CpuUbu: 560},
	"19": {Name: "NOT", Gas: 3, CpuWin: 610, CpuUbu: 629}, // MISSING, estimated value based on average of other bitwise ops
	"1a": {Name: "BYTE", Gas: 3, CpuWin: 650, CpuUbu: 665},
	"1b": {Name: "SHL", Gas: 3, CpuWin: 610, CpuUbu: 629}, // MISSING, estimated value based on average of other bitwise ops
	"1c": {Name: "SHR", Gas: 3, CpuWin: 610, CpuUbu: 629}, // MISSING, estimated value based on average of other bitwise ops
	"30": {Name: "ADDRESS", Gas: 2, CpuWin: 1170, CpuUbu: 1164},
	"33": {Name: "CALLER", Gas: 2, CpuWin: 1142, CpuUbu: 1115},
	"34": {Name: "CALLVALUE", Gas: 2, CpuWin: 556, CpuUbu: 565},
	"35": {Name: "CALLDATALOAD", Gas: 3, CpuWin: 556, CpuUbu: 565},   // MISSING, estimated value based on callvalue
	"36": {Name: "CALLDATASIZE", Gas: 2, CpuWin: 556, CpuUbu: 565},   // MISSING, estimated value based on callvalue
	"37": {Name: "CALLDATACOPY", Gas: 3, CpuWin: 2180, CpuUbu: 2192}, // using version 1 from paper
	"38": {Name: "CODESIZE", Gas: 2, CpuWin: 556, CpuUbu: 565},       // MISSING, estimated value based on callvalue
	"39": {Name: "CODECOPY", Gas: 9, CpuWin: 1952, CpuUbu: 1942},     // using version 1 from paper
	"3a": {Name: "GASPRICE", Gas: 2, CpuWin: 557, CpuUbu: 565},
	"3b": {Name: "EXTCODESIZE", Gas: 20, CpuWin: 965, CpuUbu: 952},
	"3c": {Name: "EXTCODECOPY", Gas: 20, CpuWin: 997, CpuUbu: 1003}, // using version 1 from paper
	"50": {Name: "POP", Gas: 2, CpuWin: 570, CpuUbu: 590},
	"51": {Name: "MLOAD", Gas: 3, CpuWin: 1838, CpuUbu: 1828},
	"52": {Name: "MSTORE", Gas: 12, CpuWin: 1726, CpuUbu: 1711}, // gas assumption is, that we have a cold memory and a cost of 9
	"54": {Name: "SLOAD", Gas: 100, CpuWin: 694, CpuUbu: 694},   // dynamic cost will be way higher
	"55": {Name: "SSTORE", Gas: 100, CpuWin: 522, CpuUbu: 522},  // dynamic cost will be way higher
	"56": {Name: "JUMP", Gas: 8, CpuWin: 500, CpuUbu: 500},      // MISSING -- use 500 as a lowest threshold
	"57": {Name: "JUMPI", Gas: 10, CpuWin: 500, CpuUbu: 500},    // MISSING -- use 500 as a lowest threshold
	"58": {Name: "PC", Gas: 2, CpuWin: 566, CpuUbu: 568},
	"59": {Name: "MEMSIZE", Gas: 2, CpuWin: 558, CpuUbu: 567},
	"5b": {Name: "JUMPDEST", Gas: 1, CpuWin: 500, CpuUbu: 500}, // MISSING -- use 500 as a lowest threshold
	"5f": {Name: "PUSH0", Gas: 2, CpuWin: 600, CpuUbu: 600},    // based on push1
	"60": {Name: "PUSH1", Gas: 3, CpuWin: 600, CpuUbu: 600},
	"61": {Name: "PUSH2", Gas: 3, CpuWin: 600, CpuUbu: 600},  // based on push1
	"62": {Name: "PUSH3", Gas: 3, CpuWin: 600, CpuUbu: 600},  // based on push1
	"63": {Name: "PUSH4", Gas: 3, CpuWin: 600, CpuUbu: 600},  // based on push1
	"64": {Name: "PUSH5", Gas: 3, CpuWin: 600, CpuUbu: 600},  // based on push1
	"65": {Name: "PUSH6", Gas: 3, CpuWin: 600, CpuUbu: 600},  // based on push1
	"73": {Name: "PUSH20", Gas: 3, CpuWin: 600, CpuUbu: 600}, // based on push1
	"7c": {Name: "PUSH29", Gas: 3, CpuWin: 600, CpuUbu: 600}, // based on push1
	"80": {Name: "DUP1", Gas: 3, CpuWin: 559, CpuUbu: 581},
	"81": {Name: "DUP2", Gas: 3, CpuWin: 559, CpuUbu: 581}, // based on dup1
	"82": {Name: "DUP3", Gas: 3, CpuWin: 559, CpuUbu: 581}, // based on dup1
	"83": {Name: "DUP4", Gas: 3, CpuWin: 559, CpuUbu: 581}, // based on dup1
	"84": {Name: "DUP5", Gas: 3, CpuWin: 559, CpuUbu: 581}, // based on dup1
	"85": {Name: "DUP6", Gas: 3, CpuWin: 559, CpuUbu: 581}, // based on dup1
	"90": {Name: "SWAP1", Gas: 3, CpuWin: 528, CpuUbu: 527},
	"91": {Name: "SWAP2", Gas: 3, CpuWin: 528, CpuUbu: 527},   // based on swap1
	"92": {Name: "SWAP3", Gas: 3, CpuWin: 528, CpuUbu: 527},   // based on swap1
	"93": {Name: "SWAP4", Gas: 3, CpuWin: 528, CpuUbu: 527},   // based on swap1
	"94": {Name: "SWAP5", Gas: 3, CpuWin: 528, CpuUbu: 527},   // based on swap1
	"95": {Name: "SWAP6", Gas: 3, CpuWin: 528, CpuUbu: 527},   // based on swap1
	"f3": {Name: "RETURN", Gas: 0, CpuWin: 500, CpuUbu: 500},  // MISSING -- use 500 as a lowest threshold
	"fd": {Name: "REVERT", Gas: 0, CpuWin: 500, CpuUbu: 500},  // MISSING -- use 500 as a lowest threshold
	"fe": {Name: "INVALID", Gas: 0, CpuWin: 500, CpuUbu: 500}, // MISSING -- use 500 as a lowest threshold
}

// Push appends v on top of the stack and returns the new stack.
func push(s []int, v int) []int {
	return append(s, v)
}

// Pop removes and returns the top element.
// ok is false if the stack is empty; s is the updated stack.
func pop(s []int) (v int, s2 []int) {
	n := len(s)
	if n == 0 {
		fmt.Printf("ERROR: stack is empty\n")
		os.Exit(0)
	}
	v = s[n-1]
	s2 = s[:n-1]
	return v, s2
}

func dualPop(s []int) (v1 int, v2 int, s2 []int) {
	n := len(s)
	if n <= 1 {
		fmt.Printf("ERROR: stack is too small (%d)\n", n)
		os.Exit(0)
	}
	v1 = s[n-1]
	v2 = s[n-2]
	s2 = s[:n-2]
	return v1, v2, s2
}

func swap(s []int, n int) (s2 []int) {
	l := len(s)
	a := s[l-1]
	b := s[l-(1+n)]
	s[l-1] = b
	s[l-(1+n)] = a
	return s
}

func CalculateCpuAndGasForCall(sequence string, fnc string, pStack []int, jOff int) (totalGas uint64, totalCpuWin uint64, totalCpuUbu uint64) {
	i := 0
	seqLen := len(sequence)
	stack := pStack
	//fmt.Printf("sequence:\n%s\nOffset:\n%d\n", sequence, jOff)
	hit := false
	for i < seqLen {
		opcode := sequence[i : i+2]
		opInfo, exists := opByHex[opcode]
		if !exists {
			// Unknown opcode, skip
			fmt.Printf("opcode %s not found\n", opcode)
			//fmt.Printf("%s\n", sequence)
			i += 2
			continue
		}
		/*
			fmt.Printf("[%x] op(%s) stack:\n", i/2, opcode)
			for _, item := range stack {
				fmt.Printf("%x|", item)
			}
			fmt.Println("")
		*/
		totalGas += opInfo.Gas
		totalCpuWin += opInfo.CpuWin
		totalCpuUbu += opInfo.CpuUbu

		// emulate a minimal evm to get to the desired function
		if opcode == "5f" { // PUSH0
			stack = push(stack, 0)
		} else if opcode >= "60" && opcode <= "63" { //PUSH1 - PUSH4
			// TODO check if we push target function. so we know if we stop execution soon
			b, err := strconv.Atoi(opcode)
			if err != nil {
				fmt.Printf("error parsing PUSH call: %s\n", err)
			}
			b -= 59 // number of bytes to read
			imm := sequence[i+2 : i+2+b*2]
			// do we check for our function of desire?
			//if opcode == "63" && imm == fnc {
			//	hit = true
			//}
			v, err := strconv.ParseInt(imm, 16, 0)
			if err != nil {
				fmt.Printf("error parsing push imm: %s\n", err)
			}
			stack = push(stack, int(v))
			i += b * 2
		} else if opcode == "73" { //PUSH20
			// this is retarded as well, this is checking if the caller is a certain address... we push a fake address here
			i += 40
			stack = push(stack, 0xaffe)
		} else if opcode == "7c" { // PUSH29
			// this is ridiculous, we will just push a 1 cause its used for div instead of shr
			stack = push(stack, 1)
			i += 58
		} else if opcode == "10" || opcode == "11" || opcode == "14" { // LT, GT, EQ
			a := 0
			b := 0
			a, b, stack = dualPop(stack)
			if ((opcode == "10") && (a < b)) || ((opcode == "11") && (a > b)) || ((opcode == "14") && (a == b)) {
				stack = push(stack, 1)
				if opcode == "14" {
					fn, err := strconv.ParseInt(fnc, 16, 64)
					if err != nil {
						fmt.Printf("error parsing function %s\n", err)
					} else {
						if int(fn) == a {
							hit = true
						}
					}
				}
			} else {
				stack = push(stack, 0)
			}
		} else if opcode == "15" { // ISZERO
			t := 0
			t, stack = pop(stack)
			if t == 0 {
				stack = push(stack, 1)
			} else {
				stack = push(stack, 0)
			}
		} else if opcode == "17" { // OR
			a := 0
			b := 0
			a, b, stack = dualPop(stack)
			stack = push(stack, a|b)
		} else if opcode == "52" { // MSTORE
			_, stack = pop(stack)
			_, stack = pop(stack)
			// we actually don't care about the memory
		} else if opcode == "59" { //MSIZE
			// this is a hack before PUSH0 was a thing, cheapest way to get 0 on the stack if no memory is there
			stack = push(stack, 0)
		} else if opcode == "30" {
			// this is for checking if the caller is the deployer, we fake it
			stack = push(stack, 0xaffe) // call the deployer a monkey in german cause he is a fucking retard that should die!
		} else if opcode == "36" { // CALLDATASIZE
			stack = push(stack, 4)
		} else if opcode == "35" { // CALLDATALOAD
			v, err := strconv.ParseInt(fnc, 16, 0)
			if err != nil {
				fmt.Printf("error parsing dataload imm: %s\n", err)
			}
			_, stack = pop(stack)
			stack = push(stack, int(v))
		} else if opcode == "56" { // JUMP
			pc := 0
			pc, stack = pop(stack)
			i = pc*2 - 2 // i will be incremented by 2 later again
		} else if opcode == "57" { // JUMPI
			pc := 0
			cond := 0
			pc, cond, stack = dualPop(stack)
			if cond != 0 {
				pc = pc - jOff // could be that we have only a snippet and we must adjust the jump location.
				//fmt.Printf("jumping to %d\n", pc)
				i = pc*2 - 2 // i will be incremented by 2 later again
			}
			if hit {
				// we hit our target function, we will execute the jump to the desired selector, stop right here!
				return totalGas, totalCpuWin, totalCpuUbu
			}
		} else if opcode == "34" { // CALLVALUE
			stack = push(stack, 0)
		} else if opcode == "1c" { // SHR
			value := 0
			_, value, stack = dualPop(stack)
			stack = push(stack, value) // we will only use SHR to get the dataload to the correct 4 bytes
		} else if opcode >= "80" && opcode <= "84" { // DUP1 - DUP5
			b, err := strconv.Atoi(opcode)
			if err != nil {
				fmt.Printf("error parsing DUP call: %s\n", err)
			}
			b -= 79
			n := len(stack)
			dup := stack[n-b]
			stack = push(stack, dup)
		} else if opcode == "5b" { //JUMPDEST
			// do nothing
		} else if opcode == "50" { // POP
			_, stack = pop(stack)
		} else if opcode == "01" { // ADD
			a := 0
			b := 0
			a, b, stack = dualPop(stack)
			stack = push(stack, a+b)
		} else if opcode == "04" { // DIV
			// DIV is used instead of SHR to make the calldata right no need here
			a := 0
			b := 0
			a, b, stack = dualPop(stack)
			stack = push(stack, a/b)
		} else if opcode == "16" { // AND
			a := 0
			b := 0
			a, b, stack = dualPop(stack)
			stack = push(stack, a&b)
		} else if opcode >= "90" && opcode <= "93" { // SWAP1 - SWAP4
			b, err := strconv.Atoi(opcode)
			if err != nil {
				fmt.Printf("error parsing SWAP call: %s\n", err)
			}
			b -= 89
			stack = swap(stack, b)
		} else if opcode == "0a" { // EXP
			// there is no real reason to have EXP besides making e0 and 2 to a large number you can divide with later on.
			// this is stupid because e0 is enough with SHR to achieve the same goal with less instructions, but who knows
			// why this happens in some contracts...
			_, _, stack = dualPop(stack)
			stack = push(stack, 1)
		} else if opcode == "fd" { // REVERT

		} else {
			fmt.Printf("OPCODE unkown (%s)\n", opcode)
			os.Exit(0)
		}

		// step one instruction ahead!
		i += 2
	}

	if hit == false {
		fmt.Printf("BUG: we ran out of the dispatcher and didn't find %s i==(%d)\n", fnc, i)
		os.Exit(0)
	}
	return
}
