#include <stdio.h>
#include <stdlib.h>
#include <time.h>

extern int routing_cost(int *netlist, int *locations, int n);

#define N 8192
#define M 512
#define NUM_ITERS 1000

int main() {
    int *netlist = (int *)malloc(N * 2 * sizeof(int));
    int *locations = (int *)malloc(M * 2 * sizeof(int));

    for (int i = 0; i < N * 2; i++) {
        netlist[i] = i % M;
    }
    for (int i = 0; i < M; i++) {
        locations[2 * i] = (i * 37) % 1000;
        locations[2 * i + 1] = (i * 73) % 1000;
    }

    // Warmup
    volatile int sink = 0;
    for (int i = 0; i < 100; i++) {
        sink = routing_cost(netlist, locations, N);
    }

    // Timed run
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < NUM_ITERS; i++) {
        sink = routing_cost(netlist, locations, N);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) / 1e9;
    printf("Canneal: %d nets x %d locations x %d iterations\n", N, M, NUM_ITERS);
    printf("Total time: %.6f s\n", elapsed);
    printf("Per iteration: %.3f ns\n", elapsed / NUM_ITERS * 1e9);
    printf("cost = %d\n", sink);

    free(netlist);
    free(locations);
    return 0;
}
