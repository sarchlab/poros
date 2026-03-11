#include <math.h>

// Cholesky decomposition of an n×n symmetric positive-definite matrix A.
// Result overwrites the lower triangle of A. 
// This is the computational core of the swaptions benchmark.
void cholesky(double *A, int n) {
    for (int j = 0; j < n; j++) {
        double sum = 0.0;
        for (int k = 0; k < j; k++) {
            sum += A[j*n+k] * A[j*n+k];
        }
        A[j*n+j] = sqrt(A[j*n+j] - sum);
        
        for (int i = j+1; i < n; i++) {
            sum = 0.0;
            for (int k = 0; k < j; k++) {
                sum += A[i*n+k] * A[j*n+k];
            }
            A[i*n+j] = (A[i*n+j] - sum) / A[j*n+j];
        }
    }
}

// Simple HJM simulation step using Cholesky factor L and random normal Z.
// Computes forward rate update: dF[i] = sum_j(L[i][j] * Z[j]) * exp(-r * dt)
void hjm_step(
    double *dF,    // output: rate changes [n]
    double *L,     // Cholesky factor [n*n]
    double *Z,     // random normals [n]
    double r,      // short rate
    double dt,     // time step
    int n
) {
    double discount = exp(-r * dt);
    for (int i = 0; i < n; i++) {
        double sum = 0.0;
        for (int j = 0; j <= i; j++) {
            sum += L[i*n+j] * Z[j];
        }
        dF[i] = sum * discount;
    }
}
