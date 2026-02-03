# Native M2 Calibration Benchmarks

Native ARM64 assembly programs for calibrating M2Sim's timing model against real Apple M2 hardware.

## Overview

These benchmarks are 1:1 translations of the microbenchmarks in `benchmarks/microbenchmarks.go`. They allow direct comparison between simulator and real hardware performance.

| Benchmark | Description | Expected Exit Code |
|-----------|-------------|-------------------|
| `arithmetic_sequential` | 20 independent ADDs (ALU throughput) | 4 |
| `dependency_chain` | 20 dependent ADDs (forwarding latency) | 20 |
| `memory_sequential` | 10 store/load pairs (cache performance) | 42 |
| `function_calls` | 5 BL/RET pairs (call overhead) | 5 |
| `branch_taken` | 5 unconditional branches | 5 |
| `mixed_operations` | Mix of ALU, memory, calls | 100 |

## Requirements

- Apple Silicon Mac (M1/M2/M3)
- Xcode Command Line Tools (`xcode-select --install`)

## Building

```bash
cd benchmarks/native
make
```

## Running

```bash
# Run all benchmarks (shows exit codes)
make run

# Verify exit codes match expectations
make verify
```

## Collecting Performance Data

### Method 1: /usr/bin/time (Basic)

```bash
/usr/bin/time -l ./dependency_chain
```

Shows wall clock time, user/system time, and memory stats.

### Method 2: Instruments (Recommended)

Apple's Instruments provides access to CPU performance counters including cycle counts.

#### Using Xcode Instruments GUI:

1. Open Instruments: `open -a Instruments`
2. Choose "Time Profiler" or "CPU Counters" template
3. File → Record Options → Set target to your benchmark binary
4. Add PMC events: Cycles, Instructions, Branch Mispredictions, L1D Cache Misses
5. Record and analyze

#### Using xctrace CLI:

```bash
# Record CPU counters for a benchmark
xctrace record --template 'CPU Counters' --output trace.trace --launch -- ./dependency_chain

# Export to readable format
xctrace export --input trace.trace --output trace.xml
```

### Method 3: powermetrics (System-wide)

```bash
# Must run as root - shows system-wide CPU counters
sudo powermetrics --samplers cpu_power -i 100
```

### Method 4: Sample Script for Batch Collection

```bash
#!/bin/bash
# collect_stats.sh - Run benchmark N times and collect stats

BENCHMARK=$1
ITERATIONS=${2:-100}

echo "Benchmark: $BENCHMARK"
echo "Iterations: $ITERATIONS"
echo ""

total_ns=0
for i in $(seq 1 $ITERATIONS); do
    # Use gdate for nanosecond precision (install: brew install coreutils)
    start=$(gdate +%s%N)
    ./$BENCHMARK
    end=$(gdate +%s%N)
    elapsed=$((end - start))
    total_ns=$((total_ns + elapsed))
done

avg_ns=$((total_ns / ITERATIONS))
avg_us=$((avg_ns / 1000))

echo "Average time: ${avg_us} microseconds"
```

## Interpreting Results

### What to Compare

1. **CPI (Cycles Per Instruction)**: Key metric for timing model accuracy
   - Run simulator: `go run ./cmd/m2sim benchmark --json`
   - Run native: Collect cycles via Instruments
   - Compare CPI for each benchmark

2. **Relative Performance**: Which benchmarks are faster/slower?
   - If simulator shows dependency_chain 2x slower than arithmetic_sequential
   - Real hardware should show similar ratio

### Expected M2 Characteristics

Based on published data and testing:

| Characteristic | Expected Value |
|---------------|----------------|
| P-core frequency | 3.5 GHz |
| ALU latency (independent) | ~1 cycle |
| ALU latency (dependent) | ~1 cycle with forwarding |
| L1D hit latency | ~4 cycles |
| Branch misprediction penalty | ~12-14 cycles |
| BL/RET overhead | ~1-2 cycles (predicted) |

## Comparing with Simulator

Run the simulator benchmarks:

```bash
cd ../..
go run ./cmd/m2sim benchmark --json > sim_results.json
```

Then compare CPI values:

```bash
# Example comparison workflow (pseudo-code)
# sim_cpi = sim_results[benchmark]["cycles"] / sim_results[benchmark]["instructions"]  
# hw_cpi = hardware_cycles / hardware_instructions
# error = abs(sim_cpi - hw_cpi) / hw_cpi * 100
```

## Troubleshooting

### "This Makefile requires ARM64"
You're on an Intel Mac. These benchmarks require Apple Silicon.

### Permission denied for Instruments
Grant Terminal.app "Full Disk Access" in System Preferences → Privacy & Security.

### xctrace not found
Install Xcode Command Line Tools: `xcode-select --install`

## Files

```
benchmarks/native/
├── Makefile                    # Build system
├── README.md                   # This file
├── arithmetic_sequential.s     # ALU throughput test
├── dependency_chain.s          # RAW hazard test  
├── memory_sequential.s         # Cache/memory test
├── function_calls.s            # BL/RET overhead test
├── branch_taken.s              # Branch overhead test
└── mixed_operations.s          # Realistic workload test
```
