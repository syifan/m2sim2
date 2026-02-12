# H3 Corrected Accuracy Analysis

**Date:** February 8, 2026
**Author:** Alex (Data Analysis & Calibration Specialist)
**References:** Issues #354, #355

## Correction Summary

Previous analysis incorrectly compared **simulation speed** (~139 ns/instruction wall-clock) against **virtual time** (~0.077 ns/instruction on M2 hardware), yielding a meaningless 181,000% error. This corrected analysis compares **CPI (virtual time)** for both simulator and hardware.

### Key Definitions

| Term | Meaning | Example |
|------|---------|---------|
| **Simulation Time** | Wall-clock time to run the simulator on your computer | 139 ns/instruction |
| **Virtual Time** | Predicted M2 hardware execution time (CPI-based) | 0.114 ns/instruction |
| **Hardware Time** | Measured M2 hardware execution time | 0.077 ns/instruction |

**Accuracy target:** Virtual time matching real M2 hardware time (<2% error).

## Corrected Accuracy Results

| Benchmark | Sim CPI | M2 CPI | Error | Status |
|-----------|---------|--------|-------|--------|
| arithmetic_sequential | 0.400 | 0.268 | 49.3% | FAIL |
| dependency_chain | 1.200 | 1.009 | 18.9% | FAIL |
| branch_taken_conditional | 1.600 | 1.190 | 34.5% | FAIL |
| **Average** | | | **34.2%** | |

**Target:** <2% average error

## Error Pattern Analysis

All benchmarks show the simulator **over-estimates CPI** (runs slower than real M2):

1. **Arithmetic (49.3% error):** M2 achieves IPC ~3.73 with 8-wide superscalar + fusion. Simulator achieves IPC ~2.5 -- dispatch efficiency gap.
2. **Branch (34.5% error):** M2 branch predictor handles well-predicted branches at CPI 1.19. Simulator at CPI 1.60 -- higher penalty overhead.
3. **Dependency (18.9% error):** M2 forwarding achieves near-perfect CPI 1.009. Simulator at CPI 1.20 -- extra stall cycles from hazard resolution.

## Parameter Tuning Priorities

| Priority | Parameter | Current Error | Action |
|----------|-----------|---------------|--------|
| 1 | Superscalar dispatch efficiency | 49.3% | Improve IPC from 2.5 toward 3.7 for independent instructions |
| 2 | Branch prediction & penalty | 34.5% | Tune misprediction penalty (14 cycles), improve predictor |
| 3 | Data forwarding paths | 18.9% | Optimize ALU-to-ALU forwarding, reduce hazard stall overhead |

## Conversion Reference

At M2 P-core 3.5 GHz:
- 1 CPI = 0.2857 ns/instruction
- CPI 0.268 (arithmetic) = 0.0766 ns/instruction
- CPI 1.009 (dependency) = 0.2883 ns/instruction

## Next Steps

1. Tune superscalar dispatch to improve IPC for independent instructions
2. Optimize forwarding paths to reduce dependency chain overhead
3. Calibrate branch prediction penalty for well-predicted patterns
4. Run medium-scale benchmarks (matmul_64) with timing mode to get CPI data
5. Expand accuracy comparison to memory-intensive benchmarks
