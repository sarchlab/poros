// k-NN distance computation kernel (ferret-style similarity search)
// For each query, compute L2 distance to all database vectors,
// find the k nearest neighbors.
//
// ARM64 ABI:
//   X0 = results (double[nQueries * k]) - output distances (sorted ascending)
//   X1 = queries (double[nQueries * dim])
//   X2 = database (double[nDB * dim])
//   W3 = nQueries
//   W4 = nDB
//   W5 = dim
//   W6 = k (number of nearest neighbors)
void knn_search(
    double *results,
    double *queries,
    double *database,
    int nQueries,
    int nDB,
    int dim,
    int k
) {
    // Large initialization value
    double big = 1e30;
    
    for (int q = 0; q < nQueries; q++) {
        double *query = &queries[q * dim];
        double *res = &results[q * k];

        // Initialize results to large values (manual loop, no memset)
        for (int i = 0; i < k; i++) {
            res[i] = big;
        }

        // Scan all database vectors
        for (int d = 0; d < nDB; d++) {
            double *dbVec = &database[d * dim];

            // Compute L2 distance squared
            double dist = 0.0;
            for (int j = 0; j < dim; j++) {
                double diff = query[j] - dbVec[j];
                dist += diff * diff;
            }

            // Insert into sorted results (insertion sort)
            if (dist < res[k - 1]) {
                res[k - 1] = dist;
                // Bubble up
                for (int i = k - 2; i >= 0; i--) {
                    if (res[i] > res[i + 1]) {
                        double tmp = res[i];
                        res[i] = res[i + 1];
                        res[i + 1] = tmp;
                    } else {
                        break;  // sorted position found
                    }
                }
            }
        }
    }
}
