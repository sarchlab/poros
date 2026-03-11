#include <math.h>

// Möller-Trumbore ray-triangle intersection.
// Returns t (distance along ray) or -1 if no hit.
static inline double ray_triangle_intersect(
    double ox, double oy, double oz,    // ray origin
    double dx, double dy, double dz,    // ray direction
    double v0x, double v0y, double v0z, // vertex 0
    double v1x, double v1y, double v1z, // vertex 1
    double v2x, double v2y, double v2z  // vertex 2
) {
    double e1x = v1x - v0x, e1y = v1y - v0y, e1z = v1z - v0z;
    double e2x = v2x - v0x, e2y = v2y - v0y, e2z = v2z - v0z;

    // h = d × e2
    double hx = dy * e2z - dz * e2y;
    double hy = dz * e2x - dx * e2z;
    double hz = dx * e2y - dy * e2x;

    double a = e1x * hx + e1y * hy + e1z * hz;
    if (a > -1e-10 && a < 1e-10) return -1.0;  // FCCMP generated here

    double f = 1.0 / a;
    double sx = ox - v0x, sy = oy - v0y, sz = oz - v0z;
    double u = f * (sx * hx + sy * hy + sz * hz);
    if (u < 0.0 || u > 1.0) return -1.0;       // FCCMP generated here

    // q = s × e1
    double qx = sy * e1z - sz * e1y;
    double qy = sz * e1x - sx * e1z;
    double qz = sx * e1y - sy * e1x;

    double v = f * (dx * qx + dy * qy + dz * qz);
    if (v < 0.0 || u + v > 1.0) return -1.0;   // FCCMP generated here

    double t = f * (e2x * qx + e2y * qy + e2z * qz);
    if (t > 1e-10) return t;
    return -1.0;
}

// Ray-AABB intersection (slab method) using fmax/fmin for FMAXNM/FMINNM.
static inline int ray_aabb_intersect(
    double ox, double oy, double oz,
    double idx, double idy, double idz,  // 1/direction (precomputed)
    double bmin_x, double bmin_y, double bmin_z,
    double bmax_x, double bmax_y, double bmax_z,
    double tmax_limit
) {
    double t1x = (bmin_x - ox) * idx;
    double t2x = (bmax_x - ox) * idx;
    double t1y = (bmin_y - oy) * idy;
    double t2y = (bmax_y - oy) * idy;
    double t1z = (bmin_z - oz) * idz;
    double t2z = (bmax_z - oz) * idz;

    // Use fmin/fmax to guarantee FMINNM/FMAXNM generation
    double tmin_x = fmin(t1x, t2x);
    double tmax_x = fmax(t1x, t2x);
    double tmin_y = fmin(t1y, t2y);
    double tmax_y = fmax(t1y, t2y);
    double tmin_z = fmin(t1z, t2z);
    double tmax_z = fmax(t1z, t2z);

    double tmin = fmax(fmax(tmin_x, tmin_y), tmin_z);
    double tmax = fmin(fmin(tmax_x, tmax_y), tmax_z);

    return tmax >= tmin && tmax > 0.0 && tmin < tmax_limit;
}

// BVH traversal: trace a ray through BVH, find closest hit triangle.
// Returns closest t, or -1 if no hit.
//
// Memory layout:
//   bbox:      double[num_nodes * 6] — [minx,miny,minz,maxx,maxy,maxz] per node
//   children:  int[num_nodes * 2]    — [left, right] per node (-1 if leaf)
//   tri_verts: double[num_tris * 9]  — [v0x,v0y,v0z,v1x,v1y,v1z,v2x,v2y,v2z]
//   tri_idx:   int[num_nodes]        — triangle index for leaf nodes
//
// Calling convention:
//   X0 = ray_data (double*: ox,oy,oz,dx,dy,dz)
//   X1 = bbox (double*)
//   X2 = children (int*)
//   X3 = tri_verts (double*)
//   X4 = tri_idx (int*)
//   W5 = num_nodes
double bvh_trace(
    double *ray,         // [ox, oy, oz, dx, dy, dz]
    double *bbox,        // [minx, miny, minz, maxx, maxy, maxz] * num_nodes
    int *children,       // [left, right] * num_nodes
    double *tri_verts,   // [v0x..v2z] * num_tris (9 doubles each)
    int *tri_idx,        // triangle index per node
    int num_nodes
) {
    double ox = ray[0], oy = ray[1], oz = ray[2];
    double dx = ray[3], dy = ray[4], dz = ray[5];
    double idx = 1.0 / dx, idy = 1.0 / dy, idz = 1.0 / dz;

    double closest_t = 1e20;

    // Stack-based traversal
    int stack[64];
    int sp = 0;
    stack[sp++] = 0; // root node

    while (sp > 0) {
        int node = stack[--sp];
        if (node < 0 || node >= num_nodes) continue;

        // Test ray-AABB
        int bi = node * 6;
        if (!ray_aabb_intersect(ox, oy, oz, idx, idy, idz,
                bbox[bi], bbox[bi+1], bbox[bi+2],
                bbox[bi+3], bbox[bi+4], bbox[bi+5],
                closest_t)) {
            continue;
        }

        int left = children[node * 2];
        int right = children[node * 2 + 1];

        if (left == -1 && right == -1) {
            // Leaf node — test triangle
            int ti = tri_idx[node];
            if (ti >= 0) {
                int vi = ti * 9;
                double t = ray_triangle_intersect(
                    ox, oy, oz, dx, dy, dz,
                    tri_verts[vi], tri_verts[vi+1], tri_verts[vi+2],
                    tri_verts[vi+3], tri_verts[vi+4], tri_verts[vi+5],
                    tri_verts[vi+6], tri_verts[vi+7], tri_verts[vi+8]
                );
                if (t > 0.0 && t < closest_t) {   // FCCMP generated here
                    closest_t = t;
                }
            }
        } else {
            // Internal node — push children
            if (right >= 0) stack[sp++] = right;
            if (left >= 0) stack[sp++] = left;
        }
    }

    return closest_t >= 1e20 ? -1.0 : closest_t;
}
