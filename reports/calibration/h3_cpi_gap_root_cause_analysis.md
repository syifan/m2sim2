# H3 CPI Gap Root Cause Analysis

**Author:** Alex (Data Analysis & Calibration)
**Date:** February 8, 2026
**Status:** Complete — actionable recommendations

## Summary

Three microbenchmarks show systematic CPI overestimation vs M2 hardware baselines:

| Benchmark | Simulated CPI | M2 Hardware CPI | Error | Root Cause |
|-----------|:---:|:---:|:---:|------------|
| arithmetic_sequential | 0.400 | 0.268 | +49.3% | RAW hazard blocking prevents full 8-wide issue |
| branch_taken_conditional | 1.600 | 1.190 | +34.5% | Misprediction penalty too high (14 vs ~12 cycles) |
| dependency_chain | 1.200 | 1.009 | +18.9% | In-order pipeline structural limitation |

All errors are in the **same direction** (simulator runs slower than hardware). This is expected — the M2 is an out-of-order core while M2Sim models an in-order pipeline.

---

## 1. Arithmetic Sequential (49.3% Error) — LARGEST GAP

### The Benchmark
20 independent ADD instructions using 5 registers (X0-X4), repeating in groups:
```
ADD X0, X0, #1   ; writes X0, reads X0
ADD X1, X1, #1   ; writes X1, reads X1
ADD X2, X2, #1   ; writes X2, reads X2
ADD X3, X3, #1   ; writes X3, reads X3
ADD X4, X4, #1   ; writes X4, reads X4
(repeats 4x, then SVC)
```

### Root Cause: RAW Hazards Block Superscalar Issue

The `canIssueWith()` function in `timing/pipeline/superscalar.go:1052-1112` blocks multi-issue when **any** earlier instruction in the batch writes to a register that the new instruction reads. Critically:

```
ADD X0, X0, #1   ; Rd=X0, Rn=X0 (writes X0)
```

The **next** `ADD X0, X0, #1` (5 instructions later) reads X0 via Rn. Even though this is 5 instructions later, the in-order pipeline checks for RAW hazards across all instructions in the same issue batch. Since each register (X0-X4) appears every 5 instructions, the effective issue width is limited.

**Key check** (`superscalar.go:1085`):
```go
if newInst.Rn == prev.Rd {
    return false  // blocks issue
}
```

With only 5 distinct registers and 8-wide issue, instructions 5-7 in a batch will always hit RAW hazards with instructions 0-4 (e.g., instr 5 writes X0 and reads X0, but instr 0 also wrote X0).

### Why M2 Hardware Achieves 0.268 CPI
The real M2 is **out-of-order** with register renaming. It can:
1. Rename X0 across iterations, eliminating false dependencies
2. Issue 6+ ALU ops per cycle from its reservation stations
3. The 0.268 CPI implies ~3.7 instructions per cycle throughput

### Tuning Recommendations

**Option A (Structural fix — highest impact):**
Allow forwarding across issue slots within the same cycle. If instruction N writes Rd and instruction N+K reads Rn=Rd, permit issue if forwarding is available from the same cycle. This requires modifying `canIssueWith()` to distinguish between "same-cycle forwarding available" vs "true RAW stall."

**Option B (Parameter tuning — simpler):**
The arithmetic_8wide benchmark uses 8 distinct registers (X0-X7) specifically to avoid this problem. If the standard benchmark were changed to use 8 registers instead of 5, it would better match what the simulator can achieve with 8-wide issue.

**Option C (OoO approximation):**
Add a simple register renaming count to the superscalar config. If the number of distinct physical registers > logical registers in the batch, skip RAW checks for non-adjacent instructions. This approximates OoO without full reorder buffer modeling.

**Estimated impact:** Option A could reduce arithmetic CPI from 0.400 to ~0.300, cutting the error to ~12%.

---

## 2. Branch Taken Conditional (34.5% Error)

### The Benchmark
5 iterations of: `CMP X0, #0` + `B.GE +8` (always taken) + skipped ADD + executed ADD.
Pattern per iteration: 4 instructions, only 3 executed (CMP, B.GE, ADD).

### Root Cause: Branch Misprediction Penalty

**Config value:** `BranchMispredictPenalty = 14` cycles (in `timing/latency/config.go:74`)

The comment says "12 cycles (typical for modern out-of-order cores)" but the **actual value is 14**. This discrepancy alone adds overhead.

Additionally, for this linear (non-looping) benchmark with 5 unique branch PCs, the branch predictor encounters each PC only once. The tournament predictor starts with default predictions, which may not predict "taken" for forward branches. Each misprediction costs 14 cycles.

### Detailed CPI Breakdown

With 5 iterations × 3 executed instructions = 15 instructions + 1 SVC = 16 total:
- 16 instructions at 1.600 CPI = 25.6 simulated cycles
- 15 instructions at theoretical 1.0 CPI = 15 baseline cycles
- Delta: ~10.6 extra cycles, split across pipeline fill + branch penalties

### Why M2 Hardware Achieves 1.190 CPI
- Shorter actual misprediction penalty (~10-12 cycles)
- TAGE-based predictor that can predict forward taken branches more accurately
- Branch target buffer caches targets even for first-time branches
- CMP+B.cond macro-fusion reduces the effective instruction count

### Tuning Recommendations

**Recommendation 1 (Immediate — config change):**
Reduce `BranchMispredictPenalty` from 14 to 12. The config comment already says 12 is the target.

**Recommendation 2 (Predictor warmup):**
For forward conditional branches with immediate comparison (CMP + B.GE/B.NE), bias the initial predictor state toward "taken" for forward branches. This matches real hardware behavior where forward branches are often taken.

**Recommendation 3 (CMP+B.cond fusion improvement):**
Current fusion in `pipeline.go` fuses CMP+B.cond into a single pipeline slot. Verify this is working correctly for all 5 pairs. If fusion works, 5 CMPs should be "free" (fused), reducing effective instruction count from 16 to 11. At 1.190 CPI for 11 effective instructions = ~13 cycles, which is reasonable.

**Estimated impact:** Reducing penalty from 14 to 12 alone could lower CPI from 1.600 to ~1.450 (cutting error from 34.5% to ~22%). Combined with predictor improvements, could reach ~1.300 (~9% error).

---

## 3. Dependency Chain (18.9% Error)

### The Benchmark
20 dependent ADD instructions, each writing X0 and reading X0:
```
ADD X0, X0, #1   ; reads X0, writes X0
ADD X0, X0, #1   ; reads X0 (from previous), writes X0
... (20 times)
```

### Root Cause: In-Order Pipeline Structural Limitation

With a true dependency chain, each ADD must wait for the previous ADD to produce its result. In the 5-stage pipeline:

- With perfect forwarding (EX→EX): each ADD takes 1 cycle after the previous completes → CPI ≈ 1.0
- The simulated 1.200 CPI suggests forwarding isn't reducing latency to the theoretical minimum

The forwarding logic in `hazard.go` detects forwarding from EX/MEM and MEM/WB stages back to ID/EX. However, each forwarding path may still insert 1 bubble cycle due to the way the pipeline stages advance.

### Why M2 Hardware Achieves 1.009 CPI
- M2's out-of-order backend can execute dependent ADDs back-to-back with zero-cycle forwarding
- The 0.009 overhead is likely just pipeline fill/drain (4 fill + 4 drain cycles across 20 instructions ≈ 0.4 extra cycles → ~1.02 CPI)

### Tuning Recommendations

**Recommendation 1 (Forwarding audit):**
Trace the exact cycle-by-cycle behavior of 3-4 dependent ADDs in the pipeline to verify that forwarding is eliminating all stall cycles. The expected behavior:
- Cycle N: ADD₁ in EX stage produces result
- Cycle N+1: ADD₂ in EX stage uses forwarded result (0-cycle forwarding latency)
If there's a 1-cycle forwarding delay, that explains the 1.200 CPI (1.0 base + 0.2 forwarding overhead per instruction).

**Recommendation 2 (Pipeline fill cost):**
With 20 instructions + 1 SVC in a 5-stage pipeline: fill cost = 4 cycles, drain cost = 4 cycles. For 21 instructions at 1.0 CPI base: 21 + 8 = 29 cycles → CPI = 29/21 = 1.38. This doesn't match 1.200, so the pipeline is actually doing better than naive fill/drain. The forwarding is partially working.

A more precise model: if forwarding works for ALU→ALU but adds 0 stalls, then CPI should be ~(21+4)/21 = 1.19, which is very close to the measured 1.200. This suggests the pipeline model is actually **nearly correct** for this workload.

**Recommendation 3 (Pipeline depth reduction):**
The 18.9% error may be irreducible in an in-order pipeline model. The M2's OoO machinery effectively hides the fill/drain latency. To reduce this gap below 10%, the simulator would need to model instruction prefetch or reduce effective pipeline depth.

**Estimated impact:** This is the smallest gap and may be acceptable as-is. A forwarding audit could confirm correctness and potentially reduce to ~15% error.

---

## Summary of Tuning Priorities

| Priority | Change | File | Expected Impact |
|:---:|--------|------|:---:|
| 1 | Fix `BranchMispredictPenalty` 14→12 | `timing/latency/config.go:74` | Branch: 34.5% → ~25% |
| 2 | Allow same-cycle forwarding in superscalar dispatch | `timing/pipeline/superscalar.go:1052` | Arithmetic: 49.3% → ~20% |
| 3 | Bias branch predictor toward taken for forward branches | `timing/pipeline/branch_predictor.go` | Branch: 25% → ~15% |
| 4 | Audit forwarding path for dependent ALU chains | `timing/pipeline/hazard.go` | Dependency: 18.9% → ~15% |

### Overall Error Projection
With priorities 1-3 implemented:
- Arithmetic: ~20% error (down from 49.3%)
- Branch: ~15% error (down from 34.5%)
- Dependency: ~15% error (down from 18.9%)
- **Average: ~17% error (down from 34.2%)**

### Note on Structural Limitations
The in-order 5-stage pipeline model will always overestimate CPI compared to M2's out-of-order core. Achieving <10% average error would likely require adding OoO approximations (register renaming, instruction window modeling). The current in-order model with tuning can realistically reach 15-20% average error.
