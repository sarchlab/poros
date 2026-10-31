// Streamcluster: pairwise distance computation for online clustering
// Computes squared Euclidean distance between each pair of point sets.
//
// ARM64 ABI:
//   X0 = p1 pointer (double[npoints * dim])
//   X1 = p2 pointer (double[npoints * dim])
//   X2 = out pointer (double[npoints])
//   W3 = npoints
//   W4 = dim
void compute_distances(
    double *p1,
    double *p2,
    double *out,
    int npoints,
    int dim
) {
    for (int i = 0; i < npoints; i++) {
        double dist = 0.0;
        for (int j = 0; j < dim; j++) {
            double diff = p1[i * dim + j] - p2[i * dim + j];
            dist += diff * diff;
        }
        out[i] = dist;
    }
}
