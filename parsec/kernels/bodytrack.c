// Bodytrack particle filter likelihood kernel.
//
// For each particle, compute likelihood = exp(-0.5 * sum((A*state - obs)^2))
// where A is the projection matrix, state is the particle state, and obs are
// the observations.
//
// ARM64 ABI:
//   X0 = likelihood output (double*)
//   X1 = states (double[nParticles * stateDim])
//   X2 = observations (double[obsDim])
//   X3 = projMatrix (double[obsDim * stateDim])
//   W4 = nParticles
//   W5 = stateDim
//   W6 = obsDim
//
// Note: calls exp() via BL - needs function hook in emulator.
#include <math.h>

void bodytrack_likelihood(
    double *likelihood,
    double *states,
    double *observations,
    double *projMatrix,
    int nParticles,
    int stateDim,
    int obsDim
) {
    for (int p = 0; p < nParticles; p++) {
        double sum = 0.0;
        double *state = &states[p * stateDim];

        for (int i = 0; i < obsDim; i++) {
            // Project: predicted[i] = sum_j(projMatrix[i*stateDim + j] * state[j])
            double predicted = 0.0;
            for (int j = 0; j < stateDim; j++) {
                predicted += projMatrix[i * stateDim + j] * state[j];
            }
            double diff = predicted - observations[i];
            sum += diff * diff;
        }

        // Likelihood = exp(-0.5 * sum)
        double arg = -0.5 * sum;
        likelihood[p] = exp(arg);  // This will generate BL _exp
    }
}
