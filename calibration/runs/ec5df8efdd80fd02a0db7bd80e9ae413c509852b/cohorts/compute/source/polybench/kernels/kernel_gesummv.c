void kernel_gesummv(int n, double alpha, double beta,
                   double *A, double *B, double *tmp,
                   double *x, double *y) {
  for (int i = 0; i < n; i++) {
    tmp[i] = 0.0;
    y[i] = 0.0;
    for (int j = 0; j < n; j++) {
      tmp[i] = A[i*n+j] * x[j] + tmp[i];
      y[i] = B[i*n+j] * x[j] + y[i];
    }
    y[i] = alpha * tmp[i] + beta * y[i];
  }
}
