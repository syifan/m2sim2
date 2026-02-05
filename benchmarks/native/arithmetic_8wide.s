// arithmetic_8wide.s - 32 independent ADD operations using 8 registers
// Tests 8-wide superscalar throughput (matching M2 P-core capabilities)
// Matches: benchmarks/microbenchmarks.go arithmetic8Wide()
//
// Uses X0-X7 to maximize parallelism without WAW hazards.
// Expected: X0 = 4 at exit (X0 incremented 4 times across 4 groups)

.global _main
.align 4

_main:
    // Initialize all 8 registers to 0
    mov x0, #0
    mov x1, #0
    mov x2, #0
    mov x3, #0
    mov x4, #0
    mov x5, #0
    mov x6, #0
    mov x7, #0

    // --- Timing region starts here ---
    // Group 1: 8 independent ADDs (all can issue in one cycle on 8-wide)
    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1
    add x5, x5, #1
    add x6, x6, #1
    add x7, x7, #1

    // Group 2: 8 more ADDs
    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1
    add x5, x5, #1
    add x6, x6, #1
    add x7, x7, #1

    // Group 3: 8 more ADDs
    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1
    add x5, x5, #1
    add x6, x6, #1
    add x7, x7, #1

    // Group 4: 8 more ADDs
    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1
    add x5, x5, #1
    add x6, x6, #1
    add x7, x7, #1
    // --- Timing region ends here ---

    // Exit syscall (x0 = 4 = return value)
    mov x16, #1         // SYS_exit
    svc #0x80
