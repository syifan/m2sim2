# M2Sim Accuracy Report

**Generated:** 2026-02-04 22:26 EST (Cycle 199)
**Author:** Eric (Researcher)

## Summary

| Metric | Value |
|--------|-------|
| Target Error | <2.0% |
| Average Error | **39.8%** |
| Benchmarks Passing | 0/3 |

## Microbenchmark Results

| Benchmark | Simulator CPI | M2 Real CPI | Error |
|-----------|---------------|-------------|-------|
| arithmetic | 0.400 | 0.268 | 49.3% ❌ |
| dependency | 1.200 | 1.009 | 18.9% ❌ |
| branch | 1.800 | 1.190 | 51.3% ❌ |

## Analysis

### Arithmetic (49.3% error)
- **Problem:** Simulator models 6-wide superscalar, M2 is 8+ wide with instruction fusion
- **Fix needed:** Increase issue width, possibly add instruction fusion

### Dependency Chain (18.9% error)  
- **Problem:** Closest to target, pipeline forwarding is mostly correct
- **Fix needed:** Fine-tune forwarding latencies

### Branch (51.3% error)
- **Problem:** Branch prediction overhead too high
- **Fix needed:** Better branch predictor model, reduced misprediction penalty

## Embench Benchmarks

**Status:** Phase 1 complete, Phase 2 blocked

| Benchmark | Instructions | Exit | Status |
|-----------|-------------|------|--------|
| aha-mont64 | 1.88M | 0 | ✅ Ready for timing |
| crc32 | 1.57M | 0 | ✅ Ready for timing |
| matmult-int | 3.85M | 0 | ✅ Ready for timing |
| primecount | 256 | - | ⚠️ Blocked (PR #188) |

## Calibration Roadmap

1. **Immediate:** Run timing simulation on Phase 1 benchmarks
2. **Short-term:** Tune issue width + forwarding latencies
3. **Medium-term:** Improve branch predictor accuracy
4. **Long-term:** Compare against native M2 execution times

## Next Steps

1. Create timing harness for Embench benchmarks
2. Measure simulated cycles vs native execution time
3. Identify which pipeline parameters need tuning
4. Iteratively calibrate until <20% average error (interim target from #141)
