#!/bin/bash
# Build script for matmult-int M2Sim bare-metal benchmark

set -e

CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBENCH_DIR="$SCRIPT_DIR/../embench-iot"
SRC_DIR="$EMBENCH_DIR/src/matmult-int"

CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2"
CFLAGS+=" -I$SCRIPT_DIR"
CFLAGS+=" -I$SRC_DIR"

# Reduce LOCAL_SCALE_FACTOR for timing simulation feasibility
sed 's/^#define LOCAL_SCALE_FACTOR.*/#define LOCAL_SCALE_FACTOR 1/' \
    $SRC_DIR/matmult-int.c > $SCRIPT_DIR/matmult-int.c

echo "Building matmult-int for M2Sim..."

$CC $CFLAGS -c $SCRIPT_DIR/matmult-int.c -o $SCRIPT_DIR/matmult-int.o
$CC $CFLAGS -c $SCRIPT_DIR/startup.S -o $SCRIPT_DIR/startup.o
$CC $CFLAGS -c $SCRIPT_DIR/string.c -o $SCRIPT_DIR/string.o

$CC $CFLAGS -T $SCRIPT_DIR/linker.ld \
    $SCRIPT_DIR/startup.o \
    $SCRIPT_DIR/matmult-int.o \
    $SCRIPT_DIR/string.o \
    -o $SCRIPT_DIR/matmult-int_m2sim.elf \
    -lgcc

$OBJDUMP -d $SCRIPT_DIR/matmult-int_m2sim.elf > $SCRIPT_DIR/matmult-int_m2sim.dis

echo "Build complete: matmult-int_m2sim.elf"
ls -la $SCRIPT_DIR/matmult-int_m2sim.elf
