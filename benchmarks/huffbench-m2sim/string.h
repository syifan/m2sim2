/* Minimal string.h for M2Sim bare-metal benchmarks */
#ifndef STRING_H
#define STRING_H

#include "stddef.h"

/* Inline memcmp implementation */
static inline int memcmp(const void *s1, const void *s2, unsigned long n) {
    const unsigned char *p1 = s1;
    const unsigned char *p2 = s2;
    
    while (n--) {
        if (*p1 != *p2) {
            return *p1 - *p2;
        }
        p1++;
        p2++;
    }
    return 0;
}

/* Inline memset implementation */
static inline void *memset(void *s, int c, unsigned long n) {
    unsigned char *p = s;
    while (n--) {
        *p++ = (unsigned char)c;
    }
    return s;
}

/* Inline memcpy implementation */
static inline void *memcpy(void *dest, const void *src, unsigned long n) {
    unsigned char *d = dest;
    const unsigned char *s = src;
    while (n--) {
        *d++ = *s++;
    }
    return dest;
}

#endif /* STRING_H */
