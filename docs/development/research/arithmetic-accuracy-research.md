# Arithmetic Accuracy Research

**Author:** Eric (Researcher)
**Date:** 2026-02-05 (Cycle 230)

## Current Status

| Metric | Value |
|--------|-------|
| Benchmark | arithmetic_sequential |
| Simulator CPI | 0.400 |
| M2 Real CPI | 0.268 |
| Error | 49.3% |

## Analysis: Why M2 is Faster

The arithmetic benchmark executes 20 independent ADD instructions. Both simulator and M2 should achieve high ILP since there are no dependencies.

### Issue Width Gap

| Factor | M2 Avalanche | M2Sim Current | Gap |
|--------|--------------|---------------|-----|
| Decode width | 8-wide | 6-wide | 2 slots |
| Integer ALUs | 6+ units | 6 units | Parity |
| Effective IPC | ~3.7 | ~2.5 | 48% |

**8-wide decode (PR #215) expected impact:**
- Theoretical max IPC: 8 (with enough ALUs)
- Practical limit: ALU count (6 units)
- Expected improvement: 20-30% error reduction

### After 8-Wide: Remaining Gaps

Even with 8-wide decode, we may not match M2's CPI. Potential reasons:

1. **Micro-op Fusion**
   - M2 may fuse simple operations (e.g., ADD+shift)
   - Reduces instruction count at decode level
   - We model instructions 1:1 with ISA

2. **Pipeline Fill Overhead**
   - Short benchmark (20 instructions) has startup costs
   - M2 has deeper speculation, less visible fill
   - Longer benchmarks will show less fill impact

3. **Functional Unit Availability**
   - M2 may have >6 integer ALUs for simple ops
   - Some ops may execute on multiple unit types
   - Port scheduling may differ

4. **Memory System Effects**
   - Instruction cache behavior differs
   - Fetch bandwidth may impact short loops
   - M2 loop buffer may help hot paths

## Recommended Optimizations (Priority Order)

### 1. Complete 8-Wide Implementation (High Priority)
- PR #215 infrastructure done, needs full tick function
- Expected: 49.3% → ~30% error
- Effort: Medium (follow 6-wide pattern)

### 2. Pipeline Fill Reduction (Medium Priority)
- Improve fetch unit speculation depth
- Pre-fill pipeline registers before measurement
- Expected: 5-10% improvement on short benchmarks

### 3. Instruction Fusion (Low Priority for Now)
- Detect fusable patterns in decode
- Create macro-ops for common sequences
- Expected: 5-10% improvement, high effort
- Better ROI after 8-wide is stable

### 4. ALU Scheduling Improvements (Research)
- Model multiple ALU types (simple/complex)
- Allow simple ops on any unit
- Likely diminishing returns vs implementation effort

## Projected Accuracy After 8-Wide

| Benchmark | Current | Post 8-Wide | Notes |
|-----------|---------|-------------|-------|
| arithmetic | 49.3% | ~28% | 8 decode slots |
| dependency | 18.9% | ~18% | Latency-bound |
| branch_taken | 34.5% | ~32% | Minor ILP gain |
| **Average** | **34.2%** | **~26%** | Target: <20% |

## Path to <20% Average Error

After 8-wide implementation:
1. **Tune branch predictor** — reduce 32% branch error
2. **Optimize forwarding** — reduce 18% dependency error
3. **Consider selective OoO** — for latency-bound workloads

Realistic target with current architecture: **~20-25% average error**

Reaching <10% would require:
- Out-of-order execution modeling
- Full M2 microarchitecture fidelity
- Significantly more implementation effort

## Conclusion

8-wide decode is the highest-impact single optimization for arithmetic accuracy. After that, diminishing returns set in. The simulator should stabilize at ~25% average error with current architectural approach.
