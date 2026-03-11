#include <math.h>

// Cumulative Normal Distribution Function
static double CNDF(double x) {
    // Use a Horner-form rational approximation (Abramowitz & Stegun)
    double L, K, w;
    double a1 = 0.31938153, a2 = -0.356563782, a3 = 1.781477937;
    double a4 = -1.821255978, a5 = 1.330274429;
    
    L = fabs(x);
    K = 1.0 / (1.0 + 0.2316419 * L);
    w = 1.0 - 1.0 / sqrt(2.0 * 3.141592653589793) * 
        exp(-L * L / 2.0) * 
        (a1*K + a2*K*K + a3*K*K*K + a4*K*K*K*K + a5*K*K*K*K*K);
    if (x < 0.0) w = 1.0 - w;
    return w;
}

// Black-Scholes European option pricing
void BlkSchlsEqEuroNoDiv(
    double *prices,    // output: option prices
    double *S,         // stock prices
    double *X,         // strike prices  
    double *r,         // risk-free rate
    double *v,         // volatility
    double *T,         // time to expiration
    int *otype,        // option type (0=call, 1=put)
    int numOptions
) {
    for (int i = 0; i < numOptions; i++) {
        double d1, d2;
        double sqrtT = sqrt(T[i]);
        d1 = (log(S[i]/X[i]) + (r[i] + v[i]*v[i]*0.5) * T[i]) / (v[i] * sqrtT);
        d2 = d1 - v[i] * sqrtT;
        
        double NofXd1 = CNDF(d1);
        double NofXd2 = CNDF(d2);
        
        if (otype[i] == 0) {
            prices[i] = S[i] * NofXd1 - X[i] * exp(-r[i]*T[i]) * NofXd2;
        } else {
            prices[i] = X[i] * exp(-r[i]*T[i]) * (1.0-NofXd2) - S[i] * (1.0-NofXd1);
        }
    }
}
