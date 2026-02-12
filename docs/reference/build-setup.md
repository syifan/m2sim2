# Cross-Compiler Setup for M2Sim

## Overview

M2Sim requires ELF binaries for execution. macOS compiles to Mach-O by default, so we need a cross-compiler to build benchmarks that can run in the simulator.

## Option 1: Homebrew aarch64-elf-gcc (Recommended)

### Installation

```bash
# Install the cross-compiler toolchain
brew install aarch64-elf-gcc

# Verify installation
aarch64-elf-gcc --version
```

**Note:** This installs `aarch64-elf-binutils` as a dependency.

### Usage

```bash
# Compile to bare-metal ELF
aarch64-elf-gcc -O2 -static -nostdlib -o program.elf program.c

# Or with newlib for C library support
aarch64-elf-gcc -O2 -static -specs=nosys.specs -o program.elf program.c
```

### Pros
- Official Homebrew package
- Stable, well-maintained
- Includes binutils (objdump, etc.)

### Cons
- Large installation (~1GB with dependencies)
- Build time if not bottled

## Option 2: LLVM/Clang Cross-Compile

If you have LLVM installed:

```bash
# Use clang with explicit target
clang --target=aarch64-elf -O2 -static -nostdlib -o program.elf program.c
```

**Note:** May need additional linker configuration for bare-metal targets.

## Option 3: ARM GNU Toolchain

ARM provides official embedded toolchains:

```bash
brew install --cask gcc-arm-embedded
# or
brew install gcc-aarch64-embedded
```

## Verification

After installation, verify:

```bash
# Check compiler
aarch64-elf-gcc --version

# Check it produces ELF
echo 'int main() { return 0; }' > test.c
aarch64-elf-gcc -O2 -static -nostdlib -e main -o test.elf test.c
file test.elf  # Should show "ELF 64-bit LSB executable, ARM aarch64"
rm test.c test.elf
```

## CoreMark Cross-Compilation

Once installed, use this to compile CoreMark:

```bash
cd benchmarks/coremark
aarch64-elf-gcc -O2 -static -DITERATIONS=10000 \
  -DCOMPILER_FLAGS=\"-O2\" \
  core_list_join.c core_main.c core_matrix.c core_state.c core_util.c \
  -o coremark.elf
```

## References

- Issue #149: Cross-compiler setup task
- Issue #147: CoreMark integration
- Homebrew formula: https://formulae.brew.sh/formula/aarch64-elf-gcc
