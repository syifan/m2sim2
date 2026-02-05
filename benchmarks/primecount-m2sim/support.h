/* M2Sim support header for Embench benchmarks */
#ifndef SUPPORT_H
#define SUPPORT_H

#define CPU_MHZ 1

/* Benchmark interface */
void initialise_benchmark(void);
int benchmark(void);
int verify_benchmark(int result);

/* Time tracking (unused in M2Sim) */
#define start_trigger() ((void)0)
#define stop_trigger() ((void)0)

#endif /* SUPPORT_H */
