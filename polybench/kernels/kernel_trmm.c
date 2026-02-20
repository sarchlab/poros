void kernel_trmm(int m, int n, double alpha, double *A, double *B) {
  for (int i = 0; i < m; i++)
    for (int j = 0; j < n; j++) {
      for (int k = i+1; k < m; k++)
        B[i*n+j] += A[k*m+i] * B[k*n+j];
      B[i*n+j] = alpha * B[i*n+j];
    }
}
