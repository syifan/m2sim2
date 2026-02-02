package emu_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
)

var _ = Describe("BranchUnit", func() {
	var (
		regFile    *emu.RegFile
		branchUnit *emu.BranchUnit
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		regFile.PC = 0x1000 // Start at address 0x1000
		branchUnit = emu.NewBranchUnit(regFile)
	})

	Describe("B (unconditional branch)", func() {
		It("should branch forward", func() {
			branchUnit.B(100) // offset = 100 bytes

			Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
		})

		It("should branch backward", func() {
			branchUnit.B(-100) // offset = -100 bytes

			Expect(regFile.PC).To(Equal(uint64(0x1000 - 100)))
		})

		It("should branch to zero offset (effectively no-op)", func() {
			branchUnit.B(0)

			Expect(regFile.PC).To(Equal(uint64(0x1000)))
		})

		It("should handle large positive offset", func() {
			branchUnit.B(0x7FFFFFC) // max 26-bit signed positive * 4

			Expect(regFile.PC).To(Equal(uint64(0x1000 + 0x7FFFFFC)))
		})

		It("should handle large negative offset", func() {
			regFile.PC = 0x10000000  // Start at higher address
			branchUnit.B(-0x8000000) // max 26-bit signed negative * 4

			Expect(regFile.PC).To(Equal(uint64(0x10000000 - 0x8000000)))
		})
	})

	Describe("BL (branch with link)", func() {
		It("should branch and save return address to X30", func() {
			branchUnit.BL(200)

			Expect(regFile.PC).To(Equal(uint64(0x1000 + 200)))
			Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1000 + 4))) // return address = PC + 4
		})

		It("should branch backward and save return address", func() {
			branchUnit.BL(-200)

			Expect(regFile.PC).To(Equal(uint64(0x1000 - 200)))
			Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1000 + 4)))
		})

		It("should overwrite previous X30 value", func() {
			regFile.WriteReg(30, 0xDEADBEEF)

			branchUnit.BL(100)

			Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1000 + 4)))
		})
	})

	Describe("BR (branch to register)", func() {
		It("should branch to address in register", func() {
			regFile.WriteReg(5, 0x2000)

			branchUnit.BR(5)

			Expect(regFile.PC).To(Equal(uint64(0x2000)))
		})

		It("should branch to X0", func() {
			regFile.WriteReg(0, 0x3000)

			branchUnit.BR(0)

			Expect(regFile.PC).To(Equal(uint64(0x3000)))
		})

		It("should branch to X30 (LR)", func() {
			regFile.WriteReg(30, 0x4000)

			branchUnit.BR(30)

			Expect(regFile.PC).To(Equal(uint64(0x4000)))
		})

		It("should handle zero address", func() {
			regFile.WriteReg(1, 0)

			branchUnit.BR(1)

			Expect(regFile.PC).To(Equal(uint64(0)))
		})
	})

	Describe("BLR (branch with link to register)", func() {
		It("should branch to register and save return address", func() {
			regFile.WriteReg(5, 0x5000)

			branchUnit.BLR(5)

			Expect(regFile.PC).To(Equal(uint64(0x5000)))
			Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1000 + 4)))
		})

		It("should handle self-referential call (BLR X30)", func() {
			regFile.WriteReg(30, 0x6000)

			branchUnit.BLR(30)

			// PC should be old X30 value
			Expect(regFile.PC).To(Equal(uint64(0x6000)))
			// X30 should be return address
			Expect(regFile.ReadReg(30)).To(Equal(uint64(0x1000 + 4)))
		})
	})

	Describe("RET (return from subroutine)", func() {
		It("should return to address in X30", func() {
			regFile.WriteReg(30, 0x7000)

			branchUnit.RET(30)

			Expect(regFile.PC).To(Equal(uint64(0x7000)))
		})

		It("should return to address in specified register", func() {
			regFile.WriteReg(5, 0x8000)

			branchUnit.RET(5)

			Expect(regFile.PC).To(Equal(uint64(0x8000)))
		})
	})

	Describe("B.cond (conditional branch)", func() {
		Describe("EQ - Equal (Z == 1)", func() {
			It("should branch when Z flag is set", func() {
				regFile.PSTATE.Z = true

				branchUnit.BCond(100, emu.CondEQ)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when Z flag is clear", func() {
				regFile.PSTATE.Z = false

				branchUnit.BCond(100, emu.CondEQ)

				Expect(regFile.PC).To(Equal(uint64(0x1000))) // PC unchanged
			})
		})

		Describe("NE - Not Equal (Z == 0)", func() {
			It("should branch when Z flag is clear", func() {
				regFile.PSTATE.Z = false

				branchUnit.BCond(100, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when Z flag is set", func() {
				regFile.PSTATE.Z = true

				branchUnit.BCond(100, emu.CondNE)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("CS/HS - Carry Set (C == 1)", func() {
			It("should branch when C flag is set", func() {
				regFile.PSTATE.C = true

				branchUnit.BCond(100, emu.CondCS)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when C flag is clear", func() {
				regFile.PSTATE.C = false

				branchUnit.BCond(100, emu.CondCS)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("CC/LO - Carry Clear (C == 0)", func() {
			It("should branch when C flag is clear", func() {
				regFile.PSTATE.C = false

				branchUnit.BCond(100, emu.CondCC)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when C flag is set", func() {
				regFile.PSTATE.C = true

				branchUnit.BCond(100, emu.CondCC)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("MI - Minus/Negative (N == 1)", func() {
			It("should branch when N flag is set", func() {
				regFile.PSTATE.N = true

				branchUnit.BCond(100, emu.CondMI)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when N flag is clear", func() {
				regFile.PSTATE.N = false

				branchUnit.BCond(100, emu.CondMI)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("PL - Plus/Positive (N == 0)", func() {
			It("should branch when N flag is clear", func() {
				regFile.PSTATE.N = false

				branchUnit.BCond(100, emu.CondPL)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when N flag is set", func() {
				regFile.PSTATE.N = true

				branchUnit.BCond(100, emu.CondPL)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("VS - Overflow Set (V == 1)", func() {
			It("should branch when V flag is set", func() {
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondVS)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when V flag is clear", func() {
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondVS)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("VC - Overflow Clear (V == 0)", func() {
			It("should branch when V flag is clear", func() {
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondVC)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when V flag is set", func() {
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondVC)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("HI - Unsigned Higher (C == 1 && Z == 0)", func() {
			It("should branch when C is set and Z is clear", func() {
				regFile.PSTATE.C = true
				regFile.PSTATE.Z = false

				branchUnit.BCond(100, emu.CondHI)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when C is clear", func() {
				regFile.PSTATE.C = false
				regFile.PSTATE.Z = false

				branchUnit.BCond(100, emu.CondHI)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})

			It("should not branch when Z is set", func() {
				regFile.PSTATE.C = true
				regFile.PSTATE.Z = true

				branchUnit.BCond(100, emu.CondHI)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("LS - Unsigned Lower or Same (C == 0 || Z == 1)", func() {
			It("should branch when C is clear", func() {
				regFile.PSTATE.C = false
				regFile.PSTATE.Z = false

				branchUnit.BCond(100, emu.CondLS)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should branch when Z is set", func() {
				regFile.PSTATE.C = true
				regFile.PSTATE.Z = true

				branchUnit.BCond(100, emu.CondLS)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when C is set and Z is clear", func() {
				regFile.PSTATE.C = true
				regFile.PSTATE.Z = false

				branchUnit.BCond(100, emu.CondLS)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("GE - Signed Greater or Equal (N == V)", func() {
			It("should branch when N and V are both clear", func() {
				regFile.PSTATE.N = false
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondGE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should branch when N and V are both set", func() {
				regFile.PSTATE.N = true
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondGE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when N != V (N set, V clear)", func() {
				regFile.PSTATE.N = true
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondGE)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})

			It("should not branch when N != V (N clear, V set)", func() {
				regFile.PSTATE.N = false
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondGE)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("LT - Signed Less Than (N != V)", func() {
			It("should branch when N != V (N set, V clear)", func() {
				regFile.PSTATE.N = true
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondLT)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should branch when N != V (N clear, V set)", func() {
				regFile.PSTATE.N = false
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondLT)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when N == V (both clear)", func() {
				regFile.PSTATE.N = false
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondLT)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})

			It("should not branch when N == V (both set)", func() {
				regFile.PSTATE.N = true
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondLT)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("GT - Signed Greater Than (Z == 0 && N == V)", func() {
			It("should branch when Z clear and N == V (both clear)", func() {
				regFile.PSTATE.Z = false
				regFile.PSTATE.N = false
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondGT)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should branch when Z clear and N == V (both set)", func() {
				regFile.PSTATE.Z = false
				regFile.PSTATE.N = true
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondGT)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when Z is set", func() {
				regFile.PSTATE.Z = true
				regFile.PSTATE.N = false
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondGT)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})

			It("should not branch when N != V", func() {
				regFile.PSTATE.Z = false
				regFile.PSTATE.N = true
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondGT)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("LE - Signed Less Than or Equal (Z == 1 || N != V)", func() {
			It("should branch when Z is set", func() {
				regFile.PSTATE.Z = true
				regFile.PSTATE.N = false
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondLE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should branch when N != V (N set, V clear)", func() {
				regFile.PSTATE.Z = false
				regFile.PSTATE.N = true
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondLE)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should not branch when Z clear and N == V", func() {
				regFile.PSTATE.Z = false
				regFile.PSTATE.N = false
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondLE)

				Expect(regFile.PC).To(Equal(uint64(0x1000)))
			})
		})

		Describe("AL - Always (unconditional)", func() {
			It("should always branch regardless of flags", func() {
				regFile.PSTATE.N = false
				regFile.PSTATE.Z = false
				regFile.PSTATE.C = false
				regFile.PSTATE.V = false

				branchUnit.BCond(100, emu.CondAL)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})

			It("should always branch with all flags set", func() {
				regFile.PSTATE.N = true
				regFile.PSTATE.Z = true
				regFile.PSTATE.C = true
				regFile.PSTATE.V = true

				branchUnit.BCond(100, emu.CondAL)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})
		})

		Describe("NV - Never (reserved, behaves like AL)", func() {
			It("should always branch (same as AL)", func() {
				branchUnit.BCond(100, emu.CondNV)

				Expect(regFile.PC).To(Equal(uint64(0x1000 + 100)))
			})
		})

		Describe("backward conditional branch", func() {
			It("should branch backward when condition is met", func() {
				regFile.PSTATE.Z = true

				branchUnit.BCond(-100, emu.CondEQ)

				Expect(regFile.PC).To(Equal(uint64(0x1000 - 100)))
			})
		})
	})

	Describe("CheckCondition", func() {
		It("should return true for EQ when Z is set", func() {
			regFile.PSTATE.Z = true

			Expect(branchUnit.CheckCondition(emu.CondEQ)).To(BeTrue())
		})

		It("should return false for EQ when Z is clear", func() {
			regFile.PSTATE.Z = false

			Expect(branchUnit.CheckCondition(emu.CondEQ)).To(BeFalse())
		})
	})
})
