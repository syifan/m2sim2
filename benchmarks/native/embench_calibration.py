#!/usr/bin/env python3
"""
embench_calibration.py - Linear Regression Calibration for EmBench

Uses varying benchmark() call repetition counts to separate process startup
overhead from actual per-instruction latency via linear regression — the same
methodology as polybench_calibration.py.

Approach:
  1. Build each EmBench benchmark natively with N repetitions
  2. Measure instruction count and wall-clock time for each N
  3. Fit linear regression: time_ms = slope * instruction_count / 1e6 + overhead
  4. slope (ns/instruction) = hardware baseline latency
"""

import json
import os
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Tuple

EMBENCH_BENCHMARKS = {
    "aha-mont64": "Montgomery multiplication (cryptographic)",
    "crc32": "CRC32: Cyclic redundancy check (bit manipulation)",
    "edn": "EDN: Finite impulse response filter (DSP)",
    "huffbench": "Huffbench: Huffman compression/decompression",
    "matmult-int": "MatMult-Int: Integer matrix multiplication",
    "statemate": "Statemate: Car window lift state machine",
    "primecount": "Primecount: Prime number sieve",
}

# Source file names within embench-iot/src/<bench>/
EMBENCH_SOURCE_FILES = {
    "aha-mont64": ["mont64.c"],
    "crc32": ["crc_32.c"],
    "edn": ["edn.c"],
    "huffbench": ["huffbench.c"],
    "matmult-int": ["matmult-int.c"],
    "statemate": ["statemate.c"],
    "primecount": ["primecount.c"],
}

# Rep counts for calibration — EmBench kernels are typically O(n) or O(n^2)
# with small data sizes, so they run very quickly. Use high rep counts.
REP_COUNTS = [100, 500, 1000, 5000, 10000, 50000]

# Known instructions per benchmark() call, measured locally.
# Used as fallback when PMU counters are not available (GitHub Actions VMs).
INSTS_PER_REP = {
    "aha-mont64": 22753,
    "crc32": 13156,
    "edn": 34802,
    "huffbench": 74965,
    "matmult-int": 36270,
    "statemate": 21603,
    "primecount": 15233,
}


def get_embench_src_dir() -> Path:
    return Path(__file__).parent.parent / "embench-iot" / "src"


def get_embench_support_dir() -> Path:
    return Path(__file__).parent.parent / "embench-iot" / "support"


def get_build_dir() -> Path:
    build = Path(__file__).parent.parent / "embench-native-build"
    build.mkdir(exist_ok=True)
    return build


def build_benchmark(bench: str, reps: int) -> Optional[str]:
    """Build an EmBench benchmark natively with given repetition count."""
    src_dir = get_embench_src_dir() / bench
    support_dir = get_embench_support_dir()
    build_dir = get_build_dir()

    source_files = EMBENCH_SOURCE_FILES.get(bench)
    if not source_files:
        print(f"  ERROR: No source files configured for {bench}")
        return None

    # Generate calibration wrapper
    calib_src = build_dir / f"_calib_{bench}.c"
    with open(calib_src, "w") as f:
        f.write(f"/* Auto-generated calibration wrapper for {bench} ({reps} reps) */\n")
        f.write("#include <stdio.h>\n\n")
        # Provide stub implementations for board/trigger functions
        f.write("void initialise_board(void) {}\n")
        f.write("void start_trigger(void) {}\n")
        f.write("void stop_trigger(void) {}\n")
        f.write("void warm_caches(int t) { (void)t; }\n\n")
        # Declare benchmark functions
        f.write("void initialise_benchmark(void);\n")
        f.write("int benchmark(void) __attribute__((noinline));\n\n")
        f.write("int main(void) {\n")
        f.write("    initialise_benchmark();\n")
        f.write(f"    volatile int result = 0;\n")
        f.write(f"    for (int r = 0; r < {reps}; r++) {{\n")
        f.write(f"        result = benchmark();\n")
        f.write(f"    }}\n")
        f.write("    return 0;\n")
        f.write("}\n")

    # Build command
    outname = f"{bench}_native_r{reps}"
    out_path = build_dir / outname

    # Collect source files
    src_paths = [str(calib_src)]
    for sf in source_files:
        src_path = src_dir / sf
        if not src_path.exists():
            print(f"  ERROR: Source file not found: {src_path}")
            return None
        src_paths.append(str(src_path))

    # Also include beebsc.c from support dir (needed by some benchmarks)
    beebsc_path = support_dir / "beebsc.c"
    if beebsc_path.exists():
        src_paths.append(str(beebsc_path))

    cmd = [
        "cc",
        "-O2", "-mcpu=apple-m2",
        "-fno-vectorize", "-fno-slp-vectorize",
        f"-I{src_dir}",
        f"-I{support_dir}",
        "-DCPU_MHZ=1",
        "-DWARMUP_HEAT=0",
    ] + src_paths + ["-o", str(out_path), "-lm"]

    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"  BUILD ERROR ({bench} r{reps}): {result.stderr.strip()}")
        calib_src.unlink(missing_ok=True)
        return None

    calib_src.unlink(missing_ok=True)
    return str(out_path) if out_path.exists() else None


def count_instructions(binary_path: str, verbose: bool = False) -> Optional[int]:
    """Count retired instructions using macOS /usr/bin/time -lp."""
    try:
        result = subprocess.run(
            ["/usr/bin/time", "-lp", binary_path],
            capture_output=True, text=True,
        )
        stderr = result.stderr
        if verbose:
            print(f"    /usr/bin/time stderr: {stderr[:500]}")
        for line in stderr.split("\n"):
            stripped = line.strip()
            if "instructions retired" in stripped:
                return int(stripped.split()[0])
            if "instructions_retired" in stripped:
                parts = stripped.split()
                for p in parts:
                    if p.isdigit():
                        return int(p)
    except Exception as e:
        if verbose:
            print(f"    Exception: {e}")
    return None


def run_timed(binary_path: str, runs: int = 15, warmup: int = 3) -> List[float]:
    """Run binary multiple times with warmup, return times in seconds."""
    for _ in range(warmup):
        subprocess.run([binary_path], capture_output=True)
    times = []
    for _ in range(runs):
        start = time.perf_counter()
        subprocess.run([binary_path], capture_output=True)
        end = time.perf_counter()
        times.append(end - start)
    return times


def trimmed_mean(values: List[float], trim_pct: float = 0.2) -> float:
    """Trimmed mean, removing top/bottom trim_pct."""
    if len(values) < 3:
        return sum(values) / len(values)
    s = sorted(values)
    n = len(s)
    tc = int(n * trim_pct)
    trimmed = s[tc:-tc] if tc > 0 else s
    return sum(trimmed) / len(trimmed) if trimmed else sum(s) / n


def linear_regression(x: List[float], y: List[float]) -> Tuple[float, float, float]:
    """Returns (slope, intercept, r_squared)."""
    try:
        from scipy import stats
        slope, intercept, r, _, _ = stats.linregress(x, y)
        return slope, intercept, r ** 2
    except ImportError:
        pass
    n = len(x)
    sx = sum(x)
    sy = sum(y)
    sxy = sum(a * b for a, b in zip(x, y))
    sx2 = sum(a * a for a in x)
    d = n * sx2 - sx * sx
    if abs(d) < 1e-15:
        return 0.0, sy / n if n else 0.0, 0.0
    slope = (n * sxy - sx * sy) / d
    intercept = (sy - slope * sx) / n
    ym = sy / n
    ss_tot = sum((yi - ym) ** 2 for yi in y)
    ss_res = sum((yi - (slope * xi + intercept)) ** 2 for xi, yi in zip(x, y))
    r2 = 1 - (ss_res / ss_tot) if ss_tot > 0 else 0
    return slope, intercept, r2


@dataclass
class CalibrationResult:
    benchmark: str
    description: str
    instruction_latency_ns: float
    overhead_ms: float
    r_squared: float
    data_points: List[Dict] = field(default_factory=list)


def calibrate_benchmark(
    bench: str, rep_counts: List[int], runs: int = 15, verbose: bool = True
) -> Optional[CalibrationResult]:
    """Calibrate one benchmark using varying repetition counts."""
    desc = EMBENCH_BENCHMARKS[bench]
    if verbose:
        print(f"\n{'='*60}")
        print(f"Calibrating: {bench}")
        print(f"Description: {desc}")
        print(f"{'='*60}")

    data_points = []
    instr_list = []
    time_list = []
    reps_list = []

    first_attempt = True
    for reps in rep_counts:
        if verbose:
            print(f"  reps={reps:>6}... ", end="", flush=True)

        binary = build_benchmark(bench, reps)
        if not binary:
            if verbose:
                print("BUILD FAILED")
            continue

        insts = count_instructions(binary, verbose=first_attempt)
        first_attempt = False

        run_times = run_timed(binary, runs=runs, warmup=3)
        run_times_ms = [t * 1000 for t in run_times]
        avg_ms = trimmed_mean(run_times_ms)

        s = sorted(run_times_ms)
        tc = int(len(s) * 0.2)
        trimmed = s[tc:-tc] if tc > 0 else s
        std_ms = (sum((t - avg_ms) ** 2 for t in trimmed) / len(trimmed)) ** 0.5

        if verbose:
            if insts is not None:
                print(f"{insts:>12,} insts, {avg_ms:8.2f} ms (+/-{std_ms:.2f})")
            else:
                print(f"  no PMU,  {avg_ms:8.2f} ms (+/-{std_ms:.2f})")

        data_points.append({
            "reps": reps,
            "instructions": insts,
            "time_ms": avg_ms,
        })
        if insts is not None:
            instr_list.append(insts)
        time_list.append(avg_ms)
        reps_list.append(reps)

    if len(data_points) < 3:
        if verbose:
            print(f"  FAIL: Need >=3 data points, got {len(data_points)}")
        return None

    # If we have instruction counts, regress time vs instructions
    # Otherwise, regress time vs reps (fallback for VMs without PMU)
    has_instr_counts = len(instr_list) >= 3
    if has_instr_counts:
        slope, intercept, r2 = linear_regression(instr_list, time_list)
        latency_ns = slope * 1e6  # ms/instruction -> ns/instruction
    else:
        slope, intercept, r2 = linear_regression(reps_list, time_list)
        insts_per_rep = INSTS_PER_REP.get(bench, 30000)
        latency_ns = (slope / insts_per_rep) * 1e6
        if verbose:
            print(f"  (Fallback: {insts_per_rep} insts/rep from local measurements)")

    if verbose:
        cpi = latency_ns * 3.5
        print(f"\n  Latency: {latency_ns:.4f} ns/inst (CPI={cpi:.2f} @ 3.5 GHz)")
        print(f"  Overhead: {intercept:.2f} ms")
        print(f"  R^2 = {r2:.6f}")

    return CalibrationResult(
        benchmark=bench,
        description=desc,
        instruction_latency_ns=latency_ns,
        overhead_ms=intercept,
        r_squared=r2,
        data_points=data_points,
    )


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="EmBench Linear Regression Calibration (Issue #488)"
    )
    parser.add_argument(
        "--benchmarks", nargs="*", default=None,
        help="Benchmarks to calibrate (default: all)",
    )
    parser.add_argument(
        "--runs", type=int, default=15,
        help="Timed runs per data point (default: 15)",
    )
    parser.add_argument(
        "--output", type=str, default=None,
        help="Output JSON path",
    )
    args = parser.parse_args()

    print("=" * 70)
    print("EmBench Linear Regression Calibration Tool")
    print("Methodology: Varying benchmark() repetitions (Issue #488)")
    print("=" * 70)

    benchmarks = args.benchmarks or list(EMBENCH_BENCHMARKS.keys())
    for name in benchmarks:
        if name not in EMBENCH_BENCHMARKS:
            print(f"Error: unknown benchmark '{name}'")
            sys.exit(1)

    results = []
    for bench in benchmarks:
        r = calibrate_benchmark(bench, REP_COUNTS, runs=args.runs)
        if r:
            results.append(r)

    if not results:
        print("\nERROR: No benchmarks calibrated successfully.")
        sys.exit(1)

    # Summary
    print("\n" + "=" * 70)
    print("CALIBRATION RESULTS")
    print("=" * 70)
    print(f"{'Benchmark':<15} {'Latency (ns)':<14} {'CPI @3.5GHz':<12} {'R^2':<10}")
    print("-" * 70)
    for r in results:
        cpi = r.instruction_latency_ns * 3.5
        print(f"{r.benchmark:<15} {r.instruction_latency_ns:>11.4f}   "
              f"{cpi:>9.2f}   {r.r_squared:>8.6f}")

    # Save
    output = {
        "methodology": "linear_regression",
        "formula": "time_ms = latency_ns * instruction_count / 1e6 + overhead_ms",
        "source": "embench_rep_scaling",
        "rep_counts": REP_COUNTS,
        "results": [
            {
                "benchmark": r.benchmark,
                "description": r.description,
                "calibrated": True,
                "instruction_latency_ns": r.instruction_latency_ns,
                "overhead_ms": r.overhead_ms,
                "r_squared": r.r_squared,
                "data_points": [
                    {"instructions": d["instructions"], "time_ms": d["time_ms"]}
                    for d in r.data_points
                ],
            }
            for r in results
        ],
    }

    output_path = (
        Path(args.output) if args.output
        else Path(__file__).parent / "embench_calibration_results.json"
    )
    output_path.write_text(json.dumps(output, indent=2))
    print(f"\nResults saved to: {output_path}")


if __name__ == "__main__":
    main()
