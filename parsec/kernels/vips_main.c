#include <stdio.h>
#include <stdlib.h>
#include <mach/mach_time.h>

void convolve2d(
    double *output,
    double *input,
    double *kernel,
    int width,
    int height,
    int ksize
) {
    int half = ksize / 2;
    for (int y = half; y < height - half; y++) {
        for (int x = half; x < width - half; x++) {
            double sum = 0.0;
            for (int ky = -half; ky <= half; ky++) {
                for (int kx = -half; kx <= half; kx++) {
                    int iy = y + ky;
                    int ix = x + kx;
                    sum += input[iy * width + ix] * kernel[(ky + half) * ksize + (kx + half)];
                }
            }
            output[y * width + x] = sum;
        }
    }
}

int main() {
    int W = 64, H = 64, K = 3;
    double *input = (double *)calloc(W * H, sizeof(double));
    double *output = (double *)calloc(W * H, sizeof(double));
    double kern[9] = {1.0/16, 2.0/16, 1.0/16, 2.0/16, 4.0/16, 2.0/16, 1.0/16, 2.0/16, 1.0/16};

    for (int i = 0; i < W * H; i++) {
        input[i] = (double)(i % 256) / 255.0;
    }

    mach_timebase_info_data_t info;
    mach_timebase_info(&info);

    for (int trial = 0; trial < 7; trial++) {
        uint64_t start = mach_absolute_time();
        convolve2d(output, input, kern, W, H, K);
        uint64_t end = mach_absolute_time();
        double ns = (double)(end - start) * info.numer / info.denom;
        printf("Trial %d: %.0f ns\n", trial, ns);
    }

    // Print a sample output to prevent dead code elimination
    printf("output[100] = %f\n", output[100]);

    free(input);
    free(output);
    return 0;
}
