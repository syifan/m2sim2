// branch_taken.s - 5 unconditional branches
// Tests unconditional branch overhead
// Matches: benchmarks/microbenchmarks.go branchTaken()
//
// Expected: X0 = 5 at exit

.global _main
.align 4

_main:
    // Initialize
    mov x0, #0

    // --- Timing region starts here ---
    // Branch 1: jump over skipped instruction
    b .L1_land
    add x1, x1, #99     // skipped
.L1_land:
    add x0, x0, #1      // X0 = 1

    // Branch 2
    b .L2_land
    add x1, x1, #99     // skipped
.L2_land:
    add x0, x0, #1      // X0 = 2

    // Branch 3
    b .L3_land
    add x1, x1, #99     // skipped
.L3_land:
    add x0, x0, #1      // X0 = 3

    // Branch 4
    b .L4_land
    add x1, x1, #99     // skipped
.L4_land:
    add x0, x0, #1      // X0 = 4

    // Branch 5
    b .L5_land
    add x1, x1, #99     // skipped
.L5_land:
    add x0, x0, #1      // X0 = 5
    // --- Timing region ends here ---

    // Exit syscall (x0 = 5)
    mov x16, #1         // SYS_exit
    svc #0x80
