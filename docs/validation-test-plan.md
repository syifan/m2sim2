# M2Sim Validation Test Plan

**Issue:** #97 (Accuracy analysis and reporting)  
**Target:** <2% average error across benchmarks  
**Author:** Cathy (QA Agent)  
**Date:** 2026-02-03

## Overview

This document outlines the methodology for validating M2Sim simulator accuracy against real M2 hardware, including data collection procedures, statistical analysis methods, and CI integration for ongoing accuracy monitoring.

## 1. Data Collection Strategy

### 1.1 Hardware Measurements (Ground Truth)

**Challenge:** Process startup overhead (~18ms) dominates measurements for micro-benchmarks.

**Solution: Apple Instruments with CPU Counters**

```bash
# Collect hardware performance counters (no overhead interference)
xcrun xctrace record --template "CPU Counters" \
  --output benchmark.trace \
  --launch ./bin/benchmark_program

# Extract cycle counts from trace
xcrun xctrace export --input benchmark.trace --output cycles.json
```

**Metrics to Collect:**
- `FIXED_CYCLES` - Total CPU cycles
- `INST_RETIRED` - Instructions retired
- `BRANCH_MISPRED` - Branch mispredictions (for branch predictor validation)
- `L1D_CACHE_MISS` - L1 data cache misses (for cache validation)

### 1.2 Simulator Measurements

```bash
# Run benchmark harness with JSON output
./bin/m2sim benchmark -format=json -o simulated.json

# Key metrics captured:
# - cycles: Simulated cycle count
# - instructions: Instruction count
# - cpi: Cycles per instruction
```

### 1.3 Benchmark Requirements

To minimize measurement noise and enable accurate comparison:

| Requirement | Minimum | Rationale |
|-------------|---------|-----------|
| Instruction count | 1M+ | Amortize measurement overhead |
| Determinism | 100% | Same input → same output |
| No I/O | Required | Avoid system call variance |
| No heap allocation | Preferred | Avoid allocator variance |

**Recommended Benchmark Extensions:**
- `loop_simulation_1M` - 1 million iterations
- `matrix_operations_1K` - 1000 matrix multiplications
- `dependency_chain_100K` - 100K dependent operations

## 2. Error Calculation Methodology

### 2.1 Per-Benchmark Error

For each benchmark `b`:

```
error_b = |simulated_cycles_b - measured_cycles_b| / measured_cycles_b × 100%
```

**Signed Error (for bias detection):**

```
signed_error_b = (simulated_cycles_b - measured_cycles_b) / measured_cycles_b × 100%
```

Positive = simulator over-predicts; Negative = simulator under-predicts.

### 2.2 Aggregate Error Metrics

**Mean Absolute Percentage Error (MAPE):**

```
MAPE = (1/n) × Σ |error_b|
```

This is our primary accuracy metric. Target: MAPE < 2%.

**Root Mean Square Error (RMSE):**

```
RMSE = sqrt((1/n) × Σ (error_b)²)
```

Penalizes large outliers more heavily.

**Weighted MAPE (by instruction count):**

```
WMAPE = Σ(instructions_b × |error_b|) / Σ(instructions_b)
```

Gives more weight to longer-running benchmarks.

### 2.3 Statistical Confidence

For each benchmark, run N=10 iterations and calculate:

- Mean cycle count
- Standard deviation
- 95% confidence interval: `mean ± 1.96 × (std / √n)`

Only count error as significant if simulator prediction falls outside hardware CI.

## 3. Report Generation

### 3.1 Accuracy Report Format

```json
{
  "report_date": "2026-02-03T16:00:00Z",
  "simulator_version": "v0.6.0",
  "hardware": "Apple M2",
  "summary": {
    "mape": 1.85,
    "rmse": 2.12,
    "wmape": 1.72,
    "pass": true,
    "target": 2.0
  },
  "benchmarks": [
    {
      "name": "loop_simulation_1M",
      "simulated_cycles": 1020000,
      "measured_cycles": 1000000,
      "measured_stddev": 5000,
      "error_pct": 2.0,
      "signed_error_pct": 2.0,
      "pass": true
    }
  ],
  "analysis": {
    "bias": "slight over-prediction",
    "worst_case": "branch_heavy (3.5% error)",
    "best_case": "arithmetic_seq (0.2% error)"
  }
}
```

### 3.2 Human-Readable Report

Generate markdown report for PR comments and documentation:

```markdown
# M2Sim Accuracy Report

**Result: ✅ PASS** (MAPE: 1.85% < 2.0% target)

| Benchmark | Simulated | Measured | Error |
|-----------|-----------|----------|-------|
| loop_sim  | 1.02M     | 1.00M    | +2.0% |
| matrix_op | 5.10M     | 5.05M    | +1.0% |
| ...       | ...       | ...      | ...   |
```

## 4. CI Integration

### 4.1 Accuracy Regression Test

```yaml
# .github/workflows/accuracy.yml
name: Accuracy Validation

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  accuracy:
    runs-on: [self-hosted, m2]  # Requires M2 hardware runner
    steps:
      - uses: actions/checkout@v4
      
      - name: Build
        run: make build
        
      - name: Collect Hardware Baseline
        run: ./scripts/collect-hardware-baseline.sh
        
      - name: Run Simulator
        run: ./bin/m2sim benchmark -format=json -o sim.json
        
      - name: Calculate Accuracy
        run: ./scripts/accuracy-check.sh sim.json baseline.json
        
      - name: Post Results
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const report = require('./accuracy-report.json');
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `## Accuracy Report\n\n${report.markdown}`
            });
```

### 4.2 Baseline Management

**Storing Baselines:**
- Hardware measurements stored in `baselines/m2-hardware.json`
- Updated manually when hardware configuration changes
- Version controlled with commit hash of measurement

**Baseline Update Process:**
1. Run measurements on reference M2 hardware
2. Run 10 iterations per benchmark
3. Calculate mean and stddev
4. Review and commit to `baselines/`

## 5. Test Categories

### 5.1 Core Validation Suite (Required for <2% target)

| Test | Purpose | Weight |
|------|---------|--------|
| `loop_simulation_1M` | Basic loop overhead | High |
| `arithmetic_1M` | ALU-bound workload | High |
| `dependency_chain_100K` | Pipeline stall behavior | High |
| `memory_sequential_1M` | L1 cache behavior | Medium |
| `branch_heavy_1M` | Branch prediction accuracy | Medium |
| `function_calls_100K` | Call/return overhead | Medium |

### 5.2 Extended Validation (Informational)

| Test | Purpose |
|------|---------|
| `l2_stress` | L2 cache behavior |
| `random_memory` | Cache miss patterns |
| `simd_basic` | NEON instruction timing |
| `mixed_realistic` | Representative workload |

## 6. Failure Analysis Procedures

### 6.1 When Accuracy Drops Below 2%

1. **Identify Regressing Benchmarks**
   - Compare per-benchmark errors to previous baseline
   - Flag benchmarks with >0.5% increase

2. **Root Cause Analysis**
   - Check recent commits affecting timing model
   - Compare CPI breakdown (mem stalls, branch stalls, etc.)
   - Review pipeline stage timings

3. **Calibration Adjustment**
   - If systematic bias: adjust global timing parameters
   - If benchmark-specific: investigate instruction mix

### 6.2 Tracking Error Trends

Maintain `baselines/accuracy-history.json`:

```json
{
  "history": [
    {"date": "2026-02-01", "mape": 5.2, "version": "v0.5.0"},
    {"date": "2026-02-03", "mape": 1.8, "version": "v0.6.0"}
  ]
}
```

Plot accuracy over time to detect gradual drift.

## 7. Implementation Checklist

### Phase 1: Infrastructure (Issue #97)
- [ ] Create `scripts/accuracy-check.sh` - Compare sim vs hardware JSON
- [ ] Create `scripts/collect-hardware-baseline.sh` - xctrace wrapper
- [ ] Define accuracy report JSON schema
- [ ] Implement accuracy report generator in Go

### Phase 2: Benchmarks
- [ ] Extend existing benchmarks to 1M+ iterations
- [ ] Add iteration count CLI flag to benchmark harness
- [ ] Validate determinism (same cycles every run)

### Phase 3: CI Integration
- [ ] Set up self-hosted M2 runner (or use manual baseline)
- [ ] Add accuracy workflow to GitHub Actions
- [ ] Configure PR comment posting

### Phase 4: Baseline
- [ ] Collect initial M2 hardware measurements
- [ ] Document measurement methodology
- [ ] Commit baseline to repository

## 8. Success Criteria

| Metric | Target | Measurement |
|--------|--------|-------------|
| MAPE | <2% | Average of all benchmarks |
| Max Error | <5% | No single benchmark exceeds |
| Bias | <1% | Signed error average near zero |
| Variance | <0.5% | Simulator run-to-run variance |

## Appendix A: Statistical Formulas

**Mean Absolute Percentage Error:**
```
MAPE = (100/n) × Σᵢ |(Aᵢ - Fᵢ) / Aᵢ|
```
Where Aᵢ = actual (hardware), Fᵢ = forecast (simulator)

**Confidence Interval (95%):**
```
CI = x̄ ± 1.96 × (σ / √n)
```

**Coefficient of Variation:**
```
CV = σ / μ × 100%
```

## Appendix B: Tools Required

- `xcrun xctrace` - Apple Instruments CLI
- `jq` - JSON processing
- `go` - For accuracy analysis tool
- GitHub Actions runner with M2 hardware (or manual baseline process)
