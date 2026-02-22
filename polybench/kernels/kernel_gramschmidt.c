double __builtin_sqrt(double);

void kernel_gramschmidt(int m, int n, double *A, double *R, double *Q) {
  for (int k = 0; k < n; k++) {
    double nrm = 0.0;
    for (int i = 0; i < m; i++)
      nrm += A[i*n+k] * A[i*n+k];
    R[k*n+k] = __builtin_sqrt(nrm);
    for (int i = 0; i < m; i++)
      Q[i*n+k] = A[i*n+k] / R[k*n+k];
    for (int j = k + 1; j < n; j++) {
      R[k*n+j] = 0.0;
      for (int i = 0; i < m; i++)
        R[k*n+j] += Q[i*n+k] * A[i*n+j];
      for (int i = 0; i < m; i++)
        A[i*n+j] = A[i*n+j] - Q[i*n+k] * R[k*n+j];
    }
  }
}
