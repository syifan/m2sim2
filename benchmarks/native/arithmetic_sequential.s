// arithmetic_sequential.s - 20 independent ADD operations
// Tests ALU throughput with independent operations
// Matches: benchmarks/microbenchmarks.go arithmeticSequential()
//
// Expected: X0 = 4 at exit (X0 incremented 4 times total across rounds)

.global _main
.align 4

_main:
    // Initialize
    mov x0, #0
    mov x1, #0
    mov x2, #0
    mov x3, #0
    mov x4, #0

    // --- Timing region starts here ---
    // 20 independent ADDs to different registers
    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1

    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1

    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1

    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1
    // --- Timing region ends here ---

    // Exit syscall (x0 already has return value = 4)
    mov x16, #1         // SYS_exit
    svc #0x80
