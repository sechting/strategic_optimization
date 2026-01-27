package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Instruction represents a disassembled EVM opcode
type Instruction struct {
	PC       int    `json:"pc"`
	Opcode   string `json:"opcode"`
	Argument string `json:"argument,omitempty"`
	Gas      int    `json:"cost,omitempty"`
}

var gasTable map[byte]int

func init() {
	gasTable = buildGasTable()
}

func buildGasTable() map[byte]int {
	var gasClasses = map[int][]byte{
		0: {0x00}, // STOP

		1: {0x5B}, // JUMPDEST

		2: {
			0x30, 0x32, 0x33, 0x34, 0x36, 0x38, 0x3A, 0x3B, 0x3D,
			0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x48, 0x4A,
			0x50, 0x58, 0x59, 0x5A, 0x5F,
		},

		3: {
			0x01, 0x03, // ADD, SUB
			0x0B,                                                             // SIGNEXTEND
			0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, // comparisons & bit ops
			0x35, 0x37, 0x39, 0x3E, // CALLDATALOAD, CALLDATACOPY, CODECOPY, RETURNDATACOPY
			0x51, 0x52, 0x53, // MLOAD, MSTORE, MSTORE8
			0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x6B, 0x6C, 0x6D, 0x6E, 0x6F, 0x70, 0x71,
			0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x7B, 0x7C, 0x7D, 0x7E, 0x7F, // PUSH1–PUSH32
			0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8A, 0x8B, 0x8C, 0x8D, 0x8E, 0x8F, // DUP1–DUP16
			0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0x9B, 0x9C, 0x9D, 0x9E, 0x9F, // SWAP1–SWAP16
		},

		5: {
			0x02, 0x04, 0x05, 0x06, 0x07, // MUL, DIV, SDIV, MOD, SMOD
			0x0B, 0x47,
		},

		8: {0x08, 0x09, 0x56}, // ADDMOD, MULMOD, JUMP

		10: {0x0A, 0x57}, // EXP, JUMPI

		20: {0x40}, // BLOCKHASH

		30: {0x20}, // SHA3

		100: {0x54}, // SLOAD

		5000: {0x55}, // SSTORE

		700: {0x3C}, // EXTCODECOPY

		32000: {0xF0}, // CREATE
	}

	gasTable := make(map[byte]int)
	for cost, ops := range gasClasses {
		for _, op := range ops {
			gasTable[op] = cost
		}
	}

	return gasTable
}

func NormalizeInstructions(insts []Instruction, args bool) string {
	var sb strings.Builder
	for _, inst := range insts {
		sb.WriteString(fmt.Sprintf("%s", inst.Opcode))
		if args && inst.Argument != "" {
			sb.WriteString(fmt.Sprintf("%s", inst.Argument))
		}
	}
	return sb.String()
}

func DisassembleBytecode(codeHex string) ([]Instruction, error) {
	clean := strings.TrimPrefix(codeHex, "0x")
	code, err := hex.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	if len(code) < 2 {
		return nil, fmt.Errorf("bytecode too short to contain metadata length")
	}

	// last two bytes = metadata length (big-endian uint16)
	metaLength := int(code[len(code)-2])<<8 | int(code[len(code)-1])

	runtimeCode := code
	if metaLength > 0 && metaLength+2 <= len(code) {
		metadataStart := len(code) - metaLength - 2
		if metadataStart >= 0 {
			firstMetaByte := code[metadataStart]
			if firstMetaByte >= 0xa0 && firstMetaByte <= 0xbf {
				// Valid CBOR - cut off metadata
				runtimeCode = code[:metadataStart]
			}
		}
	}

	var instructions []Instruction
	for pc := 0; pc < len(runtimeCode); {
		op := runtimeCode[pc]
		inst := Instruction{
			PC:     pc,
			Opcode: fmt.Sprintf("%02x", op),
			Gas:    0,
		}

		if gas, ok := gasTable[op]; ok {
			inst.Gas = gas
		}

		// If PUSH1–PUSH32 (0x60–0x7f)
		if op >= 0x60 && op <= 0x7f {
			argLen := int(op - 0x5f)
			if pc+argLen >= len(runtimeCode) {
				break
			}
			argBytes := runtimeCode[pc+1 : pc+1+argLen]
			inst.Argument = hex.EncodeToString(argBytes)
			pc += 1 + argLen
		} else {
			pc++
		}

		instructions = append(instructions, inst)
	}

	return instructions, nil
}

func GetInstrPC(instrs []Instruction, pc int) int {
	return instrs[pc].PC
}
