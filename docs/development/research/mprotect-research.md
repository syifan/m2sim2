# mprotect Syscall Research for SPEC Benchmarks

Research report on mprotect syscall requirements for SPEC CPU 2017 benchmarks in M2Sim.

## Executive Summary

**Recommendation:** Implement mprotect as a **no-op returning success (0)**. This approach is used by gem5 and other CPU simulators and is sufficient for running SPEC benchmarks.

## Syscall Specification

```c
int mprotect(void *addr, size_t len, int prot)
```

**ARM64 syscall number:** 226 (0xE2)

**Parameters:**
- X0: addr - start address (must be page-aligned)
- X1: len - length of memory region
- X2: prot - protection flags

**Protection Flags:**
```
PROT_NONE  = 0x0  // Memory cannot be accessed
PROT_READ  = 0x1  // Memory can be read
PROT_WRITE = 0x2  // Memory can be modified
PROT_EXEC  = 0x4  // Memory can be executed
```

**Return:** 0 on success, -errno on error

## Where mprotect Is Used

### 1. glibc malloc

glibc malloc uses mprotect for memory management:
- Initially assigns large chunks with PROT_NONE protection
- Changes protection to PROT_READ|PROT_WRITE on first access
- The MALLOC_TOP_PAD environment variable controls this behavior

Source: [Oracle Linux blog on glibc malloc tuning](https://blogs.oracle.com/linux/tuning-glibc-malloc-on-arm-a-case-study)

### 2. Stack Guard Pages

Guard pages at the end of the stack are protected with mprotect to:
- Detect stack overflow
- Generate SIGSEGV instead of memory corruption
- Thread stacks use pthread_attr_setguardsize for this

Source: [Linux man page](https://man7.org/linux/man-pages/man3/pthread_attr_setguardsize.3.html)

### 3. Fortran Runtime

Fortran signal handling may use mprotect for:
- Stack protection
- Signal handler setup (sigaltstack)
- Crash detection and tracebacks

Source: [Intel Fortran documentation](https://www.intel.com/content/www/us/en/docs/fortran-compiler/developer-guide-reference/2024-2/signal-handling-on-linux.html)

### 4. JIT Compilation

Programs with JIT compilers use mprotect to:
- Make generated code pages executable (PROT_EXEC)
- Toggle between writable and executable states

## gem5 Approach

gem5's syscall emulation (SE) mode handles mprotect by **ignoring it with a warning**:

```
warn: ignoring syscall mprotect(...)
```

Key points from gem5 implementation:
- munmap is a no-op returning 0 (similar approach)
- Full memory protection tracking is not implemented
- This is sufficient for running SPEC benchmarks
- Full system (FS) mode uses real kernel instead

Source: [gem5 syscall_emul.cc](https://pages.cs.wisc.edu/~swilson/gem5-docs/syscall__emul_8cc_source.html)

## SPEC Benchmark Analysis

### 548.exchange2_r (Fortran Sudoku Solver)

**mprotect usage:** Minimal to none
- Pure computational benchmark
- No file I/O, no JIT
- Stack protection may be only use case
- Will work without mprotect enforcement

### 505.mcf_r (Vehicle Scheduling)

**mprotect usage:** Likely from glibc malloc
- C program with dynamic memory allocation
- No JIT compilation
- Should work with no-op mprotect

### 541.leela_r (Go AI)

**mprotect usage:** Likely from glibc malloc
- C++ program
- Monte Carlo tree search allocates memory
- No JIT, no signals
- Should work with no-op mprotect

### 531.deepsjeng_r (Chess Engine)

**mprotect usage:** Likely from glibc malloc
- C++ chess engine
- Heavy transposition table usage
- No JIT
- Should work with no-op mprotect

## Implementation Recommendation

### Phase 1: No-Op Implementation (Recommended)

```go
func handleMprotect(ctx *Context, x0, x1, x2 uint64) uint64 {
    // Log for debugging
    log.Printf("mprotect: addr=0x%x len=%d prot=%d (ignored)", x0, x1, x2)
    return 0 // Success
}
```

**Advantages:**
- Matches gem5 approach
- Sufficient for all SPEC benchmarks
- Zero implementation complexity
- No performance overhead

### Phase 2: Protection Tracking (Optional)

If benchmarks fail with the no-op approach:

1. Track protection bits per memory page
2. Store in a map: `map[pageAddr]protectionFlags`
3. Check on memory access (significant overhead)

**Not recommended unless needed** - adds complexity with no benefit for SPEC.

### Phase 3: Full Enforcement (Not Recommended)

Full enforcement would require:
- Intercepting all load/store operations
- Checking protection bits on every access
- Significant performance impact
- Unlikely to be needed for any SPEC benchmark

## Error Cases to Consider

If implementing validation (not required for no-op):

| Error | Condition | Value |
|-------|-----------|-------|
| EINVAL | addr not page-aligned | -22 |
| EINVAL | invalid prot flags | -22 |
| ENOMEM | region outside valid memory | -12 |

## Testing Strategy

1. Run 548.exchange2_r with no-op mprotect
2. If it works, validate with strace comparison
3. Only add protection tracking if a benchmark fails

## References

- [mprotect(2) Linux man page](https://man7.org/linux/man-pages/man2/mprotect.2.html)
- [gem5 SE mode documentation](https://old.gem5.org/SE_Mode.html)
- [gem5 syscall emulation source](https://pages.cs.wisc.edu/~swilson/gem5-docs/syscall__emul_8cc_source.html)
- [GNU C Library Memory Protection](https://www.gnu.org/software/libc/manual/html_node/Memory-Protection.html)
- [glibc malloc tuning on ARM](https://blogs.oracle.com/linux/tuning-glibc-malloc-on-arm-a-case-study)

---
*Research by Eric (Cycle 318)*
