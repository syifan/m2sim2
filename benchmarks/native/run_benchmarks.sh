#!/bin/bash
# Native M2 Benchmark Runner
# Measures CPU cycles and performance metrics for comparison with M2Sim
#
# Usage: ./run_benchmarks.sh [--json] [--iterations N] [--benchmark NAME]

set -e

# Default settings
ITERATIONS=100
OUTPUT_JSON=false
SINGLE_BENCHMARK=""
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# M2 P-core frequency (Hz) for cycle estimation from timing
M2_FREQ=3500000000  # 3.5 GHz

# Parse arguments
while [ $# -gt 0 ]; do
    case $1 in
        --json)
            OUTPUT_JSON=true
            shift
            ;;
        --iterations|-n)
            ITERATIONS="$2"
            shift 2
            ;;
        --benchmark|-b)
            SINGLE_BENCHMARK="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --json              Output results as JSON"
            echo "  --iterations N      Number of iterations per benchmark (default: 100)"
            echo "  --benchmark NAME    Run only the specified benchmark"
            echo "  --help              Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Benchmark metadata functions (avoids bash 4 associative arrays)
get_description() {
    case "$1" in
        arithmetic_sequential) echo "20 independent ADDs (ALU throughput)" ;;
        dependency_chain) echo "20 dependent ADDs (forwarding latency)" ;;
        memory_sequential) echo "10 store/load pairs (cache performance)" ;;
        function_calls) echo "5 BL/RET pairs (call overhead)" ;;
        branch_taken) echo "5 unconditional branches" ;;
        mixed_operations) echo "Mix of ALU, memory, calls" ;;
        *) echo "Unknown benchmark" ;;
    esac
}

get_expected_exit() {
    case "$1" in
        arithmetic_sequential) echo 4 ;;
        dependency_chain) echo 20 ;;
        memory_sequential) echo 42 ;;
        function_calls) echo 5 ;;
        branch_taken) echo 5 ;;
        mixed_operations) echo 100 ;;
        *) echo 0 ;;
    esac
}

get_instr_count() {
    case "$1" in
        arithmetic_sequential) echo 24 ;;
        dependency_chain) echo 24 ;;
        memory_sequential) echo 25 ;;
        function_calls) echo 18 ;;
        branch_taken) echo 15 ;;
        mixed_operations) echo 45 ;;
        *) echo 1 ;;
    esac
}

cd "$SCRIPT_DIR"

# Build benchmarks if needed
if [ ! -f "arithmetic_sequential" ]; then
    echo "Building benchmarks..." >&2
    make all >/dev/null 2>&1
fi

# Function to get high-precision time in nanoseconds
get_time_ns() {
    python3 -c "import time; print(int(time.time_ns()))"
}

# Function to run a single benchmark and collect stats
run_benchmark() {
    local name=$1
    local iterations=$2
    
    if [ ! -f "$name" ]; then
        echo "ERROR: Benchmark binary '$name' not found" >&2
        return 1
    fi
    
    local total_ns=0
    local min_ns=999999999999
    local max_ns=0
    local exit_code=0
    local times_file="/tmp/bench_times_$$"
    
    # Warmup run
    ./"$name" >/dev/null 2>&1 || true
    
    # Timed runs
    i=1
    while [ $i -le $iterations ]; do
        start=$(get_time_ns)
        ./"$name" >/dev/null 2>&1
        exit_code=$?
        end=$(get_time_ns)
        elapsed=$((end - start))
        
        echo "$elapsed" >> "$times_file"
        total_ns=$((total_ns + elapsed))
        
        if [ $elapsed -lt $min_ns ]; then
            min_ns=$elapsed
        fi
        if [ $elapsed -gt $max_ns ]; then
            max_ns=$elapsed
        fi
        
        i=$((i + 1))
    done
    
    # Calculate statistics
    local avg_ns=$((total_ns / iterations))
    
    # Calculate stddev using python for simplicity
    local stddev=$(python3 -c "
import math
times = [int(x) for x in open('$times_file').read().split()]
avg = sum(times) / len(times)
variance = sum((t - avg) ** 2 for t in times) / len(times)
print(int(math.sqrt(variance)))
")
    
    rm -f "$times_file"
    
    # Estimate cycles from time (assuming M2 P-core frequency)
    local est_cycles=$(python3 -c "print(int($min_ns * $M2_FREQ / 1000000000))")
    
    # Calculate CPI
    local instr=$(get_instr_count "$name")
    local cpi=$(python3 -c "print(round($est_cycles / $instr, 3))")
    
    # Return results
    echo "$name|$avg_ns|$min_ns|$max_ns|$stddev|$est_cycles|$instr|$cpi|$exit_code"
}

# Main execution
results=""

if [ -n "$SINGLE_BENCHMARK" ]; then
    benchmarks="$SINGLE_BENCHMARK"
else
    benchmarks="arithmetic_sequential dependency_chain memory_sequential function_calls branch_taken mixed_operations"
fi

if [ "$OUTPUT_JSON" = "false" ]; then
    echo "Native M2 Benchmark Results"
    echo "=========================="
    echo "Iterations: $ITERATIONS"
    echo "M2 P-core frequency: 3.5 GHz"
    echo ""
    printf "%-25s %12s %12s %12s %12s %10s %6s %8s\n" \
        "Benchmark" "Avg(ns)" "Min(ns)" "Max(ns)" "StdDev" "Est.Cycles" "Instr" "CPI"
    printf "%s\n" "==========================================================================================================="
fi

for bench in $benchmarks; do
    result=$(run_benchmark "$bench" "$ITERATIONS")
    results="$results$result
"
    
    if [ "$OUTPUT_JSON" = "false" ]; then
        name=$(echo "$result" | cut -d'|' -f1)
        avg_ns=$(echo "$result" | cut -d'|' -f2)
        min_ns=$(echo "$result" | cut -d'|' -f3)
        max_ns=$(echo "$result" | cut -d'|' -f4)
        stddev=$(echo "$result" | cut -d'|' -f5)
        cycles=$(echo "$result" | cut -d'|' -f6)
        instr=$(echo "$result" | cut -d'|' -f7)
        cpi=$(echo "$result" | cut -d'|' -f8)
        exit_code=$(echo "$result" | cut -d'|' -f9)
        
        expected=$(get_expected_exit "$name")
        status=""
        if [ "$exit_code" != "$expected" ]; then
            status=" [EXIT MISMATCH: got $exit_code, expected $expected]"
        fi
        printf "%-25s %12d %12d %12d %12d %10d %6d %8s%s\n" \
            "$name" "$avg_ns" "$min_ns" "$max_ns" "$stddev" "$cycles" "$instr" "$cpi" "$status"
    fi
done

if [ "$OUTPUT_JSON" = "true" ]; then
    echo "{"
    echo '  "platform": "Apple M2 (native)",'
    echo '  "frequency_ghz": 3.5,'
    echo "  \"iterations\": $ITERATIONS,"
    echo "  \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
    echo '  "benchmarks": ['
    
    first=true
    echo "$results" | while IFS='|' read name avg_ns min_ns max_ns stddev cycles instr cpi exit_code; do
        [ -z "$name" ] && continue
        
        if [ "$first" = "true" ]; then
            first=false
        else
            echo ","
        fi
        
        desc=$(get_description "$name")
        expected=$(get_expected_exit "$name")
        
        cat << EOF
    {
      "name": "$name",
      "description": "$desc",
      "estimated_cycles": $cycles,
      "instructions": $instr,
      "cpi": $cpi,
      "timing_ns": {
        "avg": $avg_ns,
        "min": $min_ns,
        "max": $max_ns,
        "stddev": $stddev
      },
      "exit_code": $exit_code,
      "expected_exit": $expected
    }
EOF
    done
    
    echo ""
    echo "  ]"
    echo "}"
fi

if [ "$OUTPUT_JSON" = "false" ]; then
    echo ""
    echo "Notes:"
    echo "  - Times include ~18ms process startup overhead per run"
    echo "  - Cycle estimates are dominated by this overhead, not benchmark code"
    echo "  - Use relative comparisons between benchmarks, not absolute CPI values"
    echo "  - For accurate cycle counts, use Apple Instruments (xctrace) with CPU Counters"
    echo "  - See README.md for detailed instructions"
fi
