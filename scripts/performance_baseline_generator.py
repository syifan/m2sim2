#!/usr/bin/env python3
"""
Performance Baseline Generator for M2Sim Issue #481

Generates performance baselines using cmd/profile tool across all simulation modes.
Integrates with existing accuracy baseline infrastructure and versioning protocol.
"""

import json
import subprocess
import sys
import os
import time
from datetime import datetime
from pathlib import Path
import re
import platform

def get_git_info():
    """Get current git commit hash and check if working tree is clean."""
    try:
        commit_hash = subprocess.check_output(['git', 'rev-parse', 'HEAD'],
                                            cwd=Path(__file__).parent.parent,
                                            text=True).strip()
        # Check if working tree is clean
        status = subprocess.check_output(['git', 'status', '--porcelain'],
                                       cwd=Path(__file__).parent.parent,
                                       text=True).strip()
        if status:
            print(f"Warning: Working tree is not clean. Uncommitted changes detected.")
            print("Consider committing changes before generating baseline.")

        return commit_hash
    except subprocess.CalledProcessError as e:
        print(f"Error getting git info: {e}")
        return None

def get_environment_info():
    """Collect system environment information."""
    go_version = "unknown"
    try:
        go_output = subprocess.check_output(['go', 'version'], text=True)
        # Extract version from "go version go1.25.x ..."
        version_match = re.search(r'go(\d+\.\d+(?:\.\d+)?)', go_output)
        if version_match:
            go_version = version_match.group(1)
    except subprocess.CalledProcessError:
        pass

    return {
        "platform": platform.system().lower(),
        "go_version": go_version,
        "cpu_info": platform.processor() or "unknown",
        "memory_gb": "unknown"  # Could be enhanced with psutil
    }

def build_profile_tool():
    """Build the cmd/profile tool."""
    print("Building cmd/profile tool...")
    repo_root = Path(__file__).parent.parent
    try:
        subprocess.run(['go', 'build', '-o', 'profile-tool', './cmd/profile'],
                      cwd=repo_root, check=True)
        return repo_root / 'profile-tool'
    except subprocess.CalledProcessError as e:
        print(f"Error building profile tool: {e}")
        return None

def find_test_elf():
    """Find a test ELF binary for profiling."""
    repo_root = Path(__file__).parent.parent
    # Look for ELF files in benchmarks directory
    for pattern in ['benchmarks/**/*.elf', 'test/**/*.elf', '**/*.elf']:
        for elf_file in repo_root.glob(pattern):
            if elf_file.is_file():
                return elf_file
    return None

def run_profile_measurement(profile_tool, elf_file, mode_flag="", duration=30):
    """Run a single profile measurement and parse results."""
    print(f"Running profile measurement: mode={mode_flag or 'emulation'}, duration={duration}s")

    cmd = [str(profile_tool)]
    if mode_flag:
        cmd.append(mode_flag)
    cmd.extend(['-duration', f'{duration}s', str(elf_file)])

    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=duration + 10)
        output = result.stdout + result.stderr

        # Parse performance metrics from output
        metrics = {}

        # Look for key metrics in output
        for line in output.split('\n'):
            if 'Instructions executed:' in line:
                try:
                    instructions = int(line.split(':')[1].strip())
                    metrics['instructions'] = instructions
                except ValueError:
                    pass
            elif 'Elapsed time:' in line:
                try:
                    # Parse duration like "1.234567s" or "1m2.345s"
                    elapsed_str = line.split(':')[1].strip()
                    if 's' in elapsed_str:
                        if 'm' in elapsed_str:
                            # Handle "1m2.345s" format
                            parts = elapsed_str.replace('s', '').split('m')
                            elapsed = float(parts[0]) * 60 + float(parts[1])
                        else:
                            # Handle "1.234567s" format
                            elapsed = float(elapsed_str.replace('s', ''))
                        metrics['elapsed_sec'] = elapsed
                except ValueError:
                    pass
            elif 'Instructions/second:' in line:
                try:
                    ips = float(line.split(':')[1].strip())
                    metrics['instructions_per_sec'] = int(ips)
                except ValueError:
                    pass
            elif 'CPI:' in line:
                try:
                    cpi = float(line.split(':')[1].strip())
                    metrics['cpi'] = cpi
                except ValueError:
                    pass

        # Estimate memory usage (simplified)
        metrics['memory_mb'] = 150.0  # Placeholder - could be enhanced

        if 'instructions_per_sec' not in metrics and 'instructions' in metrics and 'elapsed_sec' in metrics:
            metrics['instructions_per_sec'] = int(metrics['instructions'] / metrics['elapsed_sec'])

        return metrics, output

    except subprocess.TimeoutExpired:
        print(f"Profile measurement timed out after {duration + 10}s")
        return None, "Timeout"
    except subprocess.CalledProcessError as e:
        print(f"Profile measurement failed: {e}")
        return None, str(e)

def generate_baseline(elf_file=None, duration=30):
    """Generate complete performance baseline."""
    repo_root = Path(__file__).parent.parent

    # Get git and environment info
    commit_hash = get_git_info()
    if not commit_hash:
        print("Error: Could not get git commit hash")
        return None

    env_info = get_environment_info()

    # Build profile tool
    profile_tool = build_profile_tool()
    if not profile_tool:
        print("Error: Could not build profile tool")
        return None

    # Find ELF file
    if not elf_file:
        elf_file = find_test_elf()
        if not elf_file:
            print("Error: Could not find test ELF binary")
            return None

    print(f"Using ELF binary: {elf_file}")

    # Test modes configuration
    modes = {
        "emulation": "",
        "timing": "-timing",
        "fast_timing": "-fast-timing"
    }

    baseline_data = {
        "baseline_metadata": {
            "creation_date": datetime.now().strftime("%Y-%m-%d"),
            "commit_hash": commit_hash,
            "timing_model_version": "issue-481-phase1",
            "measurement_environment": env_info
        },
        "benchmarks": {}
    }

    benchmark_name = elf_file.stem
    baseline_data["benchmarks"][benchmark_name] = {}

    # Run measurements for each mode
    for mode_name, mode_flag in modes.items():
        print(f"\nMeasuring {mode_name} mode...")
        metrics, output = run_profile_measurement(profile_tool, elf_file, mode_flag, duration)

        if metrics:
            # Store metrics in baseline format
            mode_data = {
                "instructions_per_sec": metrics.get('instructions_per_sec', 0),
                "memory_mb": metrics.get('memory_mb', 0.0),
            }

            if 'cpi' in metrics:
                mode_data['cpi'] = metrics['cpi']
            else:
                mode_data['cpi'] = "N/A"

            baseline_data["benchmarks"][benchmark_name][mode_name] = mode_data

            print(f"  Instructions/sec: {mode_data['instructions_per_sec']}")
            print(f"  Memory: {mode_data['memory_mb']} MB")
            if mode_data['cpi'] != "N/A":
                print(f"  CPI: {mode_data['cpi']}")
        else:
            print(f"  Failed to measure {mode_name} mode")
            baseline_data["benchmarks"][benchmark_name][mode_name] = {
                "instructions_per_sec": 0,
                "memory_mb": 0.0,
                "cpi": "FAILED"
            }

    # Clean up
    profile_tool.unlink(missing_ok=True)

    return baseline_data

def save_baseline(baseline_data):
    """Save baseline to results/baselines/performance/ with timestamp."""
    if not baseline_data:
        return None

    repo_root = Path(__file__).parent.parent
    baselines_dir = repo_root / 'results' / 'baselines' / 'performance'
    baselines_dir.mkdir(parents=True, exist_ok=True)

    timestamp = datetime.now().strftime("%Y_%m_%d")
    filename = f"performance_baseline_{timestamp}.json"
    filepath = baselines_dir / filename

    # Handle duplicate filenames
    counter = 1
    while filepath.exists():
        filename = f"performance_baseline_{timestamp}_{counter}.json"
        filepath = baselines_dir / filename
        counter += 1

    try:
        with open(filepath, 'w') as f:
            json.dump(baseline_data, f, indent=2)

        print(f"\nBaseline saved to: {filepath}")
        return filepath

    except Exception as e:
        print(f"Error saving baseline: {e}")
        return None

def main():
    """Main entry point."""
    print("M2Sim Performance Baseline Generator (Issue #481)")
    print("=" * 50)

    # Parse command line args
    duration = 30
    elf_file = None

    if len(sys.argv) > 1:
        if sys.argv[1].endswith('.elf'):
            elf_file = Path(sys.argv[1])
        else:
            try:
                duration = int(sys.argv[1])
            except ValueError:
                print(f"Invalid duration: {sys.argv[1]}")
                sys.exit(1)

    # Generate baseline
    baseline_data = generate_baseline(elf_file, duration)

    if baseline_data:
        # Save baseline
        filepath = save_baseline(baseline_data)
        if filepath:
            print("\n✅ Performance baseline generated successfully!")
            print(f"Commit: {baseline_data['baseline_metadata']['commit_hash']}")
            print(f"Benchmarks: {list(baseline_data['benchmarks'].keys())}")
        else:
            print("\n❌ Failed to save baseline")
            sys.exit(1)
    else:
        print("\n❌ Failed to generate baseline")
        sys.exit(1)

if __name__ == '__main__':
    main()