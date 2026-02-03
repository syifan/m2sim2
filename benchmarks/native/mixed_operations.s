// mixed_operations.s - Mix of ADD, STR/LDR, and BL
// Tests realistic workload characteristics
// Matches: benchmarks/microbenchmarks.go mixedOperations()
//
// Expected: X0 = 100 at exit
// Calculation:
//   iter1: X0=0, X2=10, store/load X3=10, X0=10, call add_five → X0=15
//   iter2: X0=15, X2=25, store/load X3=25, X0=40, call add_five → X0=45
//   iter3: X0=45, X2=55, store/load X3=55, X0=100

.global _main
.align 4

_main:
    // Setup frame
    stp x29, x30, [sp, #-16]!
    mov x29, sp

    // Allocate buffer (24 bytes = 3 * 8 bytes)
    sub sp, sp, #32     // Keep 16-byte aligned

    // Initialize
    mov x0, #0          // X0 = sum
    mov x1, sp          // X1 = buffer address

    // --- Timing region starts here ---
    // Iteration 1: compute, store, load, call
    add x2, x0, #10     // X2 = X0 + 10 = 10
    str x2, [x1, #0]    // [X1] = X2
    ldr x3, [x1, #0]    // X3 = [X1] = 10
    add x0, x0, x3      // X0 += X3 → X0 = 10
    bl _add_five        // X0 += 5 → X0 = 15

    // Iteration 2
    add x2, x0, #10     // X2 = 25
    str x2, [x1, #8]
    ldr x3, [x1, #8]    // X3 = 25
    add x0, x0, x3      // X0 = 40
    bl _add_five        // X0 = 45

    // Iteration 3
    add x2, x0, #10     // X2 = 55
    str x2, [x1, #16]
    ldr x3, [x1, #16]   // X3 = 55
    add x0, x0, x3      // X0 = 100
    // --- Timing region ends here ---

    // Cleanup
    add sp, sp, #32
    ldp x29, x30, [sp], #16

    // Exit syscall (x0 = 100)
    mov x16, #1         // SYS_exit
    svc #0x80

// add_five: increments x0 by 5
_add_five:
    add x0, x0, #5
    ret
