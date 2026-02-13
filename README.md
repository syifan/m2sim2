# M2Sim: Cycle-Accurate Apple M2 CPU Simulator

[![Build Status](https://github.com/sarchlab/m2sim/workflows/CI/badge.svg)](https://github.com/sarchlab/m2sim/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/sarchlab/m2sim)](https://goreportcard.com/report/github.com/sarchlab/m2sim)
[![License](https://img.shields.io/github/license/sarchlab/m2sim.svg)](LICENSE)

**M2Sim** is a cycle-accurate simulator for the Apple M2 CPU that achieves **14.22% average timing error** across 11 microbenchmarks with hardware CPI comparison. Built on the [Akita simulation framework](https://github.com/sarchlab/akita), M2Sim enables detailed performance analysis of ARM64 workloads on Apple Silicon architectures.

## Project Status: In Development

**Current Achievement:** 14.22% average timing error across 11 microbenchmarks with hardware CPI comparison.

| Success Criterion | Target | Achieved | Status |
|------------------|---------|----------|--------|
| **Functional Emulation** | ARM64 user-space execution | Complete | Done |
| **Timing Accuracy** | <20% average error | 14.22% (11 microbenchmarks) | Done |
| **Modular Design** | Separate functional/timing | Implemented | Done |
| **Benchmark Coverage** | μs to ms range | 11 micro + 4 PolyBench + 1 EmBench | Done |

## 🚀 Quick Start

### Prerequisites
- Go 1.21 or later
- ARM64 cross-compiler (`aarch64-linux-musl-gcc`)
- Python 3.8+ (for analysis tools)

### Installation
```bash
# Clone the repository
git clone https://github.com/sarchlab/m2sim.git
cd m2sim

# Build the simulator
go build ./...

# Run tests
ginkgo -r

# Build main binary
go build -o m2sim ./cmd/m2sim
```

### Basic Usage
```bash
# Functional emulation only
./m2sim -elf benchmarks/arithmetic.elf

# Cycle-accurate timing simulation
./m2sim -elf benchmarks/arithmetic.elf -timing

# Fast timing approximation
./m2sim -elf benchmarks/arithmetic.elf -fasttiming
```


## 📊 Performance Results

### Timing Accuracy Summary

| **Benchmark Category** | **Count** | **Average Error** | **Range** |
|----------------------|-----------|------------------|-----------|
| **Microbenchmarks** | 11 | 14.22% | 1.27% - 24.67% |
| **PolyBench** | 4 | sim-only | no hardware CPI comparison |
| **EmBench** | 1 | sim-only | no hardware CPI comparison |
| **Total with error** | **11** | **14.22%** | **1.27% - 24.67%** |

> **Note:** PolyBench and EmBench benchmarks run successfully in simulation but use different dataset scales than hardware measurements (MINI vs LARGE), so direct error comparison is not possible.

### Key Architectural Insights

- **Branch Prediction:** 1.3% error - validates M2's exceptional prediction accuracy
- **Cache Hierarchy:** 3-11% error range - efficient L1I/L1D/L2 hierarchy modeling
- **Memory Bandwidth:** High bandwidth utilization confirmed through concurrent operations
- **SIMD Performance:** 24-30% error indicates complex vector unit timing (improvement area)

## 🏗️ Architecture Overview

### Simulator Components

```
M2Sim Architecture
├── Functional Emulator (emu/)     # ARM64 instruction execution
│   ├── Decoder                    # 200+ ARM64 instructions
│   ├── Register File              # ARM64 register state
│   └── Syscall Interface          # Linux syscall emulation
├── Timing Model (timing/)         # Cycle-accurate performance
│   ├── Pipeline                   # 8-wide superscalar, 5-stage
│   ├── Cache Hierarchy            # L1I (192KB), L1D (128KB), L2 (24MB)
│   └── Branch Prediction          # Two-level adaptive predictor
└── Integration Layer              # ELF loading, measurement framework
```

### Pipeline Configuration
- **Architecture:** 8-wide superscalar, in-order execution
- **Stages:** Fetch → Decode → Execute → Memory → Writeback
- **Branch Predictor:** Two-level adaptive with 12-cycle misprediction penalty
- **Cache Hierarchy:** L1I (192KB, 6-way, 1-cycle), L1D (128KB, 8-way, 4-cycle), L2 (24MB, 16-way, 12-cycle)

## 📁 Project Structure

```
m2sim/
├── cmd/m2sim/                 # Main simulator binary
├── emu/                       # Functional ARM64 emulator
├── timing/                    # Cycle-accurate timing model
│   ├── core/                  # CPU core timing
│   ├── cache/                 # Cache hierarchy
│   ├── pipeline/              # Pipeline implementation
│   └── latency/               # Instruction latencies
├── benchmarks/                # Validation benchmark suite
│   ├── microbenchmarks/       # Targeted stress tests
│   └── polybench/            # Linear algebra kernels
├── docs/                      # Documentation
│   ├── reference/             # Core technical references
│   ├── development/           # Historical development docs
│   └── archive/               # Archived analysis
├── results/                   # Experimental results
│   ├── final/                 # Completion reports
│   └── baselines/             # Hardware measurement data
├── paper/                     # Research paper and figures
└── reproduce_experiments.py   # Complete reproducibility script
```

## 🔬 Research Usage

### Adding New Benchmarks

1. **Compile to ARM64 ELF:**
   ```bash
   aarch64-linux-musl-gcc -static -O2 -o benchmark.elf benchmark.c
   ```

2. **Collect Hardware Baseline:**
   ```python
   # Use multi-scale regression methodology
   # Measure at multiple input sizes: 100, 500, 1K, 5K, 10K instructions
   # Apply linear regression: y = mx + b (m = per-instruction latency)
   ```

3. **Run Simulation:**
   ```bash
   ./m2sim -elf benchmark.elf -timing -limit 100000
   ```

4. **Calculate Error:**
   ```
   error = |t_sim - t_real| / min(t_sim, t_real)
   ```

### Extending the Simulator

**Multi-Core Support:** Framework ready for cache coherence and shared memory
**SIMD Enhancement:** Detailed vector pipeline for improved accuracy
**Out-of-Order:** Register renaming for arithmetic co-issue
**Power Modeling:** Leverage M2's efficiency characteristics

## 📋 Validation Methodology

### Hardware Baseline Collection
- **Platform:** Apple M2 MacBook Air (2022)
- **Measurement:** 15 runs per data point, trimmed mean
- **Regression:** Multi-scale linear fitting (R² > 0.999 required)
- **Validation:** Statistical confidence intervals

### Benchmark Suite Design
- **Microbenchmarks:** Target individual architectural features
- **PolyBench:** Intermediate-complexity linear algebra kernels
- **Coverage:** Arithmetic, memory, branches, SIMD, dependencies

### Error Analysis
- **Formula:** Symmetric relative error measurement
- **Target:** <20% average error across benchmark suite
- **Categories:** Excellent (<10%), Good (10-20%), Acceptable (20-30%)

## 📖 Documentation

### Core References
- **[Architecture Guide](docs/reference/architecture.md)** - M2 microarchitecture research
- **[Timing Guide](docs/reference/timing-guide.md)** - Performance modeling details
- **[Build Setup](docs/reference/build-setup.md)** - Cross-compilation and environment
- **[Calibration Reference](docs/reference/calibration.md)** - Parameter tuning guide

### Research Papers
- **[Project Report](results/final/project_report.md)** - Comprehensive completion analysis
- **[Accuracy Validation](results/final/accuracy_validation.md)** - Detailed experimental results

### Development History
- **[Development Docs](docs/development/)** - Research and analysis from development
- **[Historical Reports](results/archive/)** - Evolution of accuracy and methodology

## 🏆 Achievements

### Technical Milestones
- **H1:** Core simulator with pipeline timing and cache hierarchy
- **H2:** SPEC benchmark enablement with syscall coverage
- **H3:** Microbenchmark calibration achieving 14.22% accuracy
- **H4:** Multi-core analysis framework (statistical foundation complete)
- **H5:** 16 CI-verified benchmarks with 14.22% average accuracy (11 microbenchmarks with hardware CPI)

### Research Contributions
1. **First Open-Source M2 Simulator:** Enables reproducible Apple Silicon research
2. **Validated Methodology:** Multi-scale regression baseline collection
3. **Architectural Insights:** Quantified M2 performance characteristics
4. **Production Accuracy:** 14.22% error on 11 microbenchmarks suitable for research conclusions

## 🔧 Development

### Building from Source
```bash
# Development build with all checks
go build ./...
golangci-lint run ./...
ginkgo -r

# Performance profiling
go build -o profile ./cmd/profile
./profile -elf benchmark.elf -cpuprofile cpu.prof
```

### Contributing
1. **Read:** [CLAUDE.md](CLAUDE.md) for development guidelines
2. **Test:** Ensure all tests pass and lint checks succeed
3. **Document:** Update relevant documentation for changes
4. **Validate:** Verify accuracy on affected benchmarks


## 🤝 Related Projects

- **[Akita](https://github.com/sarchlab/akita)** - Underlying simulation framework
- **[MGPUSim](https://github.com/sarchlab/mgpusim)** - GPU simulator using Akita
- **[SARCH Lab](https://github.com/sarchlab)** - Computer architecture research

## 📞 Support

- **Issues:** [GitHub Issues](https://github.com/sarchlab/m2sim/issues)
- **Documentation:** [Project Wiki](https://github.com/sarchlab/m2sim/wiki)
- **Research:** Contact [SARCH Lab](https://github.com/sarchlab)

## 📜 License

This project is developed by the [SARCH Lab](https://github.com/sarchlab) at [University/Institution].

---

**M2Sim** - Enabling Apple Silicon research through cycle-accurate simulation.

*Generated: February 13, 2026 | Status: In Development*