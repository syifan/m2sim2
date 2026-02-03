/*
 * measure.c - High-precision benchmark measurement wrapper
 * 
 * Runs a benchmark binary many times in a subprocess and measures
 * wall-clock time, then divides by iterations to get per-execution time.
 * 
 * IMPORTANT: These measurements include process startup/exit overhead
 * (~1-2ms per run). For tiny benchmarks, this dominates the actual
 * execution time. Use xctrace with performance counters for accurate
 * CPU cycle measurements.
 *
 * Build: clang -O2 -o measure measure.c
 * Usage: ./measure <benchmark> [iterations]
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/wait.h>
#include <mach/mach_time.h>
#include <fcntl.h>

#define DEFAULT_ITERATIONS 1000
#define M2_FREQ_GHZ 3.5

/* Convert mach absolute time to nanoseconds */
static double mach_to_ns(uint64_t mach_time) {
    static mach_timebase_info_data_t timebase = {0};
    if (timebase.denom == 0) {
        mach_timebase_info(&timebase);
    }
    return (double)mach_time * timebase.numer / timebase.denom;
}

/* Run benchmark once and return exit code */
static int run_benchmark(const char *path) {
    pid_t pid = fork();
    if (pid == 0) {
        /* Child: redirect stdout/stderr to /dev/null and exec */
        int devnull = open("/dev/null", O_WRONLY);
        if (devnull >= 0) {
            dup2(devnull, STDOUT_FILENO);
            dup2(devnull, STDERR_FILENO);
            close(devnull);
        }
        execl(path, path, NULL);
        _exit(127);  /* exec failed */
    }
    
    int status;
    waitpid(pid, &status, 0);
    return WIFEXITED(status) ? WEXITSTATUS(status) : -1;
}

/* Get instruction count for known benchmarks */
static int get_instruction_count(const char *name) {
    if (strstr(name, "arithmetic_sequential")) return 24;
    if (strstr(name, "dependency_chain")) return 24;
    if (strstr(name, "memory_sequential")) return 25;
    if (strstr(name, "function_calls")) return 18;
    if (strstr(name, "branch_taken")) return 15;
    if (strstr(name, "mixed_operations")) return 45;
    return 1;
}

int main(int argc, char *argv[]) {
    if (argc < 2) {
        fprintf(stderr, "Usage: %s <benchmark> [iterations]\n", argv[0]);
        fprintf(stderr, "\nMeasures benchmark execution time with high precision.\n");
        fprintf(stderr, "Default iterations: %d\n", DEFAULT_ITERATIONS);
        fprintf(stderr, "\nNote: Results include process startup overhead (~1-2ms).\n");
        fprintf(stderr, "For accurate cycle counts, use xctrace with CPU Counters.\n");
        return 1;
    }
    
    const char *benchmark = argv[1];
    int iterations = argc > 2 ? atoi(argv[2]) : DEFAULT_ITERATIONS;
    
    if (iterations < 1) {
        fprintf(stderr, "Error: iterations must be positive\n");
        return 1;
    }
    
    /* Check benchmark exists */
    if (access(benchmark, X_OK) != 0) {
        fprintf(stderr, "Error: cannot execute '%s'\n", benchmark);
        return 1;
    }
    
    fprintf(stderr, "Benchmark: %s\n", benchmark);
    fprintf(stderr, "Iterations: %d\n", iterations);
    fprintf(stderr, "\n");
    
    /* Warmup */
    fprintf(stderr, "Warming up...\n");
    for (int i = 0; i < 10; i++) {
        run_benchmark(benchmark);
    }
    
    /* Timed runs */
    fprintf(stderr, "Running benchmark...\n");
    
    uint64_t start = mach_absolute_time();
    
    int last_exit = 0;
    for (int i = 0; i < iterations; i++) {
        last_exit = run_benchmark(benchmark);
    }
    
    uint64_t end = mach_absolute_time();
    
    /* Calculate results */
    double total_ns = mach_to_ns(end - start);
    double avg_ns = total_ns / iterations;
    double avg_us = avg_ns / 1000.0;
    double avg_ms = avg_ns / 1000000.0;
    
    /* Estimate cycles - note this includes process overhead */
    double est_cycles = avg_ns * M2_FREQ_GHZ;  /* ns * GHz = cycles */
    
    int instr = get_instruction_count(benchmark);
    double cpi = est_cycles / instr;
    
    fprintf(stderr, "\nResults:\n");
    fprintf(stderr, "  Total time:     %.3f ms\n", total_ns / 1000000.0);
    fprintf(stderr, "  Per iteration:  %.3f ms (includes ~1.5ms process overhead)\n", avg_ms);
    fprintf(stderr, "  Est. cycles:    %.0f (dominated by process startup)\n", est_cycles);
    fprintf(stderr, "  Instructions:   %d (benchmark only)\n", instr);
    fprintf(stderr, "  Exit code:      %d\n", last_exit);
    fprintf(stderr, "\n");
    fprintf(stderr, "Note: For meaningful CPI, use xctrace to measure actual CPU cycles.\n");
    
    /* JSON output to stdout */
    printf("{\"name\": \"%s\", \"iterations\": %d, \"avg_ms\": %.3f, "
           "\"exit_code\": %d, \"note\": \"includes process overhead\"}\n",
           benchmark, iterations, avg_ms, last_exit);
    
    return 0;
}
