// dependency_chain.s - 20 dependent ADD operations
// Tests instruction latency with RAW hazards (data dependencies)
// Matches: benchmarks/microbenchmarks.go dependencyChain()
//
// Expected: X0 = 20 at exit

.global _main
.align 4

_main:
    // Initialize
    mov x0, #0

    // --- Timing region starts here ---
    // 20 dependent ADDs (each depends on previous result)
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1

    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1

    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1

    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    add x0, x0, #1
    // --- Timing region ends here ---

    // Exit syscall (x0 = 20)
    mov x16, #1         // SYS_exit
    svc #0x80
