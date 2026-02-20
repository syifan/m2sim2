package pipeline

import (
	"github.com/sarchlab/m2sim/insts"
)

// isLoadFwdEligible checks if a load-use hazard can be resolved by MEM→EX
// forwarding from the cache stage instead of a 1-cycle pipeline stall.
// This models OOO-style load-to-use forwarding where the cache hit result
// is available to the consumer without waiting for the writeback stage.
//
// Narrowly scoped to DataProc3Src (MADD/MSUB) consumers only:
//   - Producer is an integer load (LDR/LDRH/LDRB, not LDRQ/FP loads)
//   - Consumer is a DataProc3Src op (MADD/MSUB/SMULL etc.)
//   - Consumer doesn't write only flags (Rd==31)
//   - Consumer doesn't read load result via Ra/Rt2 (no MEM→EX path for Ra)
func isLoadFwdEligible(loadInst *insts.Instruction, loadRd uint8, consumerInst *insts.Instruction) bool {
	if loadInst == nil || consumerInst == nil {
		return false
	}
	// Producer must be an integer load
	switch loadInst.Op {
	case insts.OpLDR, insts.OpLDRB, insts.OpLDRSB, insts.OpLDRH, insts.OpLDRSH, insts.OpLDRSW:
	default:
		return false
	}
	// Consumer must be a DataProc3Src format (MADD/MSUB/SMULL etc.)
	if consumerInst.Format != insts.FormatDataProc3Src {
		return false
	}
	// Don't suppress for flag-only consumers (Rd==31)
	if consumerInst.Rd == 31 {
		return false
	}
	// Don't suppress if consumer reads load result via Rt2 (Ra for MADD/MSUB):
	// Ra is read directly from the register file with no forwarding path.
	if consumerInst.Rt2 == loadRd {
		return false
	}
	return true
}

// tickOctupleIssue executes one cycle with 8-wide superscalar support.
// This extends 6-wide to match the Apple M2's 8-wide decode bandwidth.
func (p *Pipeline) tickOctupleIssue() {
	// Stage 5: Writeback (batched processing for all 8 slots)
	savedMEMWB := p.memwb

	// Batch writeback all 8 slots to reduce function call overhead
	slots := []WritebackSlot{&p.memwb, &p.memwb2, &p.memwb3, &p.memwb4, &p.memwb5, &p.memwb6, &p.memwb7, &p.memwb8}
	retired := p.writebackStage.WritebackSlots(slots)
	p.stats.Instructions += retired

	// Handle fused instruction special case for primary slot
	if p.memwb.Valid && p.memwb.IsFused {
		p.stats.Instructions++ // Fused CMP+B.cond counts as 2 instructions
	}

	// Stage 4: Memory (primary slot only - single memory port)
	var nextMEMWB MEMWBRegister
	var nextMEMWB2 SecondaryMEMWBRegister
	var nextMEMWB3 TertiaryMEMWBRegister
	var nextMEMWB4 QuaternaryMEMWBRegister
	var nextMEMWB5 QuinaryMEMWBRegister
	var nextMEMWB6 SenaryMEMWBRegister
	var nextMEMWB7 SeptenaryMEMWBRegister
	var nextMEMWB8 OctonaryMEMWBRegister
	memStall := false

	if p.exmem.Valid {
		if p.exmem.Inst != nil && p.exmem.Inst.Op == insts.OpSVC {
			if p.syscallHandler != nil {
				result := p.syscallHandler.Handle()
				if result.Exited {
					p.halted = true
					p.exitCode = result.ExitCode
				}
			}
		}

		var memResult MemoryResult
		if p.useDCache && p.cachedMemoryStage != nil {
			memResult, memStall = p.cachedMemoryStage.Access(&p.exmem)
			if memStall {
				p.stats.MemStalls++
			}
		} else {
			// Non-cached path: immediate access (no stall).
			// Without cache simulation, memory is a direct array lookup.
			// Pipeline issue rules already enforce ordering constraints.
			if p.exmem.MemRead || p.exmem.MemWrite {
				p.memPending = false
				memResult = p.memoryStage.Access(&p.exmem)
			} else {
				p.memPending = false
			}
		}

		if !memStall {
			nextMEMWB = MEMWBRegister{
				Valid:     true,
				PC:        p.exmem.PC,
				Inst:      p.exmem.Inst,
				ALUResult: p.exmem.ALUResult,
				MemData:   memResult.MemData,
				Rd:        p.exmem.Rd,
				RegWrite:  p.exmem.RegWrite,
				MemToReg:  p.exmem.MemToReg,
				IsFused:   p.exmem.IsFused,
			}
		}
	}

	// Secondary slot memory (memory port 2) — tick in parallel with port 1
	var memStall2 bool
	var memResult2 MemoryResult
	if p.exmem2.Valid {
		if p.exmem2.MemRead || p.exmem2.MemWrite {
			memResult2, memStall2 = p.accessSecondaryMem(&p.exmem2)
		}
	}

	// Tertiary slot memory (memory port 3) — tick in parallel with ports 1 & 2
	var memStall3 bool
	var memResult3 MemoryResult
	if p.exmem3.Valid {
		if p.exmem3.MemRead || p.exmem3.MemWrite {
			memResult3, memStall3 = p.accessTertiaryMem(&p.exmem3)
		}
	}

	// Quaternary slot memory (memory port 4) — tick in parallel with ports 1-3
	var memStall4 bool
	var memResult4 MemoryResult
	if p.exmem4.Valid {
		if p.exmem4.MemRead || p.exmem4.MemWrite {
			memResult4, memStall4 = p.accessQuaternaryMem(&p.exmem4)
		}
	}

	// Quinary slot memory (memory port 5) — tick in parallel with ports 1-4
	var memStall5 bool
	var memResult5 MemoryResult
	if p.exmem5.Valid {
		if p.exmem5.MemRead || p.exmem5.MemWrite {
			memResult5, memStall5 = p.accessQuinaryMem(&p.exmem5)
		}
	}

	// Combine stall signals: pipeline stalls if ANY memory port is stalling.
	// Track whether primary port already counted this stall cycle.
	primaryStalled := memStall
	memStall = memStall || memStall2 || memStall3 || memStall4 || memStall5
	if memStall && !primaryStalled {
		p.stats.MemStalls++
	}

	if p.exmem2.Valid && !memStall {
		nextMEMWB2 = SecondaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem2.PC,
			Inst:      p.exmem2.Inst,
			ALUResult: p.exmem2.ALUResult,
			MemData:   memResult2.MemData,
			Rd:        p.exmem2.Rd,
			RegWrite:  p.exmem2.RegWrite,
			MemToReg:  p.exmem2.MemToReg,
		}
	}

	if p.exmem3.Valid && !memStall {
		nextMEMWB3 = TertiaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem3.PC,
			Inst:      p.exmem3.Inst,
			ALUResult: p.exmem3.ALUResult,
			MemData:   memResult3.MemData,
			Rd:        p.exmem3.Rd,
			RegWrite:  p.exmem3.RegWrite,
			MemToReg:  p.exmem3.MemToReg,
		}
	}

	// Quaternary slot memory (memory port 4)
	if p.exmem4.Valid && !memStall {
		nextMEMWB4 = QuaternaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem4.PC,
			Inst:      p.exmem4.Inst,
			ALUResult: p.exmem4.ALUResult,
			MemData:   memResult4.MemData,
			Rd:        p.exmem4.Rd,
			RegWrite:  p.exmem4.RegWrite,
			MemToReg:  p.exmem4.MemToReg,
		}
	}

	// Quinary slot memory (memory port 5)
	if p.exmem5.Valid && !memStall {
		nextMEMWB5 = QuinaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem5.PC,
			Inst:      p.exmem5.Inst,
			ALUResult: p.exmem5.ALUResult,
			MemData:   memResult5.MemData,
			Rd:        p.exmem5.Rd,
			RegWrite:  p.exmem5.RegWrite,
			MemToReg:  p.exmem5.MemToReg,
		}
	}

	// Senary slot memory (ALU results only, no memory port)
	if p.exmem6.Valid && !memStall {
		nextMEMWB6 = SenaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem6.PC,
			Inst:      p.exmem6.Inst,
			ALUResult: p.exmem6.ALUResult,
			MemData:   0,
			Rd:        p.exmem6.Rd,
			RegWrite:  p.exmem6.RegWrite,
			MemToReg:  false,
		}
	}

	// Septenary slot memory (ALU results only, no memory port)
	if p.exmem7.Valid && !memStall {
		nextMEMWB7 = SeptenaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem7.PC,
			Inst:      p.exmem7.Inst,
			ALUResult: p.exmem7.ALUResult,
			MemData:   0,
			Rd:        p.exmem7.Rd,
			RegWrite:  p.exmem7.RegWrite,
			MemToReg:  false,
		}
	}

	// Octonary slot memory (ALU results only, no memory port)
	if p.exmem8.Valid && !memStall {
		nextMEMWB8 = OctonaryMEMWBRegister{
			Valid:     true,
			PC:        p.exmem8.PC,
			Inst:      p.exmem8.Inst,
			ALUResult: p.exmem8.ALUResult,
			MemData:   0,
			Rd:        p.exmem8.Rd,
			RegWrite:  p.exmem8.RegWrite,
			MemToReg:  false,
		}
	}

	// Stage 3: Execute (all 8 slots)
	var nextEXMEM EXMEMRegister
	var nextEXMEM2 SecondaryEXMEMRegister
	var nextEXMEM3 TertiaryEXMEMRegister
	var nextEXMEM4 QuaternaryEXMEMRegister
	var nextEXMEM5 QuinaryEXMEMRegister
	var nextEXMEM6 SenaryEXMEMRegister
	var nextEXMEM7 SeptenaryEXMEMRegister
	var nextEXMEM8 OctonaryEXMEMRegister
	execStall := false

	// Detect forwarding for primary slot
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Execute primary slot
	if p.idex.Valid && !memStall {
		if p.exLatency == 0 {
			p.exLatency = p.getExLatency(p.idex.Inst)
		}

		if p.exLatency > 0 {
			p.exLatency--
		}

		if p.exLatency > 0 {
			execStall = true
			p.stats.ExecStalls++
		} else {
			rnValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRn, p.idex.RnValue, &p.exmem, &savedMEMWB)
			rmValue := p.hazardUnit.GetForwardedValue(
				forwarding.ForwardRm, p.idex.RmValue, &p.exmem, &savedMEMWB)

			// Forward from all secondary pipeline stages to primary slot
			rnValue = p.forwardFromAllSlots(p.idex.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex.Rm, rmValue)

			// MEM→EX forwarding: when a load in EXMEM completes its cache
			// access this cycle, forward MemData directly to the consumer
			// in IDEX. Only activates when the consumer was placed into IDEX
			// via loadFwdActive (suppressed load-use stall). This prevents
			// incorrect forwarding for unrelated instructions in IDEX.
			if p.loadFwdPendingInIDEX && !memStall {
				p.loadFwdPendingInIDEX = false
				if nextMEMWB.Valid && nextMEMWB.MemToReg && nextMEMWB.RegWrite && nextMEMWB.Rd != 31 {
					if p.idex.Rn == nextMEMWB.Rd {
						rnValue = nextMEMWB.MemData
					}
					if p.idex.Rm == nextMEMWB.Rd {
						rmValue = nextMEMWB.MemData
					}
				}
				if nextMEMWB2.Valid && nextMEMWB2.MemToReg && nextMEMWB2.RegWrite && nextMEMWB2.Rd != 31 {
					if p.idex.Rn == nextMEMWB2.Rd {
						rnValue = nextMEMWB2.MemData
					}
					if p.idex.Rm == nextMEMWB2.Rd {
						rmValue = nextMEMWB2.MemData
					}
				}
				if nextMEMWB3.Valid && nextMEMWB3.MemToReg && nextMEMWB3.RegWrite && nextMEMWB3.Rd != 31 {
					if p.idex.Rn == nextMEMWB3.Rd {
						rnValue = nextMEMWB3.MemData
					}
					if p.idex.Rm == nextMEMWB3.Rd {
						rmValue = nextMEMWB3.MemData
					}
				}
				if nextMEMWB4.Valid && nextMEMWB4.MemToReg && nextMEMWB4.RegWrite && nextMEMWB4.Rd != 31 {
					if p.idex.Rn == nextMEMWB4.Rd {
						rnValue = nextMEMWB4.MemData
					}
					if p.idex.Rm == nextMEMWB4.Rd {
						rmValue = nextMEMWB4.MemData
					}
				}
				if nextMEMWB5.Valid && nextMEMWB5.MemToReg && nextMEMWB5.RegWrite && nextMEMWB5.Rd != 31 {
					if p.idex.Rn == nextMEMWB5.Rd {
						rnValue = nextMEMWB5.MemData
					}
					if p.idex.Rm == nextMEMWB5.Rd {
						rmValue = nextMEMWB5.MemData
					}
				}
			}

			// Check for PSTATE flag forwarding from all EXMEM stages (octuple-issue).
			// CMP can execute in any slot, and B.cond in slot 0 needs the flags.
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				forwardFlags, fwdN, fwdZ, fwdC, fwdV = p.forwardPSTATEFromPrevCycleEXMEM()
			}

			// Save register checkpoint before branch execution so we can
			// roll back speculative writes on misprediction.
			if p.idex.IsBranch {
				p.branchCheckpoint.Valid = true
				for i := uint8(0); i < 31; i++ {
					p.branchCheckpoint.Regs[i] = p.regFile.ReadReg(i)
				}
				p.branchCheckpoint.SP = p.regFile.SP
				p.branchCheckpoint.PSTATE = p.regFile.PSTATE
			}

			execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
				forwardFlags, fwdN, fwdZ, fwdC, fwdV)

			storeValue := execResult.StoreValue
			if p.idex.MemWrite {
				rdValue := p.regFile.ReadReg(p.idex.Rd)
				storeValue = p.hazardUnit.GetForwardedValue(
					forwarding.ForwardRd, rdValue, &p.exmem, &savedMEMWB)
			}

			nextEXMEM = EXMEMRegister{
				Valid:      true,
				PC:         p.idex.PC,
				Inst:       p.idex.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: storeValue,
				Rd:         p.idex.Rd,
				MemRead:    p.idex.MemRead,
				MemWrite:   p.idex.MemWrite,
				RegWrite:   p.idex.RegWrite,
				MemToReg:   p.idex.MemToReg,
				IsFused:    p.idex.IsFused,
				// Store computed flags for forwarding to dependent B.cond
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}

			// Branch prediction verification for primary slot
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					// Restore register checkpoint to undo speculative writes
					if p.branchCheckpoint.Valid {
						for i := uint8(0); i < 31; i++ {
							p.regFile.WriteReg(i, p.branchCheckpoint.Regs[i])
						}
						p.regFile.SP = p.branchCheckpoint.SP
						p.regFile.PSTATE = p.branchCheckpoint.PSTATE
						p.branchCheckpoint.Valid = false
					}

					// Latch results and return early
					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2.Clear()
						p.exmem3.Clear()
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.branchCheckpoint.Valid = false
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute secondary slot
	if p.idex2.Valid && !memStall {
		if p.exLatency2 == 0 {
			p.exLatency2 = p.getExLatency(p.idex2.Inst)
		}
		if p.exLatency2 > 0 {
			p.exLatency2--
		}
		if p.exLatency2 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex2.Rn, p.idex2.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex2.Rm, p.idex2.RmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM.Valid, nextEXMEM.RegWrite, nextEXMEM.Rd, nextEXMEM.ALUResult, p.idex2.Rn, p.idex2.Rm, rnValue, rmValue)
			// Same-cycle PSTATE flag forwarding for B.cond in slot 2
			forwardFlags2 := false
			var fwdN2, fwdZ2, fwdC2, fwdV2 bool
			if p.idex2.Inst != nil && p.idex2.Inst.Op == insts.OpBCond {
				// Check same-cycle: slot 0 (nextEXMEM)
				if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags2 = true
					fwdN2, fwdZ2, fwdC2, fwdV2 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				// Check previous cycle EXMEM registers
				if !forwardFlags2 {
					if p.exmem.Valid && p.exmem.SetsFlags {
						forwardFlags2 = true
						fwdN2, fwdZ2, fwdC2, fwdV2 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
					} else if p.exmem2.Valid && p.exmem2.SetsFlags {
						forwardFlags2 = true
						fwdN2, fwdZ2, fwdC2, fwdV2 = p.exmem2.FlagN, p.exmem2.FlagZ, p.exmem2.FlagC, p.exmem2.FlagV
					}
				}
			}

			idex2 := p.idex2.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex2, rnValue, rmValue,
				forwardFlags2, fwdN2, fwdZ2, fwdC2, fwdV2)
			nextEXMEM2 = SecondaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex2.PC,
				Inst:       p.idex2.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex2.Rd,
				MemRead:    p.idex2.MemRead,
				MemWrite:   p.idex2.MemWrite,
				RegWrite:   p.idex2.RegWrite,
				MemToReg:   p.idex2.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for secondary slot (idex2)
			if p.idex2.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex2.PredictedTaken
				predictedTarget := p.idex2.PredictedTarget
				earlyResolved := p.idex2.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex2.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex2.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2.Clear()
						p.exmem3.Clear()
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute tertiary slot
	if p.idex3.Valid && !memStall {
		if p.exLatency3 == 0 {
			p.exLatency3 = p.getExLatency(p.idex3.Inst)
		}
		if p.exLatency3 > 0 {
			p.exLatency3--
		}
		if p.exLatency3 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex3.Rn, p.idex3.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex3.Rm, p.idex3.RmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM.Valid, nextEXMEM.RegWrite, nextEXMEM.Rd, nextEXMEM.ALUResult, p.idex3.Rn, p.idex3.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM2.Valid, nextEXMEM2.RegWrite, nextEXMEM2.Rd, nextEXMEM2.ALUResult, p.idex3.Rn, p.idex3.Rm, rnValue, rmValue)
			// Same-cycle PSTATE flag forwarding for B.cond in slot 3
			forwardFlags3 := false
			var fwdN3, fwdZ3, fwdC3, fwdV3 bool
			if p.idex3.Inst != nil && p.idex3.Inst.Op == insts.OpBCond {
				// Check same-cycle: slots 0-1 (nextEXMEM, nextEXMEM2)
				if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags3 = true
					fwdN3, fwdZ3, fwdC3, fwdV3 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags3 = true
					fwdN3, fwdZ3, fwdC3, fwdV3 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				// Check previous cycle EXMEM registers
				if !forwardFlags3 {
					if p.exmem.Valid && p.exmem.SetsFlags {
						forwardFlags3 = true
						fwdN3, fwdZ3, fwdC3, fwdV3 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
					} else if p.exmem2.Valid && p.exmem2.SetsFlags {
						forwardFlags3 = true
						fwdN3, fwdZ3, fwdC3, fwdV3 = p.exmem2.FlagN, p.exmem2.FlagZ, p.exmem2.FlagC, p.exmem2.FlagV
					} else if p.exmem3.Valid && p.exmem3.SetsFlags {
						forwardFlags3 = true
						fwdN3, fwdZ3, fwdC3, fwdV3 = p.exmem3.FlagN, p.exmem3.FlagZ, p.exmem3.FlagC, p.exmem3.FlagV
					}
				}
			}

			idex3 := p.idex3.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex3, rnValue, rmValue,
				forwardFlags3, fwdN3, fwdZ3, fwdC3, fwdV3)
			nextEXMEM3 = TertiaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex3.PC,
				Inst:       p.idex3.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex3.Rd,
				MemRead:    p.idex3.MemRead,
				MemWrite:   p.idex3.MemWrite,
				RegWrite:   p.idex3.RegWrite,
				MemToReg:   p.idex3.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for tertiary slot (idex3)
			if p.idex3.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex3.PredictedTaken
				predictedTarget := p.idex3.PredictedTarget
				earlyResolved := p.idex3.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex3.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex3.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3.Clear()
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute quaternary slot
	if p.idex4.Valid && !memStall {
		if p.exLatency4 == 0 {
			p.exLatency4 = p.getExLatency(p.idex4.Inst)
		}
		if p.exLatency4 > 0 {
			p.exLatency4--
		}
		if p.exLatency4 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex4.Rn, p.idex4.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex4.Rm, p.idex4.RmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM.Valid, nextEXMEM.RegWrite, nextEXMEM.Rd, nextEXMEM.ALUResult, p.idex4.Rn, p.idex4.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM2.Valid, nextEXMEM2.RegWrite, nextEXMEM2.Rd, nextEXMEM2.ALUResult, p.idex4.Rn, p.idex4.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM3.Valid, nextEXMEM3.RegWrite, nextEXMEM3.Rd, nextEXMEM3.ALUResult, p.idex4.Rn, p.idex4.Rm, rnValue, rmValue)
			// Same-cycle PSTATE flag forwarding for B.cond in slot 4
			forwardFlags4 := false
			var fwdN4, fwdZ4, fwdC4, fwdV4 bool
			if p.idex4.Inst != nil && p.idex4.Inst.Op == insts.OpBCond {
				// Check same-cycle: slots 0-2
				if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags4 = true
					fwdN4, fwdZ4, fwdC4, fwdV4 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags4 = true
					fwdN4, fwdZ4, fwdC4, fwdV4 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags4 = true
					fwdN4, fwdZ4, fwdC4, fwdV4 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				// Check previous cycle
				if !forwardFlags4 {
					if p.exmem.Valid && p.exmem.SetsFlags {
						forwardFlags4 = true
						fwdN4, fwdZ4, fwdC4, fwdV4 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
					}
				}
			}

			idex4 := p.idex4.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex4, rnValue, rmValue,
				forwardFlags4, fwdN4, fwdZ4, fwdC4, fwdV4)
			nextEXMEM4 = QuaternaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex4.PC,
				Inst:       p.idex4.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex4.Rd,
				MemRead:    p.idex4.MemRead,
				MemWrite:   p.idex4.MemWrite,
				RegWrite:   p.idex4.RegWrite,
				MemToReg:   p.idex4.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for quaternary slot (idex4)
			if p.idex4.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex4.PredictedTaken
				predictedTarget := p.idex4.PredictedTarget
				earlyResolved := p.idex4.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex4.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex4.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4.Clear()
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute quinary slot
	if p.idex5.Valid && !memStall {
		if p.exLatency5 == 0 {
			p.exLatency5 = p.getExLatency(p.idex5.Inst)
		}
		if p.exLatency5 > 0 {
			p.exLatency5--
		}
		if p.exLatency5 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex5.Rn, p.idex5.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex5.Rm, p.idex5.RmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM.Valid, nextEXMEM.RegWrite, nextEXMEM.Rd, nextEXMEM.ALUResult, p.idex5.Rn, p.idex5.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM2.Valid, nextEXMEM2.RegWrite, nextEXMEM2.Rd, nextEXMEM2.ALUResult, p.idex5.Rn, p.idex5.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM3.Valid, nextEXMEM3.RegWrite, nextEXMEM3.Rd, nextEXMEM3.ALUResult, p.idex5.Rn, p.idex5.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM4.Valid, nextEXMEM4.RegWrite, nextEXMEM4.Rd, nextEXMEM4.ALUResult, p.idex5.Rn, p.idex5.Rm, rnValue, rmValue)
			// Same-cycle PSTATE flag forwarding for B.cond in slot 5
			forwardFlags5 := false
			var fwdN5, fwdZ5, fwdC5, fwdV5 bool
			if p.idex5.Inst != nil && p.idex5.Inst.Op == insts.OpBCond {
				if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags5 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags5 = true
					fwdN5, fwdZ5, fwdC5, fwdV5 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex5 := p.idex5.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex5, rnValue, rmValue,
				forwardFlags5, fwdN5, fwdZ5, fwdC5, fwdV5)
			nextEXMEM5 = QuinaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex5.PC,
				Inst:       p.idex5.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex5.Rd,
				MemRead:    p.idex5.MemRead,
				MemWrite:   p.idex5.MemWrite,
				RegWrite:   p.idex5.RegWrite,
				MemToReg:   p.idex5.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for quinary slot (idex5)
			if p.idex5.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex5.PredictedTaken
				predictedTarget := p.idex5.PredictedTarget
				earlyResolved := p.idex5.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex5.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex5.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5.Clear()
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute senary slot
	if p.idex6.Valid && !memStall {
		if p.exLatency6 == 0 {
			p.exLatency6 = p.getExLatency(p.idex6.Inst)
		}
		if p.exLatency6 > 0 {
			p.exLatency6--
		}
		if p.exLatency6 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex6.Rn, p.idex6.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex6.Rm, p.idex6.RmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM.Valid, nextEXMEM.RegWrite, nextEXMEM.Rd, nextEXMEM.ALUResult, p.idex6.Rn, p.idex6.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM2.Valid, nextEXMEM2.RegWrite, nextEXMEM2.Rd, nextEXMEM2.ALUResult, p.idex6.Rn, p.idex6.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM3.Valid, nextEXMEM3.RegWrite, nextEXMEM3.Rd, nextEXMEM3.ALUResult, p.idex6.Rn, p.idex6.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM4.Valid, nextEXMEM4.RegWrite, nextEXMEM4.Rd, nextEXMEM4.ALUResult, p.idex6.Rn, p.idex6.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM5.Valid, nextEXMEM5.RegWrite, nextEXMEM5.Rd, nextEXMEM5.ALUResult, p.idex6.Rn, p.idex6.Rm, rnValue, rmValue)
			// Same-cycle PSTATE flag forwarding for B.cond in slot 6
			forwardFlags6 := false
			var fwdN6, fwdZ6, fwdC6, fwdV6 bool
			if p.idex6.Inst != nil && p.idex6.Inst.Op == insts.OpBCond {
				if nextEXMEM5.Valid && nextEXMEM5.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM5.FlagN, nextEXMEM5.FlagZ, nextEXMEM5.FlagC, nextEXMEM5.FlagV
				} else if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags6 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags6 = true
					fwdN6, fwdZ6, fwdC6, fwdV6 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex6 := p.idex6.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex6, rnValue, rmValue,
				forwardFlags6, fwdN6, fwdZ6, fwdC6, fwdV6)
			nextEXMEM6 = SenaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex6.PC,
				Inst:       p.idex6.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex6.Rd,
				MemRead:    p.idex6.MemRead,
				MemWrite:   p.idex6.MemWrite,
				RegWrite:   p.idex6.RegWrite,
				MemToReg:   p.idex6.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for senary slot (idex6)
			if p.idex6.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex6.PredictedTaken
				predictedTarget := p.idex6.PredictedTarget
				earlyResolved := p.idex6.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex6.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex6.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5 = nextEXMEM5
						p.exmem6.Clear()
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute septenary slot
	if p.idex7.Valid && !memStall {
		if p.exLatency7 == 0 {
			p.exLatency7 = p.getExLatency(p.idex7.Inst)
		}
		if p.exLatency7 > 0 {
			p.exLatency7--
		}
		if p.exLatency7 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex7.Rn, p.idex7.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex7.Rm, p.idex7.RmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM.Valid, nextEXMEM.RegWrite, nextEXMEM.Rd, nextEXMEM.ALUResult, p.idex7.Rn, p.idex7.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM2.Valid, nextEXMEM2.RegWrite, nextEXMEM2.Rd, nextEXMEM2.ALUResult, p.idex7.Rn, p.idex7.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM3.Valid, nextEXMEM3.RegWrite, nextEXMEM3.Rd, nextEXMEM3.ALUResult, p.idex7.Rn, p.idex7.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM4.Valid, nextEXMEM4.RegWrite, nextEXMEM4.Rd, nextEXMEM4.ALUResult, p.idex7.Rn, p.idex7.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM5.Valid, nextEXMEM5.RegWrite, nextEXMEM5.Rd, nextEXMEM5.ALUResult, p.idex7.Rn, p.idex7.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM6.Valid, nextEXMEM6.RegWrite, nextEXMEM6.Rd, nextEXMEM6.ALUResult, p.idex7.Rn, p.idex7.Rm, rnValue, rmValue)
			// Same-cycle PSTATE flag forwarding for B.cond in slot 7
			forwardFlags7 := false
			var fwdN7, fwdZ7, fwdC7, fwdV7 bool
			if p.idex7.Inst != nil && p.idex7.Inst.Op == insts.OpBCond {
				if nextEXMEM6.Valid && nextEXMEM6.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM6.FlagN, nextEXMEM6.FlagZ, nextEXMEM6.FlagC, nextEXMEM6.FlagV
				} else if nextEXMEM5.Valid && nextEXMEM5.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM5.FlagN, nextEXMEM5.FlagZ, nextEXMEM5.FlagC, nextEXMEM5.FlagV
				} else if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags7 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags7 = true
					fwdN7, fwdZ7, fwdC7, fwdV7 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex7 := p.idex7.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex7, rnValue, rmValue,
				forwardFlags7, fwdN7, fwdZ7, fwdC7, fwdV7)
			nextEXMEM7 = SeptenaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex7.PC,
				Inst:       p.idex7.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex7.Rd,
				MemRead:    p.idex7.MemRead,
				MemWrite:   p.idex7.MemWrite,
				RegWrite:   p.idex7.RegWrite,
				MemToReg:   p.idex7.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for septenary slot (idex7)
			if p.idex7.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex7.PredictedTaken
				predictedTarget := p.idex7.PredictedTarget
				earlyResolved := p.idex7.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex7.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex7.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5 = nextEXMEM5
						p.exmem6 = nextEXMEM6
						p.exmem7.Clear()
						p.exmem8.Clear()
					}
					return
				}
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute octonary slot
	if p.idex8.Valid && !memStall {
		if p.exLatency8 == 0 {
			p.exLatency8 = p.getExLatency(p.idex8.Inst)
		}
		if p.exLatency8 > 0 {
			p.exLatency8--
		}
		if p.exLatency8 == 0 {
			rnValue := p.forwardFromAllSlots(p.idex8.Rn, p.idex8.RnValue)
			rmValue := p.forwardFromAllSlots(p.idex8.Rm, p.idex8.RmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM.Valid, nextEXMEM.RegWrite, nextEXMEM.Rd, nextEXMEM.ALUResult, p.idex8.Rn, p.idex8.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM2.Valid, nextEXMEM2.RegWrite, nextEXMEM2.Rd, nextEXMEM2.ALUResult, p.idex8.Rn, p.idex8.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM3.Valid, nextEXMEM3.RegWrite, nextEXMEM3.Rd, nextEXMEM3.ALUResult, p.idex8.Rn, p.idex8.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM4.Valid, nextEXMEM4.RegWrite, nextEXMEM4.Rd, nextEXMEM4.ALUResult, p.idex8.Rn, p.idex8.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM5.Valid, nextEXMEM5.RegWrite, nextEXMEM5.Rd, nextEXMEM5.ALUResult, p.idex8.Rn, p.idex8.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM6.Valid, nextEXMEM6.RegWrite, nextEXMEM6.Rd, nextEXMEM6.ALUResult, p.idex8.Rn, p.idex8.Rm, rnValue, rmValue)
			rnValue, rmValue = sameCycleForward(nextEXMEM7.Valid, nextEXMEM7.RegWrite, nextEXMEM7.Rd, nextEXMEM7.ALUResult, p.idex8.Rn, p.idex8.Rm, rnValue, rmValue)
			// Same-cycle PSTATE flag forwarding for B.cond in slot 8
			forwardFlags8 := false
			var fwdN8, fwdZ8, fwdC8, fwdV8 bool
			if p.idex8.Inst != nil && p.idex8.Inst.Op == insts.OpBCond {
				if nextEXMEM7.Valid && nextEXMEM7.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM7.FlagN, nextEXMEM7.FlagZ, nextEXMEM7.FlagC, nextEXMEM7.FlagV
				} else if nextEXMEM6.Valid && nextEXMEM6.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM6.FlagN, nextEXMEM6.FlagZ, nextEXMEM6.FlagC, nextEXMEM6.FlagV
				} else if nextEXMEM5.Valid && nextEXMEM5.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM5.FlagN, nextEXMEM5.FlagZ, nextEXMEM5.FlagC, nextEXMEM5.FlagV
				} else if nextEXMEM4.Valid && nextEXMEM4.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM4.FlagN, nextEXMEM4.FlagZ, nextEXMEM4.FlagC, nextEXMEM4.FlagV
				} else if nextEXMEM3.Valid && nextEXMEM3.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM3.FlagN, nextEXMEM3.FlagZ, nextEXMEM3.FlagC, nextEXMEM3.FlagV
				} else if nextEXMEM2.Valid && nextEXMEM2.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM2.FlagN, nextEXMEM2.FlagZ, nextEXMEM2.FlagC, nextEXMEM2.FlagV
				} else if nextEXMEM.Valid && nextEXMEM.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = nextEXMEM.FlagN, nextEXMEM.FlagZ, nextEXMEM.FlagC, nextEXMEM.FlagV
				}
				if !forwardFlags8 && p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags8 = true
					fwdN8, fwdZ8, fwdC8, fwdV8 = p.exmem.FlagN, p.exmem.FlagZ, p.exmem.FlagC, p.exmem.FlagV
				}
			}

			idex8 := p.idex8.toIDEX()
			execResult := p.executeStage.ExecuteWithFlags(&idex8, rnValue, rmValue,
				forwardFlags8, fwdN8, fwdZ8, fwdC8, fwdV8)
			nextEXMEM8 = OctonaryEXMEMRegister{
				Valid:      true,
				PC:         p.idex8.PC,
				Inst:       p.idex8.Inst,
				ALUResult:  execResult.ALUResult,
				StoreValue: execResult.StoreValue,
				Rd:         p.idex8.Rd,
				MemRead:    p.idex8.MemRead,
				MemWrite:   p.idex8.MemWrite,
				RegWrite:   p.idex8.RegWrite,
				MemToReg:   p.idex8.MemToReg,
				SetsFlags:  execResult.SetsFlags,
				FlagN:      execResult.FlagN,
				FlagZ:      execResult.FlagZ,
				FlagC:      execResult.FlagC,
				FlagV:      execResult.FlagV,
			}

			// Branch prediction verification for octonary slot (idex8)
			if p.idex8.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				predictedTaken := p.idex8.PredictedTaken
				predictedTarget := p.idex8.PredictedTarget
				earlyResolved := p.idex8.EarlyResolved

				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						wasMispredicted = true
					}
				} else {
					if predictedTaken {
						wasMispredicted = true
					}
				}

				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				p.branchPredictor.Update(p.idex8.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					p.stats.BranchMispredictionStalls += 2
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex8.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.memwb5 = nextMEMWB5
						p.memwb6 = nextMEMWB6
						p.memwb7 = nextMEMWB7
						p.memwb8 = nextMEMWB8
						p.exmem = nextEXMEM
						p.exmem2 = nextEXMEM2
						p.exmem3 = nextEXMEM3
						p.exmem4 = nextEXMEM4
						p.exmem5 = nextEXMEM5
						p.exmem6 = nextEXMEM6
						p.exmem7 = nextEXMEM7
						p.exmem8.Clear()
					}
					return
				}
				p.clearAndRemarkAfterBranch()
				p.stats.BranchCorrect++
			}
		}
	}

	// Detect load-use hazards for primary decode.
	// Instead of stalling the entire pipeline, we use an OoO-style bypass:
	// only the dependent instruction is held; independent instructions from
	// other IFID slots can still be decoded and issued in this cycle.
	//
	// Load-use forwarding from cache stage: when the producer is an integer
	// load (LDR/LDRH/LDRB) and the consumer is an integer ALU op, suppress
	// the 1-cycle stall. The consumer enters IDEX and waits during the cache
	// stall; when the cache completes, MEM→EX forwarding provides the load
	// data directly. This models OOO-style load-to-use forwarding.
	loadUseHazard := false
	loadFwdActive := false
	loadHazardRd := uint8(31)
	if p.ifid.Valid {
		nextInst := p.decodeStage.decoder.Decode(p.ifid.InstructionWord)
		if nextInst != nil && nextInst.Op != insts.OpUnknown {
			usesRn := true
			usesRm := nextInst.Format == insts.FormatDPReg

			sourceRm := nextInst.Rm
			switch nextInst.Op {
			case insts.OpSTR, insts.OpSTRQ:
				usesRm = true
				sourceRm = nextInst.Rd
			}

			// Check primary slot (IDEX) for load-use hazard
			if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 {
				hazard := p.hazardUnit.DetectLoadUseHazardDecoded(
					p.idex.Rd, nextInst.Rn, sourceRm, usesRn, usesRm)
				if hazard {
					loadHazardRd = p.idex.Rd
					if isLoadFwdEligible(p.idex.Inst, p.idex.Rd, nextInst) {
						loadFwdActive = true
					} else {
						loadUseHazard = true
						p.stats.RAWHazardStalls++
					}
				}
			}

			// Check secondary slot (IDEX2) for load-use hazard
			if !loadUseHazard && !loadFwdActive && p.idex2.Valid && p.idex2.MemRead && p.idex2.Rd != 31 {
				hazard := p.hazardUnit.DetectLoadUseHazardDecoded(
					p.idex2.Rd, nextInst.Rn, sourceRm, usesRn, usesRm)
				if hazard {
					loadHazardRd = p.idex2.Rd
					if isLoadFwdEligible(p.idex2.Inst, p.idex2.Rd, nextInst) {
						loadFwdActive = true
					} else {
						loadUseHazard = true
						p.stats.RAWHazardStalls++
					}
				}
			}
		}
	}

	// Don't pass loadUseHazard to ComputeStalls — we handle it in the decode
	// stage below by skipping dependent instructions (OoO bypass).
	stallResult := p.hazardUnit.ComputeStalls(memStall, false)

	// Stage 2: Decode (all 8 slots)
	var nextIDEX IDEXRegister
	var nextIDEX2 SecondaryIDEXRegister
	var nextIDEX3 TertiaryIDEXRegister
	var nextIDEX4 QuaternaryIDEXRegister
	var nextIDEX5 QuinaryIDEXRegister
	var nextIDEX6 SenaryIDEXRegister
	var nextIDEX7 SeptenaryIDEXRegister
	var nextIDEX8 OctonaryIDEXRegister

	// Track CMP+B.cond fusion for issue count adjustment
	fusedCMPBcond := false

	// loadRdForBypass is the destination register of the in-flight load,
	// used to check each IFID instruction for load-use hazard during bypass.
	// When loadFwdActive, slot 0 is not stalled (MEM→EX forwarding), but
	// other IFID slots that depend on the load must still be held because
	// they don't have the MEM→EX forwarding path.
	loadRdForBypass := uint8(31)
	if loadUseHazard {
		loadRdForBypass = loadHazardRd
		p.stats.Stalls++ // count as a stall for stat tracking
	} else if loadFwdActive {
		loadRdForBypass = loadHazardRd
	}

	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !memStall {
		// During exec stall, the primary slot (slot 0) stays stalled in IDEX.
		// We still decode secondary slots so independent instructions can issue.
		if !execStall {
			decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)

			// CMP+B.cond fusion detection: check if slot 0 is CMP and slot 1 is B.cond
			// Disable fusion during load-use bypass since slot 0 may be held.
			if !loadUseHazard && IsCMP(decResult.Inst) && p.ifid2.Valid {
				decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
				if IsBCond(decResult2.Inst) {
					// Fuse CMP+B.cond: put B.cond in slot 0 with CMP operands
					fusedCMPBcond = true
					nextIDEX = IDEXRegister{
						Valid:           true,
						PC:              p.ifid2.PC,
						Inst:            decResult2.Inst,
						RnValue:         decResult2.RnValue,
						RmValue:         decResult2.RmValue,
						Rd:              decResult2.Rd,
						Rn:              decResult2.Rn,
						Rm:              decResult2.Rm,
						MemRead:         decResult2.MemRead,
						MemWrite:        decResult2.MemWrite,
						RegWrite:        decResult2.RegWrite,
						MemToReg:        decResult2.MemToReg,
						IsBranch:        decResult2.IsBranch,
						PredictedTaken:  p.ifid2.PredictedTaken,
						PredictedTarget: p.ifid2.PredictedTarget,
						EarlyResolved:   p.ifid2.EarlyResolved,
						// Fusion fields from CMP
						IsFused:    true,
						FusedRnVal: decResult.RnValue,
						FusedRmVal: decResult.RmValue,
						FusedIs64:  decResult.Inst.Is64Bit,
						FusedIsImm: decResult.Inst.Format == insts.FormatDPImm,
						FusedImmVal: func() uint64 {
							if decResult.Inst.Format == insts.FormatDPImm {
								imm := decResult.Inst.Imm
								if decResult.Inst.Shift > 0 {
									imm <<= decResult.Inst.Shift
								}
								return imm
							}
							return 0
						}(),
					}
				}
			}

			if !fusedCMPBcond {
				// During load-use hazard, skip the dependent instruction (slot 0).
				// It will be re-queued to IFID for the next cycle via consumed tracking.
				// Also block speculative stores in slot 0: instructions fetched after
				// a predicted-taken branch that write to memory cannot issue because
				// memory writes are not rolled back on misprediction.
				if !loadUseHazard && !(p.ifid.AfterBranch && decResult.MemWrite) {
					nextIDEX = IDEXRegister{
						Valid:           true,
						PC:              p.ifid.PC,
						Inst:            decResult.Inst,
						RnValue:         decResult.RnValue,
						RmValue:         decResult.RmValue,
						Rd:              decResult.Rd,
						Rn:              decResult.Rn,
						Rm:              decResult.Rm,
						MemRead:         decResult.MemRead,
						MemWrite:        decResult.MemWrite,
						RegWrite:        decResult.RegWrite,
						MemToReg:        decResult.MemToReg,
						IsBranch:        decResult.IsBranch,
						PredictedTaken:  p.ifid.PredictedTaken,
						PredictedTarget: p.ifid.PredictedTarget,
						EarlyResolved:   p.ifid.EarlyResolved,
					}
					if loadFwdActive {
						p.loadFwdPendingInIDEX = true
					}
				}
			}
		}

		// Try to issue instructions 2-8 if they can issue with earlier instructions.
		// Uses fixed-size array to avoid heap allocation per tick.
		var issuedInsts [8]*IDEXRegister
		var issued [8]bool
		var forwarded [8]bool

		// During exec stall, the stalled instruction in p.idex occupies slot 0.
		// Include it in the issued set so canIssueWith checks RAW hazards against it.
		// Mark as "forwarded" to prevent same-cycle ALU forwarding, since the
		// multi-cycle instruction's result isn't available yet.
		if execStall {
			issuedInsts[0] = &p.idex
			if p.idex.Valid {
				issued[0] = true
				forwarded[0] = true
			}
		} else {
			issuedInsts[0] = &nextIDEX
			if nextIDEX.Valid {
				issued[0] = true
			}
		}
		issuedCount := 1

		// Track if IFID2 was consumed by fusion (skip its decode)
		ifid2ConsumedByFusion := fusedCMPBcond

		// Decode slot 2 (IFID2) - skip if consumed by fusion
		// OoO-style issue: each slot independently checks canIssueWithFwd().
		// If a slot can't issue, later slots still get a chance.
		// ALU→ALU same-cycle forwarding is enabled for all slots (with 1-hop depth limit).
		if p.ifid2.Valid && !ifid2ConsumedByFusion {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
			// During load-use bypass, check if this instruction also depends on the load.
			// Unlike other hazards, load-use dependency does NOT block subsequent slots —
			// independent instructions can still issue (OoO-style bypass).
			if (loadUseHazard || loadFwdActive) && p.hazardUnit.DetectLoadUseHazardForInst(loadRdForBypass, decResult2.Inst) {
				// Dependent on load — don't issue, re-queue to IFID next cycle
				issuedCount++
			} else {
				tempIDEX2 := IDEXRegister{
					Valid:           true,
					PC:              p.ifid2.PC,
					Inst:            decResult2.Inst,
					RnValue:         decResult2.RnValue,
					RmValue:         decResult2.RmValue,
					Rd:              decResult2.Rd,
					Rn:              decResult2.Rn,
					Rm:              decResult2.Rm,
					MemRead:         decResult2.MemRead,
					MemWrite:        decResult2.MemWrite,
					RegWrite:        decResult2.RegWrite,
					MemToReg:        decResult2.MemToReg,
					IsBranch:        decResult2.IsBranch,
					PredictedTaken:  p.ifid2.PredictedTaken,
					PredictedTarget: p.ifid2.PredictedTarget,
					EarlyResolved:   p.ifid2.EarlyResolved,
				}
				if ok, fwd := canIssueWithFwd(&tempIDEX2, &issuedInsts, issuedCount, &issued, &forwarded, p.useDCache); ok && !(p.ifid2.AfterBranch && decResult2.MemWrite) {
					nextIDEX2.fromIDEX(&tempIDEX2)
					issued[issuedCount] = true
					if fwd {
						forwarded[issuedCount] = true
					}
				} else {
					p.stats.StructuralHazardStalls++
				}
				issuedInsts[issuedCount] = &tempIDEX2
				issuedCount++
			}
		}

		// Decode slot 3
		if p.ifid3.Valid {
			decResult3 := p.decodeStage.Decode(p.ifid3.InstructionWord, p.ifid3.PC)
			if (loadUseHazard || loadFwdActive) && p.hazardUnit.DetectLoadUseHazardForInst(loadRdForBypass, decResult3.Inst) {
				issuedCount++
			} else {
				tempIDEX3 := IDEXRegister{
					Valid:           true,
					PC:              p.ifid3.PC,
					Inst:            decResult3.Inst,
					RnValue:         decResult3.RnValue,
					RmValue:         decResult3.RmValue,
					Rd:              decResult3.Rd,
					Rn:              decResult3.Rn,
					Rm:              decResult3.Rm,
					MemRead:         decResult3.MemRead,
					MemWrite:        decResult3.MemWrite,
					RegWrite:        decResult3.RegWrite,
					MemToReg:        decResult3.MemToReg,
					IsBranch:        decResult3.IsBranch,
					PredictedTaken:  p.ifid3.PredictedTaken,
					PredictedTarget: p.ifid3.PredictedTarget,
					EarlyResolved:   p.ifid3.EarlyResolved,
				}
				if ok, fwd := canIssueWithFwd(&tempIDEX3, &issuedInsts, issuedCount, &issued, &forwarded, p.useDCache); ok && !(p.ifid3.AfterBranch && decResult3.MemWrite) {
					nextIDEX3.fromIDEX(&tempIDEX3)
					issued[issuedCount] = true
					if fwd {
						forwarded[issuedCount] = true
					}
				} else {
					p.stats.StructuralHazardStalls++
				}
				issuedInsts[issuedCount] = &tempIDEX3
				issuedCount++
			}
		}

		// Decode slot 4
		if p.ifid4.Valid {
			decResult4 := p.decodeStage.Decode(p.ifid4.InstructionWord, p.ifid4.PC)
			if (loadUseHazard || loadFwdActive) && p.hazardUnit.DetectLoadUseHazardForInst(loadRdForBypass, decResult4.Inst) {
				issuedCount++
			} else {
				tempIDEX4 := IDEXRegister{
					Valid:           true,
					PC:              p.ifid4.PC,
					Inst:            decResult4.Inst,
					RnValue:         decResult4.RnValue,
					RmValue:         decResult4.RmValue,
					Rd:              decResult4.Rd,
					Rn:              decResult4.Rn,
					Rm:              decResult4.Rm,
					MemRead:         decResult4.MemRead,
					MemWrite:        decResult4.MemWrite,
					RegWrite:        decResult4.RegWrite,
					MemToReg:        decResult4.MemToReg,
					IsBranch:        decResult4.IsBranch,
					PredictedTaken:  p.ifid4.PredictedTaken,
					PredictedTarget: p.ifid4.PredictedTarget,
					EarlyResolved:   p.ifid4.EarlyResolved,
				}
				if ok, fwd := canIssueWithFwd(&tempIDEX4, &issuedInsts, issuedCount, &issued, &forwarded, p.useDCache); ok && !(p.ifid4.AfterBranch && decResult4.MemWrite) {
					nextIDEX4.fromIDEX(&tempIDEX4)
					issued[issuedCount] = true
					if fwd {
						forwarded[issuedCount] = true
					}
				} else {
					p.stats.StructuralHazardStalls++
				}
				issuedInsts[issuedCount] = &tempIDEX4
				issuedCount++
			}
		}

		// Decode slot 5
		if p.ifid5.Valid {
			decResult5 := p.decodeStage.Decode(p.ifid5.InstructionWord, p.ifid5.PC)
			if (loadUseHazard || loadFwdActive) && p.hazardUnit.DetectLoadUseHazardForInst(loadRdForBypass, decResult5.Inst) {
				issuedCount++
			} else {
				tempIDEX5 := IDEXRegister{
					Valid:           true,
					PC:              p.ifid5.PC,
					Inst:            decResult5.Inst,
					RnValue:         decResult5.RnValue,
					RmValue:         decResult5.RmValue,
					Rd:              decResult5.Rd,
					Rn:              decResult5.Rn,
					Rm:              decResult5.Rm,
					MemRead:         decResult5.MemRead,
					MemWrite:        decResult5.MemWrite,
					RegWrite:        decResult5.RegWrite,
					MemToReg:        decResult5.MemToReg,
					IsBranch:        decResult5.IsBranch,
					PredictedTaken:  p.ifid5.PredictedTaken,
					PredictedTarget: p.ifid5.PredictedTarget,
					EarlyResolved:   p.ifid5.EarlyResolved,
				}
				if ok, fwd := canIssueWithFwd(&tempIDEX5, &issuedInsts, issuedCount, &issued, &forwarded, p.useDCache); ok && !(p.ifid5.AfterBranch && decResult5.MemWrite) {
					nextIDEX5.fromIDEX(&tempIDEX5)
					issued[issuedCount] = true
					if fwd {
						forwarded[issuedCount] = true
					}
				} else {
					p.stats.StructuralHazardStalls++
				}
				issuedInsts[issuedCount] = &tempIDEX5
				issuedCount++
			}
		}

		// Decode slot 6
		if p.ifid6.Valid {
			decResult6 := p.decodeStage.Decode(p.ifid6.InstructionWord, p.ifid6.PC)
			if (loadUseHazard || loadFwdActive) && p.hazardUnit.DetectLoadUseHazardForInst(loadRdForBypass, decResult6.Inst) {
				issuedCount++
			} else {
				tempIDEX6 := IDEXRegister{
					Valid:           true,
					PC:              p.ifid6.PC,
					Inst:            decResult6.Inst,
					RnValue:         decResult6.RnValue,
					RmValue:         decResult6.RmValue,
					Rd:              decResult6.Rd,
					Rn:              decResult6.Rn,
					Rm:              decResult6.Rm,
					MemRead:         decResult6.MemRead,
					MemWrite:        decResult6.MemWrite,
					RegWrite:        decResult6.RegWrite,
					MemToReg:        decResult6.MemToReg,
					IsBranch:        decResult6.IsBranch,
					PredictedTaken:  p.ifid6.PredictedTaken,
					PredictedTarget: p.ifid6.PredictedTarget,
					EarlyResolved:   p.ifid6.EarlyResolved,
				}
				if ok, fwd := canIssueWithFwd(&tempIDEX6, &issuedInsts, issuedCount, &issued, &forwarded, p.useDCache); ok && !(p.ifid6.AfterBranch && decResult6.MemWrite) {
					nextIDEX6.fromIDEX(&tempIDEX6)
					issued[issuedCount] = true
					if fwd {
						forwarded[issuedCount] = true
					}
				} else {
					p.stats.StructuralHazardStalls++
				}
				issuedInsts[issuedCount] = &tempIDEX6
				issuedCount++
			}
		}

		// Decode slot 7
		if p.ifid7.Valid {
			decResult7 := p.decodeStage.Decode(p.ifid7.InstructionWord, p.ifid7.PC)
			if (loadUseHazard || loadFwdActive) && p.hazardUnit.DetectLoadUseHazardForInst(loadRdForBypass, decResult7.Inst) {
				issuedCount++
			} else {
				tempIDEX7 := IDEXRegister{
					Valid:           true,
					PC:              p.ifid7.PC,
					Inst:            decResult7.Inst,
					RnValue:         decResult7.RnValue,
					RmValue:         decResult7.RmValue,
					Rd:              decResult7.Rd,
					Rn:              decResult7.Rn,
					Rm:              decResult7.Rm,
					MemRead:         decResult7.MemRead,
					MemWrite:        decResult7.MemWrite,
					RegWrite:        decResult7.RegWrite,
					MemToReg:        decResult7.MemToReg,
					IsBranch:        decResult7.IsBranch,
					PredictedTaken:  p.ifid7.PredictedTaken,
					PredictedTarget: p.ifid7.PredictedTarget,
					EarlyResolved:   p.ifid7.EarlyResolved,
				}
				if ok, fwd := canIssueWithFwd(&tempIDEX7, &issuedInsts, issuedCount, &issued, &forwarded, p.useDCache); ok && !(p.ifid7.AfterBranch && decResult7.MemWrite) {
					nextIDEX7.fromIDEX(&tempIDEX7)
					issued[issuedCount] = true
					if fwd {
						forwarded[issuedCount] = true
					}
				} else {
					p.stats.StructuralHazardStalls++
				}
				issuedInsts[issuedCount] = &tempIDEX7
				issuedCount++
			}
		}

		// Decode slot 8
		if p.ifid8.Valid {
			decResult8 := p.decodeStage.Decode(p.ifid8.InstructionWord, p.ifid8.PC)
			if (loadUseHazard || loadFwdActive) && p.hazardUnit.DetectLoadUseHazardForInst(loadRdForBypass, decResult8.Inst) {
				// dependent — will be re-queued
			} else {
				tempIDEX8 := IDEXRegister{
					Valid:           true,
					PC:              p.ifid8.PC,
					Inst:            decResult8.Inst,
					RnValue:         decResult8.RnValue,
					RmValue:         decResult8.RmValue,
					Rd:              decResult8.Rd,
					Rn:              decResult8.Rn,
					Rm:              decResult8.Rm,
					MemRead:         decResult8.MemRead,
					MemWrite:        decResult8.MemWrite,
					RegWrite:        decResult8.RegWrite,
					MemToReg:        decResult8.MemToReg,
					IsBranch:        decResult8.IsBranch,
					PredictedTaken:  p.ifid8.PredictedTaken,
					PredictedTarget: p.ifid8.PredictedTarget,
					EarlyResolved:   p.ifid8.EarlyResolved,
				}
				if ok, fwd := canIssueWithFwd(&tempIDEX8, &issuedInsts, issuedCount, &issued, &forwarded, p.useDCache); ok && !(p.ifid8.AfterBranch && decResult8.MemWrite) {
					nextIDEX8.fromIDEX(&tempIDEX8)
					if fwd {
						forwarded[issuedCount] = true
					}
				} else {
					p.stats.StructuralHazardStalls++
				}
				issuedInsts[issuedCount] = &tempIDEX8
			}
		}
	} else if (stallResult.StallID || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
		nextIDEX2 = p.idex2
		nextIDEX3 = p.idex3
		nextIDEX4 = p.idex4
		nextIDEX5 = p.idex5
		nextIDEX6 = p.idex6
		nextIDEX7 = p.idex7
		nextIDEX8 = p.idex8
	}
	if execStall {
		nextIDEX = p.idex
	}

	// Track which IFID slots were consumed (issued to IDEX) for fetch re-queuing
	var consumed [8]bool
	// During exec stall, slot 0 was NOT decoded — the stalled instruction
	// is in IDEX from a previous cycle, so IFID slot 0 is not consumed.
	if execStall {
		consumed[0] = false
	} else {
		consumed[0] = nextIDEX.Valid || fusedCMPBcond
	}
	consumed[1] = nextIDEX2.Valid || fusedCMPBcond // fusion consumes IFID2
	consumed[2] = nextIDEX3.Valid
	consumed[3] = nextIDEX4.Valid
	consumed[4] = nextIDEX5.Valid
	consumed[5] = nextIDEX6.Valid
	consumed[6] = nextIDEX7.Valid
	consumed[7] = nextIDEX8.Valid

	// Stage 1: Fetch (all 8 slots) using instruction window buffer.
	// The instruction window holds pre-fetched instructions that couldn't
	// issue in previous cycles, allowing the pipeline to look across loop
	// iterations and find independent instructions (OoO-style dispatch).
	var nextIFID IFIDRegister
	var nextIFID2 SecondaryIFIDRegister
	var nextIFID3 TertiaryIFIDRegister
	var nextIFID4 QuaternaryIFIDRegister
	var nextIFID5 QuinaryIFIDRegister
	var nextIFID6 SenaryIFIDRegister
	var nextIFID7 SeptenaryIFIDRegister
	var nextIFID8 OctonaryIFIDRegister
	fetchStall := false

	if !stallResult.StallIF && !stallResult.FlushIF && !memStall {
		// Step 1: Push un-consumed IFID instructions into the window buffer.
		// These are instructions that couldn't issue this cycle due to RAW
		// hazards; they get another chance next cycle from the window.
		p.pushUnconsumedToWindow(consumed[:])

		// Step 2: Fetch new instructions into the window buffer.
		fetchPC := p.pc
		fetchedAfterBranch := false
		for p.instrWindowLen < instrWindowSize {
			var word uint32
			var ok bool

			if p.useICache && p.cachedFetchStage != nil {
				word, ok, fetchStall = p.cachedFetchStage.Fetch(fetchPC)
				if fetchStall {
					p.stats.Stalls++
					break
				}
			} else {
				word, ok = p.fetchStage.Fetch(fetchPC)
			}

			if !ok {
				break
			}

			// Branch elimination
			if isEliminableBranch(word) {
				_, uncondTarget := isUnconditionalBranch(word, fetchPC)
				fetchPC = uncondTarget
				p.stats.EliminatedBranches++
				continue
			}

			p.instrWindow[p.instrWindowLen] = instrWindowEntry{
				Valid:           true,
				PC:              fetchPC,
				InstructionWord: word,
				AfterBranch:     fetchedAfterBranch,
			}
			p.instrWindowLen++

			// Check for predicted-taken branch to redirect fetch
			isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
			pred := p.branchPredictor.Predict(fetchPC)
			if isUncondBranch {
				pred.Taken = true
				pred.Target = uncondTarget
				pred.TargetKnown = true
			}
			enrichPredictionWithEncodedTarget(&pred, word, fetchPC)

			// Store prediction in the window entry
			p.instrWindow[p.instrWindowLen-1].PredictedTaken = pred.Taken
			p.instrWindow[p.instrWindowLen-1].PredictedTarget = pred.Target
			p.instrWindow[p.instrWindowLen-1].EarlyResolved = isUncondBranch

			if pred.Taken && pred.TargetKnown {
				fetchPC = pred.Target
				fetchedAfterBranch = true
			} else {
				fetchPC += 4
			}
		}
		p.pc = fetchPC

		// Step 3: Pop the first 8 entries from the window into IFID registers.
		p.popWindowToIFID(&nextIFID, &nextIFID2, &nextIFID3, &nextIFID4,
			&nextIFID5, &nextIFID6, &nextIFID7, &nextIFID8)

		if fetchStall {
			nextIFID = p.ifid
			nextIFID2 = p.ifid2
			nextIFID3 = p.ifid3
			nextIFID4 = p.ifid4
			nextIFID5 = p.ifid5
			nextIFID6 = p.ifid6
			nextIFID7 = p.ifid7
			nextIFID8 = p.ifid8
			nextIDEX = p.idex
			nextIDEX2 = p.idex2
			nextIDEX3 = p.idex3
			nextIDEX4 = p.idex4
			nextIDEX5 = p.idex5
			nextIDEX6 = p.idex6
			nextIDEX7 = p.idex7
			nextIDEX8 = p.idex8
			nextEXMEM = p.exmem
			nextEXMEM2 = p.exmem2
			nextEXMEM3 = p.exmem3
			nextEXMEM4 = p.exmem4
			nextEXMEM5 = p.exmem5
			nextEXMEM6 = p.exmem6
			nextEXMEM7 = p.exmem7
			nextEXMEM8 = p.exmem8
		}
	} else if (stallResult.StallIF || memStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		nextIFID3 = p.ifid3
		nextIFID4 = p.ifid4
		nextIFID5 = p.ifid5
		nextIFID6 = p.ifid6
		nextIFID7 = p.ifid7
		nextIFID8 = p.ifid8
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
		p.memwb2 = nextMEMWB2
		p.memwb3 = nextMEMWB3
		p.memwb4 = nextMEMWB4
		p.memwb5 = nextMEMWB5
		p.memwb6 = nextMEMWB6
		p.memwb7 = nextMEMWB7
		p.memwb8 = nextMEMWB8
	} else {
		p.memwb.Clear()
		p.memwb2.Clear()
		p.memwb3.Clear()
		p.memwb4.Clear()
		p.memwb5.Clear()
		p.memwb6.Clear()
		p.memwb7.Clear()
		p.memwb8.Clear()
	}
	if !memStall {
		p.exmem = nextEXMEM
		p.exmem2 = nextEXMEM2
		p.exmem3 = nextEXMEM3
		p.exmem4 = nextEXMEM4
		p.exmem5 = nextEXMEM5
		p.exmem6 = nextEXMEM6
		p.exmem7 = nextEXMEM7
		p.exmem8 = nextEXMEM8
	}
	if stallResult.InsertBubbleEX && !memStall {
		p.idex.Clear()
		p.idex2.Clear()
		p.idex3.Clear()
		p.idex4.Clear()
		p.idex5.Clear()
		p.idex6.Clear()
		p.idex7.Clear()
		p.idex8.Clear()
	} else if !memStall {
		p.idex = nextIDEX
		p.idex2 = nextIDEX2
		p.idex3 = nextIDEX3
		p.idex4 = nextIDEX4
		p.idex5 = nextIDEX5
		p.idex6 = nextIDEX6
		p.idex7 = nextIDEX7
		p.idex8 = nextIDEX8
	}
	p.ifid = nextIFID
	p.ifid2 = nextIFID2
	p.ifid3 = nextIFID3
	p.ifid4 = nextIFID4
	p.ifid5 = nextIFID5
	p.ifid6 = nextIFID6
	p.ifid7 = nextIFID7
	p.ifid8 = nextIFID8
}
