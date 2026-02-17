package pipeline

import (
	"github.com/sarchlab/m2sim/insts"
)

// tickSingleIssue is the original single-issue pipeline tick.
func (p *Pipeline) tickSingleIssue() {
	// Detect hazards before executing stages
	forwarding := p.hazardUnit.DetectForwarding(&p.idex, &p.exmem, &p.memwb)

	// Track data hazards (RAW hazards resolved by forwarding)
	if forwarding.ForwardRn != ForwardNone || forwarding.ForwardRm != ForwardNone || forwarding.ForwardRd != ForwardNone {
		p.stats.DataHazards++
	}

	// Detect load-use hazards between EX stage (ID/EX) and ID stage (IF/ID)
	// Load-use hazards require a stall because the loaded value isn't available
	// until after the MEM stage, so it can't be forwarded in time for EX.
	// ALU-to-ALU dependencies are handled by forwarding (no stall needed).
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 && p.ifid.Valid {
		// Peek at the next instruction to check for load-use hazard
		p.decodeStage.decoder.DecodeInto(p.ifid.InstructionWord, &p.hazardScratchInst)
		nextInst := &p.hazardScratchInst
		if nextInst.Op != insts.OpUnknown {
			usesRn := true                                 // Most instructions use Rn
			usesRm := nextInst.Format == insts.FormatDPReg // Only register format uses Rm

			// For store instructions, the store data comes from Rd (Rt in AArch64),
			// which can be the destination of a preceding load. Treat Rd as a
			// source register for load-use hazard detection.
			sourceRm := nextInst.Rm
			switch nextInst.Op {
			case insts.OpSTR, insts.OpSTRQ:
				usesRm = true
				sourceRm = nextInst.Rd
			}

			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd,
				nextInst.Rn,
				sourceRm,
				usesRn,
				usesRm,
			)
			// Note: stall cycles for load-use hazards are counted in the fetch
			// stage when the pipeline is actually stalled (see StallIF handling),
			// so we do not increment p.stats.Stalls here to avoid double-counting.
		}
	}

	// Branch prediction tracking
	branchMispredicted := false
	var branchTarget uint64

	// Stage 5: Writeback (using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
	}

	// Stage 4: Memory
	var nextMEMWB MEMWBRegister
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
			if p.exmem.MemRead || p.exmem.MemWrite {
				if p.memPending && p.memPendingPC != p.exmem.PC {
					p.memPending = false
				}
				if !p.memPending {
					p.memPending = true
					p.memPendingPC = p.exmem.PC
					memStall = true
					p.stats.MemStalls++
				} else {
					p.memPending = false
					memResult = p.memoryStage.Access(&p.exmem)
				}
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

	// Stage 3: Execute
	var nextEXMEM EXMEMRegister
	execStall := false
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

			// Check for PSTATE flag forwarding from EXMEM stage.
			// This fixes the timing hazard where CMP sets PSTATE at cycle END
			// but B.cond reads at cycle START, causing stale flag reads.
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				// Check if previous instruction (now in EXMEM) sets flags
				if p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem.FlagN
					fwdZ = p.exmem.FlagZ
					fwdC = p.exmem.FlagC
					fwdV = p.exmem.FlagV
				}
			}

			execResult := p.executeStage.ExecuteWithFlags(&p.idex, rnValue, rmValue,
				forwardFlags, fwdN, fwdZ, fwdC, fwdV)

			// Handle branch prediction verification
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				// Use the prediction info that was captured at fetch time (stored in IDEX).
				// This correctly reflects what PC was used for the next fetch.
				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				// Determine if misprediction occurred
				wasMispredicted := false
				if actualTaken {
					if !predictedTaken {
						// Predicted not taken, but was taken
						wasMispredicted = true
					} else if predictedTarget != actualTarget {
						// Predicted taken but to wrong target
						wasMispredicted = true
					}
					// Note: If earlyResolved is true and we reach here, the prediction
					// was correct (unconditional branch correctly resolved at fetch).
				} else {
					if predictedTaken {
						// Predicted taken, but was not taken
						wasMispredicted = true
					}
				}

				// For early-resolved unconditional branches, we should always be correct
				// (they are always taken and we computed the exact target at fetch).
				if earlyResolved && actualTaken {
					wasMispredicted = false // Double-check: early resolution is always correct
				}

				// Update predictor with actual outcome (for BTB training)
				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchMispredicted = true
					branchTarget = actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4 // Continue to next instruction
					}
				} else {
					p.stats.BranchCorrect++
					// Correct prediction - no flush needed!
				}
			}

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
				// Store computed flags for forwarding to dependent B.cond
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}
		}
	}

	// Compute stall signals
	// Memory stalls should also stall upstream stages
	// Note: Only load-use hazards require stalls. ALU-to-ALU dependencies
	// are resolved through forwarding without stalling the pipeline.
	// Branch mispredictions cause flushes, correct predictions don't.
	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, branchMispredicted)

	// Stage 1: Fetch (need to process fetch first to check for fetch stalls)
	var nextIFID IFIDRegister
	fetchStall := false
	if !stallResult.StallIF && !stallResult.FlushIF && !memStall {
		var word uint32
		var ok bool

		if p.useICache && p.cachedFetchStage != nil {
			word, ok, fetchStall = p.cachedFetchStage.Fetch(p.pc)
			if fetchStall {
				p.stats.Stalls++
			}
		} else {
			word, ok = p.fetchStage.Fetch(p.pc)
		}

		if ok && !fetchStall {
			// Branch elimination: unconditional B (not BL) instructions are
			// eliminated at fetch time. They never enter the pipeline, matching
			// Apple M2's behavior where B instructions never issue.
			if isEliminableBranch(word) {
				_, uncondTarget := isUnconditionalBranch(word, p.pc)
				p.pc = uncondTarget
				p.stats.EliminatedBranches++
				// Don't create IFID entry - branch is eliminated
				// nextIFID remains empty (Valid=false)
			} else {
				// Early branch resolution: detect unconditional branches (B, BL) and
				// resolve them immediately without waiting for BTB. This eliminates
				// misprediction penalties for unconditional branches.
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, p.pc)

				// Use branch predictor for conditional branches
				pred := p.branchPredictor.Predict(p.pc)

				// For unconditional branches, override prediction with actual target
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, p.pc)

				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              p.pc,
					InstructionWord: word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}

				// Speculative fetch: redirect PC based on prediction/resolution
				if pred.Taken && pred.TargetKnown {
					p.pc = pred.Target
				} else {
					p.pc += 4 // Default: sequential fetch
				}
			}
		} else if fetchStall {
			nextIFID = p.ifid
		}
	} else if (stallResult.StallIF || memStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		p.stats.Stalls++
	}

	// Stage 2: Decode
	// Note: When fetch stalls, we must NOT decode because ifid is preserved
	// for next cycle. If we decode now, the instruction would be executed twice.
	var nextIDEX IDEXRegister
	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall && !fetchStall {
		decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)
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
	} else if (stallResult.StallID || execStall || memStall || fetchStall) && !stallResult.FlushID {
		nextIDEX = p.idex
	}

	// Handle branch misprediction: update PC and flush pipeline
	// Note: Only mispredictions cause flushes. Correct predictions don't need flushing.
	if branchMispredicted {
		p.pc = branchTarget
		nextIFID.Clear()
		nextIDEX.Clear()
		p.stats.Flushes++
	}

	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
	} else {
		p.memwb.Clear()
	}
	if !execStall && !memStall && !fetchStall {
		p.exmem = nextEXMEM
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall && !fetchStall {
		p.idex.Clear()
	} else if !memStall && !fetchStall {
		p.idex = nextIDEX
	}
	if !fetchStall {
		p.ifid = nextIFID
	}
}

// tickSuperscalar executes one cycle with dual-issue support.
// Independent instructions are executed in parallel when possible.

func (p *Pipeline) tickSuperscalar() {
	// Stage 5: Writeback (both slots using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
	}
	// Writeback secondary slot
	if p.writebackStage.WritebackSlot(&p.memwb2) {
		p.stats.Instructions++
	}

	// Stage 4: Memory (primary slot only - single memory port)
	var nextMEMWB MEMWBRegister
	var nextMEMWB2 SecondaryMEMWBRegister
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
			if p.exmem.MemRead || p.exmem.MemWrite {
				if p.memPending && p.memPendingPC != p.exmem.PC {
					p.memPending = false
				}
				if !p.memPending {
					p.memPending = true
					p.memPendingPC = p.exmem.PC
					memStall = true
					p.stats.MemStalls++
				} else {
					p.memPending = false
					memResult = p.memoryStage.Access(&p.exmem)
				}
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
	// Track whether primary port already counted this stall cycle
	primaryStalled := memStall
	memStall = memStall || memStall2
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

	// Stage 3: Execute (both slots)
	var nextEXMEM EXMEMRegister
	var nextEXMEM2 SecondaryEXMEMRegister
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

			// Forward from secondary pipeline stages (exmem2, memwb2) to primary slot
			// When pairs dual-issue, primary slot may need values from previous secondary execution
			if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 {
				if p.idex.Rn == p.memwb2.Rd {
					rnValue = p.memwb2.ALUResult
				}
				if p.idex.Rm == p.memwb2.Rd {
					rmValue = p.memwb2.ALUResult
				}
			}
			if p.exmem2.Valid && p.exmem2.RegWrite && p.exmem2.Rd != 31 {
				if p.idex.Rn == p.exmem2.Rd {
					rnValue = p.exmem2.ALUResult
				}
				if p.idex.Rm == p.exmem2.Rd {
					rmValue = p.exmem2.ALUResult
				}
			}

			// Check for PSTATE flag forwarding from all EXMEM stages (dual-issue).
			forwardFlags := false
			var fwdN, fwdZ, fwdC, fwdV bool
			if p.idex.Inst != nil && p.idex.Inst.Op == insts.OpBCond && !p.idex.IsFused {
				if p.exmem.Valid && p.exmem.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem.FlagN
					fwdZ = p.exmem.FlagZ
					fwdC = p.exmem.FlagC
					fwdV = p.exmem.FlagV
				} else if p.exmem2.Valid && p.exmem2.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem2.FlagN
					fwdZ = p.exmem2.FlagZ
					fwdC = p.exmem2.FlagC
					fwdV = p.exmem2.FlagV
				}
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
				// Store computed flags for forwarding
				SetsFlags: execResult.SetsFlags,
				FlagN:     execResult.FlagN,
				FlagZ:     execResult.FlagZ,
				FlagC:     execResult.FlagC,
				FlagV:     execResult.FlagV,
			}

			// Branch prediction verification for primary slot (same logic as single-issue)
			if p.idex.IsBranch {
				actualTaken := execResult.BranchTaken
				actualTarget := execResult.BranchTarget

				p.stats.BranchPredictions++

				// Use prediction info captured at fetch time
				predictedTaken := p.idex.PredictedTaken
				predictedTarget := p.idex.PredictedTarget
				earlyResolved := p.idex.EarlyResolved

				// Determine if misprediction occurred
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

				// Early-resolved unconditional branches should always be correct
				if earlyResolved && actualTaken {
					wasMispredicted = false
				}

				// Update predictor
				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4
					}
					p.pc = branchTarget
					p.ifid.Clear()
					p.ifid2.Clear()
					p.idex.Clear()
					p.idex2.Clear()
					p.stats.Flushes++

					// Latch results and return early
					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.exmem = nextEXMEM
						p.exmem2.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute secondary slot (if not stalled and slot is valid)
	if p.idex2.Valid && !memStall && !execStall {
		// Convert to IDEXRegister for hazard detection
		idex2 := p.idex2.toIDEX()

		// Detect forwarding for secondary slot from primary pipeline stages
		forwarding2 := p.hazardUnit.DetectForwarding(&idex2, &p.exmem, &p.memwb)

		// Also check forwarding from primary execute result (same cycle)
		if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
			if p.idex2.Rn == nextEXMEM.Rd {
				forwarding2.ForwardRn = ForwardFromEXMEM
			}
			if p.idex2.Rm == nextEXMEM.Rd {
				forwarding2.ForwardRm = ForwardFromEXMEM
			}
		}

		if p.exLatency2 == 0 {
			p.exLatency2 = p.getExLatency(p.idex2.Inst)
		}

		if p.exLatency2 > 0 {
			p.exLatency2--
		}

		if p.exLatency2 == 0 {
			// Get operand values with forwarding
			// Priority (most recent first): nextEXMEM > exmem/exmem2 > memwb/memwb2 > register
			rnValue := p.idex2.RnValue
			rmValue := p.idex2.RmValue

			// Bug fix: Forward from secondary pipeline stages (exmem2, memwb2)
			// This is needed when consecutive secondary-slot instructions have dependencies.
			// Example: add x1, x1, #1 (→exmem2) followed by add x1, x1, #1 needs x1 from exmem2.

			// First check memwb2 (oldest secondary pipeline stage)
			if p.memwb2.Valid && p.memwb2.RegWrite && p.memwb2.Rd != 31 {
				if p.idex2.Rn == p.memwb2.Rd {
					rnValue = p.memwb2.ALUResult
				}
				if p.idex2.Rm == p.memwb2.Rd {
					rmValue = p.memwb2.ALUResult
				}
			}

			// Then check primary memwb (same age as memwb2, but different register)
			rnValue = p.hazardUnit.GetForwardedValue(
				forwarding2.ForwardRn, rnValue, &p.exmem, &savedMEMWB)
			rmValue = p.hazardUnit.GetForwardedValue(
				forwarding2.ForwardRm, rmValue, &p.exmem, &savedMEMWB)

			// Then check exmem2 (newer than memwb2, same priority as exmem)
			if p.exmem2.Valid && p.exmem2.RegWrite && p.exmem2.Rd != 31 {
				if p.idex2.Rn == p.exmem2.Rd {
					rnValue = p.exmem2.ALUResult
				}
				if p.idex2.Rm == p.exmem2.Rd {
					rmValue = p.exmem2.ALUResult
				}
			}

			// Finally check nextEXMEM (current cycle - highest priority)
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex2.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex2.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}

			execResult := p.executeStage.Execute(&idex2, rnValue, rmValue)

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
		}
	}

	// Detect load-use hazards for primary decode
	loadUseHazard := false
	if p.idex.Valid && p.idex.MemRead && p.idex.Rd != 31 && p.ifid.Valid {
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

			loadUseHazard = p.hazardUnit.DetectLoadUseHazardDecoded(
				p.idex.Rd, nextInst.Rn, sourceRm, usesRn, usesRm)
		}
	}

	stallResult := p.hazardUnit.ComputeStalls(loadUseHazard || execStall || memStall, false)

	// Stage 2: Decode (both slots)
	var nextIDEX IDEXRegister
	var nextIDEX2 SecondaryIDEXRegister

	if p.ifid.Valid && !stallResult.StallID && !stallResult.FlushID && !execStall && !memStall {
		decResult := p.decodeStage.Decode(p.ifid.InstructionWord, p.ifid.PC)
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

		// Decode secondary slot if available
		if p.ifid2.Valid {
			decResult2 := p.decodeStage.Decode(p.ifid2.InstructionWord, p.ifid2.PC)
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

			// Check if we can dual-issue these two instructions
			if canDualIssue(&nextIDEX, &tempIDEX2) {
				nextIDEX2.fromIDEX(&tempIDEX2)
			}
			// If cannot dual-issue, secondary slot remains clear and the
			// instruction at ifid2 will naturally flow through in the next cycle
			// (ifid2 becomes ifid when we only advance by 4 bytes)
		}
	} else if (stallResult.StallID || execStall || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
		nextIDEX2 = p.idex2
	}

	// Stage 1: Fetch (both slots)
	var nextIFID IFIDRegister
	var nextIFID2 SecondaryIFIDRegister
	fetchStall := false

	// Check if we successfully dual-issued in decode stage this cycle
	// If the secondary decode slot wasn't used, the instruction at ifid2
	// needs to become the next ifid (we only consumed one instruction)
	dualIssued := nextIDEX2.Valid

	if !stallResult.StallIF && !stallResult.FlushIF && !memStall && !execStall {
		// If we didn't dual-issue last decode, the second instruction (ifid2)
		// becomes the first instruction for this cycle
		if p.ifid2.Valid && !dualIssued {
			// Carry over the second fetched instruction to the primary slot
			// (including its prediction info)
			nextIFID = IFIDRegister{
				Valid:           true,
				PC:              p.ifid2.PC,
				InstructionWord: p.ifid2.InstructionWord,
				PredictedTaken:  p.ifid2.PredictedTaken,
				PredictedTarget: p.ifid2.PredictedTarget,
				EarlyResolved:   p.ifid2.EarlyResolved,
			}
			// Fetch a new second instruction
			var word2 uint32
			var ok2 bool
			var stall2 bool
			if p.useICache && p.cachedFetchStage != nil {
				word2, ok2, stall2 = p.cachedFetchStage.Fetch(p.pc)
				if stall2 {
					fetchStall = true
					p.stats.Stalls++
				}
			} else {
				word2, ok2 = p.fetchStage.Fetch(p.pc)
			}
			if ok2 && !stall2 {
				// Branch elimination for secondary slot
				if isEliminableBranch(word2) {
					_, uncondTarget2 := isUnconditionalBranch(word2, p.pc)
					p.pc = uncondTarget2
					p.stats.EliminatedBranches++
					// Don't create IFID2 entry
				} else {
					// Apply branch prediction to secondary slot
					isUncondBranch2, uncondTarget2 := isUnconditionalBranch(word2, p.pc)
					pred2 := p.branchPredictor.Predict(p.pc)
					earlyResolved2 := false
					if isUncondBranch2 {
						pred2.Taken = true
						pred2.Target = uncondTarget2
						pred2.TargetKnown = true
						earlyResolved2 = true
					}

					nextIFID2 = SecondaryIFIDRegister{
						Valid:           true,
						PC:              p.pc,
						InstructionWord: word2,
						PredictedTaken:  pred2.Taken,
						PredictedTarget: pred2.Target,
						EarlyResolved:   earlyResolved2,
					}

					// Handle branch speculation for secondary slot
					if pred2.Taken && pred2.TargetKnown {
						p.pc = pred2.Target
					} else {
						p.pc += 4
					}
				}
			} else if stall2 {
				// When fetch stalls, preserve the entire pipeline state
				nextIFID = p.ifid
				nextIFID2 = p.ifid2
				nextIDEX = p.idex
				nextIDEX2 = p.idex2
				nextEXMEM = p.exmem
				nextEXMEM2 = p.exmem2
			}
		} else {
			// Normal dual-fetch: fetch two new instructions
			var word uint32
			var ok bool

			if p.useICache && p.cachedFetchStage != nil {
				word, ok, fetchStall = p.cachedFetchStage.Fetch(p.pc)
				if fetchStall {
					p.stats.Stalls++
				}
			} else {
				word, ok = p.fetchStage.Fetch(p.pc)
			}

			if ok && !fetchStall {
				// Branch elimination: unconditional B (not BL) instructions are
				// eliminated at fetch time. They never enter the pipeline.
				if isEliminableBranch(word) {
					_, uncondTarget := isUnconditionalBranch(word, p.pc)
					p.pc = uncondTarget
					p.stats.EliminatedBranches++
					// Don't create IFID entry - branch is eliminated
					// Continue fetching from target in next cycle
				} else {
					// Apply branch prediction to primary slot
					isUncondBranch, uncondTarget := isUnconditionalBranch(word, p.pc)
					pred := p.branchPredictor.Predict(p.pc)
					earlyResolved := false
					if isUncondBranch {
						pred.Taken = true
						pred.Target = uncondTarget
						pred.TargetKnown = true
						earlyResolved = true
					}
					enrichPredictionWithEncodedTarget(&pred, word, p.pc)

					nextIFID = IFIDRegister{
						Valid:           true,
						PC:              p.pc,
						InstructionWord: word,
						PredictedTaken:  pred.Taken,
						PredictedTarget: pred.Target,
						EarlyResolved:   earlyResolved,
					}

					// Handle branch speculation for primary slot
					if pred.Taken && pred.TargetKnown {
						// Branch predicted taken - redirect PC
						p.pc = pred.Target
						// Don't fetch second instruction when branching
						p.pc += 0 // PC already set to target
					} else {
						// No branch or not taken - fetch second instruction
						var word2 uint32
						var ok2 bool
						if p.useICache && p.cachedFetchStage != nil {
							word2, ok2, _ = p.cachedFetchStage.Fetch(p.pc + 4)
						} else {
							word2, ok2 = p.fetchStage.Fetch(p.pc + 4)
						}

						if ok2 {
							// Branch elimination for secondary slot
							if isEliminableBranch(word2) {
								_, uncondTarget2 := isUnconditionalBranch(word2, p.pc+4)
								p.pc = uncondTarget2
								p.stats.EliminatedBranches++
								// Don't create IFID2 entry
							} else {
								// Apply branch prediction to secondary slot
								isUncondBranch2, uncondTarget2 := isUnconditionalBranch(word2, p.pc+4)
								pred2 := p.branchPredictor.Predict(p.pc + 4)
								earlyResolved2 := false
								if isUncondBranch2 {
									pred2.Taken = true
									pred2.Target = uncondTarget2
									pred2.TargetKnown = true
									earlyResolved2 = true
								}

								nextIFID2 = SecondaryIFIDRegister{
									Valid:           true,
									PC:              p.pc + 4,
									InstructionWord: word2,
									PredictedTaken:  pred2.Taken,
									PredictedTarget: pred2.Target,
									EarlyResolved:   earlyResolved2,
								}

								// Handle branch speculation for secondary slot
								if pred2.Taken && pred2.TargetKnown {
									p.pc = pred2.Target
								} else {
									p.pc += 8 // Advance PC by 2 instructions
								}
							}
						} else {
							p.pc += 4
						}
					}
				}
			} else if fetchStall {
				nextIFID = p.ifid
				nextIFID2 = p.ifid2
				// When fetch stalls, we must stall the entire pipeline to prevent
				// instructions from being executed twice. If we decoded/executed,
				// the instructions would be executed again when the stall clears.
				nextIDEX = p.idex
				nextIDEX2 = p.idex2
				nextEXMEM = p.exmem
				nextEXMEM2 = p.exmem2
			}
		}
	} else if (stallResult.StallIF || memStall || execStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
		p.memwb2 = nextMEMWB2
	} else {
		p.memwb.Clear()
		p.memwb2.Clear()
	}
	if !execStall && !memStall {
		p.exmem = nextEXMEM
		p.exmem2 = nextEXMEM2
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall {
		p.idex.Clear()
		p.idex2.Clear()
	} else if !memStall {
		p.idex = nextIDEX
		p.idex2 = nextIDEX2
	}
	p.ifid = nextIFID
	p.ifid2 = nextIFID2
}

// tickQuadIssue executes one cycle with 4-wide superscalar support.
// This extends dual-issue to support up to 4 independent instructions per cycle.
//
