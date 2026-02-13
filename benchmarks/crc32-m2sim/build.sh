#!/bin/bash
# Build script for crc32 M2Sim bare-metal benchmark

set -e

CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBENCH_DIR="$SCRIPT_DIR/../embench-iot"
SRC_DIR="$EMBENCH_DIR/src/crc32"

CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2"
CFLAGS+=" -I$SCRIPT_DIR"
CFLAGS+=" -I$SRC_DIR"

# Reduce LOCAL_SCALE_FACTOR for timing simulation feasibility
sed 's/^#define LOCAL_SCALE_FACTOR.*/#define LOCAL_SCALE_FACTOR 1/' \
    $SRC_DIR/crc_32.c > $SCRIPT_DIR/crc_32.c

echo "Building crc32 for M2Sim..."

$CC $CFLAGS -c $SCRIPT_DIR/crc_32.c -o $SCRIPT_DIR/crc_32.o
$CC $CFLAGS -c $SCRIPT_DIR/startup.S -o $SCRIPT_DIR/startup.o

$CC $CFLAGS -T $SCRIPT_DIR/linker.ld \
    $SCRIPT_DIR/startup.o \
    $SCRIPT_DIR/crc_32.o \
    -o $SCRIPT_DIR/crc32_m2sim.elf \
    -lgcc

$OBJDUMP -d $SCRIPT_DIR/crc32_m2sim.elf > $SCRIPT_DIR/crc32_m2sim.dis

echo "Build complete: crc32_m2sim.elf"
ls -la $SCRIPT_DIR/crc32_m2sim.elf
