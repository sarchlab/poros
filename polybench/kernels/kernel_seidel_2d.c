void kernel_seidel_2d(int tsteps, int n, double *A) {
  for (int t = 0; t < tsteps; t++)
    for (int i = 1; i < n - 1; i++)
      for (int j = 1; j < n - 1; j++)
        A[i*n+j] = (A[(i-1)*n+j-1] + A[(i-1)*n+j] + A[(i-1)*n+j+1]
                   + A[i*n+j-1] + A[i*n+j] + A[i*n+j+1]
                   + A[(i+1)*n+j-1] + A[(i+1)*n+j] + A[(i+1)*n+j+1]) / 9.0;
}
