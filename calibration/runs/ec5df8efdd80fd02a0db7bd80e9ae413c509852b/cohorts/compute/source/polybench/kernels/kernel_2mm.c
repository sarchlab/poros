void kernel_2mm(int ni, int nj, int nk, int nl,
                double alpha, double beta,
                double *tmp, double *A, double *B, double *C, double *D) {
  for (int i = 0; i < ni; i++)
    for (int j = 0; j < nj; j++) {
      tmp[i*nj+j] = 0.0;
      for (int k = 0; k < nk; k++)
        tmp[i*nj+j] += alpha * A[i*nk+k] * B[k*nj+j];
    }
  for (int i = 0; i < ni; i++)
    for (int j = 0; j < nl; j++) {
      D[i*nl+j] *= beta;
      for (int k = 0; k < nj; k++)
        D[i*nl+j] += tmp[i*nj+k] * C[k*nl+j];
    }
}
