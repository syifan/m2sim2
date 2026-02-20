# Stall Analysis: arithmetic and branchheavy benchmarks

Issue #25 — Profile-only cycle (no code changes).

## Summary

| Benchmark | Sim CPI | HW CPI | Error | Direction |
|-----------|---------|--------|-------|-----------|
| arithmetic_sequential | 0.220 | 0.296 | 34.5% | sim too FAST |
| branch_heavy | 0.970 | 0.714 | 35.8% | sim too SLOW |

## 1. arithmetic_sequential (sim CPI 0.220, hw CPI 0.296)

### Instruction mix
- 200 `ADD Xn, Xn, #1` instructions cycling through 5 registers (X0-X4)
- No branches, no memory operations
- Pattern: X0, X1, X2, X3, X4, X0, X1, X2, X3, X4, ... (repeat 40×)
- Final: SVC (exit)

### Stall profile
```
Cycles:                     44
Instructions Retired:       200
IPC:                        4.545  (effective 5/cycle in steady state)
RAW Hazard Stalls:          0
Structural Hazard Stalls:   125  (3 per cycle avg — inst 5,6,7 blocked)
Exec Stalls:                0
Mem Stalls:                 0
Branch Mispred Stalls:      0
Pipeline Flushes:           0
```

### Root cause analysis
The sim issues 5 instructions per cycle because:
- Slots 0-4: ADD X0..X4 — all independent, co-issue OK
- Slots 5-7: ADD X0..X2 — RAW hazard on X0/X1/X2 from slots 0-2
- `canIssueWithFwd()` blocks DPImm→DPImm same-cycle forwarding (line 1163: "serial integer chains at 1/cycle on M2")
- So 3 instructions per cycle are rejected (125 structural stall events over ~40 issue cycles)

Effective throughput: 200 insts / (44 - 4 pipeline fill) = 5.0 IPC → CPI 0.200 (steady-state)

The native benchmark (`arithmetic_sequential_long.s`) uses a **loop** with the same 20 ADD body:
```asm
.loop:
    20 ADDs (5 regs × 4 groups)
    add x10, x10, #1    // loop counter
    cmp x10, x11        // compare
    b.lt .loop           // branch
```
Each iteration: 23 instructions (20 ADDs + 3 loop overhead). The loop overhead adds:
- Branch misprediction on final iteration exit
- CMP→B.LT dependency chain (1+ cycle)
- Fetch redirect latency at loop boundary

This structural mismatch (unrolled sim vs looped native) explains ~50% of the error. The remaining gap may be from M2's decode bandwidth constraints and rename/dispatch overhead.

### Comparison: arithmetic_8wide (uses 8 registers)
- CPI = 0.278 (only 6.6% error vs hw 0.296!)
- With 8 registers, the 8-wide pipeline can issue 8 per cycle with no same-cycle RAW
- Confirms the 5-register limitation is the core issue for arithmetic_sequential

### Hypothesis: Why sim is too fast
1. **Benchmark structure mismatch**: Sim benchmark is pure straight-line code (200 ADDs, no loop). Native benchmark has a tight loop with 3 instructions of overhead per 20 ADDs, increasing effective CPI by ~15%.
2. **Missing frontend effects**: Real M2 has fetch group alignment constraints, decode-rename pipeline stages (~4 stages before dispatch), and potential front-end bubbles at fetch redirections.
3. **5-register pattern allows 5-wide issue**: With perfect forwarding from prior cycle, the sim achieves 5 IPC. M2's OoO backend may have additional scheduling constraints.

### Proposed fix direction (DO NOT implement)
- **Option A**: Restructure `arithmeticSequential()` to include a loop (matching native benchmark structure). This would add branch overhead and reduce IPC.
- **Option B**: Add 1-2 cycles of frontend/decode latency to model the rename/dispatch stages of real M2.
- **Option C**: Tighten the DPImm→DPImm forwarding gate further — but this risks regressing other benchmarks.

**Recommended**: Option A (restructure benchmark). The 8-wide variant already shows 6.6% error, proving the pipeline model is fundamentally sound. The error is primarily a benchmark structure mismatch.

---

## 2. branch_heavy (sim CPI 0.970, hw CPI 0.714)

### Instruction mix
- 10 branch blocks, each: `CMP Xn, Xm` + `B.LT +8` + `ADD (skipped or executed)` + `ADD X0, X0, #1`
- Blocks 1-5: B.LT taken (X0 < 5), skips 1 instruction → 3 instructions executed per block
- Blocks 6-10: B.LT not taken (X0 >= 5), falls through → 4 instructions per block
- Total instructions executed: 5×3 + 5×4 = 35, reported as 33 retired (CMP+B.cond fusion counts as 2)
- 10 unique branch PCs (no loop, each branch executed once → all cold in predictor)

### Stall profile
```
Cycles:                     32
Instructions Retired:       33
IPC:                        1.031
Branch Predictions:         10  (5 correct + 5 mispredicted)
Branch Mispredictions:      5   (all 5 forward-taken branches)
Branch Mispred Stalls:      10  (2 cycles × 5 mispredictions)
Structural Hazard Stalls:   116
Pipeline Flushes:           5
```

### Root cause analysis

**Primary cause: Cold branch mispredictions (10 stall cycles / 32 total = 31%)**

The branch predictor uses a tournament predictor (bimodal + gshare + choice). All counters initialize to 0, so `bimodalTaken = (counter >= 2) = false`. For cold PCs, the predictor always predicts **not-taken**.

- Branches 1-5 are forward-taken (B.LT to skip an instruction) → ALL mispredicted
- Branches 6-10 are not-taken → ALL correctly predicted
- 5 mispredictions × 2-cycle flush penalty = 10 cycles

**Without mispredictions**: 32 - 10 = 22 cycles → CPI = 22/33 = 0.667 (within 6.6% of hw 0.714!)

**Secondary cause: Branch serialization (branches only in slot 0)**

`canIssueWithFwd()` line 1003: "Cannot issue branches in superscalar mode (only in slot 0)". This means:
- Each CMP+B.cond fusion occupies slot 0
- Only non-branch instructions in the target path can fill slots 1-7
- But after a taken branch, the target instruction (ADD X0) is alone in the next fetch group
- This wastes most of the 8-wide bandwidth: 116 structural hazard events

**Tertiary: CMP+B.cond fusion works but only in slot 0**

The CMP+B.cond fusion correctly identifies CMP in slot 0 followed by B.cond in slot 1, fusing them into a single operation in slot 0. This eliminates 1 instruction of overhead per branch, but still constrains throughput to 1 branch per cycle.

### Why real M2 achieves CPI 0.714
On real M2 hardware:
- M2 uses TAGE-like predictor with much better cold-start behavior
- M2 may predict 2-3 fewer mispredictions through heuristics or biased initial counters
- M2 has OoO execution that can overlap branch resolution with later instructions
- M2 can execute branches in multiple ports (not just slot 0)
- With ~2-3 mispredictions at ~5-7 cycle penalty, plus better IPC between branches → CPI ≈ 0.714

### Hypothesis: Why sim is too slow
1. **Too many branch mispredictions**: 5/10 branches mispredicted (50% rate) due to always-not-taken default for cold branches. Real M2 likely mispredicts only 2-3 of these.
2. **Branch-only-in-slot-0 constraint**: Severely limits throughput for branch-dense code. Real M2 can execute branches in multiple execution units.
3. **Misprediction penalty (2 cycles) is actually LOW for our 5-stage pipeline**: The penalty isn't the issue — the NUMBER of mispredictions is.

### Proposed fix direction (DO NOT implement)
- **Option A (highest impact)**: Improve cold branch prediction. Ideas:
  - Initialize bimodal counters to 1 (weakly not-taken) instead of 0 (strongly not-taken). This means only 1 taken branch is needed to flip to "taken" prediction. For alternating patterns, this helps.
  - Add a backward-taken/forward-not-taken static prediction heuristic as a fallback when both predictors have low confidence.
  - Use the `enrichPredictionWithEncodedTarget` mechanism to also set the initial prediction direction for conditional branches based on the encoded offset (negative → backward → predict taken).
- **Option B**: Allow branches in secondary slots (slot 1-2 at minimum). This would allow 2+ branches per cycle, improving IPC for branch-heavy code. Complex to implement but models M2 more accurately.
- **Option C**: Increase misprediction penalty from 2 to 3-4 cycles AND improve prediction accuracy. The current 2-cycle penalty is too low for a realistic pipeline, but increasing it without improving prediction would make things worse.

**Recommended**: Option A (improve cold branch prediction). Eliminating 2-3 mispredictions would reduce CPI from 0.970 to ~0.727-0.788, matching hardware within 2-10%.

---

## Cross-cutting observations

1. **Both errors are ~35% but in opposite directions**: arithmetic is too fast, branchheavy is too slow. This suggests the pipeline model has decent average accuracy but individual benchmark characteristics expose specific gaps.

2. **The 8-wide arithmetic benchmark (8 registers) achieves 6.6% error**: This proves the pipeline issue/forwarding model is sound. The 34.5% arithmetic error is mostly benchmark structure (unrolled vs looped).

3. **Branch prediction is the single biggest lever for branchheavy**: Fixing cold-start prediction alone could bring error below 10%.

4. **Structural hazard stall counts are very high in both benchmarks** (125 for arithmetic, 116 for branchheavy). These represent wasted issue bandwidth. For arithmetic, it's the 5-register limit; for branchheavy, it's the branch-only-in-slot-0 constraint.

## Data used
- Sim CPI from local runs with config: 8-wide, no I-cache, DCache on/off (identical results since neither benchmark accesses memory)
- HW CPI from `results/final/h5_accuracy_results.json` (CI run 22215020258)
- Pipeline analysis from reading `timing/pipeline/pipeline_tick_eight.go`, `superscalar.go`, `branch_predictor.go`
