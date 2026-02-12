package pipeline

import (
	"encoding/binary"
	"testing"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
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

// setupBenchPipeline creates a pipeline with a tight ALU loop in memory.
// Loop body: 6 ADDs + SUBS + B.NE (back to start)
// After loop: SVC #0
func setupBenchPipeline(iterations uint64, width int) *Pipeline {
	regFile := &emu.RegFile{}
	mem := emu.NewMemory()

	// Write loop body at address 0x1000
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

	for i, w := range words {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], w)
		for j := 0; j < 4; j++ {
			mem.Write8(basePC+uint64(i*4+j), buf[j])
		}
	}

	// Set X0 = iteration count, X8 = 93 (exit syscall)
	regFile.WriteReg(0, iterations)
	regFile.WriteReg(8, 93)

	var opts []PipelineOption
	switch {
	case width >= 8:
		opts = append(opts, WithOctupleIssue())
	case width >= 6:
		opts = append(opts, WithSextupleIssue())
	case width >= 4:
		opts = append(opts, WithQuadIssue())
	case width >= 2:
		opts = append(opts, WithDualIssue())
	}

	p := NewPipeline(regFile, mem, opts...)
	p.SetPC(basePC)

	return p
}

// BenchmarkPipelineTick8Wide benchmarks the 8-wide superscalar tick loop
// on a tight ALU-heavy loop.
func BenchmarkPipelineTick8Wide(b *testing.B) {
	p := setupBenchPipeline(uint64(b.N), 8)
	b.ResetTimer()
	p.Run()
}

// BenchmarkPipelineTick1Wide benchmarks the single-issue tick loop.
func BenchmarkPipelineTick1Wide(b *testing.B) {
	p := setupBenchPipeline(uint64(b.N), 1)
	b.ResetTimer()
	p.Run()
}

// BenchmarkDecoderDecode benchmarks the instruction decoder.
func BenchmarkDecoderDecode(b *testing.B) {
	d := insts.NewDecoder()
	word := encodeADDImm(2, 3, 42) // ADD X2, X3, #42
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Decode(word)
	}
}

// BenchmarkDecoderDecodeInto benchmarks the allocation-free decoder path.
func BenchmarkDecoderDecodeInto(b *testing.B) {
	d := insts.NewDecoder()
	word := encodeADDImm(2, 3, 42) // ADD X2, X3, #42
	var inst insts.Instruction
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.DecodeInto(word, &inst)
	}
}
