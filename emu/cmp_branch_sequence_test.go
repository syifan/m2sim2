package emu_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
)

// This test file validates the CMP+B.NE pattern that is critical for loops.
// The timing simulator had issues with PSTATE forwarding for this pattern when
// CMP is not in decode slot 0 (where fusion with B.NE fails).
//
// ARM64 semantics:
// - CMP X0, #0  is an alias for SUBS XZR, X0, #0 (sets flags, discards result)
// - B.NE takes the branch when Z flag is clear (i.e., operands were NOT equal)
// - For CMP X0, #0: Z=1 when X0==0, Z=0 when X0!=0
//
// This matches the hot branch benchmark pattern:
//   loop:
//     SUB X0, X0, #1     ; decrement
//     CMP X0, #0         ; compare to zero
//     B.NE loop          ; branch if X0 != 0

var _ = Describe("CMP+B.NE Sequences", func() {
	var (
		regFile    *emu.RegFile
		alu        *emu.ALU
		branchUnit *emu.BranchUnit
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		regFile.PC = 0x1000
		alu = emu.NewALU(regFile)
		branchUnit = emu.NewBranchUnit(regFile)
	})

	Describe("CMP X0, #0 followed by B.NE", func() {
		Context("when X0 is non-zero (should branch)", func() {
			It("should set Z=0 and take the branch", func() {
				regFile.WriteReg(0, 5) // X0 = 5

				// CMP X0, #0 (SUBS XZR, X0, #0)
				alu.SUB64Imm(31, 0, 0, true)

				// Verify PSTATE
				Expect(regFile.PSTATE.Z).To(BeFalse(), "Z flag should be clear when X0 != 0")
				Expect(regFile.PSTATE.N).To(BeFalse(), "N flag should be clear for positive value")
				Expect(regFile.PSTATE.C).To(BeTrue(), "C flag set when no borrow (X0 >= 0)")

				// B.NE -8 (backward branch to simulate loop)
				branchUnit.BCond(-8, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000-8)), "Should have branched backward")
			})

			It("should take branch for X0=1 (loop's last iteration before exit)", func() {
				regFile.WriteReg(0, 1) // X0 = 1

				// CMP X0, #0
				alu.SUB64Imm(31, 0, 0, true)

				Expect(regFile.PSTATE.Z).To(BeFalse())

				// B.NE should branch
				branchUnit.BCond(-8, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 - 8)))
			})

			It("should take branch for large positive value", func() {
				regFile.WriteReg(0, 0x7FFFFFFFFFFFFFFF) // large positive

				alu.SUB64Imm(31, 0, 0, true)

				Expect(regFile.PSTATE.Z).To(BeFalse())
				Expect(regFile.PSTATE.N).To(BeFalse())

				branchUnit.BCond(-8, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 - 8)))
			})

			It("should take branch for negative value (high bit set)", func() {
				regFile.WriteReg(0, 0x8000000000000001) // negative (high bit set)

				alu.SUB64Imm(31, 0, 0, true)

				Expect(regFile.PSTATE.Z).To(BeFalse())
				Expect(regFile.PSTATE.N).To(BeTrue(), "N flag should be set for negative result")

				branchUnit.BCond(-8, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 - 8)))
			})
		})

		Context("when X0 is zero (should NOT branch - loop exit)", func() {
			It("should set Z=1 and NOT take the branch", func() {
				regFile.WriteReg(0, 0) // X0 = 0

				// CMP X0, #0 (SUBS XZR, X0, #0)
				alu.SUB64Imm(31, 0, 0, true)

				// Verify PSTATE
				Expect(regFile.PSTATE.Z).To(BeTrue(), "Z flag should be set when X0 == 0")
				Expect(regFile.PSTATE.N).To(BeFalse(), "N flag should be clear for zero")
				Expect(regFile.PSTATE.C).To(BeTrue(), "C flag set when no borrow")
				Expect(regFile.PSTATE.V).To(BeFalse(), "V flag should be clear")

				// B.NE -8 (should NOT branch because Z=1)
				branchUnit.BCond(-8, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000)), "Should NOT have branched (fall through)")
			})
		})
	})

	Describe("CMP Xn, Xm followed by B.NE", func() {
		Context("when operands are equal", func() {
			It("should NOT branch", func() {
				regFile.WriteReg(0, 42)
				regFile.WriteReg(1, 42)

				// CMP X0, X1
				alu.SUB64(31, 0, 1, true)

				Expect(regFile.PSTATE.Z).To(BeTrue())

				branchUnit.BCond(16, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000)), "Should NOT have branched")
			})
		})

		Context("when operands are different", func() {
			It("should branch when X0 > X1", func() {
				regFile.WriteReg(0, 100)
				regFile.WriteReg(1, 50)

				alu.SUB64(31, 0, 1, true)

				Expect(regFile.PSTATE.Z).To(BeFalse())
				Expect(regFile.PSTATE.N).To(BeFalse())
				Expect(regFile.PSTATE.C).To(BeTrue(), "No borrow when X0 > X1")

				branchUnit.BCond(16, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 16)))
			})

			It("should branch when X0 < X1", func() {
				regFile.WriteReg(0, 50)
				regFile.WriteReg(1, 100)

				alu.SUB64(31, 0, 1, true)

				Expect(regFile.PSTATE.Z).To(BeFalse())
				Expect(regFile.PSTATE.N).To(BeTrue(), "Result is negative when X0 < X1")
				Expect(regFile.PSTATE.C).To(BeFalse(), "Borrow occurred when X0 < X1")

				branchUnit.BCond(16, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 16)))
			})
		})
	})

	Describe("Loop iteration sequence (hot branch pattern)", func() {
		It("should correctly iterate 4 times", func() {
			iterations := 0
			regFile.WriteReg(0, 4) // X0 = 4 (loop counter)
			regFile.PC = 0x1000

			for {
				// SUB X0, X0, #1
				alu.SUB64Imm(0, 0, 1, false)

				// CMP X0, #0
				alu.SUB64Imm(31, 0, 0, true)

				iterations++

				if regFile.PSTATE.Z {
					// B.NE would NOT branch, exit loop
					break
				}

				// B.NE would branch, continue loop
				if iterations > 10 {
					Fail("Loop did not terminate - PSTATE bug detected")
				}
			}

			Expect(iterations).To(Equal(4), "Should have iterated exactly 4 times")
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)), "X0 should be 0 at loop exit")
		})

		It("should handle 16 iterations (original hot benchmark)", func() {
			iterations := 0
			regFile.WriteReg(0, 16)

			for {
				alu.SUB64Imm(0, 0, 1, false)
				alu.SUB64Imm(31, 0, 0, true)

				iterations++

				if regFile.PSTATE.Z {
					break
				}

				if iterations > 20 {
					Fail("Loop did not terminate")
				}
			}

			Expect(iterations).To(Equal(16))
			Expect(regFile.ReadReg(0)).To(Equal(uint64(0)))
		})
	})

	Describe("Edge cases for PSTATE flags", func() {
		It("should handle underflow correctly", func() {
			regFile.WriteReg(0, 0)

			// SUB X0, X0, #1 (0 - 1 = 0xFFFFFFFFFFFFFFFF)
			alu.SUB64Imm(0, 0, 1, true)

			Expect(regFile.ReadReg(0)).To(Equal(uint64(0xFFFFFFFFFFFFFFFF)))
			Expect(regFile.PSTATE.Z).To(BeFalse())
			Expect(regFile.PSTATE.N).To(BeTrue(), "Result is negative (high bit set)")
			Expect(regFile.PSTATE.C).To(BeFalse(), "Borrow occurred (unsigned underflow)")
		})

		It("should handle max value correctly", func() {
			regFile.WriteReg(0, 0xFFFFFFFFFFFFFFFF)

			alu.SUB64Imm(31, 0, 0, true)

			Expect(regFile.PSTATE.Z).To(BeFalse())
			Expect(regFile.PSTATE.N).To(BeTrue())
			Expect(regFile.PSTATE.C).To(BeTrue())
		})
	})

	Describe("32-bit CMP+B.NE", func() {
		It("should work with 32-bit comparisons", func() {
			// Write a 32-bit value (upper 32 bits should be ignored for 32-bit ops)
			regFile.WriteReg(0, 0xFFFFFFFF00000005) // W0 = 5

			// CMP W0, #0 (32-bit comparison)
			// The ALU.SUB32Imm should mask to lower 32 bits
			alu.SUB32Imm(31, 0, 0, true)

			// For 32-bit CMP, we compare the lower 32 bits
			// W0 = 5, so Z should be false
			Expect(regFile.PSTATE.Z).To(BeFalse())
		})

		It("should set Z=1 when W0 is zero", func() {
			regFile.WriteReg(0, 0xFFFFFFFF00000000) // W0 = 0 (upper bits don't matter)

			alu.SUB32Imm(31, 0, 0, true)

			Expect(regFile.PSTATE.Z).To(BeTrue())
		})
	})
})
