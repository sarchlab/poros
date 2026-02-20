void kernel_syr2k(int n, int m, double alpha, double beta, double *C, double *A, double *B) {
  for (int i = 0; i < n; i++) {
    for (int j = 0; j <= i; j++)
      C[i*n+j] *= beta;
    for (int k = 0; k < m; k++)
      for (int j = 0; j <= i; j++)
        C[i*n+j] += A[j*m+k]*alpha*B[i*m+k] + B[j*m+k]*alpha*A[i*m+k];
  }
}
