# SPEC CPU 2017 Integration Guide

This document describes how to set up SPEC CPU 2017 benchmarks for use with M2Sim.

## Prerequisites

- SPEC CPU 2017 installation (version 1.1.x)
- ARM64 cross-compilation toolchain (or native ARM64 build)
- M2Sim built and working

## Setup

### 1. Link SPEC Directory

SPEC cannot be included in the repository due to licensing. Create a symlink to your SPEC installation:

```bash
# From the m2sim root directory
ln -s /path/to/your/spec benchmarks/spec
```

On Marin-2 (development machine):
```bash
ln -s /Users/yifan/Documents/spec benchmarks/spec
```

### 2. Build SPEC Benchmarks for ARM64

SPEC benchmarks must be compiled for ARM64 to run in M2Sim. Using SPEC's build system:

```bash
cd benchmarks/spec
source shrc
# Use an ARM64 config file
runcpu --config=arm64-m2sim --action=build 525.x264_r
```

### 3. Available Benchmarks

SPEC CPU 2017 includes integer (SPECrate) and floating-point benchmarks:

**Integer Rate (for initial testing):**
- `505.mcf_r` - Vehicle scheduling (combinatorial optimization)
- `525.x264_r` - Video compression
- `531.deepsjeng_r` - Chess AI
- `541.leela_r` - Go AI (Monte Carlo tree search)
- `557.xz_r` - Data compression

**Recommended starting point:** `557.xz_r` (relatively simple, good instruction mix)

## Running Benchmarks

### Basic Usage

```bash
./m2sim --benchmark=spec --name=557.xz_r --input=test
```

### Input Sizes

SPEC benchmarks have three input sizes:
- `test` - Minimal (for validation, ~seconds)
- `train` - Medium (for tuning)
- `ref` - Reference (for official runs, minutes to hours)

**For M2Sim validation, use `test` inputs only.** Reference inputs are too large for cycle-accurate simulation.

## Integration Status

| Benchmark | Build | Run | Validated |
|-----------|-------|-----|-----------|
| 557.xz_r  | ⏳    | ⏳  | ⏳        |
| 505.mcf_r | ⏳    | ⏳  | ⏳        |
| 525.x264_r| ⏳    | ⏳  | ⏳        |

## Next Steps

1. ✅ Document SPEC setup (this file)
2. ⏳ Build ARM64 binaries for target benchmarks
3. ⏳ Create benchmark runner scripts
4. ⏳ Validate execution correctness
5. ⏳ Collect timing data for M6 validation
