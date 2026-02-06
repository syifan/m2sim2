# huffbench M2Sim Benchmark

Huffman compression/decompression benchmark from Embench-IoT, ported for M2Sim bare-metal execution.

## Description

This benchmark implements the Huffman compression algorithm. It compresses and decompresses a 500-byte test data set.

## Building

```bash
./build.sh
```

## Dependencies

This benchmark requires the BEEBS heap library for dynamic memory allocation (included as beebsc.c).

## Notes

- Uses 8KB static heap for malloc_beebs allocations
- Scale factor set to 11 iterations (configurable in libhuffbench.c)
- Original source from Embench-IoT: src/huffbench/libhuffbench.c

## Author

Original benchmark by Scott Robert Ladd.
Embench packaging by Bristol/Embecosm.
M2Sim port for bare-metal execution.
