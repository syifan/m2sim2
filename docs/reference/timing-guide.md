# Timing Benchmark Guide

## Overview

Timing simulation collects simulated cycle counts for benchmarks, comparing against native M2 execution to measure accuracy.

**Target:** <20% average error (per issue #141)

## Available Benchmarks

| Benchmark | Instructions | Status |
|-----------|-------------|--------|
| aha-mont64 | 1.88M | Ready |
| crc32 | 1.57M | Ready |
| matmult-int | 3.85M | Ready |
| primecount | 2.84M | Ready |
| edn | ~1M | Ready |

## Running Timing Simulation

### Quick Test (single benchmark)

```bash
cd ~/dev/src/github.com/sarchlab/m2sim
go test -v -run "TestBenchmark.*ahamont64" ./benchmarks/ -timeout 10m
```

### Batch Run (all benchmarks)

```bash
./scripts/batch-timing.sh 2>&1 | tee reports/timing-output.log
```

**Warning:** Timing simulation is slow. Each benchmark may take 5-10+ minutes. Run overnight or in a dedicated session.

### Recommended: Background/Overnight Run

```bash
nohup ./scripts/batch-timing.sh > reports/timing-$(date +%Y-%m-%d).log 2>&1 &
echo $! > reports/timing.pid
```

Check progress:
```bash
tail -f reports/timing-$(date +%Y-%m-%d).log
```

## Results Location

- Raw output: `reports/embench-timing-YYYY-MM-DD.json`
- Logs: `reports/timing-YYYY-MM-DD.log`

## Next Steps After Timing

1. Extract simulated cycles from timing output
2. Compare against native M2 results (if available)
3. Calculate error percentages
4. Tune pipeline parameters (issue width, branch penalty)
5. Re-run timing to verify improvements

## Current Accuracy Baseline (Microbenchmarks)

| Benchmark | Error |
|-----------|-------|
| arithmetic | 49.3% |
| dependency | 18.9% |
| branch | 51.3% |
| **Average** | 39.8% |

Key tuning targets:
- Arithmetic: 8-wide issue + instruction fusion
- Branch: Reduce misprediction penalty
