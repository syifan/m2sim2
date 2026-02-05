# primecount-m2sim

Embench-IoT primecount benchmark adapted for M2Sim bare-metal execution.

## Description

Prime counting benchmark - counts primes using the Sieve of Eratosthenes algorithm.
Pure integer math, good for testing ALU operations and loop performance.

## Building

```bash
./build.sh
```

Requires `aarch64-elf-gcc` cross-compiler.

## Output

- `primecount_m2sim.elf` - bare-metal executable
- `primecount_m2sim.dis` - disassembly for debugging

## Source

Original source from Embench-IoT repository: `../embench-iot/src/primecount/`
