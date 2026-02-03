#!/bin/bash
# Compare native M2 benchmark results with M2Sim simulator
#
# This script helps identify calibration gaps by comparing:
# 1. Native M2 hardware behavior
# 2. M2Sim simulator predictions
#
# Usage: ./compare_with_simulator.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$SCRIPT_DIR/../.."

cd "$SCRIPT_DIR"

echo "M2Sim vs Native Hardware Comparison"
echo "===================================="
echo ""
echo "This comparison shows relative behavior between benchmarks."
echo "Due to process overhead, absolute cycle counts aren't meaningful."
echo "Focus on: Do the benchmarks show similar relative performance?"
echo ""

# Step 1: Run native benchmarks
echo "Step 1: Running native benchmarks..."
echo "---"

make all >/dev/null 2>&1

echo ""
printf "%-25s %10s %10s\n" "Benchmark" "Exit Code" "Status"
printf "%s\n" "================================================"

for bench in arithmetic_sequential dependency_chain memory_sequential function_calls branch_taken mixed_operations; do
    ./"$bench" >/dev/null 2>&1
    exit_code=$?
    
    case "$bench" in
        arithmetic_sequential) expected=4 ;;
        dependency_chain) expected=20 ;;
        memory_sequential) expected=42 ;;
        function_calls) expected=5 ;;
        branch_taken) expected=5 ;;
        mixed_operations) expected=100 ;;
    esac
    
    if [ "$exit_code" = "$expected" ]; then
        status="✓ OK"
    else
        status="✗ MISMATCH (expected $expected)"
    fi
    
    printf "%-25s %10d %s\n" "$bench" "$exit_code" "$status"
done

echo ""

# Step 2: Run simulator timing tests
echo "Step 2: Running simulator timing validation..."
echo "---"

cd "$REPO_ROOT"

# Run the timing validation test and capture CPI info
echo ""
echo "Simulator Benchmark CPIs:"
printf "%-25s %10s %10s\n" "Benchmark" "Cycles" "CPI"
printf "%s\n" "================================================"

# Run the test and parse output
go test -v -run TestMicrobenchmarkCPI ./benchmarks/ 2>&1 | while IFS= read -r line; do
    # Look for lines like: benchmark_name: cycles=X instructions=Y CPI=Z
    if echo "$line" | grep -q "cycles="; then
        name=$(echo "$line" | sed -n 's/.*\(arithmetic_sequential\|dependency_chain\|memory_sequential\|function_calls\|branch_taken\|mixed_operations\).*/\1/p')
        cycles=$(echo "$line" | sed -n 's/.*cycles=\([0-9]*\).*/\1/p')
        cpi=$(echo "$line" | sed -n 's/.*CPI=\([0-9.]*\).*/\1/p')
        
        if [ -n "$name" ] && [ -n "$cycles" ]; then
            printf "%-25s %10s %10s\n" "$name" "${cycles:-N/A}" "${cpi:-N/A}"
        fi
    fi
done

echo ""
echo "Step 3: Analysis"
echo "---"
echo ""
echo "Compare the exit codes and simulator CPIs above to identify:"
echo ""
echo "1. Exit code mismatches - indicates functional correctness issues"
echo "2. CPI values - compare relative relationships:"
echo "   - dependency_chain should have higher CPI than arithmetic_sequential"
echo "   - memory_sequential should reflect cache latency"
echo "   - function_calls measures BL/RET overhead"
echo ""
echo "For absolute cycle accuracy, run benchmarks with Apple Instruments"
echo "using CPU Counters template to get real hardware cycle counts."
echo ""
echo "See benchmarks/native/README.md for detailed instructions."
