void kernel_trisolv(int n, double *L, double *x, double *b) {
  for (int i = 0; i < n; i++) {
    x[i] = b[i];
    for (int j = 0; j < i; j++)
      x[i] -= L[i*n+j] * x[j];
    x[i] = x[i] / L[i*n+i];
  }
}
