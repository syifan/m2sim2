# Leo — Workspace Note (Cycle 2)

## What I Did
- Fixed gofmt alignment on PR #299 (exit_group) and PR #300 (mprotect) — pushed fixes
- Installed `musl-cross` cross-compiler (`aarch64-linux-musl-gcc`)
- Cross-compiled 3 SPEC benchmarks as static ARM64 Linux ELF:
  - 505.mcf_r (C) — direct musl-gcc
  - 531.deepsjeng_r (C++) — direct musl-g++
  - 548.exchange2_r (Fortran) — Docker Alpine ARM64 with gfortran
- Added 548.exchange2_r to spec_runner.go config
- Created build_spec.sh script documenting compilation commands
- Opened PR #306 for all cross-compilation work (closes #296)

## Context for Next Cycle
- PR #299 and #300 should be CI-green now — need to get them merged
- PR #306 needs CI check and review — then merge
- After #299/#300/#306 merge, check #277 (validate 548.exchange2_r execution)
- The binaries are placed in SPEC exe/ dirs on this machine; CI won't have them
- Next priorities: #273 (getpid/getuid/gettid), #274 (clock_gettime), or help with validation

## Lessons Learned
- musl-cross doesn't include Fortran; use Docker Alpine ARM64 for Fortran cross-compilation
- 548.exchange2_r needs `-DSPEC -cpp` flags and reads puzzles.txt + control file
- 505.mcf_r needs `-DSPEC -DSPEC_LP64` and spec_qsort subdirectory
- 531.deepsjeng_r needs `-DSPEC -DSMALL_MEMORY`
- Always run `gofmt -w` after modifying Go files to avoid CI lint failures
