/**
 * 3mm.c - Three Matrix Multiplications for M2Sim
 *
 * Computes:
 *   E := A x B  (NI x NK) × (NK x NJ) = (NI x NJ)
 *   F := C x D  (NJ x NL) × (NL x NM) = (NJ x NM)
 *   G := E x F  (NI x NJ) × (NJ x NM) = (NI x NM)
 *
 * This is a bare-metal adaptation of the PolyBench 3mm kernel,
 * using integer arithmetic and static arrays for M2Sim validation.
 *
 * Original: PolyBench/C 4.2.1 (linear-algebra/kernels/3mm)
 * Modified: Bare-metal, integer-only, MINI dataset
 */

#include "../common/polybench.h"

/* Matrix dimensions for 3MM - all 16 for MINI dataset */
/* Use NI, NJ, NK from polybench.h, define NL and NM locally */
#ifndef NL
#define NL 16
#endif
#ifndef NM
#define NM 16
#endif

/* Static arrays for MINI dataset (16x16) */
static DATA_TYPE A[NI][NK];
static DATA_TYPE B[NK][NJ];
static DATA_TYPE C[NJ][NL];
static DATA_TYPE D[NL][NM];
static DATA_TYPE E[NI][NJ];  /* E := A x B */
static DATA_TYPE F[NJ][NM];  /* F := C x D */
static DATA_TYPE G[NI][NM];  /* G := E x F */

/**
 * Initialize arrays with deterministic values
 */
static void init_array(void) {
    int i, j;
    
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NK; j++) {
            A[i][j] = (i * NK + j) % 256;
        }
    }
    
    for (i = 0; i < NK; i++) {
        for (j = 0; j < NJ; j++) {
            B[i][j] = (i * NJ + j + 1) % 256;
        }
    }
    
    for (i = 0; i < NJ; i++) {
        for (j = 0; j < NL; j++) {
            C[i][j] = (i * NL + j + 2) % 256;
        }
    }
    
    for (i = 0; i < NL; i++) {
        for (j = 0; j < NM; j++) {
            D[i][j] = (i * NM + j + 3) % 256;
        }
    }
    
    /* Zero output matrices */
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NJ; j++) {
            E[i][j] = 0;
        }
    }
    
    for (i = 0; i < NJ; i++) {
        for (j = 0; j < NM; j++) {
            F[i][j] = 0;
        }
    }
    
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NM; j++) {
            G[i][j] = 0;
        }
    }
}

/**
 * 3MM kernel: Three matrix multiplications
 * E := A x B
 * F := C x D
 * G := E x F
 */
static void kernel_3mm(void) {
    int i, j, k;
    
    polybench_start_instruments;
    
    /* E := A x B */
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NJ; j++) {
            for (k = 0; k < NK; k++) {
                E[i][j] += A[i][k] * B[k][j];
            }
        }
    }
    
    /* F := C x D */
    for (i = 0; i < NJ; i++) {
        for (j = 0; j < NM; j++) {
            for (k = 0; k < NL; k++) {
                F[i][j] += C[i][k] * D[k][j];
            }
        }
    }
    
    /* G := E x F */
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NM; j++) {
            for (k = 0; k < NJ; k++) {
                G[i][j] += E[i][k] * F[k][j];
            }
        }
    }
    
    polybench_stop_instruments;
}

/**
 * Compute checksum of result matrix G
 * Returns lower 8 bits of sum for validation
 */
static int compute_checksum(void) {
    int i, j;
    int sum = 0;
    
    for (i = 0; i < NI; i++) {
        for (j = 0; j < NM; j++) {
            sum += G[i][j];
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
    
    /* Run 3MM kernel */
    kernel_3mm();
    
    /* Return checksum for validation */
    return compute_checksum();
}
