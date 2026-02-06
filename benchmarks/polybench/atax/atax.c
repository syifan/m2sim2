/**
 * atax.c - Matrix Transpose and Vector Multiplication for M2Sim
 *
 * Computes: y = A^T * (A * x)
 *
 * This is a bare-metal adaptation of the PolyBench ATAX kernel,
 * using integer arithmetic and static arrays for M2Sim validation.
 *
 * Original: PolyBench/C 4.2.1 (linear-algebra/kernels/atax)
 * Modified: Bare-metal, integer-only
 * Dataset: Configured via polybench.h (default: MEDIUM)
 */

#include "../common/polybench.h"

/* Static arrays - dimensions from polybench.h (NX, NY) */
static DATA_TYPE A[NX][NY];
static DATA_TYPE x[NY];
static DATA_TYPE y[NY];
static DATA_TYPE tmp[NX];

/**
 * Initialize arrays with deterministic values
 * A[i][j] = (i * NY + j) % 256
 * x[i] = i % 256
 * y[i] = 0
 * tmp[i] = 0
 */
static void init_array(void) {
    int i, j;
    
    for (i = 0; i < NX; i++) {
        for (j = 0; j < NY; j++) {
            A[i][j] = ((i * NY) + j) % 256;
        }
    }
    
    for (i = 0; i < NY; i++) {
        x[i] = i % 256;
        y[i] = 0;
    }
    
    for (i = 0; i < NX; i++) {
        tmp[i] = 0;
    }
}

/**
 * ATAX kernel: y = A^T * (A * x)
 *
 * Step 1: tmp = A * x
 * Step 2: y = A^T * tmp
 */
static void kernel_atax(void) {
    int i, j;
    
    polybench_start_instruments;
    
    /* tmp = A * x */
    for (i = 0; i < NX; i++) {
        tmp[i] = 0;
        for (j = 0; j < NY; j++) {
            tmp[i] += A[i][j] * x[j];
        }
    }
    
    /* y = A^T * tmp */
    for (j = 0; j < NY; j++) {
        y[j] = 0;
        for (i = 0; i < NX; i++) {
            y[j] += A[i][j] * tmp[i];
        }
    }
    
    polybench_stop_instruments;
}

/**
 * Compute checksum of result vector
 * Returns lower 8 bits of sum for validation
 */
static int compute_checksum(void) {
    int i;
    int sum = 0;
    
    for (i = 0; i < NY; i++) {
        sum += y[i];
    }
    
    return sum & 0xFF;
}

/**
 * Main entry point
 */
int main(void) {
    /* Initialize arrays */
    init_array();
    
    /* Run ATAX kernel */
    kernel_atax();
    
    /* Return checksum for validation */
    return compute_checksum();
}
