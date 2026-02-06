/**
 * gemm.c - General Matrix Multiply for M2Sim
 *
 * Computes: C := alpha*A*B + beta*C
 *
 * This is a bare-metal adaptation of the PolyBench GEMM kernel,
 * using integer arithmetic and static arrays for M2Sim validation.
 *
 * Original: PolyBench/C 4.2.1 (linear-algebra/blas/gemm)
 * Modified: Bare-metal, integer-only
 * Dataset: Configured via polybench.h (default: MEDIUM)
 */

#include "../common/polybench.h"

/* Scaling factors (integer version) */
#define ALPHA 1
#define BETA 1

/* Static arrays - dimensions from polybench.h (NI, NJ, NK) */
static DATA_TYPE A[NI][NK];
static DATA_TYPE B[NK][NJ];
static DATA_TYPE C[NI][NJ];

/**
 * Initialize arrays with deterministic values
 * A[i][k] = (i * NK + k) % 256
 * B[k][j] = (k * NJ + j) % 256
 * C[i][j] = (i * NJ + j) % 256
 */
static void init_array(void) {
    int i, j, k;
    
    for (i = 0; i < NI; i++) {
        for (k = 0; k < NK; k++) {
            A[i][k] = (i * NK + k) % 256;
        }
    }
    
    for (k = 0; k < NK; k++) {
        for (j = 0; j < NJ; j++) {
            B[k][j] = (k * NJ + j) % 256;
        }
    }
    
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NJ; j++) {
            C[i][j] = (i * NJ + j) % 256;
        }
    }
}

/**
 * GEMM kernel: C := alpha*A*B + beta*C
 *
 * Triple-nested loop with O(n^3) operations.
 * Using i-k-j loop order for better cache behavior.
 */
static void kernel_gemm(void) {
    int i, j, k;
    
    polybench_start_instruments;
    
    /* Scale C by beta */
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NJ; j++) {
            C[i][j] *= BETA;
        }
    }
    
    /* Compute C += alpha * A * B */
    for (i = 0; i < NI; i++) {
        for (k = 0; k < NK; k++) {
            for (j = 0; j < NJ; j++) {
                C[i][j] += ALPHA * A[i][k] * B[k][j];
            }
        }
    }
    
    polybench_stop_instruments;
}

/**
 * Compute checksum of result matrix
 * Returns lower 8 bits of sum for validation
 */
static int compute_checksum(void) {
    int i, j;
    int sum = 0;
    
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NJ; j++) {
            sum += C[i][j];
        }
    }
    
    return sum & 0xFF;
}

/**
 * Main entry point
 */
int main(void) {
    /* Initialize arrays */
    init_array();
    
    /* Run GEMM kernel */
    kernel_gemm();
    
    /* Return checksum for validation */
    return compute_checksum();
}
