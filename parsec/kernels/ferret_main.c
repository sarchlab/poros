#include <stdio.h>
#include <stdlib.h>
#include <mach/mach_time.h>

void knn_search(
    double *results,
    double *queries,
    double *database,
    int nQueries,
    int nDB,
    int dim,
    int k
) {
    double big = 1e30;

    for (int q = 0; q < nQueries; q++) {
        double *query = &queries[q * dim];
        double *res = &results[q * k];

        for (int i = 0; i < k; i++) {
            res[i] = big;
        }

        for (int d = 0; d < nDB; d++) {
            double *dbVec = &database[d * dim];

            double dist = 0.0;
            for (int j = 0; j < dim; j++) {
                double diff = query[j] - dbVec[j];
                dist += diff * diff;
            }

            if (dist < res[k - 1]) {
                res[k - 1] = dist;
                for (int i = k - 2; i >= 0; i--) {
                    if (res[i] > res[i + 1]) {
                        double tmp = res[i];
                        res[i] = res[i + 1];
                        res[i + 1] = tmp;
                    } else {
                        break;
                    }
                }
            }
        }
    }
}

int main() {
    int nQueries = 16, nDB = 128, dim = 16, k = 4;

    double *queries = (double *)calloc(nQueries * dim, sizeof(double));
    double *database = (double *)calloc(nDB * dim, sizeof(double));
    double *results = (double *)calloc(nQueries * k, sizeof(double));

    for (int i = 0; i < nQueries * dim; i++) {
        queries[i] = (double)i * 0.1 + 0.5;
    }
    for (int i = 0; i < nDB * dim; i++) {
        database[i] = (double)(i % 17) * 0.2 - 1.0;
    }

    mach_timebase_info_data_t info;
    mach_timebase_info(&info);

    for (int trial = 0; trial < 7; trial++) {
        uint64_t start = mach_absolute_time();
        knn_search(results, queries, database, nQueries, nDB, dim, k);
        uint64_t end = mach_absolute_time();
        double ns = (double)(end - start) * info.numer / info.denom;
        printf("Trial %d: %.0f ns\n", trial, ns);
    }

    // Print a sample output to prevent dead code elimination
    printf("results[0] = %f\n", results[0]);

    free(queries);
    free(database);
    free(results);
    return 0;
}
