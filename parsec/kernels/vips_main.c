#include <stdio.h>
#include <stdlib.h>
#include <time.h>

extern void convolve2d(
    double *output, double *input, double *kernel,
    int width, int height, int ksize);

#define W 64
#define H 64
#define K 3
#define NUM_ITERS 10000

int main() {
    double *input = (double *)calloc(W * H, sizeof(double));
    double *output = (double *)calloc(W * H, sizeof(double));
    double kern[9] = {1.0/16, 2.0/16, 1.0/16, 2.0/16, 4.0/16, 2.0/16, 1.0/16, 2.0/16, 1.0/16};

    for (int i = 0; i < W * H; i++) {
        input[i] = (double)(i % 256) / 255.0;
    }

    // Warmup
    for (int i = 0; i < 100; i++) {
        convolve2d(output, input, kern, W, H, K);
    }

    // Timed run
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < NUM_ITERS; i++) {
        convolve2d(output, input, kern, W, H, K);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) / 1e9;
    printf("Vips: %dx%d convolution (k=%d) x %d iterations\n", W, H, K, NUM_ITERS);
    printf("Total time: %.6f s\n", elapsed);
    printf("Per iteration: %.3f ns\n", elapsed / NUM_ITERS * 1e9);
    printf("output[100] = %.15f\n", output[100]);

    free(input);
    free(output);
    return 0;
}
