# 8-Wide Benchmark Design

**Author:** Eric (Researcher)
**Date:** 2026-02-05 (Cycle 233)
**Purpose:** Design benchmarks that properly exercise 8-wide superscalar execution

## Background

Bob's analysis (cycle 232) revealed a critical insight: even with PR #220 enabling 8-wide decode in the harness, current microbenchmarks may not show improvement because:

- `arithmetic_sequential` uses only X0-X4 (5 registers)
- `arithmetic_6wide` uses only X0-X5 (6 registers)
- Register reuse creates WAW hazards limiting parallelism

To properly validate 8-wide decode infrastructure (PR #215), we need benchmarks that use 8+ independent registers.

## Proposed Benchmark: arithmetic_8wide

### Design

```go
// arithmetic8Wide - Tests full 8-wide superscalar throughput
// Uses 8 different registers to avoid RAW hazards between groups
func arithmetic8Wide() Benchmark {
    return Benchmark{
        Name:        "arithmetic_8wide",
        Description: "32 independent ADDs using 8 registers - tests full 8-wide issue",
        Setup: func(regFile *emu.RegFile, memory *emu.Memory) {
            regFile.WriteReg(8, 93) // X8 = 93 (exit syscall) -- note: X8 is syscall number
        },
        Program: BuildProgram(
            // Use X0-X7 for 8-wide parallelism (X8 reserved for syscall)
            // Actually need X9-X15 or adjust syscall setup
            // Group 1: 8 independent ADDs
            EncodeADDImm(0, 0, 1, false),   // X0 = X0 + 1
            EncodeADDImm(1, 1, 1, false),   // X1 = X1 + 1
            EncodeADDImm(2, 2, 1, false),   // X2 = X2 + 1
            EncodeADDImm(3, 3, 1, false),   // X3 = X3 + 1
            EncodeADDImm(4, 4, 1, false),   // X4 = X4 + 1
            EncodeADDImm(5, 5, 1, false),   // X5 = X5 + 1
            EncodeADDImm(6, 6, 1, false),   // X6 = X6 + 1
            EncodeADDImm(7, 7, 1, false),   // X7 = X7 + 1
            // Group 2: 8 more ADDs (RAW with group 1, but forwarding resolves)
            EncodeADDImm(0, 0, 1, false),
            EncodeADDImm(1, 1, 1, false),
            EncodeADDImm(2, 2, 1, false),
            EncodeADDImm(3, 3, 1, false),
            EncodeADDImm(4, 4, 1, false),
            EncodeADDImm(5, 5, 1, false),
            EncodeADDImm(6, 6, 1, false),
            EncodeADDImm(7, 7, 1, false),
            // Groups 3-4: repeat
            // ...
            EncodeSVC(0),
        ),
        ExpectedExit: 4, // X0 = 0 + 4*1 = 4
    }
}
```

### Register Allocation Issue

**Problem:** X8 is used for syscall number (93 = exit). This conflicts with using X0-X7.

**Solution Options:**

1. **Use X9-X16 instead** - Avoids X8 conflict
2. **Move syscall setup to X20** - Keep X0-X7 for benchmark, use X20 for syscall number
3. **Accept X8 as read-only** - Use X0-X7, X9 for 8 independent registers

**Recommendation:** Option 2 - Modify syscall convention in benchmark setup to use a higher register (X20 or X28) for syscall number, freeing X0-X8 for benchmarks.

## Expected Results

| Benchmark | Pipeline Width | Expected CPI | Error vs M2 |
|-----------|----------------|--------------|-------------|
| arithmetic_sequential | 5-wide (reg limited) | ~0.45 | ~50% |
| arithmetic_6wide | 6-wide | ~0.40 | ~45% |
| arithmetic_8wide | 8-wide | ~0.30 | ~12% |

**Key insight:** The 8-wide benchmark should show significant improvement over 6-wide if the infrastructure is working correctly.

## Implementation Plan

1. **Pre-req:** PR #220 merges (enables 8-wide in harness)
2. **Step 1:** Add `arithmetic8Wide()` to microbenchmarks.go
3. **Step 2:** Run calibration and compare 6-wide vs 8-wide results
4. **Step 3:** Update accuracy analysis with new baseline

## M2 Native Validation

Need corresponding native benchmark (`benchmarks/native/arithmetic_8wide.s`) to measure real M2 performance with 8-register parallelism.

```asm
// arithmetic_8wide.s
.global _main
_main:
    mov x0, #0
    mov x1, #0
    mov x2, #0
    mov x3, #0
    mov x4, #0
    mov x5, #0
    mov x6, #0
    mov x7, #0
    
    // 8-wide independent ADDs (repeat 4x)
    add x0, x0, #1
    add x1, x1, #1
    add x2, x2, #1
    add x3, x3, #1
    add x4, x4, #1
    add x5, x5, #1
    add x6, x6, #1
    add x7, x7, #1
    // ... repeat 3 more times ...
    
    // exit
    mov x16, #1     // exit syscall (macOS)
    svc #0x80
```

## References

- PR #215 - 8-wide decode infrastructure (merged)
- PR #220 - Enable 8-wide in harness (pending)
- Issue #219 - Harness update request
- Bob's cycle 232 comment - Register usage analysis
