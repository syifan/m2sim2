# Intermediate ARM64 Benchmarks Research

**Last updated:** 2026-02-05 (Cycle 213)
**Issue:** #132

## Goal
Find benchmarks between micro-benchmarks (too simple) and SPEC (too long).
Target execution time: 100ms - 10s range.

## Candidates Evaluated

### 1. PolyBench (Recommended)
- **Source:** https://github.com/MatthiasJReisworczyk/PolyBench-ACC
- **Status:** Human opened issue #191 to explore
- **Pros:** 
  - Well-defined kernels
  - Scalable problem sizes
  - Already mentioned by human for evaluation
- **Cons:** May need build configuration for ARM64

### 2. Embench (Currently Used)
- **Status:** 5 benchmarks working
- **Pros:** Already integrated
- **Cons:** Some need libc stubs (#186, #187 closed)

### 3. CoreMark
- **Source:** EEMBC
- **Pros:** Industry standard, single-threaded focus
- **Cons:** Narrow workload type

### 4. MiBench
- **Source:** University of Michigan
- **Pros:** Diverse embedded workloads
- **Cons:** Older, may need porting

### 5. SPEC CPU 2017
- **Status:** Issue #146 tracking installation
- **Pros:** Gold standard for CPU validation
- **Cons:** Long execution times, complex setup

## Recommendation

**Short-term:** Focus on PolyBench (#191) — scalable problem sizes allow targeting 100ms-10s range.

**Medium-term:** Continue SPEC setup (#146) for comprehensive validation.

## References
- Issue #191 (PolyBench)
- Issue #146 (SPEC)
- Issue #183 (Embench expansion)
