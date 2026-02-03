// memory_sequential.s - 10 store/load pairs to sequential addresses
// Tests cache/memory performance
// Matches: benchmarks/microbenchmarks.go memorySequential()
//
// Expected: X0 = 42 at exit (value stored and loaded back)

.global _main
.align 4

_main:
    // Allocate stack space for buffer (80 bytes = 10 * 8 bytes)
    sub sp, sp, #80

    // Initialize
    mov x0, #42         // Value to store/load
    mov x1, sp          // Base address (stack buffer)

    // --- Timing region starts here ---
    // 10 store/load pairs at sequential 8-byte offsets
    str x0, [x1, #0]
    ldr x0, [x1, #0]

    str x0, [x1, #8]
    ldr x0, [x1, #8]

    str x0, [x1, #16]
    ldr x0, [x1, #16]

    str x0, [x1, #24]
    ldr x0, [x1, #24]

    str x0, [x1, #32]
    ldr x0, [x1, #32]

    str x0, [x1, #40]
    ldr x0, [x1, #40]

    str x0, [x1, #48]
    ldr x0, [x1, #48]

    str x0, [x1, #56]
    ldr x0, [x1, #56]

    str x0, [x1, #64]
    ldr x0, [x1, #64]

    str x0, [x1, #72]
    ldr x0, [x1, #72]
    // --- Timing region ends here ---

    // Cleanup stack
    add sp, sp, #80

    // Exit syscall (x0 = 42)
    mov x16, #1         // SYS_exit
    svc #0x80
