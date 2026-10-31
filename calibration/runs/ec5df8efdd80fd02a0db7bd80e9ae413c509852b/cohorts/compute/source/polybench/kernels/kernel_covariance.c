void kernel_covariance(int m, int n, double float_n,
                       double *data, double *cov, double *mean) {
  for (int j = 0; j < m; j++) {
    mean[j] = 0.0;
    for (int i = 0; i < n; i++)
      mean[j] += data[i*m+j];
    mean[j] /= float_n;
  }

  for (int i = 0; i < n; i++)
    for (int j = 0; j < m; j++)
      data[i*m+j] -= mean[j];

  for (int i = 0; i < m; i++)
    for (int j = i; j < m; j++) {
      cov[i*m+j] = 0.0;
      for (int k = 0; k < n; k++)
        cov[i*m+j] += data[k*m+i] * data[k*m+j];
      cov[i*m+j] /= (float_n - 1.0);
      cov[j*m+i] = cov[i*m+j];
    }
}
