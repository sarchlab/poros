/* y = A^T (A x), the original ATAX compute kernel. */
static void kernel_atax(int m, int n, double *A, double *x, double *y,
                        double *tmp) {
  for (int i = 0; i < m; i++) {
    for (int j = 0; j < n; j++)
      tmp[i] += A[i * n + j] * x[j];
    for (int j = 0; j < n; j++)
      y[j] += A[i * n + j] * tmp[i];
  }
}
