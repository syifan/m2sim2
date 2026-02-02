package insts_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/insts"
)

var _ = Describe("Decoder", func() {
	var decoder *insts.Decoder

	BeforeEach(func() {
		decoder = insts.NewDecoder()
	})

	Describe("Data Processing (Immediate) - Add/Sub", func() {
		// ADD X0, X1, #42    -> 0x9100A820
		// Encoding: sf=1, op=0, S=0, 100010, sh=0, imm12=42, Rn=1, Rd=0
		It("should decode ADD X0, X1, #42", func() {
			inst := decoder.Decode(0x9100A820)

			Expect(inst.Op).To(Equal(insts.OpADD))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(42)))
			Expect(inst.Format).To(Equal(insts.FormatDPImm))
		})

		// ADD W0, W1, #100   -> 0x11019020
		// Encoding: sf=0, op=0, S=0, 100010, sh=0, imm12=100, Rn=1, Rd=0
		It("should decode ADD W0, W1, #100", func() {
			inst := decoder.Decode(0x11019020)

			Expect(inst.Op).To(Equal(insts.OpADD))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(100)))
			Expect(inst.Format).To(Equal(insts.FormatDPImm))
		})

		// ADDS X2, X3, #10   -> 0xB1002862
		// Encoding: sf=1, op=0, S=1, 100010, sh=0, imm12=10, Rn=3, Rd=2
		It("should decode ADDS X2, X3, #10", func() {
			inst := decoder.Decode(0xB1002862)

			Expect(inst.Op).To(Equal(insts.OpADD))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Rn).To(Equal(uint8(3)))
			Expect(inst.Imm).To(Equal(uint64(10)))
		})

		// ADD X0, X1, #1, LSL #12 -> 0x91400420
		// Encoding: sf=1, op=0, S=0, 100010, sh=1, imm12=1, Rn=1, Rd=0
		It("should decode ADD X0, X1, #1, LSL #12", func() {
			inst := decoder.Decode(0x91400420)

			Expect(inst.Op).To(Equal(insts.OpADD))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(1)))
			Expect(inst.Shift).To(Equal(uint8(12)))
		})

		// SUB X5, X6, #20    -> 0xD10050C5
		// Encoding: sf=1, op=1, S=0, 100010, sh=0, imm12=20, Rn=6, Rd=5
		It("should decode SUB X5, X6, #20", func() {
			inst := decoder.Decode(0xD10050C5)

			Expect(inst.Op).To(Equal(insts.OpSUB))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(5)))
			Expect(inst.Rn).To(Equal(uint8(6)))
			Expect(inst.Imm).To(Equal(uint64(20)))
		})

		// SUB W7, W8, #50    -> 0x5100C907
		// Encoding: sf=0, op=1, S=0, 100010, sh=0, imm12=50, Rn=8, Rd=7
		It("should decode SUB W7, W8, #50", func() {
			inst := decoder.Decode(0x5100C907)

			Expect(inst.Op).To(Equal(insts.OpSUB))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(7)))
			Expect(inst.Rn).To(Equal(uint8(8)))
			Expect(inst.Imm).To(Equal(uint64(50)))
		})

		// SUBS X9, X10, #5   -> 0xF1001549
		// Encoding: sf=1, op=1, S=1, 100010, sh=0, imm12=5, Rn=10, Rd=9
		It("should decode SUBS X9, X10, #5", func() {
			inst := decoder.Decode(0xF1001549)

			Expect(inst.Op).To(Equal(insts.OpSUB))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(9)))
			Expect(inst.Rn).To(Equal(uint8(10)))
			Expect(inst.Imm).To(Equal(uint64(5)))
		})
	})

	Describe("Data Processing (Register) - Add/Sub", func() {
		// ADD X0, X1, X2     -> 0x8B020020
		// Encoding: sf=1, op=0, S=0, 01011, shift=00, 0, Rm=2, imm6=0, Rn=1, Rd=0
		It("should decode ADD X0, X1, X2", func() {
			inst := decoder.Decode(0x8B020020)

			Expect(inst.Op).To(Equal(insts.OpADD))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
			Expect(inst.Format).To(Equal(insts.FormatDPReg))
		})

		// ADD W3, W4, W5     -> 0x0B050083
		// Encoding: sf=0, op=0, S=0, 01011, shift=00, 0, Rm=5, imm6=0, Rn=4, Rd=3
		It("should decode ADD W3, W4, W5", func() {
			inst := decoder.Decode(0x0B050083)

			Expect(inst.Op).To(Equal(insts.OpADD))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(3)))
			Expect(inst.Rn).To(Equal(uint8(4)))
			Expect(inst.Rm).To(Equal(uint8(5)))
		})

		// ADDS X6, X7, X8    -> 0xAB0800E6
		// Encoding: sf=1, op=0, S=1, 01011, shift=00, 0, Rm=8, imm6=0, Rn=7, Rd=6
		It("should decode ADDS X6, X7, X8", func() {
			inst := decoder.Decode(0xAB0800E6)

			Expect(inst.Op).To(Equal(insts.OpADD))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(6)))
			Expect(inst.Rn).To(Equal(uint8(7)))
			Expect(inst.Rm).To(Equal(uint8(8)))
		})

		// SUB X9, X10, X11   -> 0xCB0B0149
		// Encoding: sf=1, op=1, S=0, 01011, shift=00, 0, Rm=11, imm6=0, Rn=10, Rd=9
		It("should decode SUB X9, X10, X11", func() {
			inst := decoder.Decode(0xCB0B0149)

			Expect(inst.Op).To(Equal(insts.OpSUB))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(9)))
			Expect(inst.Rn).To(Equal(uint8(10)))
			Expect(inst.Rm).To(Equal(uint8(11)))
		})

		// SUB W12, W13, W14  -> 0x4B0E01AC
		// Encoding: sf=0, op=1, S=0, 01011, shift=00, 0, Rm=14, imm6=0, Rn=13, Rd=12
		It("should decode SUB W12, W13, W14", func() {
			inst := decoder.Decode(0x4B0E01AC)

			Expect(inst.Op).To(Equal(insts.OpSUB))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(12)))
			Expect(inst.Rn).To(Equal(uint8(13)))
			Expect(inst.Rm).To(Equal(uint8(14)))
		})

		// SUBS X15, X16, X17 -> 0xEB11020F
		// Encoding: sf=1, op=1, S=1, 01011, shift=00, 0, Rm=17, imm6=0, Rn=16, Rd=15
		It("should decode SUBS X15, X16, X17", func() {
			inst := decoder.Decode(0xEB11020F)

			Expect(inst.Op).To(Equal(insts.OpSUB))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(15)))
			Expect(inst.Rn).To(Equal(uint8(16)))
			Expect(inst.Rm).To(Equal(uint8(17)))
		})
	})

	Describe("Data Processing (Register) - Logical", func() {
		// AND X0, X1, X2     -> 0x8A020020
		// Encoding: sf=1, opc=00, 01010, shift=00, N=0, Rm=2, imm6=0, Rn=1, Rd=0
		It("should decode AND X0, X1, X2", func() {
			inst := decoder.Decode(0x8A020020)

			Expect(inst.Op).To(Equal(insts.OpAND))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
			Expect(inst.Format).To(Equal(insts.FormatDPReg))
		})

		// AND W3, W4, W5     -> 0x0A050083
		// Encoding: sf=0, opc=00, 01010, shift=00, N=0, Rm=5, imm6=0, Rn=4, Rd=3
		It("should decode AND W3, W4, W5", func() {
			inst := decoder.Decode(0x0A050083)

			Expect(inst.Op).To(Equal(insts.OpAND))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(3)))
			Expect(inst.Rn).To(Equal(uint8(4)))
			Expect(inst.Rm).To(Equal(uint8(5)))
		})

		// ANDS X6, X7, X8    -> 0xEA0800E6
		// Encoding: sf=1, opc=11, 01010, shift=00, N=0, Rm=8, imm6=0, Rn=7, Rd=6
		It("should decode ANDS X6, X7, X8", func() {
			inst := decoder.Decode(0xEA0800E6)

			Expect(inst.Op).To(Equal(insts.OpAND))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(6)))
			Expect(inst.Rn).To(Equal(uint8(7)))
			Expect(inst.Rm).To(Equal(uint8(8)))
		})

		// ORR X9, X10, X11   -> 0xAA0B0149
		// Encoding: sf=1, opc=01, 01010, shift=00, N=0, Rm=11, imm6=0, Rn=10, Rd=9
		It("should decode ORR X9, X10, X11", func() {
			inst := decoder.Decode(0xAA0B0149)

			Expect(inst.Op).To(Equal(insts.OpORR))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(9)))
			Expect(inst.Rn).To(Equal(uint8(10)))
			Expect(inst.Rm).To(Equal(uint8(11)))
		})

		// ORR W12, W13, W14  -> 0x2A0E01AC
		// Encoding: sf=0, opc=01, 01010, shift=00, N=0, Rm=14, imm6=0, Rn=13, Rd=12
		It("should decode ORR W12, W13, W14", func() {
			inst := decoder.Decode(0x2A0E01AC)

			Expect(inst.Op).To(Equal(insts.OpORR))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(12)))
			Expect(inst.Rn).To(Equal(uint8(13)))
			Expect(inst.Rm).To(Equal(uint8(14)))
		})

		// EOR X15, X16, X17  -> 0xCA11020F
		// Encoding: sf=1, opc=10, 01010, shift=00, N=0, Rm=17, imm6=0, Rn=16, Rd=15
		It("should decode EOR X15, X16, X17", func() {
			inst := decoder.Decode(0xCA11020F)

			Expect(inst.Op).To(Equal(insts.OpEOR))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(15)))
			Expect(inst.Rn).To(Equal(uint8(16)))
			Expect(inst.Rm).To(Equal(uint8(17)))
		})

		// EOR W18, W19, W20  -> 0x4A140272
		// Encoding: sf=0, opc=10, 01010, shift=00, N=0, Rm=20, imm6=0, Rn=19, Rd=18
		It("should decode EOR W18, W19, W20", func() {
			inst := decoder.Decode(0x4A140272)

			Expect(inst.Op).To(Equal(insts.OpEOR))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.SetFlags).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(18)))
			Expect(inst.Rn).To(Equal(uint8(19)))
			Expect(inst.Rm).To(Equal(uint8(20)))
		})
	})

	Describe("Branch Instructions", func() {
		// B #0x100           -> 0x14000040
		// Encoding: 000101, imm26=0x40 (64 instructions = 256 bytes)
		It("should decode B #0x100", func() {
			inst := decoder.Decode(0x14000040)

			Expect(inst.Op).To(Equal(insts.OpB))
			Expect(inst.Format).To(Equal(insts.FormatBranch))
			Expect(inst.Imm).To(Equal(uint64(0x100)))
		})

		// B #-0x8            -> 0x17FFFFFE
		// Encoding: 000101, imm26=-2 (signed)
		It("should decode B #-0x8 (backward branch)", func() {
			inst := decoder.Decode(0x17FFFFFE)

			Expect(inst.Op).To(Equal(insts.OpB))
			Expect(inst.Format).To(Equal(insts.FormatBranch))
			// Signed offset: -8 bytes
			Expect(inst.BranchOffset).To(Equal(int64(-8)))
		})

		// BL #0x200          -> 0x94000080
		// Encoding: 100101, imm26=0x80 (128 instructions = 512 bytes)
		It("should decode BL #0x200", func() {
			inst := decoder.Decode(0x94000080)

			Expect(inst.Op).To(Equal(insts.OpBL))
			Expect(inst.Format).To(Equal(insts.FormatBranch))
			Expect(inst.Imm).To(Equal(uint64(0x200)))
		})

		// B.EQ #0x10         -> 0x54000080
		// Encoding: 01010100, imm19=4 (4 instructions = 16 bytes), 0, cond=0000 (EQ)
		It("should decode B.EQ #0x10", func() {
			inst := decoder.Decode(0x54000080)

			Expect(inst.Op).To(Equal(insts.OpBCond))
			Expect(inst.Format).To(Equal(insts.FormatBranchCond))
			Expect(inst.Cond).To(Equal(insts.CondEQ))
			Expect(inst.Imm).To(Equal(uint64(0x10)))
		})

		// B.NE #0x20         -> 0x54000101
		// Encoding: 01010100, imm19=8 (8 instructions = 32 bytes), 0, cond=0001 (NE)
		It("should decode B.NE #0x20", func() {
			inst := decoder.Decode(0x54000101)

			Expect(inst.Op).To(Equal(insts.OpBCond))
			Expect(inst.Format).To(Equal(insts.FormatBranchCond))
			Expect(inst.Cond).To(Equal(insts.CondNE))
			Expect(inst.Imm).To(Equal(uint64(0x20)))
		})

		// B.LT #0x40         -> 0x5400020B
		// Encoding: 01010100, imm19=16 (16 instructions = 64 bytes), 0, cond=1011 (LT)
		It("should decode B.LT #0x40", func() {
			inst := decoder.Decode(0x5400020B)

			Expect(inst.Op).To(Equal(insts.OpBCond))
			Expect(inst.Format).To(Equal(insts.FormatBranchCond))
			Expect(inst.Cond).To(Equal(insts.CondLT))
			Expect(inst.Imm).To(Equal(uint64(0x40)))
		})

		// BR X30             -> 0xD61F03C0
		// Encoding: 1101011 0 0 00 11111 0000 0 0 Rn=30 00000
		It("should decode BR X30", func() {
			inst := decoder.Decode(0xD61F03C0)

			Expect(inst.Op).To(Equal(insts.OpBR))
			Expect(inst.Format).To(Equal(insts.FormatBranchReg))
			Expect(inst.Rn).To(Equal(uint8(30)))
		})

		// BLR X10            -> 0xD63F0140
		// Encoding: 1101011 0 0 01 11111 0000 0 0 Rn=10 00000
		It("should decode BLR X10", func() {
			inst := decoder.Decode(0xD63F0140)

			Expect(inst.Op).To(Equal(insts.OpBLR))
			Expect(inst.Format).To(Equal(insts.FormatBranchReg))
			Expect(inst.Rn).To(Equal(uint8(10)))
		})

		// RET (X30)          -> 0xD65F03C0
		// Encoding: 1101011 0 0 10 11111 0000 0 0 Rn=30 00000
		It("should decode RET", func() {
			inst := decoder.Decode(0xD65F03C0)

			Expect(inst.Op).To(Equal(insts.OpRET))
			Expect(inst.Format).To(Equal(insts.FormatBranchReg))
			Expect(inst.Rn).To(Equal(uint8(30)))
		})
	})

	Describe("Unknown Instructions", func() {
		It("should mark unrecognized instructions as unknown", func() {
			// Arbitrary unimplemented encoding
			inst := decoder.Decode(0x00000000)

			Expect(inst.Op).To(Equal(insts.OpUnknown))
		})
	})
})
