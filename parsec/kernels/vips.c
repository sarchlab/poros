// 2D convolution for image processing (VIPS-style)
// Applies a KxK convolution kernel to a WxH image.
//
// ARM64 ABI:
//   X0 = output (double*)
//   X1 = input (double*)
//   X2 = kernel (double*)
//   W3 = width
//   W4 = height
//   W5 = ksize (kernel size, must be odd)
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
