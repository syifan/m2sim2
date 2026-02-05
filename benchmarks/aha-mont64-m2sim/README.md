# aha-mont64 M2Sim Benchmark

Embench-IoT aha-mont64 benchmark ported for bare-metal ARM64 execution in M2Sim.

## Description

Montgomery multiplication (64-bit). Pure integer ALU operations with minimal memory access.
Good for testing arithmetic instruction accuracy without memory subsystem effects.

## Building

```bash
./build.sh
```

Requires `aarch64-elf-gcc` cross-compiler (install via `brew install aarch64-elf-gcc`).

## Output

- `aha-mont64_m2sim.elf` - Executable for M2Sim timing simulation
- `aha-mont64_m2sim.dis` - Disassembly for debugging

## Usage

```bash
# Run in M2Sim timing mode
go run . --timing benchmarks/aha-mont64-m2sim/aha-mont64_m2sim.elf
```

## Source

From [embench-iot](https://github.com/embench/embench-iot) `src/aha-mont64/mont64.c`
