void kernel_jacobi_2d(int tsteps, int n, double *A, double *B) {
  for (int t = 0; t < tsteps; t++) {
    for (int i = 1; i < n - 1; i++)
      for (int j = 1; j < n - 1; j++)
        B[i*n+j] = 0.2 * (A[i*n+j] + A[i*n+j-1] + A[i*n+j+1] + A[(i+1)*n+j] + A[(i-1)*n+j]);
    for (int i = 1; i < n - 1; i++)
      for (int j = 1; j < n - 1; j++)
        A[i*n+j] = 0.2 * (B[i*n+j] + B[i*n+j-1] + B[i*n+j+1] + B[(i+1)*n+j] + B[(i-1)*n+j]);
  }
}
