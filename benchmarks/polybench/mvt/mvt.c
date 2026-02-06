/**
 * mvt.c - Matrix Vector Transpose for M2Sim
 *
 * Computes:
 *   x1 = x1 + A * y1  (matrix-vector multiply)
 *   x2 = x2 + A^T * y2  (transpose matrix-vector multiply)
 *
 * This is a bare-metal adaptation of the PolyBench MVT kernel,
 * using integer arithmetic and static arrays for M2Sim validation.
 *
 * Original: PolyBench/C 4.2.1 (linear-algebra/kernels/mvt)
 * Modified: Bare-metal, integer-only
 * Dataset: Configured via polybench.h (default: MEDIUM)
 */

#include "../common/polybench.h"

/* MVT uses N dimension - alias to NX */
#define N NX

/* Static arrays - dimensions from polybench.h (N=NX) */
static DATA_TYPE A[N][N];
static DATA_TYPE x1[N];
static DATA_TYPE x2[N];
static DATA_TYPE y1[N];
static DATA_TYPE y2[N];

/**
 * Initialize arrays with deterministic values
 * A[i][j] = (i * j) % 256
 * x1[i] = i % 256
 * x2[i] = (i + 1) % 256
 * y1[i] = (i + 3) % 256
 * y2[i] = (i + 4) % 256
 */
static void init_array(void) {
    int i, j;
    
    for (i = 0; i < N; i++) {
        x1[i] = i % 256;
        x2[i] = (i + 1) % 256;
        y1[i] = (i + 3) % 256;
        y2[i] = (i + 4) % 256;
        
        for (j = 0; j < N; j++) {
            A[i][j] = (i * j) % 256;
        }
    }
}

/**
 * MVT kernel:
 *   x1 = x1 + A * y1
 *   x2 = x2 + A^T * y2
 */
static void kernel_mvt(void) {
    int i, j;
    
    polybench_start_instruments;
    
    /* x1 = x1 + A * y1 */
    for (i = 0; i < N; i++) {
        for (j = 0; j < N; j++) {
            x1[i] = x1[i] + A[i][j] * y1[j];
        }
    }
    
    /* x2 = x2 + A^T * y2 (note: A[j][i] for transpose) */
    for (i = 0; i < N; i++) {
        for (j = 0; j < N; j++) {
            x2[i] = x2[i] + A[j][i] * y2[j];
        }
    }
    
    polybench_stop_instruments;
}

/**
 * Compute checksum of result vectors x1 and x2
 * Returns lower 8 bits of sum for validation
 */
static int compute_checksum(void) {
    int i;
    int sum = 0;
    
    for (i = 0; i < N; i++) {
        sum += x1[i];
        sum += x2[i];
    }
    
    return sum & 0xFF;
}

/**
 * Main entry point
 */
int main(void) {
    /* Initialize arrays */
    init_array();
    
    /* Run MVT kernel */
    kernel_mvt();
    
    /* Return checksum for validation */
    return compute_checksum();
}
