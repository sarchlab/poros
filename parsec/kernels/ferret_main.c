#include <stdio.h>
#include <stdlib.h>
#include <time.h>

extern void knn_search(
    double *results, double *queries, double *database,
    int nQueries, int nDB, int dim, int k);

#define NQUERIES 16
#define NDB 128
#define DIM 16
#define K 4
#define NUM_ITERS 10000

int main() {
    double *queries = (double *)calloc(NQUERIES * DIM, sizeof(double));
    double *database = (double *)calloc(NDB * DIM, sizeof(double));
    double *results = (double *)calloc(NQUERIES * K, sizeof(double));

    for (int i = 0; i < NQUERIES * DIM; i++) {
        queries[i] = (double)i * 0.1 + 0.5;
    }
    for (int i = 0; i < NDB * DIM; i++) {
        database[i] = (double)(i % 17) * 0.2 - 1.0;
    }

    // Warmup
    for (int i = 0; i < 100; i++) {
        knn_search(results, queries, database, NQUERIES, NDB, DIM, K);
    }

    // Timed run
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < NUM_ITERS; i++) {
        knn_search(results, queries, database, NQUERIES, NDB, DIM, K);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) / 1e9;
    printf("Ferret: %d queries x %d DB x dim=%d x k=%d x %d iterations\n",
           NQUERIES, NDB, DIM, K, NUM_ITERS);
    printf("Total time: %.6f s\n", elapsed);
    printf("Per iteration: %.3f ns\n", elapsed / NUM_ITERS * 1e9);
    printf("results[0] = %.15f\n", results[0]);

    free(queries);
    free(database);
    free(results);
    return 0;
}
