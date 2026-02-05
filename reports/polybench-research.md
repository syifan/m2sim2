# PolyBench Research Report

**Generated:** 2026-02-04 22:49 EST
**Author:** Eric (Researcher)
**Issue:** #191

## Summary

PolyBench/C is a benchmark suite of **30 numerical computations** with static control flow, extracted from various application domains:
- Linear algebra
- Image processing
- Physics simulation
- Dynamic programming
- Statistics

## Key Features

- Single file, tunable at compile-time
- Cache flushing before kernel execution
- Real-time scheduling to prevent OS interference
- Parametric loop bounds (generalizable)
- Clear kernel marking with pragma delimiters

## Available Benchmarks (30 total)

### Linear Algebra - Kernels
| Benchmark | Description |
|-----------|-------------|
| 2mm | 2 Matrix Multiplications (alpha * A * B * C + beta * D) |
| 3mm | 3 Matrix Multiplications ((A*B)*(C*D)) |
| atax | Matrix Transpose and Vector Multiplication |
| bicg | BiCG Sub Kernel of BiCGStab Linear Solver |
| gemm | Matrix-multiply C=alpha.A.B+beta.C |
| gemver | Vector Multiplication and Matrix Addition |
| gesummv | Scalar, Vector and Matrix Multiplication |
| mvt | Matrix Vector Product and Transpose |
| symm | Symmetric matrix-multiply |
| syr2k | Symmetric rank-2k update |
| syrk | Symmetric rank-k update |
| trmm | Triangular matrix-multiply |

### Linear Algebra - Solvers
| Benchmark | Description |
|-----------|-------------|
| cholesky | Cholesky Decomposition |
| durbin | Toeplitz system solver |
| gramschmidt | Gram-Schmidt decomposition |
| lu | LU decomposition |
| ludcmp | LU decomposition + Forward Substitution |
| trisolv | Triangular solver |

### Stencils
| Benchmark | Description |
|-----------|-------------|
| adi | Alternating Direction Implicit solver |
| fdtd-2d | 2-D Finite Different Time Domain Kernel |
| heat-3d | Heat equation over 3D data domain |
| jacobi-1d | 1-D Jacobi stencil computation |
| jacobi-2d | 2-D Jacobi stencil computation |
| seidel | 2-D Seidel stencil computation |

### Other
| Benchmark | Description |
|-----------|-------------|
| correlation | Correlation Computation |
| covariance | Covariance Computation |
| deriche | Edge detection filter |
| doitgen | Multi-resolution analysis kernel |
| nussinov | Dynamic programming for sequence alignment |

## Comparison with Embench

| Feature | Embench | PolyBench |
|---------|---------|-----------|
| Focus | Embedded systems | Numerical/HPC |
| Benchmarks | 19 | 30 |
| Memory Usage | Low | Configurable (SMALL/MEDIUM/LARGE) |
| Operations | Integer + control | Floating-point heavy |
| Complexity | Simple | Complex loop nests |

## M2Sim Suitability Assessment

### Pros
1. **Static control flow** — predictable loop bounds, good for timing simulation
2. **Pure computation** — minimal I/O, no OS dependencies
3. **Multiple sizes** — can start with MINI/SMALL datasets
4. **Well-documented** — clear kernel boundaries

### Cons
1. **Floating-point heavy** — M2Sim currently focuses on integer operations
2. **Large matrices** — may need reduced problem sizes for bare-metal
3. **Build complexity** — needs adaptation for bare-metal (no libc printf)

## Recommendation

**Priority: Medium** — Add after Embench Phase 2 is complete.

**Suggested approach:**
1. Start with integer-friendly benchmarks: `atax`, `bicg`, `mvt` (mostly integer indices)
2. Use MINI dataset sizes initially
3. Adapt build system for bare-metal (similar to Embench approach)
4. Consider as Phase 3 benchmark target

## Source

- GitHub: https://github.com/MatthiasJReisinger/PolyBenchC-4.2.1
- Original: http://web.cse.ohio-state.edu/~pouchet/software/polybench/

