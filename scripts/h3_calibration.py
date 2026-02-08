#!/usr/bin/env python3
"""
H3 Calibration Framework for M2Sim

This script runs timing simulations on benchmarks and compares results
with M2 hardware baseline data for accuracy calibration.
"""

import os
import sys
import subprocess
import json
import time
from pathlib import Path

class H3Calibrator:
    def __init__(self, repo_root=None):
        if repo_root is None:
            repo_root = Path(__file__).parent.parent
        self.repo_root = Path(repo_root)
        self.results = []

    def run_timing_simulation(self, benchmark_path, runs=3):
        """Run M2Sim timing simulation on benchmark"""
        print(f"Running timing simulation: {benchmark_path}")

        cmd = ["go", "run", "./cmd/m2sim/main.go", "-timing", str(benchmark_path)]
        times = []

        for run in range(runs):
            print(f"  Run {run + 1}/{runs}...")
            start_time = time.time()

            result = subprocess.run(
                cmd,
                cwd=self.repo_root,
                capture_output=True,
                text=True
            )

            elapsed = time.time() - start_time

            if result.returncode != 0:
                print(f"    Error: {result.stderr}")
                return None

            times.append(elapsed)
            print(f"    Time: {elapsed:.3f}s")

        avg_time = sum(times) / len(times)
        print(f"  Average time: {avg_time:.3f}s")
        return {
            "times": times,
            "avg_time": avg_time,
            "stdout": result.stdout,
            "stderr": result.stderr
        }

    def collect_hardware_baseline(self, benchmark_path, runs=5):
        """Collect M2 hardware timing baseline"""
        print(f"Collecting M2 hardware baseline: {benchmark_path}")

        if not os.path.exists(benchmark_path):
            print(f"  Error: Benchmark not found: {benchmark_path}")
            return None

        times = []

        for run in range(runs):
            print(f"  Run {run + 1}/{runs}...")
            start_time = time.time()

            result = subprocess.run([benchmark_path], capture_output=True, text=True)
            elapsed = time.time() - start_time

            if result.returncode != 0:
                print(f"    Error: {result.stderr}")
                return None

            times.append(elapsed)
            print(f"    Time: {elapsed:.3f}s")

        avg_time = sum(times) / len(times)
        print(f"  Average time: {avg_time:.3f}s")
        return {
            "times": times,
            "avg_time": avg_time,
            "stdout": result.stdout,
            "stderr": result.stderr
        }

    def compare_results(self, sim_result, hw_result, benchmark_name):
        """Compare simulation vs hardware results"""
        if sim_result is None or hw_result is None:
            return None

        sim_time = sim_result["avg_time"]
        hw_time = hw_result["avg_time"]

        if hw_time == 0:
            error_pct = float('inf')
        else:
            error_pct = abs((sim_time - hw_time) / hw_time) * 100

        result = {
            "benchmark": benchmark_name,
            "sim_time": sim_time,
            "hw_time": hw_time,
            "error_pct": error_pct,
            "sim_runs": sim_result["times"],
            "hw_runs": hw_result["times"]
        }

        self.results.append(result)
        print(f"\nCalibration Result for {benchmark_name}:")
        print(f"  Simulation time: {sim_time:.3f}s")
        print(f"  Hardware time:   {hw_time:.3f}s")
        print(f"  Error:          {error_pct:.1f}%")

        return result

    def save_results(self, output_path="calibration_results_h3.json"):
        """Save calibration results to JSON file"""
        output_file = self.repo_root / output_path

        summary = {
            "timestamp": time.strftime("%Y-%m-%d %H:%M:%S"),
            "total_benchmarks": len(self.results),
            "results": self.results
        }

        if self.results:
            avg_error = sum(r["error_pct"] for r in self.results if r["error_pct"] != float('inf')) / len(self.results)
            summary["average_error_pct"] = avg_error

        with open(output_file, 'w') as f:
            json.dump(summary, f, indent=2)

        print(f"\nResults saved to: {output_file}")
        return summary

    def run_microbenchmark_calibration(self):
        """Run calibration on existing microbenchmarks"""
        print("=== H3 Calibration: Microbenchmarks ===\n")

        # Check for existing microbenchmarks
        native_path = self.repo_root / "benchmarks" / "native"

        if not native_path.exists():
            print("Error: benchmarks/native directory not found")
            return

        # List of microbenchmarks to test
        benchmarks = [
            "arithmetic_sequential",
            "dependency_chain",
            "branch_taken_conditional"
        ]

        success_count = 0

        for benchmark in benchmarks:
            print(f"\n--- Calibrating {benchmark} ---")

            benchmark_path = native_path / benchmark
            if not benchmark_path.exists():
                print(f"  Skipping {benchmark}: not found")
                continue

            # Run timing simulation
            sim_result = self.run_timing_simulation(benchmark_path)

            # Collect hardware baseline
            hw_result = self.collect_hardware_baseline(benchmark_path)

            # Compare results
            comparison = self.compare_results(sim_result, hw_result, benchmark)

            if comparison is not None:
                success_count += 1

        print(f"\n=== Calibration Summary ===")
        print(f"Benchmarks processed: {success_count}/{len(benchmarks)}")

        if success_count > 0:
            self.save_results()


def main():
    calibrator = H3Calibrator()

    if len(sys.argv) > 1:
        command = sys.argv[1]

        if command == "microbenchmarks":
            calibrator.run_microbenchmark_calibration()
        elif command == "benchmark" and len(sys.argv) > 2:
            benchmark_path = sys.argv[2]
            sim_result = calibrator.run_timing_simulation(benchmark_path)
            hw_result = calibrator.collect_hardware_baseline(benchmark_path)
            calibrator.compare_results(sim_result, hw_result, os.path.basename(benchmark_path))
            calibrator.save_results()
        else:
            print("Usage:")
            print("  python3 h3_calibration.py microbenchmarks")
            print("  python3 h3_calibration.py benchmark <path>")
    else:
        calibrator.run_microbenchmark_calibration()


if __name__ == "__main__":
    main()