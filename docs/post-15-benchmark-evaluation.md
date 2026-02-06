# Post-15 Benchmark Candidate Evaluation

**Author:** Bob (AI Coder)  
**Created:** 2026-02-06 (Cycle 277)  
**Purpose:** Evaluate seidel-2d, gesummv, and trisolv for post-15 expansion

## Executive Summary

| Candidate | Effort | Instructions (Est.) | Priority | Rationale |
|-----------|--------|---------------------|----------|-----------|
| **gesummv** | ⭐ Low | ~5-8K | **#1** | Simplest, follows atax pattern exactly |
| **seidel-2d** | ⭐ Low | ~3-6K | #2 | Extends jacobi-1d to 2D, adds stencil diversity |
| **trisolv** | ⭐ Low | ~4-7K | #3 | Useful linear algebra primitive |

**Recommendation:** Implement gesummv first — it's the simplest addition with minimal risk.

---

## Candidate 1: gesummv (Recommended First)

### Algorithm
```c
// y = alpha*A*x + beta*B*x
// Scalar-Vector-Matrix Multiplication
for (i = 0; i < N; i++) {
    tmp[i] = 0;
    y[i] = 0;
    for (j = 0; j < N; j++) {
        tmp[i] += A[i][j] * x[j];
        y[i] += B[i][j] * x[j];
    }
    y[i] = alpha * tmp[i] + beta * y[i];
}
```

### Why First?
- **Very similar to atax** — same matrix-vector multiply pattern
- Single loop nest, no complex dependencies
- Tests simultaneous two-matrix access (A and B)
- ~5-8K instructions for MINI dataset (16×16)

### Implementation Effort
- Copy atax directory structure
- Add second matrix B
- Add alpha/beta scalar multiplications
- Minimal header changes needed

---

## Candidate 2: seidel-2d

### Algorithm
```c
// 2D iterative stencil - 9-point averaging
for (t = 0; t < TSTEPS; t++) {
    for (i = 1; i < N-1; i++) {
        for (j = 1; j < N-1; j++) {
            A[i][j] = (A[i-1][j-1] + A[i-1][j] + A[i-1][j+1]
                     + A[i][j-1]   + A[i][j]   + A[i][j+1]
                     + A[i+1][j-1] + A[i+1][j] + A[i+1][j+1]) / 9;
        }
    }
}
```

### Why Second?
- Extends jacobi-1d patterns to 2D
- Tests different memory access pattern (2D stencil)
- In-place update (no double buffering like jacobi-1d)
- ~3-6K instructions for MINI dataset

### Implementation Effort
- Add 2D array bounds to polybench.h
- Implement 9-point stencil with integer division
- May need larger array for meaningful computation

---

## Candidate 3: trisolv

### Algorithm
```c
// Triangular solver: L*x = b
for (i = 0; i < N; i++) {
    x[i] = b[i];
    for (j = 0; j < i; j++) {
        x[i] -= L[i][j] * x[j];
    }
    x[i] /= L[i][i];
}
```

### Why Third?
- Different pattern — triangular iteration (j < i bound)
- Important linear algebra primitive
- ~4-7K instructions for MINI dataset
- Division in inner loop (slower but tests different path)

### Implementation Effort
- Add triangular matrix initialization
- Handle diagonal division
- Ensure L[i][i] != 0 to avoid div-by-zero

---

## Implementation Recommendation

For post-15 expansion, I recommend implementing in this order:

### Priority Order
1. **gesummv** — Simplest, follows existing patterns, low risk
2. **seidel-2d** — Adds workload diversity (2D stencil)
3. **trisolv** — Adds algorithm diversity (triangular)

### Expected Outcome
| After | Benchmark Count | Notes |
|-------|-----------------|-------|
| gesummv | 16 | +1 matrix/vector |
| +seidel-2d | 17 | +1 2D stencil |
| +trisolv | 18 | +1 triangular solver |

---

## gesummv Implementation Template

```c
/* gesummv.c - Scalar, Vector and Matrix Multiplication */

#include "../common/polybench.h"
#include "../common/startup.h"

/* Define GESUMMV dimensions in polybench.h:
 * #define N_GESUMMV 16
 */

static int A[16][16], B[16][16], x[16], y[16], tmp[16];

static void kernel_gesummv(int alpha, int beta) {
    int i, j;
    for (i = 0; i < 16; i++) {
        tmp[i] = 0;
        y[i] = 0;
        for (j = 0; j < 16; j++) {
            tmp[i] += A[i][j] * x[j];
            y[i] += B[i][j] * x[j];
        }
        y[i] = alpha * tmp[i] + beta * y[i];
    }
}

void main_benchmark(void) {
    int i, j;
    
    /* Initialize with deterministic values */
    for (i = 0; i < 16; i++) {
        x[i] = i + 1;
        for (j = 0; j < 16; j++) {
            A[i][j] = (i * 16 + j) % 100 + 1;
            B[i][j] = ((i + j) * 7) % 100 + 1;
        }
    }
    
    kernel_gesummv(2, 3);  /* alpha=2, beta=3 */
    
    /* Compute checksum for verification */
    int checksum = 0;
    for (i = 0; i < 16; i++) {
        checksum ^= y[i];
    }
    
    return_value = checksum;
}
```

---

## Conclusion

The 15 benchmark target is already met. Post-15 expansion is optional but would increase validation confidence. **gesummv** is the recommended next benchmark due to its simplicity and pattern similarity to existing code.

---
*This evaluation supports issue #141 (accuracy validation) and post-15 expansion planning.*
