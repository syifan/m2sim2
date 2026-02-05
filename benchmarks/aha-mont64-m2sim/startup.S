/* M2Sim bare-metal startup for aha-mont64 */
.section .text.startup
.global _start
.type _start, %function

_start:
    /* Set up stack */
    adrp x0, __stack_top
    add x0, x0, :lo12:__stack_top
    mov sp, x0
    
    /* Clear BSS */
    adrp x0, __bss_start
    add x0, x0, :lo12:__bss_start
    adrp x1, __bss_end
    add x1, x1, :lo12:__bss_end
    mov x2, #0
1:
    cmp x0, x1
    b.ge 2f
    str x2, [x0], #8
    b 1b
2:
    
    /* Call benchmark */
    bl initialise_benchmark
    bl benchmark
    
    /* Store result and halt */
    adrp x1, result
    add x1, x1, :lo12:result
    str w0, [x1]
    
    /* Halt with success */
    mov x0, #0
    brk #0

.section .data
.global result
result:
    .word 0
