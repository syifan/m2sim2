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

	Describe("Load/Store Instructions", func() {
		// LDR X0, [X1]       -> 0xF9400020
		// Encoding: 11 111 0 01 01 imm12=0 Rn=1 Rt=0
		It("should decode LDR X0, [X1]", func() {
			inst := decoder.Decode(0xF9400020)

			Expect(inst.Op).To(Equal(insts.OpLDR))
			Expect(inst.Format).To(Equal(insts.FormatLoadStore))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0))) // Rt
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(0)))
		})

		// LDR X2, [X3, #8]   -> 0xF9400462
		// Encoding: 11 111 0 01 01 imm12=1 Rn=3 Rt=2 (imm12 is scaled by 8)
		It("should decode LDR X2, [X3, #8]", func() {
			inst := decoder.Decode(0xF9400462)

			Expect(inst.Op).To(Equal(insts.OpLDR))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Rn).To(Equal(uint8(3)))
			Expect(inst.Imm).To(Equal(uint64(8)))
		})

		// LDR X4, [X5, #32760] -> 0xF947FC04 (max offset for 64-bit)
		// imm12 = 32760/8 = 4095 = 0xFFF
		It("should decode LDR X4, [X5, #32760]", func() {
			inst := decoder.Decode(0xF97FFCA4)

			Expect(inst.Op).To(Equal(insts.OpLDR))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(4)))
			Expect(inst.Rn).To(Equal(uint8(5)))
			Expect(inst.Imm).To(Equal(uint64(32760)))
		})

		// LDR W0, [X1]       -> 0xB9400020
		// Encoding: 10 111 0 01 01 imm12=0 Rn=1 Rt=0
		It("should decode LDR W0, [X1]", func() {
			inst := decoder.Decode(0xB9400020)

			Expect(inst.Op).To(Equal(insts.OpLDR))
			Expect(inst.Format).To(Equal(insts.FormatLoadStore))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(0)))
		})

		// LDR W2, [X3, #4]   -> 0xB9400462
		// Encoding: 10 111 0 01 01 imm12=1 Rn=3 Rt=2 (imm12 scaled by 4)
		It("should decode LDR W2, [X3, #4]", func() {
			inst := decoder.Decode(0xB9400462)

			Expect(inst.Op).To(Equal(insts.OpLDR))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Rn).To(Equal(uint8(3)))
			Expect(inst.Imm).To(Equal(uint64(4)))
		})

		// STR X0, [X1]       -> 0xF9000020
		// Encoding: 11 111 0 01 00 imm12=0 Rn=1 Rt=0
		It("should decode STR X0, [X1]", func() {
			inst := decoder.Decode(0xF9000020)

			Expect(inst.Op).To(Equal(insts.OpSTR))
			Expect(inst.Format).To(Equal(insts.FormatLoadStore))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0))) // Rt (source register for store)
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(0)))
		})

		// STR X2, [X3, #16]  -> 0xF9000862
		// Encoding: 11 111 0 01 00 imm12=2 Rn=3 Rt=2 (imm12 scaled by 8)
		It("should decode STR X2, [X3, #16]", func() {
			inst := decoder.Decode(0xF9000862)

			Expect(inst.Op).To(Equal(insts.OpSTR))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Rn).To(Equal(uint8(3)))
			Expect(inst.Imm).To(Equal(uint64(16)))
		})

		// STR W0, [X1]       -> 0xB9000020
		// Encoding: 10 111 0 01 00 imm12=0 Rn=1 Rt=0
		It("should decode STR W0, [X1]", func() {
			inst := decoder.Decode(0xB9000020)

			Expect(inst.Op).To(Equal(insts.OpSTR))
			Expect(inst.Format).To(Equal(insts.FormatLoadStore))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(0)))
		})

		// STR W2, [X3, #8]   -> 0xB9000862
		// Encoding: 10 111 0 01 00 imm12=2 Rn=3 Rt=2 (imm12 scaled by 4)
		It("should decode STR W2, [X3, #8]", func() {
			inst := decoder.Decode(0xB9000862)

			Expect(inst.Op).To(Equal(insts.OpSTR))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Rn).To(Equal(uint8(3)))
			Expect(inst.Imm).To(Equal(uint64(8)))
		})

		// LDR X0, [SP, #8]   -> 0xF94007E0
		// Using SP (reg 31) as base
		It("should decode LDR X0, [SP, #8]", func() {
			inst := decoder.Decode(0xF94007E0)

			Expect(inst.Op).To(Equal(insts.OpLDR))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(31))) // SP
			Expect(inst.Imm).To(Equal(uint64(8)))
		})

		// STR X0, [SP, #16]  -> 0xF9000BE0
		It("should decode STR X0, [SP, #16]", func() {
			inst := decoder.Decode(0xF9000BE0)

			Expect(inst.Op).To(Equal(insts.OpSTR))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(31))) // SP
			Expect(inst.Imm).To(Equal(uint64(16)))
		})
	})

	Describe("Exception Generation Instructions", func() {
		// SVC #0             -> 0xD4000001
		// Encoding: 11010100 000 | imm16=0 | 00001
		It("should decode SVC #0", func() {
			inst := decoder.Decode(0xD4000001)

			Expect(inst.Op).To(Equal(insts.OpSVC))
			Expect(inst.Format).To(Equal(insts.FormatException))
			Expect(inst.Imm).To(Equal(uint64(0)))
		})

		// SVC #1             -> 0xD4000021
		// Encoding: 11010100 000 | imm16=1 | 00001
		It("should decode SVC #1", func() {
			inst := decoder.Decode(0xD4000021)

			Expect(inst.Op).To(Equal(insts.OpSVC))
			Expect(inst.Format).To(Equal(insts.FormatException))
			Expect(inst.Imm).To(Equal(uint64(1)))
		})

		// SVC #0xFFFF        -> 0xD41FFFE1
		// Encoding: 11010100 000 | imm16=0xFFFF | 00001
		It("should decode SVC #0xFFFF (max imm16)", func() {
			inst := decoder.Decode(0xD41FFFE1)

			Expect(inst.Op).To(Equal(insts.OpSVC))
			Expect(inst.Format).To(Equal(insts.FormatException))
			Expect(inst.Imm).To(Equal(uint64(0xFFFF)))
		})

		// BRK #0x3e8        -> 0xD4207D00
		// Encoding: 11010100 001 | imm16=0x3e8 | 00000
		// This is the BRK instruction that CoreMark uses for assertions
		It("should decode BRK #0x3e8 (from CoreMark)", func() {
			inst := decoder.Decode(0xD4207D00)

			Expect(inst.Op).To(Equal(insts.OpBRK))
			Expect(inst.Format).To(Equal(insts.FormatException))
			Expect(inst.Imm).To(Equal(uint64(0x3e8)))
		})

		// BRK #0            -> 0xD4200000
		// Encoding: 11010100 001 | imm16=0 | 00000
		It("should decode BRK #0", func() {
			inst := decoder.Decode(0xD4200000)

			Expect(inst.Op).To(Equal(insts.OpBRK))
			Expect(inst.Format).To(Equal(insts.FormatException))
			Expect(inst.Imm).To(Equal(uint64(0)))
		})
	})

	Describe("Unknown Instructions", func() {
		It("should mark unrecognized instructions as unknown", func() {
			// Arbitrary unimplemented encoding
			inst := decoder.Decode(0x00000000)

			Expect(inst.Op).To(Equal(insts.OpUnknown))
		})
	})

	Describe("SIMD Load/Store Instructions", func() {
		// LDR Q0, [X1]       -> 0x3DC00020
		// Encoding: 00 111 1 01 11 imm12=0 Rn=1 Rt=0 (128-bit load)
		It("should decode LDR Q0, [X1] (128-bit vector load)", func() {
			inst := decoder.Decode(0x3DC00020)

			Expect(inst.Op).To(Equal(insts.OpLDRQ))
			Expect(inst.Format).To(Equal(insts.FormatSIMDLoadStore))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
		})

		// STR Q0, [X1]       -> 0x3D800020
		// Encoding: 00 111 1 01 10 imm12=0 Rn=1 Rt=0 (128-bit store)
		It("should decode STR Q0, [X1] (128-bit vector store)", func() {
			inst := decoder.Decode(0x3D800020)

			Expect(inst.Op).To(Equal(insts.OpSTRQ))
			Expect(inst.Format).To(Equal(insts.FormatSIMDLoadStore))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
		})

		// LDR Q2, [X3, #32]  -> 0x3DC00862
		// Encoding with offset (imm12=2, scaled by 16)
		It("should decode LDR Q2, [X3, #32]", func() {
			inst := decoder.Decode(0x3DC00862)

			Expect(inst.Op).To(Equal(insts.OpLDRQ))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Rn).To(Equal(uint8(3)))
			Expect(inst.Imm).To(Equal(uint64(32)))
		})

		// LDR D0, [X1]       -> 0xFD400020
		// 64-bit SIMD load (D register) - decoder sets Arr16B as default
		It("should decode LDR D0, [X1] (64-bit vector load)", func() {
			inst := decoder.Decode(0xFD400020)

			Expect(inst.Op).To(Equal(insts.OpLDRQ))
			Expect(inst.IsSIMD).To(BeTrue())
			// Note: decoder defaults to Arr16B for this encoding
			Expect(inst.Arrangement).To(Equal(insts.Arr16B))
		})

		// LDR S0, [X1]       -> 0xBD400020
		// 32-bit SIMD load (S register)
		It("should decode LDR S0, [X1] (32-bit vector load)", func() {
			inst := decoder.Decode(0xBD400020)

			Expect(inst.Op).To(Equal(insts.OpLDRQ))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Arrangement).To(Equal(insts.Arr2S))
		})
	})

	Describe("SIMD Three Same Instructions", func() {
		// ADD V0.16B, V1.16B, V2.16B -> 0x4E228420
		// Encoding: 0 | Q=1 | U=0 | 01110 | size=00 | 1 | Rm=2 | opcode=10000 | 1 | Rn=1 | Rd=0
		It("should decode VADD V0.16B, V1.16B, V2.16B (128-bit byte add)", func() {
			inst := decoder.Decode(0x4E228420)

			Expect(inst.Op).To(Equal(insts.OpVADD))
			Expect(inst.Format).To(Equal(insts.FormatSIMDReg))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Arrangement).To(Equal(insts.Arr16B))
		})

		// ADD V0.8B, V1.8B, V2.8B  -> 0x0E228420
		// Q=0 for 64-bit
		It("should decode VADD V0.8B, V1.8B, V2.8B (64-bit byte add)", func() {
			inst := decoder.Decode(0x0E228420)

			Expect(inst.Op).To(Equal(insts.OpVADD))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Arrangement).To(Equal(insts.Arr8B))
		})

		// SUB V0.4S, V1.4S, V2.4S  -> 0x6EA28420
		// Encoding: 0 | Q=1 | U=1 | 01110 | size=10 | 1 | Rm=2 | opcode=10000 | 1 | Rn=1 | Rd=0
		It("should decode VSUB V0.4S, V1.4S, V2.4S (128-bit 32-bit sub)", func() {
			inst := decoder.Decode(0x6EA28420)

			Expect(inst.Op).To(Equal(insts.OpVSUB))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
			Expect(inst.Arrangement).To(Equal(insts.Arr4S))
		})

		// MUL V0.4S, V1.4S, V2.4S  -> 0x4EA29C20
		// Encoding: 0 | Q=1 | U=0 | 01110 | size=10 | 1 | Rm=2 | opcode=10011 | 1 | Rn=1 | Rd=0
		It("should decode VMUL V0.4S, V1.4S, V2.4S (128-bit 32-bit mul)", func() {
			inst := decoder.Decode(0x4EA29C20)

			Expect(inst.Op).To(Equal(insts.OpVMUL))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Arrangement).To(Equal(insts.Arr4S))
		})

		// FADD V0.4S, V1.4S, V2.4S -> 0x4EA2D420
		// Encoding: 0 | Q=1 | U=0 | 01110 | size=10 | 1 | Rm=2 | opcode=11010 | 1 | Rn=1 | Rd=0
		It("should decode VFADD V0.4S, V1.4S, V2.4S (floating-point add)", func() {
			inst := decoder.Decode(0x4EA2D420)

			Expect(inst.Op).To(Equal(insts.OpVFADD))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.IsFloat).To(BeTrue())
			Expect(inst.Arrangement).To(Equal(insts.Arr4S))
		})

		// FSUB V0.4S, V1.4S, V2.4S -> 0x6EA2D420
		// U=1 for FSUB
		It("should decode VFSUB V0.4S, V1.4S, V2.4S (floating-point sub)", func() {
			inst := decoder.Decode(0x6EA2D420)

			Expect(inst.Op).To(Equal(insts.OpVFSUB))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.IsFloat).To(BeTrue())
		})

		// FMUL V0.4S, V1.4S, V2.4S -> 0x6EA2DC20
		// opcode=11011, U=1
		It("should decode VFMUL V0.4S, V1.4S, V2.4S (floating-point mul)", func() {
			inst := decoder.Decode(0x6EA2DC20)

			Expect(inst.Op).To(Equal(insts.OpVFMUL))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.IsFloat).To(BeTrue())
		})

		// ADD V0.8H, V1.8H, V2.8H  -> 0x4E628420
		// size=01 for halfword
		It("should decode VADD V0.8H, V1.8H, V2.8H (halfword add)", func() {
			inst := decoder.Decode(0x4E628420)

			Expect(inst.Op).To(Equal(insts.OpVADD))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Arrangement).To(Equal(insts.Arr8H))
		})

		// ADD V0.2D, V1.2D, V2.2D  -> 0x4EE28420
		// size=11 for doubleword
		It("should decode VADD V0.2D, V1.2D, V2.2D (doubleword add)", func() {
			inst := decoder.Decode(0x4EE28420)

			Expect(inst.Op).To(Equal(insts.OpVADD))
			Expect(inst.IsSIMD).To(BeTrue())
			Expect(inst.Arrangement).To(Equal(insts.Arr2D))
		})

		// Unrecognized SIMD opcode should return OpUnknown
		It("should return OpUnknown for unrecognized SIMD opcode", func() {
			// Use opcode=00001 which is not ADD/SUB/MUL/FADD/FSUB/FMUL
			inst := decoder.Decode(0x4E220820) // opcode=00001

			Expect(inst.Op).To(Equal(insts.OpUnknown))
		})
	})

	Describe("PC-Relative Addressing (ADR, ADRP)", func() {
		// ADRP X0, 0x93000 (from CoreMark startup)
		// Encoding: 1 | immlo | 10000 | immhi | Rd
		// Example: f0000080 -> ADRP X0, offset
		It("should decode ADRP X0, #offset", func() {
			// ADRP X0, with a specific page offset
			// Encoding: 10010000 00000000 00000000 10000000 = 0x90000080
			inst := decoder.Decode(0x90000080)

			Expect(inst.Op).To(Equal(insts.OpADRP))
			Expect(inst.Format).To(Equal(insts.FormatPCRel))
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Is64Bit).To(BeTrue())
		})

		// ADR X1, #8
		// Encoding: 0 | immlo | 10000 | immhi | Rd
		// ADR with small offset
		It("should decode ADR X1, #offset", func() {
			// ADR X1, #8 -> 0x10000041
			inst := decoder.Decode(0x10000041)

			Expect(inst.Op).To(Equal(insts.OpADR))
			Expect(inst.Format).To(Equal(insts.FormatPCRel))
			Expect(inst.Rd).To(Equal(uint8(1)))
			Expect(inst.Is64Bit).To(BeTrue())
		})
	})

	Describe("Load Literal (PC-relative)", func() {
		// LDR X0, label (PC-relative)
		// Encoding: opc | 011 | V | 00 | imm19 | Rt
		// 64-bit: opc=01
		It("should decode LDR X0, label (64-bit literal)", func() {
			// LDR X0, #offset -> 0x58000000 (with imm19=0)
			inst := decoder.Decode(0x58000000)

			Expect(inst.Op).To(Equal(insts.OpLDRLit))
			Expect(inst.Format).To(Equal(insts.FormatLoadStoreLit))
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.IsSIMD).To(BeFalse())
		})

		// LDR W0, label (32-bit)
		// opc=00
		It("should decode LDR W0, label (32-bit literal)", func() {
			// LDR W0, #offset -> 0x18000000
			inst := decoder.Decode(0x18000000)

			Expect(inst.Op).To(Equal(insts.OpLDRLit))
			Expect(inst.Format).To(Equal(insts.FormatLoadStoreLit))
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Is64Bit).To(BeFalse())
		})

		// LDR with positive offset
		It("should decode LDR with positive offset", func() {
			// LDR X5, #16 -> 0x58000085 (imm19 = 4, offset = 16)
			inst := decoder.Decode(0x58000085)

			Expect(inst.Op).To(Equal(insts.OpLDRLit))
			Expect(inst.Rd).To(Equal(uint8(5)))
			Expect(inst.BranchOffset).To(Equal(int64(16)))
		})
	})

	Describe("Move Wide Instructions (MOVZ, MOVN, MOVK)", func() {
		// MOVZ X0, #0x1234
		// Encoding: sf | opc | 100101 | hw | imm16 | Rd
		// MOVZ: opc=10, sf=1 for 64-bit
		It("should decode MOVZ X0, #0x1234", func() {
			// MOVZ X0, #0x1234 -> 0xD2824680
			inst := decoder.Decode(0xD2824680)

			Expect(inst.Op).To(Equal(insts.OpMOVZ))
			Expect(inst.Format).To(Equal(insts.FormatMoveWide))
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Imm).To(Equal(uint64(0x1234)))
			Expect(inst.Shift).To(Equal(uint8(0)))
		})

		// MOVZ X1, #0xABCD, LSL #16
		// hw=1 means shift by 16
		It("should decode MOVZ X1, #0xABCD, LSL #16", func() {
			// MOVZ X1, #0xABCD, LSL #16 -> 0xD2B579A1
			// sf=1, opc=10, hw=01, imm16=0xABCD, Rd=1
			inst := decoder.Decode(0xD2B579A1)

			Expect(inst.Op).To(Equal(insts.OpMOVZ))
			Expect(inst.Rd).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(0xABCD)))
			Expect(inst.Shift).To(Equal(uint8(16)))
		})

		// MOVK X0, #0x5678, LSL #32
		// opc=11 for MOVK, hw=2
		It("should decode MOVK X0, #0x5678, LSL #32", func() {
			// MOVK X0, #0x5678, LSL #32 -> 0xF2CACF00
			inst := decoder.Decode(0xF2CACF00)

			Expect(inst.Op).To(Equal(insts.OpMOVK))
			Expect(inst.Format).To(Equal(insts.FormatMoveWide))
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Imm).To(Equal(uint64(0x5678)))
			Expect(inst.Shift).To(Equal(uint8(32)))
		})

		// MOVN X2, #0xFF
		// opc=00 for MOVN
		It("should decode MOVN X2, #0xFF", func() {
			// MOVN X2, #0xFF -> 0x92801FE2
			inst := decoder.Decode(0x92801FE2)

			Expect(inst.Op).To(Equal(insts.OpMOVN))
			Expect(inst.Format).To(Equal(insts.FormatMoveWide))
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Imm).To(Equal(uint64(0xFF)))
		})

		// MOVZ W0, #100 (32-bit)
		// sf=0
		It("should decode MOVZ W0, #100 (32-bit)", func() {
			// MOVZ W0, #100 -> 0x52800C80
			inst := decoder.Decode(0x52800C80)

			Expect(inst.Op).To(Equal(insts.OpMOVZ))
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Imm).To(Equal(uint64(100)))
		})
	})

	Describe("Conditional Select Instructions", func() {
		// CSEL X0, X1, X2, EQ -> 0x9A820020
		// Format: sf=1, op=0, S=0, 11010100, Rm=2, cond=0000 (EQ), op2=00, Rn=1, Rd=0
		It("should decode CSEL X0, X1, X2, EQ", func() {
			inst := decoder.Decode(0x9A820020)

			Expect(inst.Op).To(Equal(insts.OpCSEL))
			Expect(inst.Format).To(Equal(insts.FormatCondSelect))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
			Expect(inst.Cond).To(Equal(insts.CondEQ))
		})

		// CSINC X3, X4, X5, NE -> 0x9A851483
		// Format: sf=1, op=0, S=0, 11010100, Rm=5, cond=0001 (NE), op2=01, Rn=4, Rd=3
		It("should decode CSINC X3, X4, X5, NE", func() {
			inst := decoder.Decode(0x9A851483)

			Expect(inst.Op).To(Equal(insts.OpCSINC))
			Expect(inst.Format).To(Equal(insts.FormatCondSelect))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(3)))
			Expect(inst.Rn).To(Equal(uint8(4)))
			Expect(inst.Rm).To(Equal(uint8(5)))
			Expect(inst.Cond).To(Equal(insts.CondNE))
		})

		// CSINV X6, X7, X8, GE -> 0xDA88A0E6
		// Format: sf=1, op=1, S=0, 11010100, Rm=8, cond=1010 (GE), op2=00, Rn=7, Rd=6
		It("should decode CSINV X6, X7, X8, GE", func() {
			inst := decoder.Decode(0xDA88A0E6)

			Expect(inst.Op).To(Equal(insts.OpCSINV))
			Expect(inst.Format).To(Equal(insts.FormatCondSelect))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(6)))
			Expect(inst.Rn).To(Equal(uint8(7)))
			Expect(inst.Rm).To(Equal(uint8(8)))
			Expect(inst.Cond).To(Equal(insts.CondGE))
		})

		// CSNEG X9, X10, X11, LT -> 0xDA8BB549
		// Format: sf=1, op=1, S=0, 11010100, Rm=11, cond=1011 (LT), op2=01, Rn=10, Rd=9
		It("should decode CSNEG X9, X10, X11, LT", func() {
			inst := decoder.Decode(0xDA8BB549)

			Expect(inst.Op).To(Equal(insts.OpCSNEG))
			Expect(inst.Format).To(Equal(insts.FormatCondSelect))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(9)))
			Expect(inst.Rn).To(Equal(uint8(10)))
			Expect(inst.Rm).To(Equal(uint8(11)))
			Expect(inst.Cond).To(Equal(insts.CondLT))
		})

		// CSEL W0, W1, W2, EQ -> 0x1A820020
		// 32-bit version
		It("should decode CSEL W0, W1, W2, EQ (32-bit)", func() {
			inst := decoder.Decode(0x1A820020)

			Expect(inst.Op).To(Equal(insts.OpCSEL))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
		})
	})

	Describe("Division Instructions", func() {
		// UDIV X0, X1, X2 -> 0x9AC20820
		// Format: sf=1, 0, S=0, 11010110, Rm=2, opcode=000010, Rn=1, Rd=0
		It("should decode UDIV X0, X1, X2", func() {
			inst := decoder.Decode(0x9AC20820)

			Expect(inst.Op).To(Equal(insts.OpUDIV))
			Expect(inst.Format).To(Equal(insts.FormatDataProc2Src))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
		})

		// SDIV X3, X4, X5 -> 0x9AC50C83
		// Format: sf=1, 0, S=0, 11010110, Rm=5, opcode=000011, Rn=4, Rd=3
		It("should decode SDIV X3, X4, X5", func() {
			inst := decoder.Decode(0x9AC50C83)

			Expect(inst.Op).To(Equal(insts.OpSDIV))
			Expect(inst.Format).To(Equal(insts.FormatDataProc2Src))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(3)))
			Expect(inst.Rn).To(Equal(uint8(4)))
			Expect(inst.Rm).To(Equal(uint8(5)))
		})

		// UDIV W0, W1, W2 -> 0x1AC20820
		// 32-bit version
		It("should decode UDIV W0, W1, W2 (32-bit)", func() {
			inst := decoder.Decode(0x1AC20820)

			Expect(inst.Op).To(Equal(insts.OpUDIV))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
		})
	})

	Describe("Multiply-Add Instructions", func() {
		// MADD X0, X1, X2, X3 -> 0x9B020C20
		// Format: sf=1, op54=00, 11011, op31=000, Rm=2, o0=0, Ra=3, Rn=1, Rd=0
		It("should decode MADD X0, X1, X2, X3", func() {
			inst := decoder.Decode(0x9B020C20)

			Expect(inst.Op).To(Equal(insts.OpMADD))
			Expect(inst.Format).To(Equal(insts.FormatDataProc3Src))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
			Expect(inst.Rt2).To(Equal(uint8(3))) // Ra reuses Rt2 field
		})

		// MSUB X4, X5, X6, X7 -> 0x9B069CA4
		// Format: sf=1, op54=00, 11011, op31=000, Rm=6, o0=1, Ra=7, Rn=5, Rd=4
		It("should decode MSUB X4, X5, X6, X7", func() {
			inst := decoder.Decode(0x9B069CA4)

			Expect(inst.Op).To(Equal(insts.OpMSUB))
			Expect(inst.Format).To(Equal(insts.FormatDataProc3Src))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(4)))
			Expect(inst.Rn).To(Equal(uint8(5)))
			Expect(inst.Rm).To(Equal(uint8(6)))
			Expect(inst.Rt2).To(Equal(uint8(7)))
		})

		// MUL X0, X1, X2 (alias: MADD with Ra=XZR)
		// MADD X0, X1, X2, XZR -> 0x9B027C20
		It("should decode MUL X0, X1, X2 (MADD alias)", func() {
			inst := decoder.Decode(0x9B027C20)

			Expect(inst.Op).To(Equal(insts.OpMADD))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Rm).To(Equal(uint8(2)))
			Expect(inst.Rt2).To(Equal(uint8(31))) // XZR
		})
	})

	Describe("Test and Branch Instructions", func() {
		// TBZ X0, #5, .+8 -> 0x36280020
		// Format: b5=0, 011011, op=0, b40=00101, imm14=2, Rt=0
		It("should decode TBZ X0, #5, .+8", func() {
			inst := decoder.Decode(0x36280040)

			Expect(inst.Op).To(Equal(insts.OpTBZ))
			Expect(inst.Format).To(Equal(insts.FormatTestBranch))
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.Imm).To(Equal(uint64(5))) // bit number
			Expect(inst.BranchOffset).To(Equal(int64(8)))
		})

		// TBNZ X1, #63, .-4 -> 0xB7FFFFE1
		// Format: b5=1, 011011, op=1, b40=11111, imm14=-1, Rt=1
		It("should decode TBNZ X1, #63, .-4", func() {
			inst := decoder.Decode(0xB7FFFFE1)

			Expect(inst.Op).To(Equal(insts.OpTBNZ))
			Expect(inst.Format).To(Equal(insts.FormatTestBranch))
			Expect(inst.Rd).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(63))) // bit number
			Expect(inst.BranchOffset).To(Equal(int64(-4)))
			Expect(inst.Is64Bit).To(BeTrue()) // b5=1 means 64-bit
		})

		// TBZ W2, #0, .+16 -> 0x36000082
		// 32-bit test
		It("should decode TBZ W2, #0, .+16", func() {
			inst := decoder.Decode(0x36000082)

			Expect(inst.Op).To(Equal(insts.OpTBZ))
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.Imm).To(Equal(uint64(0)))
			Expect(inst.BranchOffset).To(Equal(int64(16)))
			Expect(inst.Is64Bit).To(BeFalse())
		})
	})

	Describe("Compare and Branch Instructions", func() {
		// CBZ X0, .+8 -> 0xB4000040
		// Format: sf=1, 011010, op=0, imm19=2, Rt=0
		It("should decode CBZ X0, .+8", func() {
			inst := decoder.Decode(0xB4000040)

			Expect(inst.Op).To(Equal(insts.OpCBZ))
			Expect(inst.Format).To(Equal(insts.FormatCompareBranch))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(0)))
			Expect(inst.BranchOffset).To(Equal(int64(8)))
		})

		// CBNZ X1, .-4 -> 0xB5FFFFE1
		// Format: sf=1, 011010, op=1, imm19=-1, Rt=1
		It("should decode CBNZ X1, .-4", func() {
			inst := decoder.Decode(0xB5FFFFE1)

			Expect(inst.Op).To(Equal(insts.OpCBNZ))
			Expect(inst.Format).To(Equal(insts.FormatCompareBranch))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rd).To(Equal(uint8(1)))
			Expect(inst.BranchOffset).To(Equal(int64(-4)))
		})

		// CBZ W2, .+16 -> 0x34000082
		// 32-bit version
		It("should decode CBZ W2, .+16 (32-bit)", func() {
			inst := decoder.Decode(0x34000082)

			Expect(inst.Op).To(Equal(insts.OpCBZ))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(2)))
			Expect(inst.BranchOffset).To(Equal(int64(16)))
		})

		// CBNZ W3, .+100 -> 0x35000323
		// imm19 = 100/4 = 25 = 0x19, Rt=3
		It("should decode CBNZ W3, .+100 (32-bit)", func() {
			inst := decoder.Decode(0x35000323)

			Expect(inst.Op).To(Equal(insts.OpCBNZ))
			Expect(inst.Is64Bit).To(BeFalse())
			Expect(inst.Rd).To(Equal(uint8(3)))
			Expect(inst.BranchOffset).To(Equal(int64(100)))
		})
	})

	Describe("Logical Immediate Instructions", func() {
		// ANDS X1, X1, #0xffffffffffff -> 0xf240bc21
		// This is the instruction that was blocking CoreMark
		// sf=1, opc=11, 100100, N=0, immr=0, imms=47, Rn=1, Rd=1
		It("should decode ANDS X1, X1, #0xffffffffffff", func() {
			inst := decoder.Decode(0xf240bc21)

			Expect(inst.Op).To(Equal(insts.OpAND))
			Expect(inst.Format).To(Equal(insts.FormatLogicalImm))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeTrue()) // ANDS sets flags
			Expect(inst.Rd).To(Equal(uint8(1)))
			Expect(inst.Rn).To(Equal(uint8(1)))
			Expect(inst.Imm).To(Equal(uint64(0xffffffffffff)))
		})

		// AND X0, X1, #0xff -> 0x9240181f (or similar)
		// sf=1, opc=00, 100100, N=0, immr=0, imms=7, Rn=1, Rd=0
		It("should decode AND with 8-bit mask", func() {
			inst := decoder.Decode(0x92401c20) // AND X0, X1, #0xff

			Expect(inst.Op).To(Equal(insts.OpAND))
			Expect(inst.Format).To(Equal(insts.FormatLogicalImm))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.SetFlags).To(BeFalse())
		})

		// ORR X0, XZR, #1 -> MOV X0, #1 (this is common)
		// sf=1, opc=01, 100100, N=1, immr=0, imms=0, Rn=31, Rd=0
		It("should decode ORR (immediate) as MOV pattern", func() {
			inst := decoder.Decode(0xb2400000) // ORR X0, XZR, #1

			Expect(inst.Op).To(Equal(insts.OpORR))
			Expect(inst.Format).To(Equal(insts.FormatLogicalImm))
			Expect(inst.Is64Bit).To(BeTrue())
			Expect(inst.Rn).To(Equal(uint8(0)))
			Expect(inst.Rd).To(Equal(uint8(0)))
		})

		// EOR X2, X3, #0xffffffff00000000
		// sf=1, opc=10, 100100, N=1, immr=32, imms=31, Rn=3, Rd=2
		It("should decode EOR (immediate)", func() {
			inst := decoder.Decode(0xd2607c62) // EOR X2, X3, #mask

			Expect(inst.Op).To(Equal(insts.OpEOR))
			Expect(inst.Format).To(Equal(insts.FormatLogicalImm))
			Expect(inst.Is64Bit).To(BeTrue())
		})
	})
})
