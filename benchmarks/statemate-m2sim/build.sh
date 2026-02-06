#!/bin/bash
# Build script for statemate M2Sim bare-metal benchmark
#
# Statemate is a car window lift control state machine from Embench-IoT.
# Uses #define float int to avoid FPU operations - pure integer arithmetic.

set -e

CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBENCH_DIR="$SCRIPT_DIR/../embench-iot"
SRC_DIR="$EMBENCH_DIR/src/statemate"

CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2 -mgeneral-regs-only"
CFLAGS+=" -I$SCRIPT_DIR"
CFLAGS+=" -I$SRC_DIR"

echo "Building statemate for M2Sim..."

# Use local patched source (FP literals converted to int)
$CC $CFLAGS -c $SCRIPT_DIR/statemate.c -o $SCRIPT_DIR/statemate.o
$CC $CFLAGS -c $SCRIPT_DIR/startup.S -o $SCRIPT_DIR/startup.o

$CC $CFLAGS -T $SCRIPT_DIR/linker.ld \
    $SCRIPT_DIR/startup.o \
    $SCRIPT_DIR/statemate.o \
    -o $SCRIPT_DIR/statemate_m2sim.elf \
    -lgcc

$OBJDUMP -d $SCRIPT_DIR/statemate_m2sim.elf > $SCRIPT_DIR/statemate_m2sim.dis

echo "Build complete: statemate_m2sim.elf"
ls -la $SCRIPT_DIR/statemate_m2sim.elf
