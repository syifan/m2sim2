#!/bin/bash
# Build script for PolyBench M2Sim bare-metal benchmarks
#
# Usage: ./build.sh [benchmark]
#   benchmark: gemm (default: all)

set -e

# Cross-compiler
CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

# Script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Compiler flags
# -fno-tree-vectorize: Disable auto-vectorization (M2Sim doesn't support NEON yet)
CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2"
CFLAGS+=" -fno-tree-vectorize -fno-tree-loop-vectorize"
CFLAGS+=" -I$SCRIPT_DIR/common"
CFLAGS+=" -DPOLYBENCH_USE_RESTRICT"
CFLAGS+=" -DMINI_DATASET"

# Available benchmarks
BENCHMARKS="gemm atax 2mm mvt jacobi-1d"

# Build function
build_benchmark() {
    local name=$1
    local src_dir="$SCRIPT_DIR/$name"
    
    if [ ! -d "$src_dir" ]; then
        echo "Error: Benchmark directory $src_dir not found"
        return 1
    fi
    
    echo "Building $name for M2Sim..."
    
    # Compile benchmark source
    $CC $CFLAGS -c "$src_dir/$name.c" -o "$SCRIPT_DIR/$name.o"
    
    # Compile startup code
    $CC $CFLAGS -c "$SCRIPT_DIR/common/startup.S" -o "$SCRIPT_DIR/startup.o"
    
    # Link
    $CC $CFLAGS -T "$SCRIPT_DIR/linker.ld" \
        "$SCRIPT_DIR/startup.o" \
        "$SCRIPT_DIR/$name.o" \
        -o "$SCRIPT_DIR/${name}_m2sim.elf" \
        -lgcc
    
    # Generate disassembly
    $OBJDUMP -d "$SCRIPT_DIR/${name}_m2sim.elf" > "$SCRIPT_DIR/${name}_m2sim.dis"
    
    echo "Build complete: ${name}_m2sim.elf"
    ls -la "$SCRIPT_DIR/${name}_m2sim.elf"
}

# Clean function
clean() {
    echo "Cleaning build artifacts..."
    rm -f "$SCRIPT_DIR"/*.o
    rm -f "$SCRIPT_DIR"/*_m2sim.elf
    rm -f "$SCRIPT_DIR"/*_m2sim.dis
}

# Main
case "${1:-all}" in
    clean)
        clean
        ;;
    all)
        for bench in $BENCHMARKS; do
            build_benchmark "$bench"
        done
        ;;
    *)
        build_benchmark "$1"
        ;;
esac

echo "Done."
