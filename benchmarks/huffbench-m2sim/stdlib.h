/* Minimal stdlib.h for M2Sim bare-metal benchmarks */
#ifndef STDLIB_H
#define STDLIB_H

#include "stddef.h"
#include "stdint.h"

/* exit() stub - just spin */
static inline void exit(int status) {
    (void)status;
    while(1);
}

#endif /* STDLIB_H */
