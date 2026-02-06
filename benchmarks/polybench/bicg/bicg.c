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
 * Dataset: Configured via polybench.h (default: MEDIUM)
 */

#include "../common/polybench.h"

/* Static arrays - dimensions from polybench.h (NX_BICG, NY_BICG) */
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
