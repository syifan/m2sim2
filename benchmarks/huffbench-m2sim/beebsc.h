/* BEEBS local library variants header
   Adapted for M2Sim bare-metal benchmarks

   Copyright (C) 2019 Embecosm Limited.
   Contributor Jeremy Bennett <jeremy.bennett@embecosm.com>

   SPDX-License-Identifier: GPL-3.0-or-later */

#ifndef BEEBSC_H
#define BEEBSC_H

#include "stddef.h"

/* BEEBS fixes RAND_MAX to its lowest permitted value, 2^15-1 */
#ifdef RAND_MAX
#undef RAND_MAX
#endif
#define RAND_MAX ((1U << 15) - 1)

/* Local simplified versions of library functions */
int rand_beebs(void);
void srand_beebs(unsigned int new_seed);

void init_heap_beebs(void *heap, size_t heap_size);
int check_heap_beebs(void *heap);
void *malloc_beebs(size_t size);
void *calloc_beebs(size_t nmemb, size_t size);
void *realloc_beebs(void *ptr, size_t size);
void free_beebs(void *ptr);

#endif /* BEEBSC_H */
