/* Minimal stddef.h for M2Sim bare-metal benchmarks */
#ifndef STDDEF_H
#define STDDEF_H

typedef unsigned long size_t;
typedef long ptrdiff_t;

#define NULL ((void *)0)
#define offsetof(type, member) __builtin_offsetof(type, member)

#endif /* STDDEF_H */
