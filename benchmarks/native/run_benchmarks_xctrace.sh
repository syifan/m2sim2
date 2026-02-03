#!/bin/bash
# Native M2 Benchmark Runner using xctrace (Apple Instruments)
# Attempts to collect actual CPU cycle counts from performance counters
#
# Requirements:
# - Xcode Command Line Tools
# - Terminal must have Full Disk Access (System Settings > Privacy)
#
# Usage: ./run_benchmarks_xctrace.sh [--benchmark NAME]

set -e

SINGLE_BENCHMARK=""
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TRACE_DIR="$SCRIPT_DIR/.traces"

# Parse arguments
while [ $# -gt 0 ]; do
    case $1 in
        --benchmark|-b)
            SINGLE_BENCHMARK="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Attempts to collect CPU cycle counts using Apple Instruments (xctrace)."
            echo ""
            echo "Options:"
            echo "  --benchmark NAME    Run only the specified benchmark"
            echo "  --help              Show this help"
            echo ""
            echo "Note: This requires Xcode Command Line Tools and may need"
            echo "      Terminal to have Full Disk Access in System Settings."
            echo ""
            echo "If xctrace fails, falls back to timing-based estimation."
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check for xctrace
if ! command -v xctrace >/dev/null 2>&1; then
    echo "ERROR: xctrace not found. Install Xcode Command Line Tools:" >&2
    echo "  xcode-select --install" >&2
    exit 1
fi

# Benchmark metadata functions
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

# Build if needed
if [ ! -f "arithmetic_sequential" ]; then
    echo "Building benchmarks..." >&2
    make all >/dev/null 2>&1
fi

# Create trace directory
mkdir -p "$TRACE_DIR"

echo "Native M2 Benchmark Results (xctrace)"
echo "======================================"
echo ""
echo "Attempting to use xctrace for cycle counts..."
echo "Note: If xctrace fails, results will use timing estimation."
echo ""
printf "%-25s %10s %8s %10s %s\n" "Benchmark" "Cycles" "Instr" "Exit" "Source"
printf "%s\n" "========================================================================"

# Benchmarks to run
if [ -n "$SINGLE_BENCHMARK" ]; then
    benchmarks="$SINGLE_BENCHMARK"
else
    benchmarks="arithmetic_sequential dependency_chain memory_sequential function_calls branch_taken mixed_operations"
fi

for name in $benchmarks; do
    if [ ! -f "$name" ]; then
        echo "ERROR: Benchmark '$name' not found" >&2
        continue
    fi
    
    trace_path="$TRACE_DIR/${name}.trace"
    cycles=0
    source="failed"
    
    # Try xctrace
    rm -rf "$trace_path" 2>/dev/null || true
    
    if xctrace record \
        --template 'CPU Counters' \
        --output "$trace_path" \
        --launch -- "./$name" 2>/dev/null; then
        
        # Try to extract cycle count from the trace
        export_path="$TRACE_DIR/${name}_export"
        rm -rf "$export_path" 2>/dev/null || true
        
        if xctrace export --input "$trace_path" --output "$export_path" 2>/dev/null; then
            # Search for cycle count in exported data
            found_cycles=$(grep -rh "CYCLES\|cycle" "$export_path" 2>/dev/null | \
                          grep -oE '[0-9]+' | sort -rn | head -1 || echo "0")
            
            if [ -n "$found_cycles" ] && [ "$found_cycles" != "0" ]; then
                cycles=$found_cycles
                source="xctrace"
            fi
        fi
    fi
    
    # Fallback to timing if xctrace didn't work
    if [ "$cycles" = "0" ]; then
        # Run 50 times and take minimum time
        min_ns=999999999999
        for i in $(seq 1 50); do
            start=$(python3 -c "import time; print(int(time.time_ns()))")
            ./"$name" >/dev/null 2>&1
            end=$(python3 -c "import time; print(int(time.time_ns()))")
            elapsed=$((end - start))
            if [ $elapsed -lt $min_ns ]; then
                min_ns=$elapsed
            fi
        done
        # Estimate cycles at 3.5 GHz
        cycles=$(python3 -c "print(int($min_ns * 3.5))")
        source="timing-est"
    fi
    
    # Get exit code
    ./"$name" >/dev/null 2>&1
    exit_code=$?
    
    instr=$(get_instr_count "$name")
    
    printf "%-25s %10d %8d %10d %s\n" "$name" "$cycles" "$instr" "$exit_code" "$source"
done

# Cleanup
rm -rf "$TRACE_DIR"

echo ""
echo "Legend:"
echo "  xctrace     - Cycles from hardware performance counters (accurate)"
echo "  timing-est  - Estimated from wall-clock time (includes process overhead)"
echo ""
echo "For accurate cycle counts, ensure:"
echo "  1. Terminal has Full Disk Access in System Settings"
echo "  2. Xcode Command Line Tools are installed"
