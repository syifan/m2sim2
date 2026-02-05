// Package emu provides functional ARM64 emulation.
package emu

// LoadStoreUnit implements ARM64 load and store operations.
type LoadStoreUnit struct {
	regFile *RegFile
	memory  *Memory
}

// NewLoadStoreUnit creates a new LoadStoreUnit connected to the given
// register file and memory.
func NewLoadStoreUnit(regFile *RegFile, memory *Memory) *LoadStoreUnit {
	return &LoadStoreUnit{
		regFile: regFile,
		memory:  memory,
	}
}

// LDR64 performs a 64-bit load: Xd = mem[Xn + offset]
func (lsu *LoadStoreUnit) LDR64(rd, rn uint8, offset uint64) {
	base := lsu.regFile.ReadReg(rn)
	addr := base + offset
	value := lsu.memory.Read64(addr)
	lsu.regFile.WriteReg(rd, value)
}

// LDR64SP performs a 64-bit load using SP as base: Xd = mem[SP + offset]
func (lsu *LoadStoreUnit) LDR64SP(rd uint8, offset uint64) {
	addr := lsu.regFile.SP + offset
	value := lsu.memory.Read64(addr)
	lsu.regFile.WriteReg(rd, value)
}

// LDR32 performs a 32-bit load with zero extension: Xd = zero_extend(mem[Xn + offset])
func (lsu *LoadStoreUnit) LDR32(rd, rn uint8, offset uint64) {
	base := lsu.regFile.ReadReg(rn)
	addr := base + offset
	value := lsu.memory.Read32(addr)
	// Zero-extend to 64 bits by storing as uint64
	lsu.regFile.WriteReg(rd, uint64(value))
}

// LDR32SP performs a 32-bit load using SP as base: Xd = zero_extend(mem[SP + offset])
func (lsu *LoadStoreUnit) LDR32SP(rd uint8, offset uint64) {
	addr := lsu.regFile.SP + offset
	value := lsu.memory.Read32(addr)
	lsu.regFile.WriteReg(rd, uint64(value))
}

// STR64 performs a 64-bit store: mem[Xn + offset] = Xd
func (lsu *LoadStoreUnit) STR64(rd, rn uint8, offset uint64) {
	base := lsu.regFile.ReadReg(rn)
	addr := base + offset
	value := lsu.regFile.ReadReg(rd)
	lsu.memory.Write64(addr, value)
}

// STR64SP performs a 64-bit store using SP as base: mem[SP + offset] = Xd
func (lsu *LoadStoreUnit) STR64SP(rd uint8, offset uint64) {
	addr := lsu.regFile.SP + offset
	value := lsu.regFile.ReadReg(rd)
	lsu.memory.Write64(addr, value)
}

// STR32 performs a 32-bit store: mem[Xn + offset] = Wd (lower 32 bits)
func (lsu *LoadStoreUnit) STR32(rd, rn uint8, offset uint64) {
	base := lsu.regFile.ReadReg(rn)
	addr := base + offset
	value := uint32(lsu.regFile.ReadReg(rd))
	lsu.memory.Write32(addr, value)
}

// STR32SP performs a 32-bit store using SP as base: mem[SP + offset] = Wd
func (lsu *LoadStoreUnit) STR32SP(rd uint8, offset uint64) {
	addr := lsu.regFile.SP + offset
	value := uint32(lsu.regFile.ReadReg(rd))
	lsu.memory.Write32(addr, value)
}

// LDRB loads a byte with zero extension: Xd = zero_extend(mem[addr])
func (lsu *LoadStoreUnit) LDRB(rd uint8, addr uint64) {
	value := lsu.memory.Read8(addr)
	lsu.regFile.WriteReg(rd, uint64(value))
}

// STRB stores a byte: mem[addr] = Xd[7:0]
func (lsu *LoadStoreUnit) STRB(rd uint8, addr uint64) {
	value := uint8(lsu.regFile.ReadReg(rd))
	lsu.memory.Write8(addr, value)
}

// LDRSB loads a signed byte with sign extension to 64-bit
func (lsu *LoadStoreUnit) LDRSB64(rd uint8, addr uint64) {
	value := lsu.memory.Read8(addr)
	// Sign extend from 8 to 64 bits
	signExtended := int64(int8(value))
	lsu.regFile.WriteReg(rd, uint64(signExtended))
}

// LDRSB32 loads a signed byte with sign extension to 32-bit
func (lsu *LoadStoreUnit) LDRSB32(rd uint8, addr uint64) {
	value := lsu.memory.Read8(addr)
	// Sign extend from 8 to 32 bits (upper 32 bits cleared)
	signExtended := int32(int8(value))
	lsu.regFile.WriteReg(rd, uint64(uint32(signExtended)))
}

// LDRH loads a halfword with zero extension: Xd = zero_extend(mem[addr])
func (lsu *LoadStoreUnit) LDRH(rd uint8, addr uint64) {
	value := lsu.memory.Read16(addr)
	lsu.regFile.WriteReg(rd, uint64(value))
}

// STRH stores a halfword: mem[addr] = Xd[15:0]
func (lsu *LoadStoreUnit) STRH(rd uint8, addr uint64) {
	value := uint16(lsu.regFile.ReadReg(rd))
	lsu.memory.Write16(addr, value)
}

// LDRSH64 loads a signed halfword with sign extension to 64-bit
func (lsu *LoadStoreUnit) LDRSH64(rd uint8, addr uint64) {
	value := lsu.memory.Read16(addr)
	// Sign extend from 16 to 64 bits
	signExtended := int64(int16(value))
	lsu.regFile.WriteReg(rd, uint64(signExtended))
}

// LDRSH32 loads a signed halfword with sign extension to 32-bit
func (lsu *LoadStoreUnit) LDRSH32(rd uint8, addr uint64) {
	value := lsu.memory.Read16(addr)
	// Sign extend from 16 to 32 bits (upper 32 bits cleared)
	signExtended := int32(int16(value))
	lsu.regFile.WriteReg(rd, uint64(uint32(signExtended)))
}

// LDRSW loads a signed word with sign extension to 64-bit
func (lsu *LoadStoreUnit) LDRSW(rd uint8, addr uint64) {
	value := lsu.memory.Read32(addr)
	// Sign extend from 32 to 64 bits
	signExtended := int64(int32(value))
	lsu.regFile.WriteReg(rd, uint64(signExtended))
}
