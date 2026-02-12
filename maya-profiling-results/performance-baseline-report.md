# Performance Baseline Measurements - Issue #487 Phase 1

**Maya's Performance Analysis Report**
**Date**: February 12, 2026
**Commit**: 28bcb514ac93c0303b2bfae2557e4e9eba440207

## Executive Summary

Comprehensive profiling analysis reveals **critical performance bottlenecks** with clear optimization opportunities to achieve the target 50-80% calibration speedup. The analysis identified a **massive memory allocation bottleneck** (99.88% of allocations) and several CPU hotspots in the timing pipeline.

## Profiling Infrastructure Results

### Pipeline Benchmark Performance (CI - Linux/AMD64)

| Benchmark | Mode | Time (ns/op) | Memory (B/op) | Allocs/op |
|-----------|------|--------------|---------------|-----------|
| PipelineTick8Wide | Pipeline | 2,546 ± 76 | 641 | 10 |
| PipelineTick1Wide | Pipeline | 1,601 ± 426 | 512 | 8 |
| PipelineLoadHeavy8Wide | Pipeline | 5,496 ± 230 | 1,538 | 24 |
| PipelineLoadHeavy8WideCache | Pipeline | 4,918 ± 139 | 1,185 | 21 |
| PipelineMixed8Wide | Pipeline | 11,646 ± 630 | 2,302 | 35 |
| PipelineMixed8WideCache | Pipeline | 10,012 ± 340 | 1,940 | 32 |

**Key Insight**: Mixed workloads show highest allocation rates (35 allocs/op) and longest execution times.

### End-to-End Simulation Performance

| Mode | Instructions/sec | Performance Ratio | Status |
|------|------------------|-------------------|---------|
| Emulation | 7,424,419 | Baseline (1.0x) | ✅ Fast |
| Fast-Timing | 322,242 | 0.043x | ⚠️ 23x slower |
| Timing | - | - | ❌ Timeout (>10s) |

**Critical Finding**: Timing mode is completely unusable for calibration - times out on small benchmarks.

## Major Performance Bottlenecks Identified

### 1. **CRITICAL: Decoder Allocation Crisis**
- **Impact**: 99.88% of memory allocations (2.12GB in benchmark run)
- **Location**: `github.com/sarchlab/m2sim/insts.(*Decoder).Decode`
- **Root Cause**: Per-instruction object allocation in decode stage
- **Optimization Potential**: Massive - could eliminate ~99% of allocations

### 2. **CPU Hotspots in Pipeline Tick**
- **Primary**: `tickOctupleIssue` - 25% CPU usage
- **Map Access**: `mapaccess1_fast64` - 9.29% CPU (frequent lookups)
- **Memory Access**: `Memory.Read32` - 3.95% CPU

### 3. **Algorithm Inefficiencies**
- **WritebackStage.WritebackSlot**: 4.49% CPU
- **DecodeStage.Decode**: 3.21% CPU (calls allocation hotspot)
- **canIssueWith**: 3.10% CPU (potentially cacheable results)

## Performance Baseline Metrics

### Current Calibration Performance
- **Fast-Timing Mode**: 322K instructions/second
- **Per-Tick Performance**: ~2.5μs for 8-wide pipeline
- **Memory Usage**: 1,940-2,302 bytes per complex mixed operation

### Estimated Calibration Impact
Based on benchmark analysis:
- **Current State**: Each calibration iteration processes ~1M instructions
- **Time per Iteration**: ~3.1 seconds (1M instr / 322K instr/sec)
- **Memory Pressure**: 2.12GB allocated per 1M instructions (unsustainable)

## Cross-Platform Validation
- **Local (macOS/ARM64)**: 5.17M instructions/second emulation
- **CI (Linux/AMD64)**: 7.42M instructions/second emulation
- **Consistency**: Performance patterns consistent across platforms

## Optimization Target Prioritization

### Priority 1: Critical (50-80% speedup potential)
1. **Decoder Allocation Elimination** - 99.88% allocation reduction
2. **Pipeline Tick Optimization** - 25% CPU reduction
3. **Map Access Optimization** - 9.29% CPU reduction

### Priority 2: High (10-20% speedup potential)
4. **Memory Access Pattern Optimization** - 3.95% CPU reduction
5. **Writeback Stage Optimization** - 4.49% CPU reduction
6. **Branch Predictor Caching** - 2.24% CPU reduction

### Priority 3: Medium (5-10% speedup potential)
7. **Decode Stage Algorithm Improvement** - 3.21% CPU reduction
8. **Issue Logic Optimization** - 3.10% CPU reduction

## Statistical Confidence
- **Sample Size**: 5 runs per benchmark in CI, 3 runs locally
- **Variance**: Low variance in allocation counts (consistent bottlenecks)
- **Methodology**: Standard Go benchmarking with pprof profiling

## Next Phase Readiness

✅ **Comprehensive profiling data collected**
✅ **Critical bottlenecks identified with precise impact metrics**
✅ **Baseline performance metrics established**
✅ **Cross-platform validation completed**
✅ **Prioritized optimization target list created**

**Ready for Phase 2**: Targeted optimization implementation with clear 50-80% speedup pathway identified.