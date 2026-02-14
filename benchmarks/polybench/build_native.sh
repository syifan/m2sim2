#!/bin/bash
# Build PolyBench benchmarks natively on macOS ARM64 for hardware calibration.
#
# Builds with a calibration wrapper that calls the kernel N times in a loop,
# enabling linear regression to separate startup overhead from kernel latency.
#
# Usage:
#   ./build_native.sh [benchmark] [reps] [dataset]
#     benchmark: gemm, atax, etc. (default: all)
#     reps: kernel repetition count (default: 1)
#     dataset: MINI, SMALL (default: SMALL)
#
# Output: <bench>_native_r<reps> in this directory

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Dataset selection (default: SMALL)
DATASET="${3:-SMALL}"
case "$DATASET" in
    MINI|SMALL|MEDIUM|LARGE) ;;
    *) echo "Error: Invalid dataset '$DATASET'. Use MINI, SMALL, MEDIUM, or LARGE."; exit 1 ;;
esac

CC=cc
CFLAGS="-O2 -mcpu=apple-m2 -fno-vectorize -fno-slp-vectorize"
CFLAGS+=" -I$SCRIPT_DIR/common"
CFLAGS+=" -DPOLYBENCH_USE_RESTRICT"
CFLAGS+=" -D${DATASET}_DATASET"

BENCHMARKS="gemm atax 2mm mvt jacobi-1d 3mm bicg"

build_native() {
    local name=$1
    local reps=$2
    local src_dir="$SCRIPT_DIR/$name"

    if [ ! -d "$src_dir" ]; then
        echo "Error: Benchmark directory $src_dir not found"
        return 1
    fi

    local outname="${name}_native_r${reps}"
    # kernel function name: replace hyphens with underscores
    local kernel_fn="kernel_$(echo "$name" | tr '-' '_')"

    local calib_src="$SCRIPT_DIR/_calib_${name}.c"
    printf '/* Auto-generated calibration wrapper for %s (%d reps) */\n' "$name" "$reps" > "$calib_src"
    printf '#define main benchmark_main\n' >> "$calib_src"
    printf '#include "%s/%s.c"\n' "$name" "$name" >> "$calib_src"
    printf '#undef main\n\n' >> "$calib_src"
    printf 'int main(void) {\n' >> "$calib_src"
    printf '    init_array();\n' >> "$calib_src"
    printf '    for (int r = 0; r < %d; r++) {\n' "$reps" >> "$calib_src"
    printf '        %s();\n' "$kernel_fn" >> "$calib_src"
    printf '    }\n' >> "$calib_src"
    printf '    return compute_checksum();\n' >> "$calib_src"
    printf '}\n' >> "$calib_src"

    echo "Building $name (reps=$reps)..."
    $CC $CFLAGS "$calib_src" -o "$SCRIPT_DIR/$outname"
    rm -f "$calib_src"
    echo "  -> $outname"
}

clean() {
    echo "Cleaning native build artifacts..."
    rm -f "$SCRIPT_DIR"/*_native_*
    rm -f "$SCRIPT_DIR"/_calib_*
}

BENCH_ARG="${1:-all}"
REPS_ARG="${2:-1}"

case "$BENCH_ARG" in
    clean)
        clean
        ;;
    all)
        for bench in $BENCHMARKS; do
            build_native "$bench" "$REPS_ARG"
        done
        ;;
    *)
        build_native "$BENCH_ARG" "$REPS_ARG"
        ;;
esac

echo "Done."
