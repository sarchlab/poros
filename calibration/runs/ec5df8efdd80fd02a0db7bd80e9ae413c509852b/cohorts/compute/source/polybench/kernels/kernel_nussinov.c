void kernel_nussinov(int n, unsigned char *seq, int *table) {
  for (int i = n-1; i >= 0; i--) {
    for (int j = i+1; j < n; j++) {

      if (j-1 >= 0)
        table[i*n+j] = table[i*n+j] >= table[i*n+j-1] ? table[i*n+j] : table[i*n+j-1];
      if (i+1 < n)
        table[i*n+j] = table[i*n+j] >= table[(i+1)*n+j] ? table[i*n+j] : table[(i+1)*n+j];

      if (j-1 >= 0 && i+1 < n) {
        int s = table[(i+1)*n+j-1] + ((seq[i]+seq[j]) == 3 ? 1 : 0);
        if (i < j-1)
          table[i*n+j] = table[i*n+j] >= s ? table[i*n+j] : s;
        else
          table[i*n+j] = table[i*n+j] >= table[(i+1)*n+j-1] ? table[i*n+j] : table[(i+1)*n+j-1];
      }

      for (int k = i+1; k < j; k++) {
        int s = table[i*n+k] + table[(k+1)*n+j];
        table[i*n+j] = table[i*n+j] >= s ? table[i*n+j] : s;
      }
    }
  }
}
