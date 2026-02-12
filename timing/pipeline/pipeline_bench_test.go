package pipeline

import (
	"encoding/binary"
	"testing"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/cache"
	"github.com/sarchlab/m2sim/timing/latency"
)

// encodeADDImm encodes ADD Xd, Xn, #imm (64-bit).
func encodeADDImm(rd, rn uint8, imm uint16) uint32 {
	// sf=1, op=0, S=0, 100010, sh=0, imm12, Rn, Rd
	return (1 << 31) | (0b100010 << 23) | (uint32(imm&0xFFF) << 10) |
		(uint32(rn) << 5) | uint32(rd)
}

// encodeSUBSImm encodes SUBS Xd, Xn, #imm (64-bit, sets flags).
func encodeSUBSImm(rd, rn uint8, imm uint16) uint32 {
	// sf=1, op=1, S=1, 100010, sh=0, imm12, Rn, Rd
	return (1 << 31) | (1 << 30) | (1 << 29) | (0b100010 << 23) |
		(uint32(imm&0xFFF) << 10) | (uint32(rn) << 5) | uint32(rd)
}

// encodeBCond encodes B.cond with a signed offset in instructions.
func encodeBCond(offsetInsts int32, cond uint8) uint32 {
	imm19 := uint32(offsetInsts) & 0x7FFFF
	return (0x54 << 24) | (imm19 << 5) | uint32(cond&0xF)
}

// encodeSVC encodes SVC #imm.
func encodeSVC(imm uint16) uint32 {
	return (0xD4 << 24) | (uint32(imm) << 5) | 0x01
}

// encodeLDR64 encodes LDR Xt, [Xn, #imm] (unsigned offset, scaled by 8).
func encodeLDR64(rt, rn uint8, imm12 uint16) uint32 {
	return (0b11 << 30) | (0b111 << 27) | (0b01 << 24) | (0b01 << 22) |
		(uint32(imm12&0xFFF) << 10) | (uint32(rn&0x1F) << 5) | uint32(rt&0x1F)
}

// encodeSTR64 encodes STR Xt, [Xn, #imm] (unsigned offset, scaled by 8).
func encodeSTR64(rt, rn uint8, imm12 uint16) uint32 {
	return (0b11 << 30) | (0b111 << 27) | (0b01 << 24) | (0b00 << 22) |
		(uint32(imm12&0xFFF) << 10) | (uint32(rn&0x1F) << 5) | uint32(rt&0x1F)
}

// writeWords writes instruction words into memory at the given address.
func writeWords(mem *emu.Memory, basePC uint64, words []uint32) {
	for i, w := range words {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], w)
		for j := 0; j < 4; j++ {
			mem.Write8(basePC+uint64(i*4+j), buf[j])
		}
	}
}

// setupBenchPipeline creates a pipeline with a tight ALU loop in memory.
// Loop body: 6 ADDs + SUBS + B.NE (back to start)
// After loop: SVC #0
func setupBenchPipeline(iterations uint64, width int) *Pipeline {
	regFile := &emu.RegFile{}
	mem := emu.NewMemory()

	basePC := uint64(0x1000)
	words := []uint32{
		encodeADDImm(2, 2, 1),  // ADD X2, X2, #1
		encodeADDImm(3, 3, 1),  // ADD X3, X3, #1
		encodeADDImm(4, 4, 1),  // ADD X4, X4, #1
		encodeADDImm(5, 5, 1),  // ADD X5, X5, #1
		encodeADDImm(6, 6, 1),  // ADD X6, X6, #1
		encodeADDImm(7, 7, 1),  // ADD X7, X7, #1
		encodeSUBSImm(0, 0, 1), // SUBS X0, X0, #1
		encodeBCond(-7, 1),     // B.NE -7 instructions (back to start)
		encodeSVC(0),           // SVC #0 — exit
	}

	writeWords(mem, basePC, words)

	regFile.WriteReg(0, iterations)
	regFile.WriteReg(8, 93)

	p := NewPipeline(regFile, mem, widthOpts(width)...)
	p.SetPC(basePC)

	return p
}

// widthOpts returns pipeline options for a given issue width.
func widthOpts(width int) []PipelineOption {
	switch {
	case width >= 8:
		return []PipelineOption{WithOctupleIssue()}
	case width >= 6:
		return []PipelineOption{WithSextupleIssue()}
	case width >= 4:
		return []PipelineOption{WithQuadIssue()}
	case width >= 2:
		return []PipelineOption{WithDualIssue()}
	default:
		return nil
	}
}

// setupLoadHeavyPipeline creates a pipeline with a load-heavy loop.
// Loop: 4 LDRs (sequential, cache-friendly) + SUBS + B.NE
// Data region at 0x80000, code at 0x1000.
func setupLoadHeavyPipeline(iterations uint64, width int, withCache bool) *Pipeline {
	regFile := &emu.RegFile{}
	mem := emu.NewMemory()

	dataBase := uint64(0x80000)
	// Pre-fill data region so loads return valid values.
	for i := 0; i < 256; i++ {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(i))
		for j := 0; j < 8; j++ {
			mem.Write8(dataBase+uint64(i*8+j), buf[j])
		}
	}

	basePC := uint64(0x1000)
	// X1 = data base address
	words := []uint32{
		encodeLDR64(2, 1, 0), // LDR X2, [X1, #0]
		encodeLDR64(3, 1, 1), // LDR X3, [X1, #8]
		encodeLDR64(4, 1, 2), // LDR X4, [X1, #16]
		encodeLDR64(5, 1, 3), // LDR X5, [X1, #24]
		encodeSUBSImm(0, 0, 1),
		encodeBCond(-5, 1), // B.NE back
		encodeSVC(0),
	}

	writeWords(mem, basePC, words)

	regFile.WriteReg(0, iterations)
	regFile.WriteReg(1, dataBase)
	regFile.WriteReg(8, 93)
	regFile.SP = 0x10000

	opts := widthOpts(width)
	if withCache {
		opts = append(opts,
			WithDCache(cache.DefaultL1DConfig()),
			WithICache(cache.DefaultL1IConfig()),
		)
	}
	opts = append(opts, WithLatencyTable(latency.NewTableWithConfig(latency.DefaultTimingConfig())))

	p := NewPipeline(regFile, mem, opts...)
	p.SetPC(basePC)

	return p
}

// setupStoreHeavyPipeline creates a pipeline with a store-heavy loop.
func setupStoreHeavyPipeline(iterations uint64, width int, withCache bool) *Pipeline {
	regFile := &emu.RegFile{}
	mem := emu.NewMemory()

	dataBase := uint64(0x80000)

	basePC := uint64(0x1000)
	words := []uint32{
		encodeSTR64(2, 1, 0), // STR X2, [X1, #0]
		encodeSTR64(3, 1, 1), // STR X3, [X1, #8]
		encodeSTR64(4, 1, 2), // STR X4, [X1, #16]
		encodeSTR64(5, 1, 3), // STR X5, [X1, #24]
		encodeSUBSImm(0, 0, 1),
		encodeBCond(-5, 1), // B.NE back
		encodeSVC(0),
	}

	writeWords(mem, basePC, words)

	regFile.WriteReg(0, iterations)
	regFile.WriteReg(1, dataBase)
	regFile.WriteReg(2, 42)
	regFile.WriteReg(3, 43)
	regFile.WriteReg(4, 44)
	regFile.WriteReg(5, 45)
	regFile.WriteReg(8, 93)
	regFile.SP = 0x10000

	opts := widthOpts(width)
	if withCache {
		opts = append(opts,
			WithDCache(cache.DefaultL1DConfig()),
			WithICache(cache.DefaultL1IConfig()),
		)
	}
	opts = append(opts, WithLatencyTable(latency.NewTableWithConfig(latency.DefaultTimingConfig())))

	p := NewPipeline(regFile, mem, opts...)
	p.SetPC(basePC)

	return p
}

// setupDepChainPipeline creates a pipeline with a serial dependency chain.
// Each ADD depends on the previous result: ADD X2,X2,#1; ADD X3,X2,#1; ADD X4,X3,#1; ...
func setupDepChainPipeline(iterations uint64, width int) *Pipeline {
	regFile := &emu.RegFile{}
	mem := emu.NewMemory()

	basePC := uint64(0x1000)
	words := []uint32{
		encodeADDImm(2, 2, 1),  // ADD X2, X2, #1
		encodeADDImm(3, 2, 1),  // ADD X3, X2, #1  (depends on X2)
		encodeADDImm(4, 3, 1),  // ADD X4, X3, #1  (depends on X3)
		encodeADDImm(5, 4, 1),  // ADD X5, X4, #1  (depends on X4)
		encodeADDImm(6, 5, 1),  // ADD X6, X5, #1  (depends on X5)
		encodeADDImm(7, 6, 1),  // ADD X7, X6, #1  (depends on X6)
		encodeSUBSImm(0, 0, 1), // SUBS X0, X0, #1
		encodeBCond(-7, 1),     // B.NE back
		encodeSVC(0),
	}

	writeWords(mem, basePC, words)

	regFile.WriteReg(0, iterations)
	regFile.WriteReg(8, 93)

	p := NewPipeline(regFile, mem, widthOpts(width)...)
	p.SetPC(basePC)

	return p
}

// setupMixedPipeline creates a pipeline with a mixed ALU/memory workload.
// Alternates loads, ALU ops, and stores.
func setupMixedPipeline(iterations uint64, width int, withCache bool) *Pipeline {
	regFile := &emu.RegFile{}
	mem := emu.NewMemory()

	dataBase := uint64(0x80000)
	// Pre-fill data
	for i := 0; i < 64; i++ {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(i*10))
		for j := 0; j < 8; j++ {
			mem.Write8(dataBase+uint64(i*8+j), buf[j])
		}
	}

	basePC := uint64(0x1000)
	words := []uint32{
		encodeLDR64(2, 1, 0),   // LDR X2, [X1, #0]
		encodeADDImm(3, 2, 1),  // ADD X3, X2, #1
		encodeSTR64(3, 1, 4),   // STR X3, [X1, #32]
		encodeLDR64(4, 1, 1),   // LDR X4, [X1, #8]
		encodeADDImm(5, 4, 2),  // ADD X5, X4, #2
		encodeSTR64(5, 1, 5),   // STR X5, [X1, #40]
		encodeSUBSImm(0, 0, 1), // SUBS X0, X0, #1
		encodeBCond(-7, 1),     // B.NE back
		encodeSVC(0),
	}

	writeWords(mem, basePC, words)

	regFile.WriteReg(0, iterations)
	regFile.WriteReg(1, dataBase)
	regFile.WriteReg(8, 93)
	regFile.SP = 0x10000

	opts := widthOpts(width)
	if withCache {
		opts = append(opts,
			WithDCache(cache.DefaultL1DConfig()),
			WithICache(cache.DefaultL1IConfig()),
		)
	}
	opts = append(opts, WithLatencyTable(latency.NewTableWithConfig(latency.DefaultTimingConfig())))

	p := NewPipeline(regFile, mem, opts...)
	p.SetPC(basePC)

	return p
}

// --- ALU benchmarks ---

func BenchmarkPipelineTick8Wide(b *testing.B) {
	p := setupBenchPipeline(uint64(b.N), 8)
	b.ResetTimer()
	p.Run()
}

func BenchmarkPipelineTick1Wide(b *testing.B) {
	p := setupBenchPipeline(uint64(b.N), 1)
	b.ResetTimer()
	p.Run()
}

// --- Dependency chain benchmarks ---

func BenchmarkPipelineDepChain8Wide(b *testing.B) {
	p := setupDepChainPipeline(uint64(b.N), 8)
	b.ResetTimer()
	p.Run()
}

func BenchmarkPipelineDepChain1Wide(b *testing.B) {
	p := setupDepChainPipeline(uint64(b.N), 1)
	b.ResetTimer()
	p.Run()
}

// --- Load-heavy benchmarks ---

func BenchmarkPipelineLoadHeavy8Wide(b *testing.B) {
	p := setupLoadHeavyPipeline(uint64(b.N), 8, false)
	b.ResetTimer()
	p.Run()
}

func BenchmarkPipelineLoadHeavy8WideCache(b *testing.B) {
	p := setupLoadHeavyPipeline(uint64(b.N), 8, true)
	b.ResetTimer()
	p.Run()
}

// --- Store-heavy benchmarks ---

func BenchmarkPipelineStoreHeavy8Wide(b *testing.B) {
	p := setupStoreHeavyPipeline(uint64(b.N), 8, false)
	b.ResetTimer()
	p.Run()
}

func BenchmarkPipelineStoreHeavy8WideCache(b *testing.B) {
	p := setupStoreHeavyPipeline(uint64(b.N), 8, true)
	b.ResetTimer()
	p.Run()
}

// --- Mixed workload benchmarks ---

func BenchmarkPipelineMixed8Wide(b *testing.B) {
	p := setupMixedPipeline(uint64(b.N), 8, false)
	b.ResetTimer()
	p.Run()
}

func BenchmarkPipelineMixed8WideCache(b *testing.B) {
	p := setupMixedPipeline(uint64(b.N), 8, true)
	b.ResetTimer()
	p.Run()
}

// --- Decoder benchmarks ---

func BenchmarkDecoderDecode(b *testing.B) {
	d := insts.NewDecoder()
	word := encodeADDImm(2, 3, 42) // ADD X2, X3, #42
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Decode(word)
	}
}

func BenchmarkDecoderDecodeInto(b *testing.B) {
	d := insts.NewDecoder()
	word := encodeADDImm(2, 3, 42) // ADD X2, X3, #42
	var inst insts.Instruction
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.DecodeInto(word, &inst)
	}
}
