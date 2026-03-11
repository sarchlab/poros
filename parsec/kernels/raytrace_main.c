#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <math.h>

// Prototype for the BVH trace function (defined in raytrace.c)
double bvh_trace(
    double *ray,
    double *bbox,
    int *children,
    double *tri_verts,
    int *tri_idx,
    int num_nodes
);

int main(void) {
    int num_nodes = 7;
    int num_tris = 4;

    // 4 triangles forming a ground plane at z=0
    double tri_verts[] = {
        -2, -2, 0,  0, -2, 0,  0,  0, 0,   // tri 0
        -2, -2, 0,  0,  0, 0, -2,  0, 0,   // tri 1
         0,  0, 0,  2,  0, 0,  2,  2, 0,   // tri 2
         0,  0, 0,  2,  2, 0,  0,  2, 0,   // tri 3
    };

    double bbox[] = {
        -2, -2, -0.1,  2,  2, 0.1,   // node 0 (root)
        -2, -2, -0.1,  0,  0, 0.1,   // node 1
         0,  0, -0.1,  2,  2, 0.1,   // node 2
        -2, -2, -0.1,  0,  0, 0.1,   // node 3 (leaf)
        -2, -2, -0.1,  0,  0, 0.1,   // node 4 (leaf)
         0,  0, -0.1,  2,  2, 0.1,   // node 5 (leaf)
         0,  0, -0.1,  2,  2, 0.1,   // node 6 (leaf)
    };

    int children[] = {
        1, 2,   // node 0
        3, 4,   // node 1
        5, 6,   // node 2
       -1,-1,   // node 3
       -1,-1,   // node 4
       -1,-1,   // node 5
       -1,-1,   // node 6
    };

    int tri_idx[] = {-1, -1, -1, 0, 1, 2, 3};

    double ray[] = {-1, -1, 5, 0, 0, -1};

    (void)num_tris;

    // Warm up
    volatile double result = 0;
    for (int i = 0; i < 1000; i++) {
        result = bvh_trace(ray, bbox, children, tri_verts, tri_idx, num_nodes);
    }

    // Timed run
    int N = 1000000;
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < N; i++) {
        result = bvh_trace(ray, bbox, children, tri_verts, tri_idx, num_nodes);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) * 1e-9;
    double per_call = elapsed / N;

    printf("result = %f\n", result);
    printf("Total: %.6f s for %d calls\n", elapsed, N);
    printf("Per call: %.9f s (%.3f us)\n", per_call, per_call * 1e6);

    return 0;
}
