# SPEC CPU 2017 Syscall Requirements for M2Sim

This document maps the syscalls required to run SPEC CPU 2017 benchmarks in M2Sim's
syscall emulation mode.

## Current M2Sim Syscall Support

| Syscall | Number (ARM64) | Status |
|---------|----------------|--------|
| exit    | 93 (0x5D)      | ✅ Implemented |
| write   | 64 (0x40)      | ✅ Implemented (FD extension in PR #280) |
| read    | 63 (0x3F)      | ✅ Implemented (FD extension in PR #280) |
| close   | 57 (0x39)      | ✅ Implemented (PR #267) |
| openat  | 56 (0x38)      | ✅ Implemented (PR #268) |
| brk     | 214 (0xD6)     | ✅ Implemented (PR #275) |
| mmap    | 222 (0xDE)     | ✅ Ready to Merge (PR #276) |
| fstat   | 80 (0x50)      | ✅ Ready to Merge (PR #279) |

**File Descriptor Table:** ✅ Implemented (PR #266)

## Required Syscalls for SPEC Benchmarks

Based on research into SPEC CPU 2017 benchmarks and common patterns in CPU simulators
(gem5, etc.), the following syscalls are needed:

### Priority 1: Essential (Blocking SPEC execution)

| Syscall | Number (ARM64) | Purpose | Priority |
|---------|----------------|---------|----------|
| read    | 63 (0x3F)      | Read input files | Critical |
| openat  | 56 (0x38)      | Open input/output files | Critical |
| close   | 57 (0x39)      | Close file descriptors | Critical |
| brk     | 214 (0xD6)     | Heap memory allocation | Critical |
| mmap    | 222 (0xDE)     | Memory mapping, large allocations | Critical |

### Priority 2: Common Operations

| Syscall | Number (ARM64) | Purpose | Priority |
|---------|----------------|---------|----------|
| fstat   | 80 (0x50)      | Get file statistics | High |
| newfstatat | 79 (0x4F)   | Get file status (at) | High |
| mprotect | 226 (0xE2)    | Set memory protection | Medium |
| munmap  | 215 (0xD7)     | Unmap memory regions | Medium |
| exit_group | 94 (0x5E)   | Exit all threads | Medium |
| lseek   | 62 (0x3E)      | Seek in file | Medium |

### Priority 3: Process/Thread (if needed)

| Syscall | Number (ARM64) | Purpose | Priority |
|---------|----------------|---------|----------|
| getpid  | 172 (0xAC)     | Get process ID | Low |
| getuid  | 174 (0xAE)     | Get user ID | Low |
| gettimeofday | 169 (0xA9) | Get time | Low |

## Benchmark-Specific Analysis

### 505.mcf_r (Vehicle Scheduling)

**I/O Pattern:**
- Reads single input file (network problem specification)
- Writes two output files: `inp.out` (log), `mcf.out` (solution)
- Simplified I/O compared to commercial version

**Expected Syscalls:**
- `openat`, `read`, `close` (file I/O)
- `write` (already implemented)
- `brk` or `mmap` (memory for data structures)
- `fstat` (file size checks)

**Memory Characteristics:**
- Uses mostly integer arithmetic
- Pointer-intensive data structures
- Moderate memory footprint

### 531.deepsjeng_r (Chess AI)

**I/O Pattern:**
- Reads position file (FEN notation chess positions)
- Writes analysis output
- Needs dummy `reftime` and `refpower` files

**Expected Syscalls:**
- `openat`, `read`, `close` (file I/O)
- `write` (already implemented)
- `brk`, `mmap` (heap for search trees)

**Memory Characteristics:**
- ~700 MiB for rate version
- Stores transposition tables, search trees
- Alpha-beta tree search heavy

### Other SPEC Integer Benchmarks

| Benchmark | Primary Syscalls | Memory Pattern |
|-----------|-----------------|----------------|
| 525.x264_r | read/write intensive | Frame buffers |
| 541.leela_r | minimal I/O | Monte Carlo trees |
| 557.xz_r | read/write (compression) | Sliding window |

## Implementation Strategy

Based on current task board guidance, implement syscalls in this order:

### Phase 1: File I/O Basics
1. **read (63)** - Essential for all benchmarks
2. **close (57)** - File descriptor cleanup

### Phase 2: File Opening
3. **openat (56)** - Open files (ARM64 uses openat, not open)

### Phase 3: Memory Management
4. **brk (214)** - Simple heap allocation
5. **mmap (222)** - Memory mapping for large allocations

### Phase 4: File Metadata
6. **fstat (80)** - File statistics
7. **lseek (62)** - File seeking

## File Descriptor Management

M2Sim will need a simple file descriptor table:

```
FD 0: stdin (read-only, may return EOF)
FD 1: stdout (write - already working)
FD 2: stderr (write - already working)
FD 3+: Opened files
```

## Notes on ARM64 Syscalls

- ARM64 uses `openat` (56) instead of `open` (deprecated)
- `newfstatat` (79) is preferred over `fstat` for new code
- All syscall numbers are in X8 register
- Return value in X0 (negative for -errno on error)

## References

- [ARM64 Syscall Table](https://arm64.syscall.sh/)
- [SPEC CPU 2017 Documentation](https://www.spec.org/cpu2017/Docs/)
- [gem5 SPEC Tutorial](https://www.gem5.org/documentation/gem5art/tutorials/spec-tutorial)
- [505.mcf_r Description](https://www.spec.org/cpu2017/Docs/benchmarks/505.mcf_r.html)
- [531.deepsjeng_r Description](https://www.spec.org/cpu2017/Docs/benchmarks/531.deepsjeng_r.html)

---
*Research compiled by Eric (Cycle 301)*
*Updated by Eric (Cycle 304) — 5 syscalls now implemented*
*Updated by Eric (Cycle 305) — 6 syscalls implemented (brk merged), mmap in review*
*Updated by Eric (Cycle 306) — 8 syscalls ready (mmap, fstat PRs approved, pending merge)*
