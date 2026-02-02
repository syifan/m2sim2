package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
	"github.com/sarchlab/m2sim/insts"
	"github.com/sarchlab/m2sim/timing/pipeline"
)

var _ = Describe("Pipeline Stages", func() {
	var (
		regFile *emu.RegFile
		memory  *emu.Memory
	)

	BeforeEach(func() {
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
	})

	Describe("FetchStage", func() {
		var fetchStage *pipeline.FetchStage

		BeforeEach(func() {
			fetchStage = pipeline.NewFetchStage(memory)
		})

		It("should fetch instruction from memory", func() {
			// Write an ADD instruction to memory at address 0x1000
			// ADD X0, X1, #10 => 0x91002820
			memory.Write32(0x1000, 0x91002820)

			word, ok := fetchStage.Fetch(0x1000)

			Expect(ok).To(BeTrue())
			Expect(word).To(Equal(uint32(0x91002820)))
		})

		It("should fetch sequential instructions", func() {
			memory.Write32(0x1000, 0x91002820) // ADD X0, X1, #10
			memory.Write32(0x1004, 0xCB020020) // SUB X0, X1, X2

			word1, ok1 := fetchStage.Fetch(0x1000)
			word2, ok2 := fetchStage.Fetch(0x1004)

			Expect(ok1).To(BeTrue())
			Expect(ok2).To(BeTrue())
			Expect(word1).To(Equal(uint32(0x91002820)))
			Expect(word2).To(Equal(uint32(0xCB020020)))
		})

		It("should handle zero address", func() {
			memory.Write32(0x0, 0x12345678)

			word, ok := fetchStage.Fetch(0x0)

			Expect(ok).To(BeTrue())
			Expect(word).To(Equal(uint32(0x12345678)))
		})
	})

	Describe("DecodeStage", func() {
		var decodeStage *pipeline.DecodeStage

		BeforeEach(func() {
			decodeStage = pipeline.NewDecodeStage(regFile)
			// Set up some register values
			regFile.WriteReg(1, 100)
			regFile.WriteReg(2, 50)
			regFile.WriteReg(30, 0x2000) // Link register for RET
		})

		Context("Data Processing Immediate", func() {
			It("should decode ADD immediate and read registers", func() {
				// ADD X0, X1, #10 => sf=1, op=0, S=0, shift=0, imm12=10, Rn=1, Rd=0
				word := uint32(0x91002820)

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpADD))
				Expect(result.Inst.Format).To(Equal(insts.FormatDPImm))
				Expect(result.Rd).To(Equal(uint8(0)))
				Expect(result.Rn).To(Equal(uint8(1)))
				Expect(result.RnValue).To(Equal(uint64(100)))
				Expect(result.RegWrite).To(BeTrue())
				Expect(result.MemRead).To(BeFalse())
				Expect(result.MemWrite).To(BeFalse())
			})

			It("should decode SUB immediate", func() {
				// SUB X0, X1, #10 => sf=1, op=1, S=0, shift=0, imm12=10, Rn=1, Rd=0
				word := uint32(0xD1002820)

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpSUB))
				Expect(result.RegWrite).To(BeTrue())
			})
		})

		Context("Data Processing Register", func() {
			It("should decode ADD register and read both operands", func() {
				// ADD X0, X1, X2 => 0x8B020020
				word := uint32(0x8B020020)

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpADD))
				Expect(result.Inst.Format).To(Equal(insts.FormatDPReg))
				Expect(result.Rd).To(Equal(uint8(0)))
				Expect(result.Rn).To(Equal(uint8(1)))
				Expect(result.Rm).To(Equal(uint8(2)))
				Expect(result.RnValue).To(Equal(uint64(100)))
				Expect(result.RmValue).To(Equal(uint64(50)))
				Expect(result.RegWrite).To(BeTrue())
			})
		})

		Context("Load/Store", func() {
			It("should decode LDR and set control signals", func() {
				// LDR X0, [X1, #8] => 0xF9400420
				word := uint32(0xF9400420)

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpLDR))
				Expect(result.Inst.Format).To(Equal(insts.FormatLoadStore))
				Expect(result.MemRead).To(BeTrue())
				Expect(result.MemToReg).To(BeTrue())
				Expect(result.RegWrite).To(BeTrue())
				Expect(result.MemWrite).To(BeFalse())
			})

			It("should decode STR and set control signals", func() {
				// STR X0, [X1, #8] => 0xF9000420
				word := uint32(0xF9000420)

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpSTR))
				Expect(result.MemWrite).To(BeTrue())
				Expect(result.MemRead).To(BeFalse())
				Expect(result.RegWrite).To(BeFalse())
			})
		})

		Context("Branch", func() {
			It("should decode B (unconditional branch)", func() {
				// B .+100 (offset 100 bytes = 25 words)
				// B imm26: op=0, imm26=25
				word := uint32(0x14000019) // B #100

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpB))
				Expect(result.Inst.Format).To(Equal(insts.FormatBranch))
				Expect(result.IsBranch).To(BeTrue())
				Expect(result.RegWrite).To(BeFalse())
			})

			It("should decode BL and set link register write", func() {
				// BL .+100
				word := uint32(0x94000019) // BL #100

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpBL))
				Expect(result.IsBranch).To(BeTrue())
				Expect(result.RegWrite).To(BeTrue())
				Expect(result.Rd).To(Equal(uint8(30))) // Link register
			})

			It("should decode conditional branch", func() {
				// B.EQ .+20
				word := uint32(0x540000A0) // B.EQ #20

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpBCond))
				Expect(result.Inst.Format).To(Equal(insts.FormatBranchCond))
				Expect(result.IsBranch).To(BeTrue())
			})

			It("should decode RET", func() {
				// RET (defaults to X30)
				word := uint32(0xD65F03C0)

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpRET))
				Expect(result.Inst.Format).To(Equal(insts.FormatBranchReg))
				Expect(result.IsBranch).To(BeTrue())
				Expect(result.Rn).To(Equal(uint8(30))) // X30
				Expect(result.RnValue).To(Equal(uint64(0x2000)))
			})
		})

		Context("Exception", func() {
			It("should decode SVC", func() {
				// SVC #0 => 0xD4000001
				word := uint32(0xD4000001)

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Inst.Op).To(Equal(insts.OpSVC))
				Expect(result.Inst.Format).To(Equal(insts.FormatException))
				Expect(result.IsSyscall).To(BeTrue())
			})
		})

		Context("XZR handling", func() {
			It("should not set RegWrite when destination is XZR", func() {
				// ADD XZR, X1, #10 (discard result)
				// This encodes as Rd=31
				word := uint32(0x9100283F) // ADD XZR, X1, #10

				result := decodeStage.Decode(word, 0x1000)

				Expect(result.Rd).To(Equal(uint8(31)))
				Expect(result.RegWrite).To(BeFalse())
			})
		})
	})

	Describe("ExecuteStage", func() {
		var executeStage *pipeline.ExecuteStage

		BeforeEach(func() {
			executeStage = pipeline.NewExecuteStage(regFile)
		})

		Context("ALU operations", func() {
			It("should execute ADD immediate (64-bit)", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:      insts.OpADD,
						Format:  insts.FormatDPImm,
						Is64Bit: true,
						Imm:     10,
					},
					RnValue: 100,
				}

				result := executeStage.Execute(idex, idex.RnValue, 0)

				Expect(result.ALUResult).To(Equal(uint64(110)))
				Expect(result.BranchTaken).To(BeFalse())
			})

			It("should execute ADD register (64-bit)", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:      insts.OpADD,
						Format:  insts.FormatDPReg,
						Is64Bit: true,
					},
					RnValue: 100,
					RmValue: 50,
				}

				result := executeStage.Execute(idex, idex.RnValue, idex.RmValue)

				Expect(result.ALUResult).To(Equal(uint64(150)))
			})

			It("should execute SUB (64-bit)", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:      insts.OpSUB,
						Format:  insts.FormatDPReg,
						Is64Bit: true,
					},
					RnValue: 100,
					RmValue: 30,
				}

				result := executeStage.Execute(idex, idex.RnValue, idex.RmValue)

				Expect(result.ALUResult).To(Equal(uint64(70)))
			})

			It("should execute AND (64-bit)", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:      insts.OpAND,
						Format:  insts.FormatDPReg,
						Is64Bit: true,
					},
					RnValue: 0xFF00FF00,
					RmValue: 0x0F0F0F0F,
				}

				result := executeStage.Execute(idex, idex.RnValue, idex.RmValue)

				Expect(result.ALUResult).To(Equal(uint64(0x0F000F00)))
			})

			It("should execute ORR (64-bit)", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:      insts.OpORR,
						Format:  insts.FormatDPReg,
						Is64Bit: true,
					},
					RnValue: 0xF0F0F0F0,
					RmValue: 0x0F0F0F0F,
				}

				result := executeStage.Execute(idex, idex.RnValue, idex.RmValue)

				Expect(result.ALUResult).To(Equal(uint64(0xFFFFFFFF)))
			})

			It("should execute EOR (64-bit)", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:      insts.OpEOR,
						Format:  insts.FormatDPReg,
						Is64Bit: true,
					},
					RnValue: 0xFFFF0000,
					RmValue: 0xFF00FF00,
				}

				result := executeStage.Execute(idex, idex.RnValue, idex.RmValue)

				Expect(result.ALUResult).To(Equal(uint64(0x00FFFF00)))
			})

			It("should handle 32-bit operations", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:      insts.OpADD,
						Format:  insts.FormatDPReg,
						Is64Bit: false,
					},
					RnValue: 0xFFFFFFFF,
					RmValue: 1,
				}

				result := executeStage.Execute(idex, idex.RnValue, idex.RmValue)

				Expect(result.ALUResult).To(Equal(uint64(0))) // Wraps at 32 bits
			})
		})

		Context("Address calculation", func() {
			It("should calculate load address", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:     insts.OpLDR,
						Format: insts.FormatLoadStore,
						Imm:    8,
					},
					Rn:      1, // Not SP
					RnValue: 0x2000,
				}

				result := executeStage.Execute(idex, idex.RnValue, 0)

				Expect(result.ALUResult).To(Equal(uint64(0x2008)))
			})

			It("should calculate store address with store value", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:     insts.OpSTR,
						Format: insts.FormatLoadStore,
						Imm:    16,
					},
					Rn:      1,
					RnValue: 0x3000,
					RmValue: 42, // Value to store
				}

				result := executeStage.Execute(idex, idex.RnValue, idex.RmValue)

				Expect(result.ALUResult).To(Equal(uint64(0x3010)))
				Expect(result.StoreValue).To(Equal(uint64(42)))
			})

			It("should use SP when base register is 31", func() {
				regFile.SP = 0x8000
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst: &insts.Instruction{
						Op:     insts.OpLDR,
						Format: insts.FormatLoadStore,
						Imm:    8,
						Rn:     31, // SP
					},
					Rn:      31,
					RnValue: 0, // Will use SP instead
				}

				result := executeStage.Execute(idex, 0, 0)

				Expect(result.ALUResult).To(Equal(uint64(0x8008)))
			})
		})

		Context("Branch execution", func() {
			It("should execute unconditional branch B", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:           insts.OpB,
						Format:       insts.FormatBranch,
						BranchOffset: 100,
					},
				}

				result := executeStage.Execute(idex, 0, 0)

				Expect(result.BranchTaken).To(BeTrue())
				Expect(result.BranchTarget).To(Equal(uint64(0x1064))) // 0x1000 + 100
			})

			It("should execute BL with return address", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:           insts.OpBL,
						Format:       insts.FormatBranch,
						BranchOffset: 100,
					},
				}

				result := executeStage.Execute(idex, 0, 0)

				Expect(result.BranchTaken).To(BeTrue())
				Expect(result.BranchTarget).To(Equal(uint64(0x1064)))
				Expect(result.ALUResult).To(Equal(uint64(0x1004))) // Return address
			})

			It("should handle negative branch offset", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:           insts.OpB,
						Format:       insts.FormatBranch,
						BranchOffset: -20,
					},
				}

				result := executeStage.Execute(idex, 0, 0)

				Expect(result.BranchTaken).To(BeTrue())
				Expect(result.BranchTarget).To(Equal(uint64(0x0FEC))) // 0x1000 - 20
			})

			It("should execute conditional branch when condition met", func() {
				regFile.PSTATE.Z = true // Set Z flag for EQ condition
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:           insts.OpBCond,
						Format:       insts.FormatBranchCond,
						BranchOffset: 40,
						Cond:         insts.CondEQ,
					},
				}

				result := executeStage.Execute(idex, 0, 0)

				Expect(result.BranchTaken).To(BeTrue())
				Expect(result.BranchTarget).To(Equal(uint64(0x1028)))
			})

			It("should not take conditional branch when condition not met", func() {
				regFile.PSTATE.Z = false // Clear Z flag
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:           insts.OpBCond,
						Format:       insts.FormatBranchCond,
						BranchOffset: 40,
						Cond:         insts.CondEQ,
					},
				}

				result := executeStage.Execute(idex, 0, 0)

				Expect(result.BranchTaken).To(BeFalse())
			})

			It("should execute BR (branch to register)", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:     insts.OpBR,
						Format: insts.FormatBranchReg,
					},
					RnValue: 0x3000,
				}

				result := executeStage.Execute(idex, idex.RnValue, 0)

				Expect(result.BranchTaken).To(BeTrue())
				Expect(result.BranchTarget).To(Equal(uint64(0x3000)))
			})

			It("should execute BLR with return address", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x1000,
					Inst: &insts.Instruction{
						Op:     insts.OpBLR,
						Format: insts.FormatBranchReg,
					},
					RnValue: 0x4000,
				}

				result := executeStage.Execute(idex, idex.RnValue, 0)

				Expect(result.BranchTaken).To(BeTrue())
				Expect(result.BranchTarget).To(Equal(uint64(0x4000)))
				Expect(result.ALUResult).To(Equal(uint64(0x1004)))
			})

			It("should execute RET", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					PC:    0x5000,
					Inst: &insts.Instruction{
						Op:     insts.OpRET,
						Format: insts.FormatBranchReg,
					},
					Rn:      30,
					RnValue: 0x2000, // Return address in X30
				}

				result := executeStage.Execute(idex, idex.RnValue, 0)

				Expect(result.BranchTaken).To(BeTrue())
				Expect(result.BranchTarget).To(Equal(uint64(0x2000)))
			})
		})

		Context("Invalid input handling", func() {
			It("should return empty result for nil instruction", func() {
				idex := &pipeline.IDEXRegister{
					Valid: true,
					Inst:  nil,
				}

				result := executeStage.Execute(idex, 0, 0)

				Expect(result.ALUResult).To(Equal(uint64(0)))
				Expect(result.BranchTaken).To(BeFalse())
			})
		})
	})

	Describe("MemoryStage", func() {
		var memoryStage *pipeline.MemoryStage

		BeforeEach(func() {
			memoryStage = pipeline.NewMemoryStage(memory)
		})

		Context("Load operations", func() {
			It("should perform 64-bit load", func() {
				memory.Write64(0x2000, 0x123456789ABCDEF0)

				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					ALUResult: 0x2000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: true},
				}

				result := memoryStage.Access(exmem)

				Expect(result.MemData).To(Equal(uint64(0x123456789ABCDEF0)))
			})

			It("should perform 32-bit load", func() {
				memory.Write32(0x2000, 0xDEADBEEF)

				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					ALUResult: 0x2000,
					MemRead:   true,
					Inst:      &insts.Instruction{Is64Bit: false},
				}

				result := memoryStage.Access(exmem)

				Expect(result.MemData).To(Equal(uint64(0xDEADBEEF)))
			})
		})

		Context("Store operations", func() {
			It("should perform 64-bit store", func() {
				exmem := &pipeline.EXMEMRegister{
					Valid:      true,
					ALUResult:  0x3000,
					StoreValue: 0xCAFEBABE12345678,
					MemWrite:   true,
					Inst:       &insts.Instruction{Is64Bit: true},
				}

				memoryStage.Access(exmem)

				Expect(memory.Read64(0x3000)).To(Equal(uint64(0xCAFEBABE12345678)))
			})

			It("should perform 32-bit store", func() {
				exmem := &pipeline.EXMEMRegister{
					Valid:      true,
					ALUResult:  0x3000,
					StoreValue: 0xDEADBEEF,
					MemWrite:   true,
					Inst:       &insts.Instruction{Is64Bit: false},
				}

				memoryStage.Access(exmem)

				Expect(memory.Read32(0x3000)).To(Equal(uint32(0xDEADBEEF)))
			})
		})

		Context("No memory access", func() {
			It("should not access memory for ALU instructions", func() {
				exmem := &pipeline.EXMEMRegister{
					Valid:     true,
					ALUResult: 150,
					MemRead:   false,
					MemWrite:  false,
				}

				result := memoryStage.Access(exmem)

				Expect(result.MemData).To(Equal(uint64(0)))
			})

			It("should handle invalid register", func() {
				exmem := &pipeline.EXMEMRegister{
					Valid: false,
				}

				result := memoryStage.Access(exmem)

				Expect(result.MemData).To(Equal(uint64(0)))
			})
		})
	})

	Describe("WritebackStage", func() {
		var writebackStage *pipeline.WritebackStage

		BeforeEach(func() {
			writebackStage = pipeline.NewWritebackStage(regFile)
		})

		Context("ALU result writeback", func() {
			It("should write ALU result to register", func() {
				memwb := &pipeline.MEMWBRegister{
					Valid:     true,
					ALUResult: 150,
					Rd:        5,
					RegWrite:  true,
					MemToReg:  false,
				}

				writebackStage.Writeback(memwb)

				Expect(regFile.ReadReg(5)).To(Equal(uint64(150)))
			})
		})

		Context("Memory data writeback", func() {
			It("should write memory data when MemToReg is true", func() {
				memwb := &pipeline.MEMWBRegister{
					Valid:     true,
					ALUResult: 0x2000, // Address (not written)
					MemData:   1000,   // Loaded value
					Rd:        3,
					RegWrite:  true,
					MemToReg:  true,
				}

				writebackStage.Writeback(memwb)

				Expect(regFile.ReadReg(3)).To(Equal(uint64(1000)))
			})
		})

		Context("No writeback", func() {
			It("should not write when RegWrite is false", func() {
				regFile.WriteReg(5, 999)

				memwb := &pipeline.MEMWBRegister{
					Valid:     true,
					ALUResult: 150,
					Rd:        5,
					RegWrite:  false,
				}

				writebackStage.Writeback(memwb)

				Expect(regFile.ReadReg(5)).To(Equal(uint64(999))) // Unchanged
			})

			It("should not write to XZR", func() {
				memwb := &pipeline.MEMWBRegister{
					Valid:     true,
					ALUResult: 150,
					Rd:        31,
					RegWrite:  true,
				}

				writebackStage.Writeback(memwb)

				Expect(regFile.ReadReg(31)).To(Equal(uint64(0))) // Always 0
			})

			It("should not write when register is invalid", func() {
				regFile.WriteReg(5, 999)

				memwb := &pipeline.MEMWBRegister{
					Valid:     false,
					ALUResult: 150,
					Rd:        5,
					RegWrite:  true,
				}

				writebackStage.Writeback(memwb)

				Expect(regFile.ReadReg(5)).To(Equal(uint64(999)))
			})
		})
	})
})
