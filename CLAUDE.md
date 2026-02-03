# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

M2Sim is a cycle-accurate Apple M2 CPU simulator built on the Akita simulation framework. It provides both functional emulation and timing simulation for ARM64 user-space programs.

## Build, Test, and Lint Commands

```bash
# Build all packages
go build ./...

# Run unit tests (uses Ginkgo framework)
ginkgo -r

# Run specific package tests
go test ./emu/... -v

# Lint code
golangci-lint run ./... --timeout=10m

# Run a sample simulation
cd samples/basic && go build && ./basic
```

## Architecture Overview

### Two Simulation Modes

1. **Emulation Mode** (`emu/`): Fast functional simulation
   - Executes ARM64 instructions
   - Used for correctness verification
   - No timing information

2. **Timing Mode** (`timing/`): Cycle-accurate performance simulation
   - Pipeline stages (Fetch, Decode, Execute, Memory, Writeback)
   - Cache hierarchy (L1I, L1D, L2)
   - Branch prediction
   - Produces execution time estimates

### Key Packages

- **`emu/`**: Functional ARM64 emulator
- **`timing/`**: Cycle-accurate timing model
- **`insts/`**: ARM64 instruction definitions and decoder
- **`driver/`**: OS service emulation (syscalls)
- **`benchmarks/`**: Test programs
- **`samples/`**: Runnable examples

### Dependencies

- Akita v4: `github.com/sarchlab/akita/v4` (simulation engine)

## Reusing Akita Components

**Important**: Akita already provides production-ready implementations of cache and memory controllers. **Do not create new implementations in M2Sim.** Instead, use the existing Akita components:

- **Cache**: Use Akita's cache implementation for L1I, L1D, and L2 caches
- **Memory Controller**: Use Akita's memory controller for DRAM simulation

If you encounter issues or limitations with Akita's cache or memory components:
1. **Do not work around it in M2Sim**
2. **Raise an issue in the Akita repository** with the specific problem
3. Fix it upstream so all Akita-based simulators benefit

This keeps M2Sim focused on CPU-specific modeling (pipeline, branch prediction, ARM64 semantics) rather than reinventing general-purpose simulation components.

## ARM64 Instruction Support

Track instruction support status in `insts/SUPPORTED.md`.

## Testing Against Real Hardware

Test programs should be:
1. Compiled with `clang -target arm64-apple-macos`
2. Run on real M2 with timing measurements
3. Compared with simulator predictions

## Code Style

- Follow Go best practices
- Use Akita component/port patterns for timing model
- Separate functional and timing logic
- Write tests for all instruction implementations

## Design Philosophy

See `DESIGN.md` for guidance on naming and architecture decisions. Key point: **M2Sim is not bound to follow MGPUSim's structure** — make decisions that best fit a CPU simulator.
