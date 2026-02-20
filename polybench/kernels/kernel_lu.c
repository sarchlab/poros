void kernel_lu(int n, double *A) {
  for (int i = 0; i < n; i++) {
    for (int j = 0; j < i; j++) {
      for (int k = 0; k < j; k++) {
        A[i*n+j] -= A[i*n+k] * A[k*n+j];
      }
      A[i*n+j] /= A[j*n+j];
    }
    for (int j = i; j < n; j++) {
      for (int k = 0; k < i; k++) {
        A[i*n+j] -= A[i*n+k] * A[k*n+j];
      }
    }
  }
}
