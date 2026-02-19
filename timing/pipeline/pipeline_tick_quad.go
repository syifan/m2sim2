package pipeline

import (
	"github.com/sarchlab/m2sim/insts"
)

//nolint:unused // Scaffolding for 4-wide implementation (PR #114)
func (p *Pipeline) tickQuadIssue() {
	// Stage 5: Writeback (all 4 slots using WritebackSlot helper)
	savedMEMWB := p.memwb
	if p.writebackStage.WritebackSlot(&p.memwb) {
		p.stats.Instructions++
	}

	// Writeback secondary slot
	if p.writebackStage.WritebackSlot(&p.memwb2) {
		p.stats.Instructions++
	}

	// Writeback tertiary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb3) {
		p.stats.Instructions++
	}

	// Writeback quaternary slot (using WritebackSlot helper)
	if p.writebackStage.WritebackSlot(&p.memwb4) {
		p.stats.Instructions++
	}

	// Stage 4: Memory (primary slot only - single memory port)
	var nextMEMWB MEMWBRegister
	var nextMEMWB2 SecondaryMEMWBRegister
	var nextMEMWB3 TertiaryMEMWBRegister
	var nextMEMWB4 QuaternaryMEMWBRegister
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

	// Combine stall signals: pipeline stalls if ANY memory port is stalling.
	// Track whether primary port already counted this stall cycle.
	primaryStalled := memStall
	memStall = memStall || memStall2 || memStall3 || memStall4
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

	// Stage 3: Execute (all 4 slots)
	var nextEXMEM EXMEMRegister
	var nextEXMEM2 SecondaryEXMEMRegister
	var nextEXMEM3 TertiaryEXMEMRegister
	var nextEXMEM4 QuaternaryEXMEMRegister
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

			// Check for PSTATE flag forwarding from all EXMEM stages (quad-issue).
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
				} else if p.exmem3.Valid && p.exmem3.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem3.FlagN
					fwdZ = p.exmem3.FlagZ
					fwdC = p.exmem3.FlagC
					fwdV = p.exmem3.FlagV
				} else if p.exmem4.Valid && p.exmem4.SetsFlags {
					forwardFlags = true
					fwdN = p.exmem4.FlagN
					fwdZ = p.exmem4.FlagZ
					fwdC = p.exmem4.FlagC
					fwdV = p.exmem4.FlagV
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

			// Branch prediction verification for primary slot
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

				// Update predictor with actual outcome
				p.branchPredictor.Update(p.idex.PC, actualTaken, actualTarget)

				if wasMispredicted {
					p.stats.BranchMispredictions++
					branchTarget := actualTarget
					if !actualTaken {
						branchTarget = p.idex.PC + 4
					}
					p.pc = branchTarget
					p.flushAllIFID()
					p.flushAllIDEX()
					p.stats.Flushes++

					// Latch results and return early
					if !memStall {
						p.memwb = nextMEMWB
						p.memwb2 = nextMEMWB2
						p.memwb3 = nextMEMWB3
						p.memwb4 = nextMEMWB4
						p.exmem = nextEXMEM
						p.exmem2.Clear()
						p.exmem3.Clear()
						p.exmem4.Clear()
					}
					return
				}
				p.stats.BranchCorrect++
			}
		}
	}

	// Execute secondary slot (if not stalled and slot is valid)
	if p.idex2.Valid && !memStall && !execStall {
		if p.exLatency2 == 0 {
			p.exLatency2 = p.getExLatency(p.idex2.Inst)
		}

		if p.exLatency2 > 0 {
			p.exLatency2--
		}

		if p.exLatency2 == 0 {
			rnValue := p.idex2.RnValue
			rmValue := p.idex2.RmValue

			// Forward from all pipeline stages
			rnValue = p.forwardFromAllSlots(p.idex2.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex2.Rm, rmValue)

			// Forward from current cycle's primary execution
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex2.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex2.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}

			idex2 := p.idex2.toIDEX()
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

	// Execute tertiary slot
	if p.idex3.Valid && !memStall && !execStall {
		if p.exLatency3 == 0 {
			p.exLatency3 = p.getExLatency(p.idex3.Inst)
		}

		if p.exLatency3 > 0 {
			p.exLatency3--
		}

		if p.exLatency3 == 0 {
			rnValue := p.idex3.RnValue
			rmValue := p.idex3.RmValue

			// Forward from all pipeline stages
			rnValue = p.forwardFromAllSlots(p.idex3.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex3.Rm, rmValue)

			// Forward from current cycle's earlier executions
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex3.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex3.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex3.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex3.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}

			idex3 := p.idex3.toIDEX()
			execResult := p.executeStage.Execute(&idex3, rnValue, rmValue)

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
		}
	}

	// Execute quaternary slot
	if p.idex4.Valid && !memStall && !execStall {
		if p.exLatency4 == 0 {
			p.exLatency4 = p.getExLatency(p.idex4.Inst)
		}

		if p.exLatency4 > 0 {
			p.exLatency4--
		}

		if p.exLatency4 == 0 {
			rnValue := p.idex4.RnValue
			rmValue := p.idex4.RmValue

			// Forward from all pipeline stages
			rnValue = p.forwardFromAllSlots(p.idex4.Rn, rnValue)
			rmValue = p.forwardFromAllSlots(p.idex4.Rm, rmValue)

			// Forward from current cycle's earlier executions
			if nextEXMEM.Valid && nextEXMEM.RegWrite && nextEXMEM.Rd != 31 {
				if p.idex4.Rn == nextEXMEM.Rd {
					rnValue = nextEXMEM.ALUResult
				}
				if p.idex4.Rm == nextEXMEM.Rd {
					rmValue = nextEXMEM.ALUResult
				}
			}
			if nextEXMEM2.Valid && nextEXMEM2.RegWrite && nextEXMEM2.Rd != 31 {
				if p.idex4.Rn == nextEXMEM2.Rd {
					rnValue = nextEXMEM2.ALUResult
				}
				if p.idex4.Rm == nextEXMEM2.Rd {
					rmValue = nextEXMEM2.ALUResult
				}
			}
			if nextEXMEM3.Valid && nextEXMEM3.RegWrite && nextEXMEM3.Rd != 31 {
				if p.idex4.Rn == nextEXMEM3.Rd {
					rnValue = nextEXMEM3.ALUResult
				}
				if p.idex4.Rm == nextEXMEM3.Rd {
					rmValue = nextEXMEM3.ALUResult
				}
			}

			idex4 := p.idex4.toIDEX()
			execResult := p.executeStage.Execute(&idex4, rnValue, rmValue)

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

	// Stage 2: Decode (all 4 slots)
	var nextIDEX IDEXRegister
	var nextIDEX2 SecondaryIDEXRegister
	var nextIDEX3 TertiaryIDEXRegister
	var nextIDEX4 QuaternaryIDEXRegister

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

		// Try to issue instructions 2, 3, 4 if they can issue with earlier instructions.
		// Uses fixed-size array to avoid heap allocation per tick.
		var issuedInsts [8]*IDEXRegister
		var issued [8]bool
		issuedInsts[0] = &nextIDEX
		issued[0] = true
		issuedCount := 1

		// Decode slot 2
		// OoO-style issue: each slot independently checks canIssueWith().
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

			if canIssueWith(&tempIDEX2, &issuedInsts, issuedCount, &issued, p.useDCache) {
				nextIDEX2.fromIDEX(&tempIDEX2)
				issued[issuedCount] = true
			}
			issuedInsts[issuedCount] = &tempIDEX2
			issuedCount++
		}

		// Decode slot 3
		if p.ifid3.Valid {
			decResult3 := p.decodeStage.Decode(p.ifid3.InstructionWord, p.ifid3.PC)
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

			if canIssueWith(&tempIDEX3, &issuedInsts, issuedCount, &issued, p.useDCache) {
				nextIDEX3.fromIDEX(&tempIDEX3)
				issued[issuedCount] = true
			}
			issuedInsts[issuedCount] = &tempIDEX3
			issuedCount++
		}

		// Decode slot 4
		if p.ifid4.Valid {
			decResult4 := p.decodeStage.Decode(p.ifid4.InstructionWord, p.ifid4.PC)
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

			if canIssueWith(&tempIDEX4, &issuedInsts, issuedCount, &issued, p.useDCache) {
				nextIDEX4.fromIDEX(&tempIDEX4)
			}
		}
	} else if (stallResult.StallID || execStall || memStall) && !stallResult.FlushID {
		nextIDEX = p.idex
		nextIDEX2 = p.idex2
		nextIDEX3 = p.idex3
		nextIDEX4 = p.idex4
	}

	// Track which IFID slots were consumed (issued to IDEX) for fetch re-queuing
	var consumed [8]bool
	consumed[0] = nextIDEX.Valid
	consumed[1] = nextIDEX2.Valid
	consumed[2] = nextIDEX3.Valid
	consumed[3] = nextIDEX4.Valid

	// Stage 1: Fetch (all 4 slots)
	var nextIFID IFIDRegister
	var nextIFID2 SecondaryIFIDRegister
	var nextIFID3 TertiaryIFIDRegister
	var nextIFID4 QuaternaryIFIDRegister
	fetchStall := false

	if !stallResult.StallIF && !stallResult.FlushIF && !memStall && !execStall {
		// Shift unissued instructions forward
		pendingInsts, pendingCount := p.collectPendingFetchInstructionsSelective(consumed[:4])

		// Fill slots with pending instructions first, then fetch new ones
		fetchPC := p.pc
		slotIdx := 0

		// Place pending instructions
		branchPredictedTaken := false
		for pi := 0; pi < pendingCount; pi++ {
			pending := pendingInsts[pi]
			// If slot 0 had a predicted-taken branch, discard remaining pending instructions
			// (they're on the wrong path)
			if branchPredictedTaken {
				break
			}
			switch slotIdx {
			case 0:
				// Apply branch prediction when placing in primary slot
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              pending.PC,
					InstructionWord: pending.Word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}
				// If branch predicted taken in slot 0, redirect fetch and discard other pending
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			default:
				isUncondBranch, uncondTarget := isUnconditionalBranch(pending.Word, pending.PC)
				pred := p.branchPredictor.Predict(pending.PC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, pending.Word, pending.PC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: pending.PC, InstructionWord: pending.Word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					branchPredictedTaken = true
				}
			}
			slotIdx++
		}

		// Fetch new instructions to fill remaining slots
		for slotIdx < 4 {
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

			// Branch elimination: unconditional B (not BL) instructions are
			// eliminated at fetch time. They never enter the pipeline.
			if isEliminableBranch(word) {
				_, uncondTarget := isUnconditionalBranch(word, fetchPC)
				fetchPC = uncondTarget
				p.stats.EliminatedBranches++
				// Don't create IFID entry - branch is eliminated
				// Continue fetching from target without advancing slotIdx
				continue
			}

			// Apply branch prediction for slot 0 (branches can only execute from primary slot)
			if slotIdx == 0 {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)

				nextIFID = IFIDRegister{
					Valid:           true,
					PC:              fetchPC,
					InstructionWord: word,
					PredictedTaken:  pred.Taken,
					PredictedTarget: pred.Target,
					EarlyResolved:   earlyResolved,
				}

				// If branch predicted taken, redirect fetch to target
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			} else {
				isUncondBranch, uncondTarget := isUnconditionalBranch(word, fetchPC)
				pred := p.branchPredictor.Predict(fetchPC)
				earlyResolved := false
				if isUncondBranch {
					pred.Taken = true
					pred.Target = uncondTarget
					pred.TargetKnown = true
					earlyResolved = true
				}
				enrichPredictionWithEncodedTarget(&pred, word, fetchPC)
				switch slotIdx {
				case 1:
					nextIFID2 = SecondaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 2:
					nextIFID3 = TertiaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				case 3:
					nextIFID4 = QuaternaryIFIDRegister{Valid: true, PC: fetchPC, InstructionWord: word, PredictedTaken: pred.Taken, PredictedTarget: pred.Target, EarlyResolved: earlyResolved}
				}
				if pred.Taken && pred.TargetKnown {
					fetchPC = pred.Target
					slotIdx++
					continue
				}
			}
			fetchPC += 4
			slotIdx++
		}
		p.pc = fetchPC

		if fetchStall {
			// Preserve all pipeline state on fetch stall
			nextIFID = p.ifid
			nextIFID2 = p.ifid2
			nextIFID3 = p.ifid3
			nextIFID4 = p.ifid4
			nextIDEX = p.idex
			nextIDEX2 = p.idex2
			nextIDEX3 = p.idex3
			nextIDEX4 = p.idex4
			nextEXMEM = p.exmem
			nextEXMEM2 = p.exmem2
			nextEXMEM3 = p.exmem3
			nextEXMEM4 = p.exmem4
		}
	} else if (stallResult.StallIF || memStall || execStall) && !stallResult.FlushIF {
		nextIFID = p.ifid
		nextIFID2 = p.ifid2
		nextIFID3 = p.ifid3
		nextIFID4 = p.ifid4
		p.stats.Stalls++
	}

	// Latch all pipeline registers
	if !memStall && !fetchStall {
		p.memwb = nextMEMWB
		p.memwb2 = nextMEMWB2
		p.memwb3 = nextMEMWB3
		p.memwb4 = nextMEMWB4
	} else {
		p.memwb.Clear()
		p.memwb2.Clear()
		p.memwb3.Clear()
		p.memwb4.Clear()
	}
	if !execStall && !memStall {
		p.exmem = nextEXMEM
		p.exmem2 = nextEXMEM2
		p.exmem3 = nextEXMEM3
		p.exmem4 = nextEXMEM4
	}
	if stallResult.InsertBubbleEX && !execStall && !memStall {
		p.idex.Clear()
		p.idex2.Clear()
		p.idex3.Clear()
		p.idex4.Clear()
	} else if !memStall {
		p.idex = nextIDEX
		p.idex2 = nextIDEX2
		p.idex3 = nextIDEX3
		p.idex4 = nextIDEX4
	}
	p.ifid = nextIFID
	p.ifid2 = nextIFID2
	p.ifid3 = nextIFID3
	p.ifid4 = nextIFID4
}
