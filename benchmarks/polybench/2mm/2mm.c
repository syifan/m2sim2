/**
 * 2mm.c - Two Matrix Multiplications for M2Sim
 *
 * Computes: D := alpha*A*B*C + beta*D
 *   Step 1: tmp = alpha * A * B
 *   Step 2: D = tmp * C + beta * D
 *
 * This is a bare-metal adaptation of the PolyBench 2MM kernel,
 * using integer arithmetic and static arrays for M2Sim validation.
 *
 * Original: PolyBench/C 4.2.1 (linear-algebra/kernels/2mm)
 * Modified: Bare-metal, integer-only
 * Dataset: Configured via polybench.h (default: MEDIUM)
 */

#include "../common/polybench.h"

/* Scaling factors (integer version) */
#define ALPHA 1
#define BETA 1

/* Static arrays - dimensions from polybench.h (NI, NJ, NK, NL) */
static DATA_TYPE A[NI][NK];
static DATA_TYPE B[NK][NJ];
static DATA_TYPE C[NJ][NL];
static DATA_TYPE D[NI][NL];
static DATA_TYPE tmp[NI][NJ];

/**
 * Initialize arrays with deterministic values
 * A[i][k] = (i * k + 1) % 256
 * B[k][j] = (k * (j + 1)) % 256
 * C[j][l] = ((j * (l + 3) + 1)) % 256
 * D[i][l] = (i * (l + 2)) % 256
 */
static void init_array(void) {
    int i, j, k, l;
    
    for (i = 0; i < NI; i++) {
        for (k = 0; k < NK; k++) {
            A[i][k] = (i * k + 1) % 256;
        }
    }
    
    for (k = 0; k < NK; k++) {
        for (j = 0; j < NJ; j++) {
            B[k][j] = (k * (j + 1)) % 256;
        }
    }
    
    for (j = 0; j < NJ; j++) {
        for (l = 0; l < NL; l++) {
            C[j][l] = ((j * (l + 3) + 1)) % 256;
        }
    }
    
    for (i = 0; i < NI; i++) {
        for (l = 0; l < NL; l++) {
            D[i][l] = (i * (l + 2)) % 256;
        }
    }
}

/**
 * 2MM kernel: D := alpha*A*B*C + beta*D
 *
 * Step 1: tmp = alpha * A * B
 * Step 2: D = tmp * C + beta * D
 */
static void kernel_2mm(void) {
    int i, j, k;
    
    polybench_start_instruments;
    
    /* tmp = alpha * A * B */
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NJ; j++) {
            tmp[i][j] = 0;
            for (k = 0; k < NK; k++) {
                tmp[i][j] += ALPHA * A[i][k] * B[k][j];
            }
        }
    }
    
    /* D = tmp * C + beta * D */
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NL; j++) {
            D[i][j] *= BETA;
            for (k = 0; k < NJ; k++) {
                D[i][j] += tmp[i][k] * C[k][j];
            }
        }
    }
    
    polybench_stop_instruments;
}

/**
 * Compute checksum of result matrix D
 * Returns lower 8 bits of sum for validation
 */
static int compute_checksum(void) {
    int i, j;
    int sum = 0;
    
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NL; j++) {
            sum += D[i][j];
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
    
    /* Run 2MM kernel */
    kernel_2mm();
    
    /* Return checksum for validation */
    return compute_checksum();
}
