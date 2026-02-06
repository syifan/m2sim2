package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

// Helper to create a conditional branch IDEX register
func makeCondBranch(cond insts.Cond) *pipeline.IDEXRegister {
	return &pipeline.IDEXRegister{
		Valid: true,
		PC:    0x1000,
		Inst: &insts.Instruction{
			Op:           insts.OpBCond,
			Format:       insts.FormatBranchCond,
			BranchOffset: 40,
			Cond:         cond,
		},
	}
}

var _ = Describe("Branch Conditions", func() {
	var (
		regFile      *emu.RegFile
		executeStage *pipeline.ExecuteStage
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		executeStage = pipeline.NewExecuteStage(regFile)
	})

	Describe("CondEQ (Equal / Z=1)", func() {
		It("should branch when Z=1", func() {
			regFile.PSTATE.Z = true
			result := executeStage.Execute(makeCondBranch(insts.CondEQ), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when Z=0", func() {
			regFile.PSTATE.Z = false
			result := executeStage.Execute(makeCondBranch(insts.CondEQ), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondNE (Not Equal / Z=0)", func() {
		It("should branch when Z=0", func() {
			regFile.PSTATE.Z = false
			result := executeStage.Execute(makeCondBranch(insts.CondNE), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when Z=1", func() {
			regFile.PSTATE.Z = true
			result := executeStage.Execute(makeCondBranch(insts.CondNE), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondCS (Carry Set / C=1)", func() {
		It("should branch when C=1", func() {
			regFile.PSTATE.C = true
			result := executeStage.Execute(makeCondBranch(insts.CondCS), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when C=0", func() {
			regFile.PSTATE.C = false
			result := executeStage.Execute(makeCondBranch(insts.CondCS), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondCC (Carry Clear / C=0)", func() {
		It("should branch when C=0", func() {
			regFile.PSTATE.C = false
			result := executeStage.Execute(makeCondBranch(insts.CondCC), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when C=1", func() {
			regFile.PSTATE.C = true
			result := executeStage.Execute(makeCondBranch(insts.CondCC), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondMI (Minus / N=1)", func() {
		It("should branch when N=1", func() {
			regFile.PSTATE.N = true
			result := executeStage.Execute(makeCondBranch(insts.CondMI), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when N=0", func() {
			regFile.PSTATE.N = false
			result := executeStage.Execute(makeCondBranch(insts.CondMI), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondPL (Plus / N=0)", func() {
		It("should branch when N=0", func() {
			regFile.PSTATE.N = false
			result := executeStage.Execute(makeCondBranch(insts.CondPL), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when N=1", func() {
			regFile.PSTATE.N = true
			result := executeStage.Execute(makeCondBranch(insts.CondPL), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondVS (Overflow Set / V=1)", func() {
		It("should branch when V=1", func() {
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondVS), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when V=0", func() {
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondVS), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondVC (Overflow Clear / V=0)", func() {
		It("should branch when V=0", func() {
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondVC), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when V=1", func() {
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondVC), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondHI (Unsigned Higher / C=1 && Z=0)", func() {
		It("should branch when C=1 and Z=0", func() {
			regFile.PSTATE.C = true
			regFile.PSTATE.Z = false
			result := executeStage.Execute(makeCondBranch(insts.CondHI), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when C=0", func() {
			regFile.PSTATE.C = false
			regFile.PSTATE.Z = false
			result := executeStage.Execute(makeCondBranch(insts.CondHI), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})

		It("should not branch when Z=1", func() {
			regFile.PSTATE.C = true
			regFile.PSTATE.Z = true
			result := executeStage.Execute(makeCondBranch(insts.CondHI), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondLS (Unsigned Lower or Same / C=0 || Z=1)", func() {
		It("should branch when C=0", func() {
			regFile.PSTATE.C = false
			regFile.PSTATE.Z = false
			result := executeStage.Execute(makeCondBranch(insts.CondLS), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should branch when Z=1", func() {
			regFile.PSTATE.C = true
			regFile.PSTATE.Z = true
			result := executeStage.Execute(makeCondBranch(insts.CondLS), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when C=1 and Z=0", func() {
			regFile.PSTATE.C = true
			regFile.PSTATE.Z = false
			result := executeStage.Execute(makeCondBranch(insts.CondLS), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondGE (Signed Greater or Equal / N==V)", func() {
		It("should branch when N=0 and V=0", func() {
			regFile.PSTATE.N = false
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondGE), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should branch when N=1 and V=1", func() {
			regFile.PSTATE.N = true
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondGE), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when N=0 and V=1", func() {
			regFile.PSTATE.N = false
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondGE), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})

		It("should not branch when N=1 and V=0", func() {
			regFile.PSTATE.N = true
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondGE), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondLT (Signed Less Than / N!=V)", func() {
		It("should branch when N=0 and V=1", func() {
			regFile.PSTATE.N = false
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondLT), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should branch when N=1 and V=0", func() {
			regFile.PSTATE.N = true
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondLT), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when N=0 and V=0", func() {
			regFile.PSTATE.N = false
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondLT), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})

		It("should not branch when N=1 and V=1", func() {
			regFile.PSTATE.N = true
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondLT), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondGT (Signed Greater Than / Z=0 && N==V)", func() {
		It("should branch when Z=0 and N=V=0", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = false
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondGT), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should branch when Z=0 and N=V=1", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = true
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondGT), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when Z=1", func() {
			regFile.PSTATE.Z = true
			regFile.PSTATE.N = false
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondGT), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})

		It("should not branch when N!=V", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = true
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondGT), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondLE (Signed Less or Equal / Z=1 || N!=V)", func() {
		It("should branch when Z=1", func() {
			regFile.PSTATE.Z = true
			regFile.PSTATE.N = false
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondLE), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should branch when N!=V (N=1,V=0)", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = true
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondLE), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should branch when N!=V (N=0,V=1)", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = false
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondLE), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should not branch when Z=0 and N==V", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = true
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondLE), 0, 0)
			Expect(result.BranchTaken).To(BeFalse())
		})
	})

	Describe("CondAL (Always)", func() {
		It("should always branch regardless of flags", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = false
			regFile.PSTATE.C = false
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondAL), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})

		It("should branch with all flags set", func() {
			regFile.PSTATE.Z = true
			regFile.PSTATE.N = true
			regFile.PSTATE.C = true
			regFile.PSTATE.V = true
			result := executeStage.Execute(makeCondBranch(insts.CondAL), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})
	})

	Describe("CondNV (Never - treated as always in AArch64)", func() {
		It("should branch (NV is AL in AArch64)", func() {
			regFile.PSTATE.Z = false
			regFile.PSTATE.N = false
			regFile.PSTATE.C = false
			regFile.PSTATE.V = false
			result := executeStage.Execute(makeCondBranch(insts.CondNV), 0, 0)
			Expect(result.BranchTaken).To(BeTrue())
		})
	})
})
