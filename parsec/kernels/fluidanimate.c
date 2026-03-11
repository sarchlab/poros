// SPH kernel: compute densities and forces for particles
// Simplified Poly6 kernel for density, Spiky gradient for pressure force
void sph_compute(
    double *px, double *py, double *pz,     // particle positions
    double *vx, double *vy, double *vz,     // particle velocities
    double *density, double *pressure,       // output arrays
    double *fx, double *fy, double *fz,     // force output
    int n,                                   // number of particles
    double h,                                // smoothing radius
    double mass,                             // particle mass
    double rho0,                             // rest density
    double k,                                // gas constant
    double mu                                // viscosity coefficient
) {
    double h2 = h * h;
    double poly6_coeff = 315.0 / (64.0 * 3.141592653589793 * h * h * h * h * h * h * h * h * h);
    double spiky_coeff = -45.0 / (3.141592653589793 * h * h * h * h * h * h);
    double visc_coeff = 45.0 / (3.141592653589793 * h * h * h * h * h * h);

    // Phase 1: Compute densities
    for (int i = 0; i < n; i++) {
        double rho = 0.0;
        for (int j = 0; j < n; j++) {
            double dx = px[i] - px[j];
            double dy = py[i] - py[j];
            double dz = pz[i] - pz[j];
            double r2 = dx*dx + dy*dy + dz*dz;
            if (r2 < h2) {
                double diff = h2 - r2;
                rho += mass * poly6_coeff * diff * diff * diff;
            }
        }
        density[i] = rho;
        pressure[i] = k * (rho - rho0);
    }

    // Phase 2: Compute forces
    for (int i = 0; i < n; i++) {
        double fxi = 0.0, fyi = 0.0, fzi = 0.0;
        for (int j = 0; j < n; j++) {
            if (i == j) continue;
            double dx = px[i] - px[j];
            double dy = py[i] - py[j];
            double dz = pz[i] - pz[j];
            double r2 = dx*dx + dy*dy + dz*dz;
            if (r2 < h2 && r2 > 1e-12) {
                double r = __builtin_sqrt(r2);
                double diff = h - r;
                // Pressure force (Spiky gradient kernel)
                double pterm = -mass * (pressure[i] + pressure[j]) / (2.0 * density[j]);
                double fp = spiky_coeff * diff * diff * pterm / r;
                fxi += fp * dx;
                fyi += fp * dy;
                fzi += fp * dz;
                // Viscosity force
                double vterm = mu * mass / density[j];
                double fv = visc_coeff * diff * vterm;
                fxi += fv * (vx[j] - vx[i]);
                fyi += fv * (vy[j] - vy[i]);
                fzi += fv * (vz[j] - vz[i]);
            }
        }
        fx[i] = fxi;
        fy[i] = fyi;
        fz[i] = fzi;
    }
}
