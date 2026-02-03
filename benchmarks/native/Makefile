# Makefile for M2Sim Native Calibration Benchmarks
# Compiles ARM64 assembly benchmarks for macOS

# Detect architecture
ARCH := $(shell uname -m)

# Compiler settings
AS = as
LD = ld

# macOS-specific linker flags
LDFLAGS = -lSystem -L$(shell xcrun --show-sdk-path)/usr/lib -syslibroot $(shell xcrun --show-sdk-path) -e _main

# Assembly sources
SRCS = arithmetic_sequential.s \
       dependency_chain.s \
       memory_sequential.s \
       function_calls.s \
       branch_taken.s \
       mixed_operations.s

# Output binaries
BINS = $(SRCS:.s=)

# Object files
OBJS = $(SRCS:.s=.o)

.PHONY: all clean run check

all: check_arch $(BINS)

check_arch:
ifneq ($(ARCH),arm64)
	$(error This Makefile requires ARM64/Apple Silicon. Current arch: $(ARCH))
endif

# Pattern rule: .s -> .o
%.o: %.s
	$(AS) -o $@ $<

# Pattern rule: .o -> binary
%: %.o
	$(LD) $(LDFLAGS) -o $@ $<

# Build all binaries
$(BINS): %: %.o

clean:
	rm -f $(OBJS) $(BINS)

# Run all benchmarks and verify exit codes
run: all
	@echo "Running native calibration benchmarks..."
	@echo ""
	@for bin in $(BINS); do \
		printf "%-25s: " "$$bin"; \
		./$$bin; \
		echo "exit=$$?"; \
	done

# Verify expected exit codes match
verify: all
	@echo "Verifying benchmark exit codes..."
	@failed=0; \
	./arithmetic_sequential; test $$? -eq 4 || { echo "FAIL: arithmetic_sequential (expected 4)"; failed=1; }; \
	./dependency_chain; test $$? -eq 20 || { echo "FAIL: dependency_chain (expected 20)"; failed=1; }; \
	./memory_sequential; test $$? -eq 42 || { echo "FAIL: memory_sequential (expected 42)"; failed=1; }; \
	./function_calls; test $$? -eq 5 || { echo "FAIL: function_calls (expected 5)"; failed=1; }; \
	./branch_taken; test $$? -eq 5 || { echo "FAIL: branch_taken (expected 5)"; failed=1; }; \
	./mixed_operations; test $$? -eq 100 || { echo "FAIL: mixed_operations (expected 100)"; failed=1; }; \
	if [ $$failed -eq 0 ]; then echo "All benchmarks verified!"; else exit 1; fi

# Time a single benchmark with /usr/bin/time
time-%: %
	@echo "Timing $<..."
	/usr/bin/time -l ./$<

# Run benchmark N times for statistical sampling
bench-%: %
	@echo "Running $< 1000 times..."
	@for i in $$(seq 1 1000); do ./$<; done
