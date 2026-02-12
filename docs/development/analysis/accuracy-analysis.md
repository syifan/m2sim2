# M2Sim Accuracy Analysis

*Analysis by Bob — 2026-02-04*

## Current Status

| Benchmark | Sim CPI | M2 CPI | Error | Explanation |
|-----------|---------|--------|-------|-------------|
| arithmetic_sequential | 0.400 | 0.268 | 49.3% | Pipeline fill overhead |
| dependency_chain | 1.200 | 1.009 | 18.9% | Forwarding latency delta |
| branch_taken | 1.800 | 1.190 | 51.3% | Branch elimination overhead |

**Average Error: 39.8%**

## Root Cause Analysis

### 1. Arithmetic Sequential (49.3% error)

**What we observed:**
- 20 independent ADD instructions
- 8 cycles total on 6-wide superscalar
- CPI = 0.400

**Expected (ideal 6-wide):**
- Steady state: ceil(20/6) = 4 cycles
- With pipeline fill (~4 cycles): 8 cycles total
- This matches! The simulator is correct for our model.

**Why M2 is faster (CPI 0.268):**
- M2 has 8+ integer ALUs (not just 6)
- Micro-op fusion combines operations
- Out-of-order execution hides fill latency
- Register renaming enables more parallelism

**Potential fixes:**
1. Implement 8-wide superscalar
2. Add instruction fusion for common patterns
3. Model out-of-order execution (complex)

### 2. Branch Taken (51.3% error)

**What we observed:**
- 5 B instructions (eliminated) + 5 ADDs + 1 SVC
- 9 cycles total
- CPI = 1.800 (counting only 5 ADDs)

**Why the gap:**
- Branch elimination removes ALU cycles but not fetch/decode overhead
- Each branch causes a control flow redirect
- Sequential code between branches can't be speculatively fetched

**Why M2 is faster (CPI 1.190):**
- Deeper speculation across multiple branches
- Branch target prediction allows prefetching
- Better instruction buffer management

**Potential fixes:**
1. Improve fetch to not stall on eliminated branches
2. Add speculative fetch-ahead for branch targets
3. Model branch target buffer (BTB)

### 3. Dependency Chain (18.9% error)

**What we observed:**
- 20 dependent ADD instructions
- CPI = 1.200 (1 cycle per instruction + forwarding)

**Why the gap:**
- M2 achieves CPI 1.009 — nearly 1 cycle per dependent op
- Our model has ~0.2 CPI overhead per instruction

**Potential fixes:**
1. Reduce forwarding latency
2. Model M2's precise pipeline timing

## Recommended Next Steps

### Phase 1: Low-hanging fruit (effort: low)
- [ ] Update test log messages to reflect 6-wide reality
- [ ] Document current accuracy gaps
- [ ] Add verbose stats output to benchmarks

### Phase 2: Architecture improvements (effort: medium)
- [ ] Increase superscalar width to 8-wide
- [ ] Improve branch elimination to not stall fetch
- [ ] Fine-tune forwarding latency

### Phase 3: Major enhancements (effort: high)
- [ ] Model out-of-order execution
- [ ] Add branch target buffer (BTB)
- [ ] Implement instruction fusion

## Conclusion

The simulator is **functionally correct** but has accuracy gaps due to:
1. M2 being wider than our 6-wide model
2. Missing microarchitectural features (OoO, BTB, fusion)
3. Pipeline fill overhead on short benchmarks

For intermediate benchmarks (100ms+ runtime), the pipeline fill overhead will be negligible, and we should see better accuracy.
