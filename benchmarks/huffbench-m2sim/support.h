/* Minimal support.h for M2Sim bare-metal benchmarks */
#ifndef SUPPORT_H
#define SUPPORT_H

/* CPU MHz for scale factor - set to 1 for minimal iterations */
#define CPU_MHZ 1

/* Include BEEBS heap library for benchmarks that need malloc */
#include "beebsc.h"

/* Standard benchmark interface */
void initialise_benchmark(void);
int benchmark(void);
int verify_benchmark(int result);
void warm_caches(int heat);

#endif /* SUPPORT_H */
