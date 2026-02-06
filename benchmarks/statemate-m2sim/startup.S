/* M2Sim startup for edn benchmark */

    .section .text.startup
    .global _start
    .type _start, %function

_start:
    /* Set up stack pointer */
    ldr x0, =0x80000
    mov sp, x0
    
    /* Clear BSS */
    ldr x0, =__bss_start
    ldr x1, =__bss_end
1:
    cmp x0, x1
    b.ge 2f
    str xzr, [x0], #8
    b 1b
2:
    
    /* Initialize benchmark */
    bl initialise_benchmark
    
    /* Run benchmark */
    bl benchmark
    mov x19, x0          /* Save result */
    
    /* Verify result */
    mov x0, x19
    bl verify_benchmark
    
    /* Exit with verification result (0 = success, 1 = failure) */
    cmp x0, #0
    cset x0, eq          /* x0 = 1 if verify returned 0 (success), else 0 */
    eor x0, x0, #1       /* Invert: 0 = success exit code */
    mov x8, #93          /* exit syscall */
    svc #0
    
    /* Fallback halt */
    b .

    .size _start, . - _start
