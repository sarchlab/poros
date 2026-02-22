double sqrt(double);

void kernel_correlation(int m, int n, double float_n,
                        double *data, double *corr,
                        double *mean, double *stddev) {
  double eps = 0.1;

  for (int j = 0; j < m; j++) {
    mean[j] = 0.0;
    for (int i = 0; i < n; i++)
      mean[j] += data[i*m+j];
    mean[j] /= float_n;
  }

  for (int j = 0; j < m; j++) {
    stddev[j] = 0.0;
    for (int i = 0; i < n; i++)
      stddev[j] += (data[i*m+j] - mean[j]) * (data[i*m+j] - mean[j]);
    stddev[j] /= float_n;
    stddev[j] = sqrt(stddev[j]);
    if (stddev[j] <= eps)
      stddev[j] = 1.0;
  }

  double sqrt_n = sqrt(float_n);
  for (int i = 0; i < n; i++)
    for (int j = 0; j < m; j++) {
      data[i*m+j] -= mean[j];
      data[i*m+j] /= (sqrt_n * stddev[j]);
    }

  for (int i = 0; i < m - 1; i++) {
    corr[i*m+i] = 1.0;
    for (int j = i + 1; j < m; j++) {
      corr[i*m+j] = 0.0;
      for (int k = 0; k < n; k++)
        corr[i*m+j] += data[k*m+i] * data[k*m+j];
      corr[j*m+i] = corr[i*m+j];
    }
  }
  corr[(m-1)*m+(m-1)] = 1.0;
}
