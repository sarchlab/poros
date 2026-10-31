void kernel_ludcmp(int n, double *A, double *b, double *x, double *y) {
  double w;

  for (int i = 0; i < n; i++) {
    for (int j = 0; j < i; j++) {
      w = A[i*n+j];
      for (int k = 0; k < j; k++) {
        w -= A[i*n+k] * A[k*n+j];
      }
      A[i*n+j] = w / A[j*n+j];
    }
    for (int j = i; j < n; j++) {
      w = A[i*n+j];
      for (int k = 0; k < i; k++) {
        w -= A[i*n+k] * A[k*n+j];
      }
      A[i*n+j] = w;
    }
  }

  for (int i = 0; i < n; i++) {
    w = b[i];
    for (int j = 0; j < i; j++)
      w -= A[i*n+j] * y[j];
    y[i] = w;
  }

  for (int i = n-1; i >= 0; i--) {
    w = y[i];
    for (int j = i+1; j < n; j++)
      w -= A[i*n+j] * x[j];
    x[i] = w / A[i*n+i];
  }
}
