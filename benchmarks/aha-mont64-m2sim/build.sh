#!/bin/bash
# Build script for aha-mont64 M2Sim bare-metal benchmark

set -e

# Cross-compiler
CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

# Paths
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBENCH_DIR="$SCRIPT_DIR/../embench-iot"
SRC_DIR="$EMBENCH_DIR/src/aha-mont64"

# Compiler flags - local include FIRST for our minimal headers
CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2"
CFLAGS+=" -I$SCRIPT_DIR"  # Our support.h and minimal libc headers FIRST
CFLAGS+=" -I$SRC_DIR"     # Embench source

# Reduce LOCAL_SCALE_FACTOR for timing simulation feasibility
sed 's/^#define LOCAL_SCALE_FACTOR.*/#define LOCAL_SCALE_FACTOR 1/' \
    $SRC_DIR/mont64.c > $SCRIPT_DIR/mont64.c

# Build
echo "Building aha-mont64 for M2Sim..."

$CC $CFLAGS -c $SCRIPT_DIR/mont64.c -o $SCRIPT_DIR/mont64.o
$CC $CFLAGS -c $SCRIPT_DIR/startup.S -o $SCRIPT_DIR/startup.o

$CC $CFLAGS -T $SCRIPT_DIR/linker.ld \
    $SCRIPT_DIR/startup.o \
    $SCRIPT_DIR/mont64.o \
    -o $SCRIPT_DIR/aha-mont64_m2sim.elf \
    -lgcc

# Generate disassembly
$OBJDUMP -d $SCRIPT_DIR/aha-mont64_m2sim.elf > $SCRIPT_DIR/aha-mont64_m2sim.dis

echo "Build complete: aha-mont64_m2sim.elf"
ls -la $SCRIPT_DIR/aha-mont64_m2sim.elf
