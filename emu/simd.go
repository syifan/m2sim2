// Package emu provides functional ARM64 emulation.
package emu

import (
	"math"
)

// SIMDArrangement represents the SIMD vector arrangement specifier.
type SIMDArrangement = uint8

// SIMD arrangement specifiers matching insts package.
const (
	Arr8B  SIMDArrangement = 0 // 8 bytes (64-bit, D register)
	Arr16B SIMDArrangement = 1 // 16 bytes (128-bit, Q register)
	Arr4H  SIMDArrangement = 2 // 4 halfwords (64-bit)
	Arr8H  SIMDArrangement = 3 // 8 halfwords (128-bit)
	Arr2S  SIMDArrangement = 4 // 2 singles (64-bit)
	Arr4S  SIMDArrangement = 5 // 4 singles (128-bit)
	Arr2D  SIMDArrangement = 6 // 2 doubles (128-bit)
)

// SIMD implements ARM64 SIMD (NEON) operations.
type SIMD struct {
	simdRegFile *SIMDRegFile
	regFile     *RegFile // For accessing base registers in load/store
	memory      *Memory
}

// NewSIMD creates a new SIMD execution unit.
func NewSIMD(simdRegFile *SIMDRegFile, regFile *RegFile, memory *Memory) *SIMD {
	return &SIMD{
		simdRegFile: simdRegFile,
		regFile:     regFile,
		memory:      memory,
	}
}

// VADD performs vector integer addition.
// Arrangement specifies the element size: 8B, 16B, 4H, 8H, 2S, 4S, 2D.
func (s *SIMD) VADD(vd, vn, vm uint8, arrangement SIMDArrangement) {
	switch arrangement {
	case Arr8B:
		s.vaddBytes(vd, vn, vm, 8)
	case Arr16B:
		s.vaddBytes(vd, vn, vm, 16)
	case Arr4H:
		s.vaddHalfwords(vd, vn, vm, 4)
	case Arr8H:
		s.vaddHalfwords(vd, vn, vm, 8)
	case Arr2S:
		s.vaddWords(vd, vn, vm, 2)
	case Arr4S:
		s.vaddWords(vd, vn, vm, 4)
	case Arr2D:
		s.vaddDoubles(vd, vn, vm, 2)
	}
}

func (s *SIMD) vaddBytes(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane8(vn, uint8(i))
		b := s.simdRegFile.ReadLane8(vm, uint8(i))
		s.simdRegFile.WriteLane8(vd, uint8(i), a+b)
	}
	// Clear upper bits if using 64-bit arrangement
	if count <= 8 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vaddHalfwords(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane16(vn, uint8(i))
		b := s.simdRegFile.ReadLane16(vm, uint8(i))
		s.simdRegFile.WriteLane16(vd, uint8(i), a+b)
	}
	if count <= 4 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vaddWords(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane32(vn, uint8(i))
		b := s.simdRegFile.ReadLane32(vm, uint8(i))
		s.simdRegFile.WriteLane32(vd, uint8(i), a+b)
	}
	if count <= 2 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vaddDoubles(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane64(vn, uint8(i))
		b := s.simdRegFile.ReadLane64(vm, uint8(i))
		s.simdRegFile.WriteLane64(vd, uint8(i), a+b)
	}
}

// VSUB performs vector integer subtraction.
func (s *SIMD) VSUB(vd, vn, vm uint8, arrangement SIMDArrangement) {
	switch arrangement {
	case Arr8B:
		s.vsubBytes(vd, vn, vm, 8)
	case Arr16B:
		s.vsubBytes(vd, vn, vm, 16)
	case Arr4H:
		s.vsubHalfwords(vd, vn, vm, 4)
	case Arr8H:
		s.vsubHalfwords(vd, vn, vm, 8)
	case Arr2S:
		s.vsubWords(vd, vn, vm, 2)
	case Arr4S:
		s.vsubWords(vd, vn, vm, 4)
	case Arr2D:
		s.vsubDoubles(vd, vn, vm, 2)
	}
}

func (s *SIMD) vsubBytes(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane8(vn, uint8(i))
		b := s.simdRegFile.ReadLane8(vm, uint8(i))
		s.simdRegFile.WriteLane8(vd, uint8(i), a-b)
	}
	if count <= 8 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vsubHalfwords(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane16(vn, uint8(i))
		b := s.simdRegFile.ReadLane16(vm, uint8(i))
		s.simdRegFile.WriteLane16(vd, uint8(i), a-b)
	}
	if count <= 4 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vsubWords(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane32(vn, uint8(i))
		b := s.simdRegFile.ReadLane32(vm, uint8(i))
		s.simdRegFile.WriteLane32(vd, uint8(i), a-b)
	}
	if count <= 2 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vsubDoubles(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane64(vn, uint8(i))
		b := s.simdRegFile.ReadLane64(vm, uint8(i))
		s.simdRegFile.WriteLane64(vd, uint8(i), a-b)
	}
}

// VMUL performs vector integer multiplication (element-wise).
// Note: This produces the low bits of the result (not widening).
func (s *SIMD) VMUL(vd, vn, vm uint8, arrangement SIMDArrangement) {
	switch arrangement {
	case Arr8B:
		s.vmulBytes(vd, vn, vm, 8)
	case Arr16B:
		s.vmulBytes(vd, vn, vm, 16)
	case Arr4H:
		s.vmulHalfwords(vd, vn, vm, 4)
	case Arr8H:
		s.vmulHalfwords(vd, vn, vm, 8)
	case Arr2S:
		s.vmulWords(vd, vn, vm, 2)
	case Arr4S:
		s.vmulWords(vd, vn, vm, 4)
	}
	// Note: MUL for 2D (64-bit elements) is not defined in NEON
}

func (s *SIMD) vmulBytes(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane8(vn, uint8(i))
		b := s.simdRegFile.ReadLane8(vm, uint8(i))
		s.simdRegFile.WriteLane8(vd, uint8(i), a*b)
	}
	if count <= 8 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vmulHalfwords(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane16(vn, uint8(i))
		b := s.simdRegFile.ReadLane16(vm, uint8(i))
		s.simdRegFile.WriteLane16(vd, uint8(i), a*b)
	}
	if count <= 4 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vmulWords(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		a := s.simdRegFile.ReadLane32(vn, uint8(i))
		b := s.simdRegFile.ReadLane32(vm, uint8(i))
		s.simdRegFile.WriteLane32(vd, uint8(i), a*b)
	}
	if count <= 2 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

// VFADD performs vector floating-point addition.
func (s *SIMD) VFADD(vd, vn, vm uint8, arrangement SIMDArrangement) {
	switch arrangement {
	case Arr2S:
		s.vfaddFloat32(vd, vn, vm, 2)
	case Arr4S:
		s.vfaddFloat32(vd, vn, vm, 4)
	case Arr2D:
		s.vfaddFloat64(vd, vn, vm, 2)
	}
}

func (s *SIMD) vfaddFloat32(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		aBits := s.simdRegFile.ReadLane32(vn, uint8(i))
		bBits := s.simdRegFile.ReadLane32(vm, uint8(i))
		a := math.Float32frombits(aBits)
		b := math.Float32frombits(bBits)
		result := math.Float32bits(a + b)
		s.simdRegFile.WriteLane32(vd, uint8(i), result)
	}
	if count <= 2 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vfaddFloat64(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		aBits := s.simdRegFile.ReadLane64(vn, uint8(i))
		bBits := s.simdRegFile.ReadLane64(vm, uint8(i))
		a := math.Float64frombits(aBits)
		b := math.Float64frombits(bBits)
		result := math.Float64bits(a + b)
		s.simdRegFile.WriteLane64(vd, uint8(i), result)
	}
}

// VFSUB performs vector floating-point subtraction.
func (s *SIMD) VFSUB(vd, vn, vm uint8, arrangement SIMDArrangement) {
	switch arrangement {
	case Arr2S:
		s.vfsubFloat32(vd, vn, vm, 2)
	case Arr4S:
		s.vfsubFloat32(vd, vn, vm, 4)
	case Arr2D:
		s.vfsubFloat64(vd, vn, vm, 2)
	}
}

func (s *SIMD) vfsubFloat32(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		aBits := s.simdRegFile.ReadLane32(vn, uint8(i))
		bBits := s.simdRegFile.ReadLane32(vm, uint8(i))
		a := math.Float32frombits(aBits)
		b := math.Float32frombits(bBits)
		result := math.Float32bits(a - b)
		s.simdRegFile.WriteLane32(vd, uint8(i), result)
	}
	if count <= 2 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vfsubFloat64(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		aBits := s.simdRegFile.ReadLane64(vn, uint8(i))
		bBits := s.simdRegFile.ReadLane64(vm, uint8(i))
		a := math.Float64frombits(aBits)
		b := math.Float64frombits(bBits)
		result := math.Float64bits(a - b)
		s.simdRegFile.WriteLane64(vd, uint8(i), result)
	}
}

// VFMUL performs vector floating-point multiplication.
func (s *SIMD) VFMUL(vd, vn, vm uint8, arrangement SIMDArrangement) {
	switch arrangement {
	case Arr2S:
		s.vfmulFloat32(vd, vn, vm, 2)
	case Arr4S:
		s.vfmulFloat32(vd, vn, vm, 4)
	case Arr2D:
		s.vfmulFloat64(vd, vn, vm, 2)
	}
}

func (s *SIMD) vfmulFloat32(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		aBits := s.simdRegFile.ReadLane32(vn, uint8(i))
		bBits := s.simdRegFile.ReadLane32(vm, uint8(i))
		a := math.Float32frombits(aBits)
		b := math.Float32frombits(bBits)
		result := math.Float32bits(a * b)
		s.simdRegFile.WriteLane32(vd, uint8(i), result)
	}
	if count <= 2 {
		s.simdRegFile.WriteLane64(vd, 1, 0)
	}
}

func (s *SIMD) vfmulFloat64(vd, vn, vm uint8, count int) {
	for i := 0; i < count; i++ {
		aBits := s.simdRegFile.ReadLane64(vn, uint8(i))
		bBits := s.simdRegFile.ReadLane64(vm, uint8(i))
		a := math.Float64frombits(aBits)
		b := math.Float64frombits(bBits)
		result := math.Float64bits(a * b)
		s.simdRegFile.WriteLane64(vd, uint8(i), result)
	}
}

// LDR128 loads a 128-bit Q register from memory.
func (s *SIMD) LDR128(vd uint8, addr uint64) {
	low := s.memory.Read64(addr)
	high := s.memory.Read64(addr + 8)
	s.simdRegFile.WriteQ(vd, low, high)
}

// STR128 stores a 128-bit Q register to memory.
func (s *SIMD) STR128(vd uint8, addr uint64) {
	low, high := s.simdRegFile.ReadQ(vd)
	s.memory.Write64(addr, low)
	s.memory.Write64(addr+8, high)
}

// DUP duplicates a scalar register value into all elements of a vector register.
// The scalar comes from a general purpose register (accessed via regFile).
func (s *SIMD) DUP(vd uint8, rn uint8, arrangement SIMDArrangement) {
	// Read the scalar value from the general purpose register
	// For byte/halfword/word operations, use the lower bits of the register
	scalarValue := s.regFile.ReadReg(rn)

	switch arrangement {
	case Arr8B:
		// Duplicate byte (lower 8 bits) into 8 lanes of D register
		byteVal := uint8(scalarValue)
		for i := 0; i < 8; i++ {
			s.simdRegFile.WriteLane8(vd, uint8(i), byteVal)
		}
		// Clear upper 64 bits
		s.simdRegFile.WriteLane64(vd, 1, 0)

	case Arr16B:
		// Duplicate byte (lower 8 bits) into 16 lanes of Q register
		byteVal := uint8(scalarValue)
		for i := 0; i < 16; i++ {
			s.simdRegFile.WriteLane8(vd, uint8(i), byteVal)
		}

	case Arr4H:
		// Duplicate halfword (lower 16 bits) into 4 lanes of D register
		halfwordVal := uint16(scalarValue)
		for i := 0; i < 4; i++ {
			s.simdRegFile.WriteLane16(vd, uint8(i), halfwordVal)
		}
		// Clear upper 64 bits
		s.simdRegFile.WriteLane64(vd, 1, 0)

	case Arr8H:
		// Duplicate halfword (lower 16 bits) into 8 lanes of Q register
		halfwordVal := uint16(scalarValue)
		for i := 0; i < 8; i++ {
			s.simdRegFile.WriteLane16(vd, uint8(i), halfwordVal)
		}

	case Arr2S:
		// Duplicate word (lower 32 bits) into 2 lanes of D register
		wordVal := uint32(scalarValue)
		for i := 0; i < 2; i++ {
			s.simdRegFile.WriteLane32(vd, uint8(i), wordVal)
		}
		// Clear upper 64 bits
		s.simdRegFile.WriteLane64(vd, 1, 0)

	case Arr4S:
		// Duplicate word (lower 32 bits) into 4 lanes of Q register
		wordVal := uint32(scalarValue)
		for i := 0; i < 4; i++ {
			s.simdRegFile.WriteLane32(vd, uint8(i), wordVal)
		}

	case Arr2D:
		// Duplicate doubleword (full 64 bits) into 2 lanes of Q register
		for i := 0; i < 2; i++ {
			s.simdRegFile.WriteLane64(vd, uint8(i), scalarValue)
		}
	}
}
