# Intermediate Benchmarks Implementation Plan

*Research by Eric — 2026-02-04*

## Overview

Per human directive (#152), intermediate benchmarks are **top priority** to bridge the gap between microbenchmarks (too simple) and SPEC (too complex).

## Current State

### Available Now
- **Microbenchmarks:** ~40% average error (too simple to validate real accuracy)
- **CoreMark:** Source code in `benchmarks/coremark/`, M2 baseline captured (35,120 iter/sec)
- **SPEC CPU 2017:** Installed and ready (benchspec/CPU exists)
- **Cross-compiler:** aarch64-elf-gcc 15.2.0 installed

### Missing
- ELF-compiled CoreMark binary for M2Sim
- M2Sim timing results for CoreMark
- Additional intermediate benchmarks

## Implementation Plan

### Phase 1: CoreMark ELF (Immediate)

**Goal:** Cross-compile CoreMark to ELF, run in M2Sim, compare to M2 baseline

**Steps:**
1. Create barebones Makefile for aarch64-elf-gcc
2. Cross-compile CoreMark with minimal iterations (fast validation)
3. Run in M2Sim instruction-by-instruction mode
4. Capture CPI and timing metrics
5. Compare to real M2 baseline

**Expected output:** `benchmarks/baselines/coremark_m2sim.csv`

### Phase 2: Embench-IoT (Short-term)

**Goal:** Add diverse workloads with different characteristics

**Benchmarks to add:**
- `aha-mont64` — Modular arithmetic (tests ALU)
- `crc32` — Checksum (tests memory access patterns)
- `matmult-int` — Matrix multiply (tests cache behavior)
- `minver` — Matrix inversion (tests FP if available)
- `nbody` — Physics (tests branches + arithmetic)

**Setup:**
```bash
git clone https://github.com/embench/embench-iot.git
# Build for aarch64 baremetal
```

### Phase 3: Simple SPEC Subsets (Medium-term)

**Goal:** Use SPEC with reduced inputs for faster iteration

**Candidates:**
- `505.mcf_r` — Combinatorial optimization (small test input)
- `531.deepsjeng_r` — Game AI (predictable execution)

**Strategy:** Use `--size=test` input sets for quick validation

## Priority Matrix

| Benchmark | Complexity | Time to Implement | Value |
|-----------|------------|-------------------|-------|
| CoreMark ELF | Low | 1-2 hours | High |
| Embench subset | Medium | 2-4 hours | Medium |
| SPEC test inputs | Medium | 4-8 hours | High |

## Next Actions

→Bob: Implement CoreMark ELF cross-compilation and M2Sim timing
→Alice: Track intermediate benchmark progress
→Cathy: Validate benchmark accuracy methodology
