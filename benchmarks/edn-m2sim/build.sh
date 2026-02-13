#!/bin/bash
# Build script for edn M2Sim bare-metal benchmark

set -e

# Cross-compiler
CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

# Paths
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBENCH_DIR="$SCRIPT_DIR/../embench-iot"
SRC_DIR="$EMBENCH_DIR/src/edn"

# Compiler flags - local include FIRST for our minimal headers
CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2 -mgeneral-regs-only"
CFLAGS+=" -I$SCRIPT_DIR"  # Our support.h and minimal libc headers FIRST
CFLAGS+=" -I$SRC_DIR"     # Embench source

# Reduce LOCAL_SCALE_FACTOR for timing simulation feasibility
sed 's/^#define LOCAL_SCALE_FACTOR.*/#define LOCAL_SCALE_FACTOR 1/' \
    $SRC_DIR/libedn.c > $SCRIPT_DIR/libedn.c

# Build
echo "Building edn for M2Sim..."

$CC $CFLAGS -c $SCRIPT_DIR/libedn.c -o $SCRIPT_DIR/edn.o
$CC $CFLAGS -c $SCRIPT_DIR/startup.S -o $SCRIPT_DIR/startup.o
$CC $CFLAGS -c $SCRIPT_DIR/memlib.c -o $SCRIPT_DIR/memlib.o

$CC $CFLAGS -T $SCRIPT_DIR/linker.ld \
    $SCRIPT_DIR/startup.o \
    $SCRIPT_DIR/edn.o \
    $SCRIPT_DIR/memlib.o \
    -o $SCRIPT_DIR/edn_m2sim.elf \
    -lgcc

# Generate disassembly
$OBJDUMP -d $SCRIPT_DIR/edn_m2sim.elf > $SCRIPT_DIR/edn_m2sim.dis

echo "Build complete: edn_m2sim.elf"
ls -la $SCRIPT_DIR/edn_m2sim.elf
