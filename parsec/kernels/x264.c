#include <stdlib.h>

// x264-style SAD (Sum of Absolute Differences) for 16x16 macroblock
static int sad_16x16(const unsigned char *cur, const unsigned char *ref, int stride) {
    int sad = 0;
    for (int y = 0; y < 16; y++) {
        for (int x = 0; x < 16; x++) {
            int diff = cur[y * stride + x] - ref[y * stride + x];
            sad += diff < 0 ? -diff : diff;
        }
    }
    return sad;
}

// 4x4 Hadamard transform for SATD
static void hadamard4x4(const short *src, short *dst) {
    short tmp[16];
    // Horizontal transform
    for (int i = 0; i < 4; i++) {
        int s0 = src[i*4+0], s1 = src[i*4+1], s2 = src[i*4+2], s3 = src[i*4+3];
        int d0 = s0 + s1, d1 = s0 - s1, d2 = s2 + s3, d3 = s2 - s3;
        tmp[i*4+0] = d0 + d2;
        tmp[i*4+1] = d1 + d3;
        tmp[i*4+2] = d0 - d2;
        tmp[i*4+3] = d1 - d3;
    }
    // Vertical transform
    for (int i = 0; i < 4; i++) {
        int s0 = tmp[0*4+i], s1 = tmp[1*4+i], s2 = tmp[2*4+i], s3 = tmp[3*4+i];
        int d0 = s0 + s1, d1 = s0 - s1, d2 = s2 + s3, d3 = s2 - s3;
        dst[0*4+i] = d0 + d2;
        dst[1*4+i] = d1 + d3;
        dst[2*4+i] = d0 - d2;
        dst[3*4+i] = d1 - d3;
    }
}

// SATD for 4x4 block
static int satd_4x4(const unsigned char *cur, const unsigned char *ref, int stride) {
    short diff[16], hada[16];
    for (int y = 0; y < 4; y++) {
        for (int x = 0; x < 4; x++) {
            diff[y*4+x] = (short)(cur[y*stride+x] - ref[y*stride+x]);
        }
    }
    hadamard4x4(diff, hada);
    int satd = 0;
    for (int i = 0; i < 16; i++) {
        satd += hada[i] < 0 ? -hada[i] : hada[i];
    }
    return (satd + 1) >> 1;
}

// Full motion estimation kernel:
// For a 16x16 current macroblock, search within [-range, range]
// in the reference frame for the best matching block using SAD.
// Then refine with SATD on 4x4 sub-blocks.
//
// ARM64 ABI:
//   X0 = cur (unsigned char*) - current macroblock start
//   X1 = ref (unsigned char*) - reference frame start  
//   W2 = stride - width of frame in bytes
//   W3 = range - search range
//   X4 = best_cost output (int*)
//   X5 = best_mx output (int*) - motion vector x
//   X6 = best_my output (int*) - motion vector y
void x264_motion_search(
    const unsigned char *cur,
    const unsigned char *ref,
    int stride,
    int range,
    int *best_cost,
    int *best_mx,
    int *best_my
) {
    int best = 0x7FFFFFFF;
    int bmx = 0, bmy = 0;
    
    for (int my = -range; my <= range; my++) {
        for (int mx = -range; mx <= range; mx++) {
            const unsigned char *refBlock = ref + my * stride + mx;
            int cost = sad_16x16(cur, refBlock, stride);
            if (cost < best) {
                best = cost;
                bmx = mx;
                bmy = my;
            }
        }
    }
    
    // Refine with SATD on 4x4 sub-blocks
    const unsigned char *bestRef = ref + bmy * stride + bmx;
    int totalSATD = 0;
    for (int by = 0; by < 16; by += 4) {
        for (int bx = 0; bx < 16; bx += 4) {
            totalSATD += satd_4x4(cur + by * stride + bx,
                                   bestRef + by * stride + bx,
                                   stride);
        }
    }
    
    *best_cost = totalSATD;
    *best_mx = bmx;
    *best_my = bmy;
}
