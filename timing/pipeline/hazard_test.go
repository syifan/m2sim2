package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var _ = Describe("HazardUnit", func() {
	var hazardUnit *pipeline.HazardUnit

	BeforeEach(func() {
		hazardUnit = pipeline.NewHazardUnit()
	})

	Describe("DetectForwarding", func() {
		var idex *pipeline.IDEXRegister
		var exmem *pipeline.EXMEMRegister
		var memwb *pipeline.MEMWBRegister

		BeforeEach(func() {
			idex = &pipeline.IDEXRegister{Valid: true, Rn: 1, Rm: 2}
			exmem = &pipeline.EXMEMRegister{}
			memwb = &pipeline.MEMWBRegister{}
		})

		Context("when no forwarding is needed", func() {
			It("should return ForwardNone for both operands", func() {
				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
				Expect(result.ForwardRm).To(Equal(pipeline.ForwardNone))
			})
		})

		Context("when forwarding from EX/MEM is needed", func() {
			It("should forward Rn from EX/MEM", func() {
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 1 // Same as Rn in ID/EX

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardFromEXMEM))
				Expect(result.ForwardRm).To(Equal(pipeline.ForwardNone))
			})

			It("should forward Rm from EX/MEM", func() {
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 2 // Same as Rm in ID/EX

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
				Expect(result.ForwardRm).To(Equal(pipeline.ForwardFromEXMEM))
			})

			It("should forward both operands from EX/MEM", func() {
				idex.Rn = 3
				idex.Rm = 3
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 3

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardFromEXMEM))
				Expect(result.ForwardRm).To(Equal(pipeline.ForwardFromEXMEM))
			})
		})

		Context("when forwarding from MEM/WB is needed", func() {
			It("should forward Rn from MEM/WB", func() {
				memwb.Valid = true
				memwb.RegWrite = true
				memwb.Rd = 1 // Same as Rn in ID/EX

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardFromMEMWB))
			})

			It("should forward Rm from MEM/WB", func() {
				memwb.Valid = true
				memwb.RegWrite = true
				memwb.Rd = 2 // Same as Rm in ID/EX

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRm).To(Equal(pipeline.ForwardFromMEMWB))
			})
		})

		Context("priority: EX/MEM over MEM/WB", func() {
			It("should prioritize EX/MEM when both match", func() {
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 1

				memwb.Valid = true
				memwb.RegWrite = true
				memwb.Rd = 1

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardFromEXMEM))
			})
		})

		Context("XZR handling", func() {
			It("should not forward when Rn is XZR (31)", func() {
				idex.Rn = 31
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 31

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
			})

			It("should not forward when destination is XZR", func() {
				idex.Rn = 5
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 31 // Writing to XZR

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
			})
		})

		Context("store data forwarding (ForwardRd)", func() {
			It("should not forward Rd when not a store instruction", func() {
				idex.Rd = 3
				idex.MemWrite = false // Not a store
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 3

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRd).To(Equal(pipeline.ForwardNone))
			})

			It("should forward Rd from EX/MEM for store instruction", func() {
				// ADD X1, X2, X3; STR X1, [X4]
				// X1 needs forwarding from ADD (in EXMEM) to STR (in IDEX)
				idex.Rd = 1
				idex.MemWrite = true // STR instruction
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 1 // ADD wrote to X1

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRd).To(Equal(pipeline.ForwardFromEXMEM))
			})

			It("should forward Rd from MEM/WB for store instruction", func() {
				// ADD X1, X2, X3; NOP; STR X1, [X4]
				// X1 needs forwarding from ADD (in MEMWB) to STR (in IDEX)
				idex.Rd = 1
				idex.MemWrite = true
				memwb.Valid = true
				memwb.RegWrite = true
				memwb.Rd = 1

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRd).To(Equal(pipeline.ForwardFromMEMWB))
			})

			It("should not forward Rd when store data register is XZR", func() {
				idex.Rd = 31 // XZR
				idex.MemWrite = true
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 31

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRd).To(Equal(pipeline.ForwardNone))
			})
		})

		Context("invalid pipeline registers", func() {
			It("should not forward when ID/EX is invalid", func() {
				idex.Valid = false
				exmem.Valid = true
				exmem.RegWrite = true
				exmem.Rd = 1

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
			})

			It("should not forward when EX/MEM is invalid", func() {
				exmem.Valid = false
				exmem.RegWrite = true
				exmem.Rd = 1

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
			})

			It("should not forward when EX/MEM RegWrite is false", func() {
				exmem.Valid = true
				exmem.RegWrite = false
				exmem.Rd = 1

				result := hazardUnit.DetectForwarding(idex, exmem, memwb)

				Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
			})
		})
	})

	Describe("DetectLoadUseHazardDecoded", func() {
		Context("when there is no load-use hazard", func() {
			It("should return false when load destination is XZR", func() {
				result := hazardUnit.DetectLoadUseHazardDecoded(31, 1, 2, true, true)
				Expect(result).To(BeFalse())
			})

			It("should return false when no registers match", func() {
				result := hazardUnit.DetectLoadUseHazardDecoded(5, 1, 2, true, true)
				Expect(result).To(BeFalse())
			})

			It("should return false when next instruction doesn't use Rn", func() {
				result := hazardUnit.DetectLoadUseHazardDecoded(1, 1, 2, false, true)
				Expect(result).To(BeFalse())
			})
		})

		Context("when there is a load-use hazard", func() {
			It("should detect hazard when Rn matches load destination", func() {
				result := hazardUnit.DetectLoadUseHazardDecoded(5, 5, 2, true, true)
				Expect(result).To(BeTrue())
			})

			It("should detect hazard when Rm matches load destination", func() {
				result := hazardUnit.DetectLoadUseHazardDecoded(5, 1, 5, true, true)
				Expect(result).To(BeTrue())
			})

			It("should detect hazard when both Rn and Rm match", func() {
				result := hazardUnit.DetectLoadUseHazardDecoded(5, 5, 5, true, true)
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("GetForwardedValue", func() {
		var exmem *pipeline.EXMEMRegister
		var memwb *pipeline.MEMWBRegister

		BeforeEach(func() {
			exmem = &pipeline.EXMEMRegister{
				Valid:     true,
				ALUResult: 100,
			}
			memwb = &pipeline.MEMWBRegister{
				Valid:     true,
				ALUResult: 200,
				MemData:   300,
				MemToReg:  false,
			}
		})

		It("should return original value for ForwardNone", func() {
			result := hazardUnit.GetForwardedValue(pipeline.ForwardNone, 42, exmem, memwb)
			Expect(result).To(Equal(uint64(42)))
		})

		It("should return ALU result for ForwardFromEXMEM", func() {
			result := hazardUnit.GetForwardedValue(pipeline.ForwardFromEXMEM, 42, exmem, memwb)
			Expect(result).To(Equal(uint64(100)))
		})

		It("should return ALU result for ForwardFromMEMWB when not MemToReg", func() {
			result := hazardUnit.GetForwardedValue(pipeline.ForwardFromMEMWB, 42, exmem, memwb)
			Expect(result).To(Equal(uint64(200)))
		})

		It("should return MemData for ForwardFromMEMWB when MemToReg is true", func() {
			memwb.MemToReg = true
			result := hazardUnit.GetForwardedValue(pipeline.ForwardFromMEMWB, 42, exmem, memwb)
			Expect(result).To(Equal(uint64(300)))
		})
	})

	Describe("ComputeStalls", func() {
		Context("with no hazards", func() {
			It("should not stall or flush", func() {
				result := hazardUnit.ComputeStalls(false, false)

				Expect(result.StallIF).To(BeFalse())
				Expect(result.StallID).To(BeFalse())
				Expect(result.InsertBubbleEX).To(BeFalse())
				Expect(result.FlushIF).To(BeFalse())
				Expect(result.FlushID).To(BeFalse())
			})
		})

		Context("with load-use hazard", func() {
			It("should stall IF and ID and insert bubble", func() {
				result := hazardUnit.ComputeStalls(true, false)

				Expect(result.StallIF).To(BeTrue())
				Expect(result.StallID).To(BeTrue())
				Expect(result.InsertBubbleEX).To(BeTrue())
			})
		})

		Context("with branch taken", func() {
			It("should flush IF and ID", func() {
				result := hazardUnit.ComputeStalls(false, true)

				Expect(result.FlushIF).To(BeTrue())
				Expect(result.FlushID).To(BeTrue())
			})
		})

		Context("with both hazards", func() {
			It("should handle both stall and flush", func() {
				result := hazardUnit.ComputeStalls(true, true)

				Expect(result.StallIF).To(BeTrue())
				Expect(result.StallID).To(BeTrue())
				Expect(result.InsertBubbleEX).To(BeTrue())
				Expect(result.FlushIF).To(BeTrue())
				Expect(result.FlushID).To(BeTrue())
			})
		})
	})

	Describe("ForwardingSource constants", func() {
		It("should have distinct values", func() {
			Expect(pipeline.ForwardNone).To(Equal(pipeline.ForwardingSource(0)))
			Expect(pipeline.ForwardFromEXMEM).To(Equal(pipeline.ForwardingSource(1)))
			Expect(pipeline.ForwardFromMEMWB).To(Equal(pipeline.ForwardingSource(2)))
		})
	})
})

var _ = Describe("Hazard Detection Integration", func() {
	var hazardUnit *pipeline.HazardUnit

	BeforeEach(func() {
		hazardUnit = pipeline.NewHazardUnit()
	})

	Context("RAW hazard scenarios", func() {
		It("should detect ADD followed by SUB using ADD result", func() {
			// Simulating: ADD X1, X2, X3; SUB X4, X1, X5
			// X1 is written by ADD and read by SUB

			idex := &pipeline.IDEXRegister{
				Valid: true,
				Inst:  &insts.Instruction{Op: insts.OpSUB},
				Rn:    1, // Uses X1 (result of ADD)
				Rm:    5,
			}

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				Inst:      &insts.Instruction{Op: insts.OpADD},
				Rd:        1, // Writes X1
				RegWrite:  true,
				ALUResult: 100,
			}

			memwb := &pipeline.MEMWBRegister{}

			result := hazardUnit.DetectForwarding(idex, exmem, memwb)
			Expect(result.ForwardRn).To(Equal(pipeline.ForwardFromEXMEM))
		})

		It("should detect LDR followed by ADD using loaded value", func() {
			// LDR X1, [X2]; ADD X3, X1, X4
			// This is a load-use hazard - need to stall

			// Instruction in ID/EX is LDR (just executed decode)
			idex := &pipeline.IDEXRegister{
				Valid:   true,
				MemRead: true, // LDR
				Rd:      1,
			}

			// The next instruction (being decoded) uses X1
			// Check if there's a load-use hazard
			hazard := hazardUnit.DetectLoadUseHazardDecoded(
				idex.Rd, // load destination
				1,       // next inst Rn
				4,       // next inst Rm
				true,    // usesRn
				true,    // usesRm
			)

			Expect(hazard).To(BeTrue())
		})
	})

	Context("No hazard scenarios", func() {
		It("should not detect hazard for independent instructions", func() {
			// ADD X1, X2, X3; SUB X5, X6, X7 (no dependency)

			idex := &pipeline.IDEXRegister{
				Valid: true,
				Rn:    6,
				Rm:    7,
			}

			exmem := &pipeline.EXMEMRegister{
				Valid:    true,
				Rd:       1,
				RegWrite: true,
			}

			memwb := &pipeline.MEMWBRegister{}

			result := hazardUnit.DetectForwarding(idex, exmem, memwb)
			Expect(result.ForwardRn).To(Equal(pipeline.ForwardNone))
			Expect(result.ForwardRm).To(Equal(pipeline.ForwardNone))
		})
	})

	Context("Store data forwarding scenarios", func() {
		It("should detect ADD followed by STR using ADD result", func() {
			// ADD X1, X2, X3; STR X1, [X4]
			// X1 is written by ADD and stored by STR

			idex := &pipeline.IDEXRegister{
				Valid:    true,
				Inst:     &insts.Instruction{Op: insts.OpSTR},
				Rd:       1, // Store data from X1
				Rn:       4, // Base address X4
				MemWrite: true,
			}

			exmem := &pipeline.EXMEMRegister{
				Valid:     true,
				Inst:      &insts.Instruction{Op: insts.OpADD},
				Rd:        1, // Writes X1
				RegWrite:  true,
				ALUResult: 42, // ADD result
			}

			memwb := &pipeline.MEMWBRegister{}

			result := hazardUnit.DetectForwarding(idex, exmem, memwb)
			Expect(result.ForwardRd).To(Equal(pipeline.ForwardFromEXMEM))

			// Verify we can get the correct forwarded value
			forwardedValue := hazardUnit.GetForwardedValue(
				result.ForwardRd, 0, exmem, memwb)
			Expect(forwardedValue).To(Equal(uint64(42)))
		})

		It("should detect LDR followed by STR using loaded value", func() {
			// LDR X1, [X2]; NOP; STR X1, [X3]
			// X1 is loaded by LDR (now in MEMWB) and stored by STR

			idex := &pipeline.IDEXRegister{
				Valid:    true,
				Inst:     &insts.Instruction{Op: insts.OpSTR},
				Rd:       1, // Store data from X1
				Rn:       3, // Base address X3
				MemWrite: true,
			}

			exmem := &pipeline.EXMEMRegister{}

			memwb := &pipeline.MEMWBRegister{
				Valid:    true,
				Inst:     &insts.Instruction{Op: insts.OpLDR},
				Rd:       1, // Loads into X1
				RegWrite: true,
				MemToReg: true,
				MemData:  100, // Loaded value
			}

			result := hazardUnit.DetectForwarding(idex, exmem, memwb)
			Expect(result.ForwardRd).To(Equal(pipeline.ForwardFromMEMWB))

			// Verify we can get the correct forwarded value (MemData for loads)
			forwardedValue := hazardUnit.GetForwardedValue(
				result.ForwardRd, 0, exmem, memwb)
			Expect(forwardedValue).To(Equal(uint64(100)))
		})
	})
})
