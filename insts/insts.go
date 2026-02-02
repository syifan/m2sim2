// Package insts provides ARM64 instruction definitions and decoding.
//
// This package implements decoding of ARM64 machine code into structured
// instruction representations. It supports:
//   - Data Processing (Immediate): ADD, SUB with immediate operands
//   - Data Processing (Register): ADD, SUB, AND, ORR, EOR with register operands
//   - Branch instructions: B, BL, B.cond, BR, BLR, RET
//
// Usage:
//
//	decoder := insts.NewDecoder()
//	inst := decoder.Decode(0x91002820) // ADD X0, X1, #42
//	fmt.Printf("Op: %v, Rd: %d, Rn: %d, Imm: %d\n", inst.Op, inst.Rd, inst.Rn, inst.Imm)
package insts
