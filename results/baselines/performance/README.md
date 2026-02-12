# M2Sim Performance Baseline Framework

## Overview
This directory implements performance baseline storage and regression monitoring for M2Sim's timing simulation optimization framework (Issue #481).

## Baseline File Format

### Schema Definition
```json
{
  "baseline_metadata": {
    "creation_date": "YYYY-MM-DD",
    "commit_hash": "sha",
    "timing_model_version": "description",
    "measurement_environment": {
      "platform": "ubuntu/macos",
      "go_version": "1.25.x",
      "cpu_info": "processor model",
      "memory_gb": 32
    }
  },
  "benchmarks": {
    "benchmark_name": {
      "emulation": {
        "instructions_per_sec": 1500000,
        "memory_mb": 125.5,
        "cpi": "N/A"
      },
      "timing": {
        "instructions_per_sec": 45000,
        "memory_mb": 280.0,
        "cpi": 1.45
      },
      "fast_timing": {
        "instructions_per_sec": 890000,
        "memory_mb": 190.2,
        "cpi": 1.12
      }
    }
  }
}
```

## Measurement Protocol

### Baseline Collection
- Use `cmd/profile` tool for consistent measurement
- Run each mode for 30 seconds minimum
- Average across 3 runs for statistical significance
- Collect CPU profiling data for bottleneck identification

### Regression Monitoring
- Performance degradation >10% triggers investigation
- Compare against most recent baseline (<14 days)
- Cross-mode consistency validation
- Integration with existing accuracy baseline framework

## Integration with Calibration Framework
- Extends existing `results/baselines/` versioning protocol
- Maintains consistency with accuracy measurement methodology
- Supports Phase 2 optimization target identification

---
*Created: 2026-02-12 by Alex as part of Issue #481 Phase 1 implementation*