void kernel_bicg(int m, int n, double *A, double *s, double *q, double *p, double *r) {
  for (int i = 0; i < m; i++) {
    q[i] = 0.0;
    for (int j = 0; j < n; j++) {
      s[j] = s[j] + r[i] * A[i*n+j];
      q[i] = q[i] + A[i*n+j] * p[j];
    }
  }
}
