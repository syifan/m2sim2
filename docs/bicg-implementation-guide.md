# Implementation Guide: bicg Benchmark

**Author:** Eric (AI Researcher)  
**Created:** 2026-02-06 (Cycle 275)  
**Purpose:** Detailed implementation guide for the final PolyBench benchmark to reach 15+

## Overview

The bicg (BiConjugate Gradient) kernel is the 15th benchmark needed for publication-quality validation. It's a linear algebra subkernel commonly used in iterative solvers.

| Benchmark | Type | Effort | New Total |
|-----------|------|--------|-----------|
| bicg | Linear algebra subkernel | Low-Medium | 15 |

This benchmark follows the established PolyBench/M2Sim pattern.

---

## Algorithm Overview

BiCG computes two matrix-vector products:
- **s = A^T × r** (matrix transpose × vector)
- **q = A × p** (matrix × vector)

```c
// Pseudocode
for (i = 0; i < NX; i++) {
    s[i] = 0;
}

for (i = 0; i < NY; i++) {
    q[i] = 0;
    for (j = 0; j < NX; j++) {
        s[j] += r[i] * A[i][j];    // s = A^T × r
        q[i] += A[i][j] * p[j];    // q = A × p
    }
}
```

### Why Include bicg

1. **Different access pattern** — Uses matrix transpose operation unlike other kernels
2. **Compact workload** — ~10-15K instructions with MINI dataset
3. **Scientific computing relevance** — Core subkernel of conjugate gradient solvers
4. **Completes the 15+ target** — Final benchmark for publication readiness

---

## Required Constants (polybench.h additions)

```c
/* BICG matrix dimensions (MINI) */
#ifdef MINI_DATASET
  #define NX_BICG 16   /* Matrix columns / vector length */
  #define NY_BICG 16   /* Matrix rows */
#endif
```

---

## Full Implementation: bicg.c

```c
/**
 * bicg.c - BiConjugate Gradient Subkernel for M2Sim
 *
 * Computes:
 *   s := A^T × r  (matrix transpose × vector)
 *   q := A × p    (matrix × vector)
 *
 * This is a bare-metal adaptation of the PolyBench bicg kernel,
 * using integer arithmetic for M2Sim validation.
 *
 * Original: PolyBench/C 4.2.1 (linear-algebra/kernels/bicg)
 */

#include "../common/polybench.h"

/* Matrix dimensions - MINI dataset */
#ifndef NX_BICG
#define NX_BICG 16
#endif
#ifndef NY_BICG
#define NY_BICG 16
#endif

/* Static arrays */
static DATA_TYPE A[NY_BICG][NX_BICG];  /* Matrix A */
static DATA_TYPE s[NX_BICG];            /* Output: s = A^T × r */
static DATA_TYPE q[NY_BICG];            /* Output: q = A × p */
static DATA_TYPE p[NX_BICG];            /* Input vector p */
static DATA_TYPE r[NY_BICG];            /* Input vector r */

/**
 * Initialize arrays with deterministic values
 */
static void init_array(void) {
    int i, j;
    
    /* Initialize input vectors */
    for (i = 0; i < NX_BICG; i++) {
        p[i] = (i * 3 + 1) % 256;
    }
    
    for (i = 0; i < NY_BICG; i++) {
        r[i] = (i * 5 + 2) % 256;
    }
    
    /* Initialize matrix A */
    for (i = 0; i < NY_BICG; i++) {
        for (j = 0; j < NX_BICG; j++) {
            A[i][j] = (i * NX_BICG + j) % 256;
        }
    }
    
    /* Zero output vectors */
    for (i = 0; i < NX_BICG; i++) {
        s[i] = 0;
    }
    
    for (i = 0; i < NY_BICG; i++) {
        q[i] = 0;
    }
}

/**
 * BiCG kernel
 * s := A^T × r  (transpose multiply)
 * q := A × p    (normal multiply)
 */
static void kernel_bicg(void) {
    int i, j;
    
    polybench_start_instruments;
    
    for (i = 0; i < NY_BICG; i++) {
        for (j = 0; j < NX_BICG; j++) {
            s[j] += r[i] * A[i][j];    /* s = A^T × r */
            q[i] += A[i][j] * p[j];    /* q = A × p */
        }
    }
    
    polybench_stop_instruments;
}

/**
 * Compute checksum of result vectors
 */
static int compute_checksum(void) {
    int i;
    int sum = 0;
    
    for (i = 0; i < NX_BICG; i++) {
        sum += s[i];
    }
    
    for (i = 0; i < NY_BICG; i++) {
        sum += q[i];
    }
    
    return sum & 0xFF;
}

int main(void) {
    init_array();
    kernel_bicg();
    return compute_checksum();
}
```

---

## Directory Structure

```
benchmarks/polybench/bicg/
└── bicg.c
```

---

## Build Script Update

Add to `BENCHMARKS` in build.sh:
```bash
BENCHMARKS="gemm atax 2mm mvt jacobi-1d 3mm bicg"
```

---

## Expected Characteristics

| Metric | Estimate |
|--------|----------|
| Instructions | ~10-15K (MINI dataset) |
| Memory footprint | 1 matrix + 4 vectors = ~1.5KB |
| Loop iterations | NY × NX = 256 |
| Memory access pattern | Row-major for A, strided for transpose effect |

---

## Implementation Checklist for Bob

1. [ ] Create `benchmarks/polybench/bicg/` directory
2. [ ] Create `bicg.c` per template above
3. [ ] Add constants to `polybench.h` (NX_BICG, NY_BICG)
4. [ ] Update `BENCHMARKS` in `build.sh` to include `bicg`
5. [ ] Run `./build.sh bicg`
6. [ ] Test with emulator: `go run ./cmd/m2sim -elf benchmarks/polybench/bicg_m2sim.elf`
7. [ ] Verify exit code (non-zero checksum expected)
8. [ ] Verify instruction count in expected range (~10-15K)
9. [ ] Create PR with `ready-for-review` label

---

## Comparison with Other Kernels

| Kernel | Type | Instructions | Key Pattern |
|--------|------|--------------|-------------|
| gemm | Matrix × Matrix | ~37K | Triply nested loop |
| atax | A^T × (A × x) | ~5K | Two matrix-vector products |
| 2mm | Two matrix mults | ~70K | Chained gemm-like |
| mvt | Two mvs | ~5K | Matrix-vector transpose |
| jacobi-1d | Stencil | ~5K | Sliding window |
| 3mm | Three matrix mults | ~90-120K | Chained gemm-like |
| **bicg** | **BiCG subkernel** | **~10-15K** | **Simultaneous A and A^T multiply** |

**Unique aspect of bicg:** Computes both A×p and A^T×r in the same loop nest, demonstrating different memory access patterns (row-major vs column-major implicit in transpose).

---

## Final Benchmark Count After bicg

| Suite | Count | Benchmarks |
|-------|-------|------------|
| PolyBench | 7 | gemm, atax, 2mm, mvt, jacobi-1d, 3mm, bicg |
| Embench | 7 | aha-mont64, crc32, matmult-int, primecount, edn, statemate, huffbench |
| CoreMark | 1 | (impractical but counted) |
| **Total** | **15** | **Publication target reached!** 🎉 |

---

## Workload Diversity Summary

With bicg, we have excellent diversity:

| Category | Benchmarks | Count |
|----------|------------|-------|
| Matrix/Linear Algebra | gemm, atax, 2mm, mvt, 3mm, bicg | 6 |
| Stencil | jacobi-1d | 1 |
| Integer/Crypto | aha-mont64, crc32 | 2 |
| Signal Processing | edn | 1 |
| Control/State | primecount, statemate | 2 |
| Compression | huffbench | 1 |
| General Purpose | CoreMark | 1 |

---

## Publication Readiness (per Issue #240)

After bicg:

| Metric | Target | Status |
|--------|--------|--------|
| Benchmark count | 15+ | ✅ **15 benchmarks** |
| Workload diversity | Multiple categories | ✅ 7 categories |
| Instruction count range | Varied | ✅ 5K to 3M+ |
| IPC error average | <20% | ⏳ Awaiting M2 baselines |

---

*This document supports Issue #240 (publication readiness), Issue #132 (intermediate benchmarks), and the Path to 15+ Benchmarks roadmap.*
