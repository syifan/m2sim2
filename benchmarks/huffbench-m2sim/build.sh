#!/bin/bash
# Build script for huffbench M2Sim bare-metal benchmark
# huffbench requires the beebs heap library for malloc_beebs

set -e

CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBENCH_DIR="$SCRIPT_DIR/../embench-iot"
SRC_DIR="$EMBENCH_DIR/src/huffbench"

CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2 -mgeneral-regs-only"
CFLAGS+=" -I$SCRIPT_DIR"
CFLAGS+=" -I$SRC_DIR"

echo "Building huffbench for M2Sim..."

# Compile the beebs support library (provides heap functions)
$CC $CFLAGS -c $SCRIPT_DIR/beebsc.c -o $SCRIPT_DIR/beebsc.o

# Compile the huffbench source
$CC $CFLAGS -c $SRC_DIR/libhuffbench.c -o $SCRIPT_DIR/huffbench.o

# Compile startup
$CC $CFLAGS -c $SCRIPT_DIR/startup.S -o $SCRIPT_DIR/startup.o

# Link
$CC $CFLAGS -T $SCRIPT_DIR/linker.ld \
    $SCRIPT_DIR/startup.o \
    $SCRIPT_DIR/huffbench.o \
    $SCRIPT_DIR/beebsc.o \
    -o $SCRIPT_DIR/huffbench_m2sim.elf \
    -lgcc

$OBJDUMP -d $SCRIPT_DIR/huffbench_m2sim.elf > $SCRIPT_DIR/huffbench_m2sim.dis

echo "Build complete: huffbench_m2sim.elf"
ls -la $SCRIPT_DIR/huffbench_m2sim.elf
