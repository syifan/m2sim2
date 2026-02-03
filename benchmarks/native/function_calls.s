// function_calls.s - 5 function calls (BL + RET pairs)
// Tests call/return overhead
// Matches: benchmarks/microbenchmarks.go functionCalls()
//
// Expected: X0 = 5 at exit

.global _main
.align 4

_main:
    // Setup frame
    stp x29, x30, [sp, #-16]!
    mov x29, sp

    // Initialize
    mov x0, #0

    // --- Timing region starts here ---
    // 5 function calls to add_one
    bl _add_one
    bl _add_one
    bl _add_one
    bl _add_one
    bl _add_one
    // --- Timing region ends here ---

    // Restore frame
    ldp x29, x30, [sp], #16

    // Exit syscall (x0 = 5)
    mov x16, #1         // SYS_exit
    svc #0x80

// add_one: increments x0 by 1
_add_one:
    add x0, x0, #1
    ret
