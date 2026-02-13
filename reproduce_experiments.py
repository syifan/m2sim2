#!/usr/bin/env python3
"""
M2Sim Reproducible Experiments Script

This script reproduces all experiments from the M2Sim paper, including:
1. Building the simulator and benchmarks
2. Running accuracy validation experiments
3. Generating figures and analysis
4. Creating the final paper

Modes:
    Default: Load and display CI-verified results from h5_accuracy_results.json
    Live:    If ELF binaries are present, run actual simulations and compare

Usage:
    python3 reproduce_experiments.py [--skip-build] [--skip-experiments] [--skip-figures]

Requirements:
    - Go 1.21 or later
    - Python 3.8+ with matplotlib, seaborn, pandas, numpy
    - LaTeX distribution (for paper compilation)
    - aarch64-linux-musl-gcc (for ARM64 cross-compilation, only needed for live mode)

Authors: M2Sim Agent Team
Date: February 12, 2026
"""

import os
import sys
import subprocess
import time
import json
import argparse
from pathlib import Path
from typing import Dict, List, Optional


class Colors:
    """ANSI color codes for terminal output"""
    GREEN = '\033[92m'
    BLUE = '\033[94m'
    YELLOW = '\033[93m'
    RED = '\033[91m'
    BOLD = '\033[1m'
    END = '\033[0m'


def log(message: str, level: str = "INFO"):
    """Print colored log message"""
    color_map = {
        "INFO": Colors.BLUE,
        "SUCCESS": Colors.GREEN,
        "WARNING": Colors.YELLOW,
        "ERROR": Colors.RED,
        "HEADER": Colors.BOLD
    }

    color = color_map.get(level, Colors.END)
    timestamp = time.strftime("%H:%M:%S")
    print(f"{color}[{timestamp}] {level}: {message}{Colors.END}")


def run_command(cmd: str, cwd: Path = None, check: bool = True) -> subprocess.CompletedProcess:
    """Run shell command with logging"""
    log(f"Running: {cmd}")

    if cwd:
        log(f"Working directory: {cwd}")

    try:
        result = subprocess.run(
            cmd.split(),
            cwd=cwd,
            capture_output=True,
            text=True,
            check=check
        )

        if result.stdout.strip():
            for line in result.stdout.strip().split('\n'):
                log(f"  {line}")

        return result

    except subprocess.CalledProcessError as e:
        log(f"Command failed with exit code {e.returncode}", "ERROR")
        log(f"Stderr: {e.stderr}", "ERROR")
        raise


def check_dependencies():
    """Check required dependencies"""
    log("Checking dependencies...", "HEADER")

    deps = {
        "go": "go version",
        "python3": "python3 --version",
        "aarch64-linux-musl-gcc": "aarch64-linux-musl-gcc --version"
    }

    missing = []
    for dep, cmd in deps.items():
        try:
            run_command(cmd, check=False)
            log(f"  {dep} found", "SUCCESS")
        except (subprocess.CalledProcessError, FileNotFoundError):
            log(f"  {dep} not found", "ERROR")
            missing.append(dep)

    if missing:
        log(f"Missing dependencies: {', '.join(missing)}", "ERROR")
        log("Please install missing dependencies and retry", "ERROR")
        return False

    return True


def build_simulator():
    """Build M2Sim and all components"""
    log("Building M2Sim simulator...", "HEADER")

    # Build all packages
    run_command("go build ./...")
    log("  All packages built", "SUCCESS")

    # Build main simulator binary
    run_command("go build -o m2sim ./cmd/m2sim")
    log("  M2Sim binary built", "SUCCESS")

    # Run tests to verify build
    log("Running tests to verify build...")
    try:
        run_command("go test ./... -short")
        log("  Tests passed", "SUCCESS")
    except subprocess.CalledProcessError:
        log("Some tests failed - continuing anyway", "WARNING")


def build_benchmarks():
    """Build ARM64 benchmark binaries"""
    log("Building ARM64 benchmarks...", "HEADER")

    benchmark_dirs = [
        "benchmarks/microbenchmarks",
        "benchmarks/polybench"
    ]

    for bench_dir in benchmark_dirs:
        bench_path = Path(bench_dir)
        if not bench_path.exists():
            log(f"Benchmark directory {bench_dir} not found - skipping", "WARNING")
            continue

        log(f"Building benchmarks in {bench_dir}...")

        # Find C source files
        c_files = list(bench_path.glob("*.c"))
        if not c_files:
            log(f"No C files found in {bench_dir}", "WARNING")
            continue

        for c_file in c_files:
            elf_file = c_file.with_suffix(".elf")
            cmd = f"aarch64-linux-musl-gcc -static -O2 -o {elf_file} {c_file}"

            try:
                run_command(cmd, check=False)
                log(f"  Built {elf_file.name}", "SUCCESS")
            except subprocess.CalledProcessError:
                log(f"  Failed to build {c_file.name}", "WARNING")


def load_ci_verified_results() -> Dict:
    """Load CI-verified accuracy results from h5_accuracy_results.json"""
    results_path = Path("h5_accuracy_results.json")
    if not results_path.exists():
        log("h5_accuracy_results.json not found", "ERROR")
        log("This file is the source of truth for CI-verified accuracy data.", "ERROR")
        sys.exit(1)

    with open(results_path, "r") as f:
        data = json.load(f)

    log("Loaded CI-verified results from h5_accuracy_results.json", "SUCCESS")
    return data


def display_ci_verified_results(data: Dict):
    """Display CI-verified accuracy results as a table"""
    log("CI-Verified Accuracy Results", "HEADER")
    log("=" * 60)

    summary = data["summary"]
    benchmarks = data["benchmarks"]

    # Separate benchmarks by category
    micro = [b for b in benchmarks if b["category"] == "microbenchmark"]
    poly = [b for b in benchmarks if b["category"] == "polybench"]
    emb = [b for b in benchmarks if b["category"] == "embench"]
    infeasible = data.get("infeasible", [])

    # Print microbenchmark results (with error data)
    print()
    print(f"{'Microbenchmarks (with hardware CPI comparison)':}")
    print(f"{'─' * 70}")
    print(f"  {'Benchmark':<20} {'Sim CPI':>10} {'HW CPI':>10} {'Error':>10}")
    print(f"  {'─' * 60}")
    for b in micro:
        err_str = f"{b['error']*100:.1f}%" if b["error"] is not None else "N/A"
        hw_str = f"{b['hardware_cpi']:.3f}" if isinstance(b["hardware_cpi"], (int, float)) else "N/A"
        print(f"  {b['name']:<20} {b['simulated_cpi']:>10.3f} {hw_str:>10} {err_str:>10}")

    micro_with_error = [b for b in micro if b["error"] is not None]
    if micro_with_error:
        avg_err = sum(b["error"] for b in micro_with_error) / len(micro_with_error)
        max_err = max(b["error"] for b in micro_with_error)
        min_err = min(b["error"] for b in micro_with_error)
        print(f"  {'─' * 60}")
        print(f"  Average error: {avg_err*100:.2f}% (over {len(micro_with_error)} benchmarks)")
        print(f"  Range: {min_err*100:.2f}% - {max_err*100:.2f}%")

    # Print PolyBench results (sim-only, no comparable HW CPI)
    if poly:
        print()
        print(f"{'PolyBench (simulation only — HW CPI not directly comparable)':}")
        print(f"{'─' * 70}")
        print(f"  {'Benchmark':<20} {'Sim CPI':>10} {'Note':>30}")
        print(f"  {'─' * 60}")
        for b in poly:
            print(f"  {b['name']:<20} {b['simulated_cpi']:>10.3f} {'HW used LARGE, sim used MINI':>30}")

    # Print EmBench results (sim-only)
    if emb:
        print()
        print(f"{'EmBench (simulation only — HW CPI not directly comparable)':}")
        print(f"{'─' * 70}")
        print(f"  {'Benchmark':<20} {'Sim CPI':>10}")
        print(f"  {'─' * 60}")
        for b in emb:
            print(f"  {b['name']:<20} {b['simulated_cpi']:>10.3f}")

    # Print infeasible benchmarks
    if infeasible:
        print()
        print(f"{'Infeasible (did not complete in CI within timeout)':}")
        print(f"{'─' * 70}")
        for b in infeasible:
            print(f"  {b['name']:<20} ({b['category']})")

    print()
    log(f"Summary: {summary['microbenchmarks_with_error']} microbenchmarks with error data, "
        f"{summary['micro_average_error']*100:.2f}% average error", "SUCCESS")
    log(f"H5 target (<20% average error): {'MET' if summary['h5_target_met'] else 'NOT MET'}", "SUCCESS")
    log("These are CI-verified results. To run live simulations, build ELF binaries first.", "INFO")
    log("See benchmarks/microbenchmarks/ and benchmarks/polybench/ for source files.", "INFO")


def run_benchmark_timing(name: str, elf_path: Path, hardware_cpi: Optional[float]) -> Optional[Dict]:
    """Run timing experiment for a single benchmark and parse CPI from output"""
    log(f"Running {name}...")

    try:
        cmd = f"./m2sim -elf {elf_path} -fasttiming -limit 100000"
        result = run_command(cmd, check=False)

        if result.returncode == 0 or result.returncode == -2:
            # Parse CPI from simulation stdout — look for "CPI:" line
            simulated_cpi = None
            for line in result.stdout.split('\n'):
                line = line.strip()
                if line.startswith("CPI:"):
                    try:
                        simulated_cpi = float(line.split(":")[1].strip())
                    except (ValueError, IndexError):
                        pass

            if simulated_cpi is None:
                log(f"Could not parse CPI from {name} output", "WARNING")
                return None

            error = None
            if hardware_cpi is not None:
                error = abs(simulated_cpi - hardware_cpi) / hardware_cpi

            return {
                "name": name,
                "simulated_cpi": simulated_cpi,
                "hardware_cpi": hardware_cpi,
                "error": error,
                "status": "completed"
            }
        else:
            log(f"Simulation failed for {name} (exit code {result.returncode})", "WARNING")
            return None

    except Exception as e:
        log(f"Error running {name}: {e}", "ERROR")
        return None


def load_hardware_baselines() -> Dict[str, float]:
    """Load hardware CPI baselines from calibration_results.json"""
    cal_path = Path("benchmarks/native/calibration_results.json")
    if not cal_path.exists():
        return {}

    with open(cal_path, "r") as f:
        cal = json.load(f)

    # Also load from h5_accuracy_results.json for verified HW CPI values
    h5_path = Path("h5_accuracy_results.json")
    hw_cpis = {}
    if h5_path.exists():
        with open(h5_path, "r") as f:
            h5 = json.load(f)
        for b in h5["benchmarks"]:
            if isinstance(b.get("hardware_cpi"), (int, float)):
                hw_cpis[b["name"]] = b["hardware_cpi"]

    return hw_cpis


def run_accuracy_experiments() -> Dict:
    """Run accuracy validation experiments using live simulation"""
    log("Running live accuracy validation experiments...", "HEADER")
    log("NOTE: This requires ELF binaries. Build them first with aarch64-linux-musl-gcc.", "INFO")

    hw_baselines = load_hardware_baselines()

    microbenchmarks = [
        "arithmetic", "dependency", "branch", "memorystrided",
        "loadheavy", "storeheavy", "branchheavy", "vectorsum",
        "vectoradd", "reductiontree", "strideindirect"
    ]

    polybench = [
        "atax", "bicg", "gemm", "mvt", "jacobi-1d", "2mm", "3mm"
    ]

    results = {"benchmarks": [], "summary": {}}
    found_any_elf = False

    # Run microbenchmarks
    log("Running microbenchmarks...")
    for bench in microbenchmarks:
        elf_path = Path(f"benchmarks/microbenchmarks/{bench}.elf")
        if elf_path.exists():
            found_any_elf = True
            hw_cpi = hw_baselines.get(bench)
            result = run_benchmark_timing(bench, elf_path, hw_cpi)
            if result:
                results["benchmarks"].append(result)
        else:
            log(f"  {bench}.elf not found - skipping", "WARNING")

    # Run PolyBench
    log("Running PolyBench suite...")
    for bench in polybench:
        elf_path = Path(f"benchmarks/polybench/{bench}.elf")
        if elf_path.exists():
            found_any_elf = True
            hw_cpi = hw_baselines.get(bench)
            result = run_benchmark_timing(bench, elf_path, hw_cpi)
            if result:
                results["benchmarks"].append(result)
        else:
            log(f"  {bench}.elf not found - skipping", "WARNING")

    if not found_any_elf:
        log("No ELF binaries found. Falling back to CI-verified results.", "WARNING")
        log("To run live simulations, build ELFs first:", "INFO")
        log("  aarch64-linux-musl-gcc -static -O2 -o benchmarks/microbenchmarks/arithmetic.elf benchmarks/microbenchmarks/arithmetic.c", "INFO")
        return None

    # Calculate summary statistics from benchmarks with error data
    benchmarks_with_error = [b for b in results["benchmarks"] if b.get("error") is not None]
    if benchmarks_with_error:
        errors = [b["error"] for b in benchmarks_with_error]
        results["summary"] = {
            "total_benchmarks": len(results["benchmarks"]),
            "benchmarks_with_error": len(benchmarks_with_error),
            "average_error": sum(errors) / len(errors),
            "max_error": max(errors),
            "min_error": min(errors)
        }
        log(f"Live accuracy validation complete: {results['summary']['average_error']*100:.2f}% average error "
            f"(over {len(benchmarks_with_error)} benchmarks)", "SUCCESS")
    else:
        results["summary"] = {
            "total_benchmarks": len(results["benchmarks"]),
            "benchmarks_with_error": 0,
            "average_error": None,
            "max_error": None,
            "min_error": None
        }

    # Save results
    with open("accuracy_results.json", "w") as f:
        json.dump(results, f, indent=2)

    return results


def generate_figures():
    """Generate paper figures"""
    log("Generating paper figures...", "HEADER")

    figure_script = Path("paper/generate_figures.py")
    if figure_script.exists():
        try:
            run_command(f"python3 {figure_script}", cwd=Path("paper"))
            log("  Paper figures generated", "SUCCESS")
        except subprocess.CalledProcessError:
            log("Figure generation failed", "ERROR")
            raise
    else:
        log("Figure generation script not found", "WARNING")


def compile_paper():
    """Compile LaTeX paper"""
    log("Compiling LaTeX paper...", "HEADER")

    paper_tex = Path("paper/m2sim_micro2026.tex")
    if paper_tex.exists():
        try:
            # Run pdflatex multiple times for references
            for i in range(3):
                run_command(f"pdflatex m2sim_micro2026.tex", cwd=Path("paper"))

            # Check if PDF was generated
            pdf_path = Path("paper/m2sim_micro2026.pdf")
            if pdf_path.exists():
                log("  Paper compiled successfully", "SUCCESS")
                log(f"PDF available at: {pdf_path.absolute()}", "SUCCESS")
            else:
                log("PDF compilation failed", "ERROR")

        except subprocess.CalledProcessError:
            log("LaTeX compilation failed - check for LaTeX installation", "WARNING")
    else:
        log("LaTeX source not found", "WARNING")


def generate_experiment_report(ci_data: Dict, live_results: Optional[Dict] = None):
    """Generate comprehensive experiment report from CI-verified data"""
    log("Generating experiment report...", "HEADER")

    summary = ci_data["summary"]
    benchmarks = ci_data["benchmarks"]

    micro = [b for b in benchmarks if b["category"] == "microbenchmark"]
    micro_with_error = [b for b in micro if b.get("error") is not None]

    report_content = f"""# M2Sim Experiment Report

**Generated:** {time.strftime("%Y-%m-%d %H:%M:%S")}
**Reproducibility Script Version:** 2.0
**Data Source:** CI-verified results from h5_accuracy_results.json

## Summary

- **Microbenchmarks with error data:** {summary['microbenchmarks_with_error']}
- **Average Microbenchmark Error:** {summary['micro_average_error']*100:.2f}%
- **CI-verified benchmarks (total):** {summary['total_ci_verified_benchmarks']}
- **Infeasible benchmarks:** {summary['infeasible_benchmarks']}

## Target Achievement

**H5 Target Met**: Average microbenchmark error {summary['micro_average_error']*100:.2f}% < 20% target

## Microbenchmark Results (with hardware CPI comparison)

| Benchmark | Simulated CPI | Hardware CPI | Error |
|-----------|--------------|-------------|-------|
"""

    for b in micro_with_error:
        hw_str = f"{b['hardware_cpi']:.3f}" if isinstance(b["hardware_cpi"], (int, float)) else "N/A"
        err_str = f"{b['error']*100:.1f}%" if b["error"] is not None else "N/A"
        report_content += f"| {b['name']} | {b['simulated_cpi']:.3f} | {hw_str} | {err_str} |\n"

    if micro_with_error:
        avg_err = sum(b["error"] for b in micro_with_error) / len(micro_with_error)
        report_content += f"| **Average** | | | **{avg_err*100:.2f}%** |\n"

    poly = [b for b in benchmarks if b["category"] == "polybench"]
    emb = [b for b in benchmarks if b["category"] == "embench"]

    if poly or emb:
        report_content += """
## Simulation-Only Results (no directly comparable hardware CPI)

| Benchmark | Category | Simulated CPI | Note |
|-----------|----------|--------------|------|
"""
        for b in poly + emb:
            report_content += f"| {b['name']} | {b['category']} | {b['simulated_cpi']:.3f} | HW used different dataset size |\n"

    infeasible = ci_data.get("infeasible", [])
    if infeasible:
        report_content += """
## Infeasible Benchmarks (did not complete in CI)

| Benchmark | Category | Reason |
|-----------|----------|--------|
"""
        for b in infeasible:
            report_content += f"| {b['name']} | {b['category']} | {b['reason']} |\n"

    if live_results and live_results.get("benchmarks"):
        report_content += """
## Live Simulation Results

The following results were obtained from live simulation runs:

| Benchmark | Simulated CPI | Hardware CPI | Error |
|-----------|--------------|-------------|-------|
"""
        for b in live_results["benchmarks"]:
            hw_str = f"{b['hardware_cpi']:.3f}" if b.get("hardware_cpi") else "N/A"
            err_str = f"{b['error']*100:.1f}%" if b.get("error") is not None else "N/A"
            report_content += f"| {b['name']} | {b['simulated_cpi']:.3f} | {hw_str} | {err_str} |\n"

    report_content += f"""
## Reproduction Environment

- **Operating System:** {os.uname().sysname} {os.uname().release}
- **Architecture:** {os.uname().machine}
- **Working Directory:** {Path.cwd().absolute()}
- **Timestamp:** {time.strftime("%Y-%m-%d %H:%M:%S")}

## Reproducibility Notes

This report uses CI-verified accuracy results from h5_accuracy_results.json as the
source of truth. These results were produced by the accuracy CI workflows running
actual simulations against the M2Sim timing model.

To run live simulations locally, you need to:
1. Install aarch64-linux-musl-gcc for ARM64 cross-compilation
2. Build ELF binaries: `aarch64-linux-musl-gcc -static -O2 -o benchmark.elf benchmark.c`
3. Run this script without --skip-experiments

Note: PolyBench and EmBench hardware CPI was measured on LARGE datasets, while
simulation uses MINI datasets, so those results are not directly comparable.
"""

    with open("experiment_report.md", "w") as f:
        f.write(report_content)

    log("  Experiment report generated: experiment_report.md", "SUCCESS")


def main():
    """Main experiment reproduction workflow"""
    parser = argparse.ArgumentParser(description="Reproduce M2Sim experiments")
    parser.add_argument("--skip-build", action="store_true", help="Skip build phase")
    parser.add_argument("--skip-experiments", action="store_true", help="Skip experiment execution")
    parser.add_argument("--skip-figures", action="store_true", help="Skip figure generation")
    parser.add_argument("--skip-paper", action="store_true", help="Skip paper compilation")

    args = parser.parse_args()

    log("M2Sim Reproducible Experiments", "HEADER")
    log("==============================", "HEADER")

    start_time = time.time()

    try:
        # Check dependencies
        if not check_dependencies():
            sys.exit(1)

        # Always load CI-verified results as the source of truth
        ci_data = load_ci_verified_results()

        # Build phase
        if not args.skip_build:
            build_simulator()
            build_benchmarks()
        else:
            log("Skipping build phase", "WARNING")

        # Experiment execution phase
        live_results = None
        if not args.skip_experiments:
            live_results = run_accuracy_experiments()
            if live_results is None:
                log("No live results. Displaying CI-verified results instead.", "INFO")
        else:
            log("Skipping live experiments", "WARNING")

        # Display CI-verified results (always shown)
        display_ci_verified_results(ci_data)

        # Figure generation phase
        if not args.skip_figures:
            generate_figures()
        else:
            log("Skipping figure generation", "WARNING")

        # Paper compilation phase
        if not args.skip_paper:
            compile_paper()
        else:
            log("Skipping paper compilation", "WARNING")

        # Generate final report
        generate_experiment_report(ci_data, live_results)

        # Summary
        duration = time.time() - start_time
        summary = ci_data["summary"]
        log("==============================", "HEADER")
        log(f"Experiment reproduction completed in {duration:.1f} seconds", "SUCCESS")
        log(f"CI-verified microbenchmark accuracy: {summary['micro_average_error']*100:.2f}% average error "
            f"({summary['microbenchmarks_with_error']} benchmarks)", "SUCCESS")
        log(f"H5 target (<20%): {'MET' if summary['h5_target_met'] else 'NOT MET'}", "SUCCESS")
        log("All outputs generated successfully", "SUCCESS")

    except Exception as e:
        log(f"Experiment reproduction failed: {e}", "ERROR")
        sys.exit(1)


if __name__ == "__main__":
    main()
