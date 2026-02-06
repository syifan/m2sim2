# Accuracy Target Evaluation

## Current Status (Cycle 266)

### Microbenchmark Results

| Benchmark | Sim CPI | M2 CPI | Error | Status |
|-----------|---------|--------|-------|--------|
| arithmetic_8wide | 0.250 | 0.268 | **7.2%** | ✅ |
| dependency_chain | 1.200 | 1.009 | **18.9%** | ✅ |
| branch_conditional | 1.600 | 1.190 | **34.5%** | ❌ |
| **Average** | — | — | **20.2%** | ⚠️ |

### Issue #141 Caveats (Human Requirements)

Per Human's requirements in issue #141, the <20% target applies WITH caveats:

1. **Benchmark Requirements:**
   - Must use **intermediate-size benchmarks**
   - **Microbenchmarks should NOT be included** in accuracy measurement
   - Real workloads with meaningful execution patterns

2. **Architectural Fidelity:**
   - Out-of-order core model required (if M2 is OoO)
   - Accuracy relaxation doesn't skip architectural features

### Assessment

**The 20.2% microbenchmark average does NOT meet the Human's requirements.**

Per caveat 1, microbenchmarks don't count. We need:
- PolyBench results (gemm merged, atax pending)
- CoreMark results (not yet implemented)
- Embench results (timing mode pending)

### Zero-Cycle Folding Status

FoldedBranches = 0 because unsafe folding was disabled (commit 1590518).

Safe reimplementation documented in `docs/safe-zero-cycle-folding.md`:
- Would reduce branch error 34.5% → ~15-20%
- Requires speculative execution with verification
- Estimated ~170 LOC, medium risk

### Next Steps

1. **PolyBench Phase 1** — atax benchmark pending
2. **CoreMark** — Industry standard, concrete issue needed
3. **Embench timing mode** — Diverse workloads
4. **M2 baseline capture** — Need Human to run gemm on M2

### Recommendation

Do NOT declare M6 complete yet. Current 20.2% is based on microbenchmarks which explicitly don't count per issue #141. Focus on:
1. Completing PolyBench Phase 1 (gemm ready, capture M2 baseline)
2. Implementing CoreMark (create issue)
3. Measuring accuracy on intermediate benchmarks
