#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

// Forward declaration — defined in x264.c
void x264_motion_search(
    const unsigned char *cur,
    const unsigned char *ref,
    int stride,
    int range,
    int *best_cost,
    int *best_mx,
    int *best_my
);

int main(void) {
    int frameW = 128;
    int searchRange = 8;
    int curOffY = 48, curOffX = 48;
    int curOff = curOffY * frameW + curOffX;
    int frameSize = frameW * frameW;

    unsigned char *curFrame = (unsigned char *)malloc(frameSize);
    unsigned char *refFrame = (unsigned char *)malloc(frameSize);

    // Fill with pseudo-random data
    for (int i = 0; i < frameSize; i++) {
        curFrame[i] = (unsigned char)((i * 73 + 17) % 256);
    }

    // Copy cur to ref with a shift of (3, -2) and small noise
    memcpy(refFrame, curFrame, frameSize);
    int shiftMx = 3, shiftMy = -2;
    for (int y = 0; y < 16; y++) {
        for (int x = 0; x < 16; x++) {
            int srcIdx = (curOffY + y) * frameW + (curOffX + x);
            int dstIdx = (curOffY + shiftMy + y) * frameW + (curOffX + shiftMx + x);
            unsigned char noise = (unsigned char)((y * 3 + x * 7) % 5);
            refFrame[dstIdx] = curFrame[srcIdx] + noise;
        }
    }

    int best_cost, best_mx, best_my;

    // Warm-up
    x264_motion_search(curFrame + curOff, refFrame + curOff,
                       frameW, searchRange,
                       &best_cost, &best_mx, &best_my);

    // Timing
    int nruns = 501;
    double *times = (double *)malloc(nruns * sizeof(double));
    for (int r = 0; r < nruns; r++) {
        struct timespec t0, t1;
        clock_gettime(CLOCK_MONOTONIC, &t0);
        x264_motion_search(curFrame + curOff, refFrame + curOff,
                           frameW, searchRange,
                           &best_cost, &best_mx, &best_my);
        clock_gettime(CLOCK_MONOTONIC, &t1);
        times[r] = (t1.tv_sec - t0.tv_sec) + (t1.tv_nsec - t0.tv_nsec) * 1e-9;
    }

    // Sort for median
    for (int i = 0; i < nruns - 1; i++) {
        for (int j = i + 1; j < nruns; j++) {
            if (times[j] < times[i]) {
                double tmp = times[i];
                times[i] = times[j];
                times[j] = tmp;
            }
        }
    }

    double median = times[nruns / 2];
    printf("x264 motion search: best_cost=%d, mx=%d, my=%d\n",
           best_cost, best_mx, best_my);
    printf("Median time (%d runs): %.6f us (%.9f s)\n",
           nruns, median * 1e6, median);

    free(curFrame);
    free(refFrame);
    free(times);
    return 0;
}
