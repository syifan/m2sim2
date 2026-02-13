#!/bin/bash
# Build script for primecount M2Sim bare-metal benchmark

set -e

# Cross-compiler
CC=aarch64-elf-gcc
OBJDUMP=aarch64-elf-objdump

# Paths
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EMBENCH_DIR="$SCRIPT_DIR/../embench-iot"
SRC_DIR="$EMBENCH_DIR/src/primecount"

# Compiler flags - local include FIRST for our minimal headers
CFLAGS="-O2 -ffreestanding -nostdlib -mcpu=apple-m2"
CFLAGS+=" -I$SCRIPT_DIR"  # Our support.h and minimal libc headers FIRST
CFLAGS+=" -I$SRC_DIR"     # Embench source

# Reduce workload for timing simulation feasibility:
# SZ=3 -> sieve primes [2,3,5], counts primes up to 25 -> NPRIMES=9
sed -e 's/^#define SZ .*/#define SZ 3/' \
    -e 's/^#define NPRIMES .*/#define NPRIMES 9/' \
    $SRC_DIR/primecount.c > $SCRIPT_DIR/primecount.c

# Build
echo "Building primecount for M2Sim..."

$CC $CFLAGS -c $SCRIPT_DIR/primecount.c -o $SCRIPT_DIR/primecount.o
$CC $CFLAGS -c $SCRIPT_DIR/startup.S -o $SCRIPT_DIR/startup.o

$CC $CFLAGS -T $SCRIPT_DIR/linker.ld \
    $SCRIPT_DIR/startup.o \
    $SCRIPT_DIR/primecount.o \
    -o $SCRIPT_DIR/primecount_m2sim.elf \
    -lgcc

# Generate disassembly
$OBJDUMP -d $SCRIPT_DIR/primecount_m2sim.elf > $SCRIPT_DIR/primecount_m2sim.dis

echo "Build complete: primecount_m2sim.elf"
ls -la $SCRIPT_DIR/primecount_m2sim.elf
