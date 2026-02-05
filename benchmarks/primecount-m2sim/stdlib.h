/* Minimal stdlib.h for M2Sim bare-metal - functions are no-ops */
#ifndef STDLIB_H
#define STDLIB_H

#define NULL ((void*)0)

static inline void *malloc(unsigned long size) { (void)size; return NULL; }
static inline void free(void *ptr) { (void)ptr; }

#endif /* STDLIB_H */
