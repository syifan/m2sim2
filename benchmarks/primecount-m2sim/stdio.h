/* Minimal stdio.h for M2Sim bare-metal - functions are no-ops */
#ifndef STDIO_H
#define STDIO_H

#define NULL ((void*)0)

static inline int printf(const char *fmt, ...) { (void)fmt; return 0; }

#endif /* STDIO_H */
