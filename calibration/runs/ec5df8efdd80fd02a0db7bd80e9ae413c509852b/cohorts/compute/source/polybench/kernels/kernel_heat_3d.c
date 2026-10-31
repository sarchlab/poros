void kernel_heat_3d(int tsteps, int n,
                    double *A, double *B) {
  int t, i, j, k;
  int nn = n * n;

  for (t = 1; t <= tsteps; t++) {
    for (i = 1; i < n - 1; i++)
      for (j = 1; j < n - 1; j++)
        for (k = 1; k < n - 1; k++)
          B[i * nn + j * n + k] =
              0.125 * (A[(i + 1) * nn + j * n + k] -
                       2.0 * A[i * nn + j * n + k] +
                       A[(i - 1) * nn + j * n + k]) +
              0.125 * (A[i * nn + (j + 1) * n + k] -
                       2.0 * A[i * nn + j * n + k] +
                       A[i * nn + (j - 1) * n + k]) +
              0.125 * (A[i * nn + j * n + k + 1] -
                       2.0 * A[i * nn + j * n + k] +
                       A[i * nn + j * n + k - 1]) +
              A[i * nn + j * n + k];
    for (i = 1; i < n - 1; i++)
      for (j = 1; j < n - 1; j++)
        for (k = 1; k < n - 1; k++)
          A[i * nn + j * n + k] =
              0.125 * (B[(i + 1) * nn + j * n + k] -
                       2.0 * B[i * nn + j * n + k] +
                       B[(i - 1) * nn + j * n + k]) +
              0.125 * (B[i * nn + (j + 1) * n + k] -
                       2.0 * B[i * nn + j * n + k] +
                       B[i * nn + (j - 1) * n + k]) +
              0.125 * (B[i * nn + j * n + k + 1] -
                       2.0 * B[i * nn + j * n + k] +
                       B[i * nn + j * n + k - 1]) +
              B[i * nn + j * n + k];
  }
}
