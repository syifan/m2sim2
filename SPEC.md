# SPEC.md - M2Sim Project Specification

## Project Goal

Build a cycle-accurate Apple M2 CPU simulator using the Akita simulation framework that can execute ARM64 user-space programs and predict execution time with high accuracy.

## Success Criteria

- [x] Execute ARM64 user-space programs correctly (functional emulation)
- [ ] Predict execution time with <2% average error across benchmarks
- [ ] Modular design: functional and timing simulation are separate
- [ ] Support benchmarks in μs to ms range

## Milestones

### M1: Foundation (MVP) ✅ COMPLETE
Basic ARM64 execution capability.

**Completion criteria:**
- [x] Project scaffolding with Akita dependency
- [x] ARM64 instruction decoder (basic formats)
- [x] Register file (X0-X30, SP, PC)
- [x] Basic ALU instructions (ADD, SUB, AND, OR, XOR)
- [x] Load/Store instructions (LDR, STR)
- [x] Branch instructions (B, BL, BR, RET, B.cond)

### M2: Memory & Control Flow ✅ COMPLETE
Complete functional emulation.

**Completion criteria:**
- [x] Syscall emulation (exit, write)
- [x] Simple memory model (flat address space)
- [x] Run simple C programs end-to-end

### M3: Timing Model ✅ COMPLETE
Cycle-accurate timing.

**Completion criteria:**
- [x] Pipeline stages (Fetch, Decode, Execute, Memory, Writeback)
- [x] Instruction timing
- [x] Basic timing predictions

### M4: Cache Hierarchy ✅ COMPLETE
Memory system timing.

**Completion criteria:**
- [x] L1 instruction cache (CachedFetchStage)
- [x] L1 data cache (CachedMemoryStage)
- [x] L2 unified cache (CacheBacking hierarchy)
- [x] Cache timing model (integrated via WithICache/WithDCache/WithDefaultCaches)

### M5: Advanced Features
Accuracy improvements.

**Completion criteria:**
- [ ] Branch prediction
- [ ] Out-of-order execution (if needed)
- [ ] SIMD basics

### M6: Validation
Final accuracy validation.

**Completion criteria:**
- [ ] Run standard benchmarks
- [ ] Compare with real M2 timing
- [ ] Achieve <2% average error

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
