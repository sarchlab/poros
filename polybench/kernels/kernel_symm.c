void kernel_symm(int m, int n, double alpha, double beta,
                 double *C, double *A, double *B) {
  for (int i = 0; i < m; i++) {
    for (int j = 0; j < n; j++) {
      double temp2 = 0;
      for (int k = 0; k < i; k++) {
        C[k*n+j] += alpha * B[i*n+j] * A[i*m+k];
        temp2 += B[k*n+j] * A[i*m+k];
      }
      C[i*n+j] = beta * C[i*n+j] + alpha * B[i*n+j] * A[i*m+i] + alpha * temp2;
    }
  }
}
