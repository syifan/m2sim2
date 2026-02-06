/**
 * polybench.h - Minimal bare-metal PolyBench header for M2Sim
 *
 * This is a stripped-down version of polybench.h that removes all
 * libc dependencies for bare-metal execution on M2Sim.
 */

#ifndef _POLYBENCH_H
#define _POLYBENCH_H

/* Dataset sizes - MEDIUM for accurate timing benchmarks */
/* Override by defining before include: -DMINI_DATASET, -DSMALL_DATASET, etc. */
#if !defined(MINI_DATASET) && !defined(SMALL_DATASET) && !defined(MEDIUM_DATASET) && !defined(LARGE_DATASET)
#define MEDIUM_DATASET
#endif

/*
 * Dataset Size Reference:
 * +---------+-------------+------------------+
 * | Dataset | Matrix Size | Expected Runtime |
 * +---------+-------------+------------------+
 * | MINI    | 16-40       | Microseconds     |
 * | SMALL   | 60-120      | Milliseconds     |
 * | MEDIUM  | 200-400     | Seconds          |
 * | LARGE   | 1000-2000   | Minutes          |
 * +---------+-------------+------------------+
 */

/* ============================================================ */
/* MINI DATASET - 16x16 matrices (for quick validation only)    */
/* ============================================================ */
#ifdef MINI_DATASET
  /* GEMM, 2mm, 3mm, MVT dimensions */
  #define NI 16
  #define NJ 16
  #define NK 16
  #define NL 16
  #define NM 16

  /* ATAX dimensions */
  #define NX 16
  #define NY 16

  /* BiCG dimensions */
  #define NX_BICG 16
  #define NY_BICG 16

  /* Jacobi-1D dimensions */
  #define N_SIZE 32
  #define TSTEPS 8
#endif

/* ============================================================ */
/* SMALL DATASET - 60-120 size (milliseconds runtime)           */
/* ============================================================ */
#ifdef SMALL_DATASET
  /* GEMM, 2mm, 3mm, MVT dimensions */
  #define NI 60
  #define NJ 70
  #define NK 80
  #define NL 90
  #define NM 100

  /* ATAX dimensions */
  #define NX 80
  #define NY 80

  /* BiCG dimensions */
  #define NX_BICG 80
  #define NY_BICG 80

  /* Jacobi-1D dimensions */
  #define N_SIZE 120
  #define TSTEPS 20
#endif

/* ============================================================ */
/* MEDIUM DATASET - 200-400 size (seconds runtime)              */
/* ============================================================ */
#ifdef MEDIUM_DATASET
  /* GEMM, 2mm, 3mm, MVT dimensions */
  #define NI 200
  #define NJ 220
  #define NK 240
  #define NL 260
  #define NM 280

  /* ATAX dimensions */
  #define NX 240
  #define NY 240

  /* BiCG dimensions */
  #define NX_BICG 240
  #define NY_BICG 240

  /* Jacobi-1D dimensions */
  #define N_SIZE 400
  #define TSTEPS 100
#endif

/* ============================================================ */
/* LARGE DATASET - 1000-2000 size (minutes runtime)             */
/* ============================================================ */
#ifdef LARGE_DATASET
  /* GEMM, 2mm, 3mm, MVT dimensions */
  #define NI 1000
  #define NJ 1100
  #define NK 1200
  #define NL 1300
  #define NM 1400

  /* ATAX dimensions */
  #define NX 1200
  #define NY 1200

  /* BiCG dimensions */
  #define NX_BICG 1200
  #define NY_BICG 1200

  /* Jacobi-1D dimensions */
  #define N_SIZE 2000
  #define TSTEPS 500
#endif

/* Integer data type for bare-metal (no FPU dependencies) */
typedef int DATA_TYPE;

/* Timing macros - stubs for bare-metal */
#define polybench_start_instruments
#define polybench_stop_instruments
#define polybench_print_instruments

/* Prevent array scalarization - use restrict qualifier */
#ifdef POLYBENCH_USE_RESTRICT
  #define POLYBENCH_RESTRICT __restrict
#else
  #define POLYBENCH_RESTRICT
#endif

/* Declare 2D array type */
#define POLYBENCH_2D_ARRAY_DECL(var, type, d1, d2, name) \
    type var[d1][d2]

/* Static allocation helpers */
#define POLYBENCH_ALLOC_2D_ARRAY(n1, n2, type) /* noop - static arrays */
#define POLYBENCH_FREE_ARRAY(x) /* noop - static arrays */

#endif /* _POLYBENCH_H */
