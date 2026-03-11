#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <math.h>

// Prototype for the kernel
void sph_compute(
    double *px, double *py, double *pz,
    double *vx, double *vy, double *vz,
    double *density, double *pressure,
    double *fx, double *fy, double *fz,
    int n,
    double h,
    double mass,
    double rho0,
    double k,
    double mu
);

int main(void) {
    int n = 16;
    double h = 1.0, mass = 1.0, rho0 = 1000.0, k_const = 3.0, mu = 3.5;

    double *px = calloc(n, sizeof(double));
    double *py = calloc(n, sizeof(double));
    double *pz = calloc(n, sizeof(double));
    double *vx = calloc(n, sizeof(double));
    double *vy = calloc(n, sizeof(double));
    double *vz = calloc(n, sizeof(double));
    double *density = calloc(n, sizeof(double));
    double *pressure = calloc(n, sizeof(double));
    double *fx = calloc(n, sizeof(double));
    double *fy = calloc(n, sizeof(double));
    double *fz = calloc(n, sizeof(double));

    for (int i = 0; i < n; i++) {
        px[i] = (i % 4) * 0.3;
        py[i] = (i / 4) * 0.3;
        pz[i] = 0.0;
        vx[i] = 0.1 * (1 - 2 * (i % 2));
        vy[i] = 0.1 * (1 - 2 * ((i / 2) % 2));
        vz[i] = 0.0;
    }

    int runs = 501;
    double *times = calloc(runs, sizeof(double));

    for (int r = 0; r < runs; r++) {
        // Reset output arrays
        for (int i = 0; i < n; i++) {
            density[i] = 0; pressure[i] = 0;
            fx[i] = 0; fy[i] = 0; fz[i] = 0;
        }
        struct timespec t0, t1;
        clock_gettime(CLOCK_MONOTONIC, &t0);
        sph_compute(px, py, pz, vx, vy, vz, density, pressure,
                     fx, fy, fz, n, h, mass, rho0, k_const, mu);
        clock_gettime(CLOCK_MONOTONIC, &t1);
        times[r] = (t1.tv_sec - t0.tv_sec) + (t1.tv_nsec - t0.tv_nsec) * 1e-9;
    }

    // Sort for median
    for (int i = 0; i < runs - 1; i++)
        for (int j = i + 1; j < runs; j++)
            if (times[j] < times[i]) {
                double tmp = times[i]; times[i] = times[j]; times[j] = tmp;
            }

    printf("median time: %.9f s\n", times[runs / 2]);
    printf("density[0]: %.12g\n", density[0]);
    printf("fx[0]: %.12g\n", fx[0]);

    free(px); free(py); free(pz);
    free(vx); free(vy); free(vz);
    free(density); free(pressure);
    free(fx); free(fy); free(fz);
    free(times);
    return 0;
}
