void kernel_floyd_warshall(int n, double *path) {
  for (int k = 0; k < n; k++)
    for (int i = 0; i < n; i++)
      for (int j = 0; j < n; j++) {
        double s = path[i*n+k] + path[k*n+j];
        if (s < path[i*n+j])
          path[i*n+j] = s;
      }
}
