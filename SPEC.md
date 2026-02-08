# SPEC.md - M2Sim Project Specification

## Project Goal

Build a cycle-accurate Apple M2 CPU simulator using the Akita simulation framework that can execute ARM64 user-space programs and predict execution time with high accuracy.

## Success Criteria

- [x] Execute ARM64 user-space programs correctly (functional emulation)
- [ ] Predict execution time with <20% average error across benchmarks
- [ ] Modular design: functional and timing simulation are separate
- [ ] Support benchmarks in μs to ms range

## Design Philosophy

### Independence from MGPUSim

While M2Sim uses Akita (like MGPUSim) and draws inspiration from MGPUSim's architecture, **M2Sim is not bound to follow MGPUSim's structure**. Make design decisions that best fit an ARM64 CPU simulator.

**Guidelines:**

1. **Choose meaningful names**: If a different name is more appropriate, use it
2. **Adapt to CPU semantics**: GPU and CPU have different abstractions (no wavefronts, warps, or GPU-specific concepts)
3. **Keep it simple**: M2Sim targets single-core initially
4. **Diverge when it makes sense**: Document why you're doing it differently

**What to Keep from MGPUSim:**
- Akita component/port patterns (they work well)
- Separation of concerns (functional vs timing)
- Testing practices (Ginkgo/Gomega)

**When in Doubt:** Ask "What would make this clearest for a CPU simulator?" — not "What does MGPUSim do?"

## Milestones

### High-Level Milestones

| # | Milestone | Status |
|---|-----------|--------|
| H1 | Core simulator (decode, execute, timing, caches) | ✅ COMPLETE |
| H2 | SPEC benchmark enablement (syscalls, ELF loading, validation) | ✅ COMPLETE |
| H3 | Accuracy calibration (<20% error on SPEC) | 🚧 IN PROGRESS |
| H4 | Multi-core support | ⬜ NOT STARTED |

---

### H1: Core Simulator ✅ COMPLETE

All foundation work is done: ARM64 decode, ALU/Load/Store/Branch instructions, pipeline timing (Fetch/Decode/Execute/Memory/Writeback), cache hierarchy (L1I, L1D, L2), branch prediction, 8-wide superscalar, macro-op fusion, SIMD basics. Microbenchmark suite established with 34.2% average CPI error.

<details>
<summary>Completed sub-milestones (M1–M5, C1)</summary>

- M1: Foundation — project scaffold, decoder, register file, ALU, load/store, branches
- M2: Memory & control flow — syscall emulation (exit, write), flat memory, end-to-end C programs
- M3: Timing model — pipeline stages, instruction timing
- M4: Cache hierarchy — L1I, L1D, L2 caches with timing
- M5: Advanced features — branch prediction, 8-wide superscalar, macro-op fusion, SIMD
- C1: Baseline — microbenchmarks created, M2 data collected, initial error 39.8% → 34.2%

</details>

---

### H2: SPEC Benchmark Enablement ✅ COMPLETE

**Goal:** Run SPEC CPU 2017 integer benchmarks end-to-end in M2Sim.

**Status:** All core infrastructure complete. PR #300 merged (syscall coverage), PR #315 needs merge (medium benchmarks). Ready for H3 calibration phase.

#### H2.1: Syscall Coverage (medium-level) ✅ COMPLETE

Complete the set of Linux syscalls needed by SPEC benchmarks.

##### H2.1.1: Core file I/O syscalls ✅ COMPLETE
- [x] read (63), write (64), close (57), openat (56) — all merged
- [x] FD table infrastructure — merged
- [x] fstat (80) — merged
- [x] File I/O acceptance tests — merged (PR #283)

##### H2.1.2: Memory management syscalls ✅ COMPLETE
- [x] brk (214) — merged
- [x] mmap (222) — merged

##### H2.1.3: Remaining syscalls ✅ COMPLETE
- [x] lseek (62) — merged (PR #282)
- [x] exit_group (94) — merged (PR #299)
- [x] mprotect (226) — **merged (PR #300)**

##### H2.1.4: Lower-priority syscalls ⬜ NOT STARTED (~10-20 cycles)
- [ ] munmap (215) — issue #271
- [ ] clock_gettime (113) — issue #274
- [ ] getpid/getuid/gettid — issue #273
- [ ] newfstatat (79) — may be needed by some benchmarks

#### H2.2: Micro & Medium Benchmarks (medium-level) 🚧 IN PROGRESS

**Human guidance (issue #107):** Going directly to SPEC is too large a leap. We need more microbenchmarks and medium-sized benchmarks first. SPEC simulations are long-running and must not be run by agents directly — they should run in CI (GitHub Actions) with sufficient time limits, triggered periodically (e.g., every 24 hours).

##### H2.2.1: Expand microbenchmark suite 🚧 NEARLY COMPLETE
- [x] Add microbenchmarks for memory access patterns (strided) — merged (PR #302)
- [x] Add microbenchmarks for instruction mix (load-heavy, store-heavy, branch-heavy) — merged (PR #302)
- [ ] Add microbenchmarks for cache behavior (L1 hit, L2 hit, cache miss)
- [x] Native assembly implementations created — Diana completed all 4 benchmarks (issue #309)
- [ ] Collect M2 hardware CPI data for new microbenchmarks — **ready for measurement** (issue #309)

##### H2.2.2: Medium-sized benchmarks ✅ FIRST BENCHMARK READY
- [x] **Matrix multiply benchmark created** — Leo completed 100x100 integer matrix multiply (PR #315, merge pending)
- [ ] Create additional medium benchmarks: linked list traversal, sorting algorithms, simple parsers (future H2 extensions)
- [ ] Issues #291 tracks additional medium benchmark work

#### H2.3: SPEC Binary Preparation (medium-level) ✅ COMPLETE

**Issue #285 resolved:** Workers successfully compiled ARM64 Linux ELF binaries using cross-compilation toolchain.

##### H2.3.1: Cross-compilation setup ✅ COMPLETE
- [x] Workers install/use ARM64 Linux cross-compiler (aarch64-linux-musl-gcc) — merged (PR #306)
- [x] Create build scripts for ARM64 Linux static ELF — merged (PR #306)
- [x] Rebuild SPEC benchmarks as ELF — merged (PR #306)

##### H2.3.2: Benchmark validation 🚧 IN PROGRESS (~10-20 cycles per benchmark)
- [ ] 548.exchange2_r — Sudoku solver, compiled as ARM64 ELF, ready for validation (issue #277)
- [ ] 505.mcf_r — vehicle scheduling, compiled as ARM64 ELF
- [ ] 541.leela_r — Go AI, minimal I/O
- [ ] 531.deepsjeng_r — chess engine, compiled as ARM64 ELF

**Important:** SPEC simulation runs must go through CI/GitHub Actions, not be run by agents directly.

#### H2.4: Instruction Coverage Gaps 🚧 IN PROGRESS

SPEC benchmarks will likely exercise ARM64 instructions not yet implemented. Expect to discover and fix gaps during validation (H2.3.2).

##### H2.4.1: SIMD/FP dispatch wiring ✅ COMPLETE
- [x] Wire FormatSIMDReg and FormatSIMDLoadStore in emulator — merged (PR #301)
- [x] VFADD, VFSUB, VFMUL now reachable through emulator dispatch

##### H2.4.2: Scalar floating-point instructions ⬜ NOT STARTED (~20-40 cycles)
- [ ] Basic scalar FP arithmetic: FADD, FSUB, FMUL, FDIV
- [ ] FP load/store: LDR/STR for S and D registers
- [ ] FP moves and comparisons: FMOV, FCMP
- [ ] Int↔FP conversions: SCVTF, FCVTZS
- [ ] Update SUPPORTED.md with all FP instructions — **blocked** (issue #305, QA responsibility)

**Strategy:** Don't implement proactively. Attempt benchmark execution first; add scalar FP support reactively when benchmarks fail on unimplemented opcodes. SPEC integer benchmarks may not need much FP.

---

### H3: Accuracy Calibration 🚧 IN PROGRESS

**Goal:** Achieve <20% average CPI error on SPEC benchmarks vs real M2 hardware.

**Strategy:** Dual-track calibration approach - immediate microbenchmark measurements while enabling medium-scale calibration.

**Current Status (Cycle 235):**
- ✅ **Infrastructure Complete**: H3 calibration framework approved and ready to merge (PR #321)
- ✅ **SIMD Foundation**: DUP instruction implemented and validated by QA (issue #322 closed)
- 🚧 **Active Execution**: MRS system instruction (issue #324) + microbenchmark ELF compilation (issue #320)

**Current microbenchmark baseline (cycle 230):**

| Benchmark | Sim CPI | M2 CPI | Error |
|-----------|---------|--------|-------|
| arithmetic_sequential | 0.400 | 0.268 | 49.3% |
| dependency_chain | 1.200 | 1.009 | 18.9% |
| branch_taken_conditional | 1.600 | 1.190 | 34.5% |
| **Average** | — | — | **34.2%** |

#### H3.1: Immediate Calibration Launch (~10-15 cycles) 🚧 IN PROGRESS
- [x] H3 calibration framework ready (PR #321 approved)
- [ ] Microbenchmark ARM64 ELF recompilation (issue #320)
- [ ] Baseline accuracy measurements collection
- [ ] Calibration methodology verification

#### H3.2: Medium-Scale Calibration (~15-25 cycles) 🚧 IN PROGRESS
- [x] SIMD DUP instruction implemented (issue #322 closed)
- [ ] MRS system instruction implementation (issue #324)
- [ ] Matrix multiply benchmark completion
- [ ] Medium-scale accuracy data collection
- [ ] Cross-scale calibration comparison

#### H3.3: Advanced Calibration Tuning (~50-100 cycles)
- [ ] Full 8-wide execution modeling
- [ ] Out-of-order execution refinements
- [ ] Memory latency calibration
- [ ] Target: <20% average error across all scales

#### H3.4: SPEC-level calibration (~100+ cycles)
- [ ] Run SPEC benchmarks in CI with timing, compare to M2 hardware
- [ ] Tune parameters to minimize error
- [ ] All benchmarks <30% individual error, <20% average

---

### H4: Multi-Core Support ⬜ NOT STARTED

Long-term goal (issue #139). Not planned in detail yet.

## Scope

### In Scope
- ARM64 user-space instructions
- CPU core simulation (single-core MVP, multi-core later)
- Cache hierarchy
- Timing prediction

### Out of Scope
- GPU / Neural Engine
- Kernel-space execution
- Full OS simulation
- I/O devices beyond basic syscalls

## Technical Constraints

- Use Akita v4 simulation framework
- Follow MGPUSim architecture patterns
- Go programming language
- Tests use Ginkgo/Gomega

## References

- Akita: https://github.com/sarchlab/akita
- MGPUSim: https://github.com/sarchlab/mgpusim
- ARM Architecture Reference Manual
- See `docs/calibration.md` for timing parameter reference
