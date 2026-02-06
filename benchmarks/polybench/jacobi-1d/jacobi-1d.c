/**
 * jacobi-1d.c - 1D Jacobi Stencil for M2Sim
 *
 * Computes iterative stencil smoothing:
 *   B[i] = (A[i-1] + A[i] + A[i+1]) / 3
 *
 * This is a bare-metal adaptation of the PolyBench jacobi-1d kernel,
 * using integer arithmetic for M2Sim validation.
 *
 * Original: PolyBench/C 4.2.1 (stencils/jacobi-1d)
 */

#include "../common/polybench.h"

/* Array dimensions - MINI dataset */
#ifndef TSTEPS
#define TSTEPS 8
#endif
#ifndef N_SIZE
#define N_SIZE 32
#endif

/* Static arrays */
static DATA_TYPE A[N_SIZE];
static DATA_TYPE B[N_SIZE];

/**
 * Initialize arrays with deterministic values
 */
static void init_array(void) {
    int i;
    for (i = 0; i < N_SIZE; i++) {
        A[i] = (i * 3) % 256;
        B[i] = (i * 2) % 256;
    }
}

/**
 * Jacobi 1D kernel
 * Iterative stencil: B[i] = (A[i-1] + A[i] + A[i+1]) / 3
 */
static void kernel_jacobi_1d(void) {
    int t, i;
    
    polybench_start_instruments;
    
    for (t = 0; t < TSTEPS; t++) {
        /* Compute B from A */
        for (i = 1; i < N_SIZE - 1; i++) {
            B[i] = (A[i-1] + A[i] + A[i+1]) / 3;
        }
        /* Copy B back to A */
        for (i = 1; i < N_SIZE - 1; i++) {
            A[i] = B[i];
        }
    }
    
    polybench_stop_instruments;
}

/**
 * Compute checksum
 */
static int compute_checksum(void) {
    int i;
    int sum = 0;
    
    for (i = 0; i < N_SIZE; i++) {
        sum += A[i];
    }
    
    return sum & 0xFF;
}

int main(void) {
    init_array();
    kernel_jacobi_1d();
    return compute_checksum();
}
