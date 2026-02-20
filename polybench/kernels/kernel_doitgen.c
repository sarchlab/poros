void kernel_doitgen(long nr, long nq, long np,
                   double *A, double *C4, double *sum) {
  for (long r = 0; r < nr; r++)
    for (long q = 0; q < nq; q++) {
      for (long p = 0; p < np; p++) {
        sum[p] = 0.0;
        for (long s = 0; s < np; s++)
          sum[p] += A[r*nq*np + q*np + s] * C4[s*np + p];
      }
      for (long p = 0; p < np; p++)
        A[r*nq*np + q*np + p] = sum[p];
    }
}
