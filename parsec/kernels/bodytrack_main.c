// Native timing harness for bodytrack particle filter likelihood kernel.
//
// Build:
//   clang -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops \
//         bodytrack.c bodytrack_main.c -o bodytrack_native -lm
//
// Run:
//   ./bodytrack_native

#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

extern void bodytrack_likelihood(
    double *likelihood,
    double *states,
    double *observations,
    double *projMatrix,
    int nParticles,
    int stateDim,
    int obsDim
);

int main(void) {
    int nParticles = 64;
    int stateDim = 6;
    int obsDim = 8;
    int nRuns = 501;

    double *states = calloc(nParticles * stateDim, sizeof(double));
    double *observations = calloc(obsDim, sizeof(double));
    double *projMatrix = calloc(obsDim * stateDim, sizeof(double));
    double *likelihood = calloc(nParticles, sizeof(double));

    // Initialize states
    for (int p = 0; p < nParticles; p++) {
        for (int j = 0; j < stateDim; j++) {
            states[p * stateDim + j] = 0.1 * (p + 1) + 0.05 * j;
        }
    }

    // Initialize observations
    for (int i = 0; i < obsDim; i++) {
        observations[i] = 1.0 + 0.5 * i;
    }

    // Initialize projection matrix
    for (int i = 0; i < obsDim; i++) {
        for (int j = 0; j < stateDim; j++) {
            projMatrix[i * stateDim + j] = ((i + j) % 5 + 1) * 0.1;
        }
    }

    // Warm up
    bodytrack_likelihood(likelihood, states, observations, projMatrix,
                         nParticles, stateDim, obsDim);

    // Timing runs
    double *times = calloc(nRuns, sizeof(double));
    for (int r = 0; r < nRuns; r++) {
        struct timespec t0, t1;
        clock_gettime(CLOCK_MONOTONIC, &t0);
        bodytrack_likelihood(likelihood, states, observations, projMatrix,
                             nParticles, stateDim, obsDim);
        clock_gettime(CLOCK_MONOTONIC, &t1);
        times[r] = (t1.tv_sec - t0.tv_sec) + (t1.tv_nsec - t0.tv_nsec) * 1e-9;
    }

    // Sort for median
    for (int i = 0; i < nRuns - 1; i++) {
        for (int j = i + 1; j < nRuns; j++) {
            if (times[j] < times[i]) {
                double tmp = times[i];
                times[i] = times[j];
                times[j] = tmp;
            }
        }
    }

    double median = times[nRuns / 2];
    printf("bodytrack_likelihood: median = %.3f ns (%.6e s)\n",
           median * 1e9, median);
    printf("  nParticles=%d stateDim=%d obsDim=%d\n",
           nParticles, stateDim, obsDim);

    // Print a few likelihood values for verification
    printf("  likelihood[0]=%.6e likelihood[1]=%.6e likelihood[%d]=%.6e\n",
           likelihood[0], likelihood[1], nParticles - 1,
           likelihood[nParticles - 1]);

    free(states);
    free(observations);
    free(projMatrix);
    free(likelihood);
    free(times);

    return 0;
}
