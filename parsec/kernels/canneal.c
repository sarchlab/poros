// Canneal: routing cost computation for simulated annealing chip placement
// Computes total Manhattan distance cost of a netlist routing.
//
// ARM64 ABI:
//   X0 = netlist pointer (int32[n * 2], pairs of location indices)
//   X1 = locations pointer (int32[m * 2], x,y coordinates)
//   W2 = n (number of net pairs)
//   Returns total_cost in W0
int routing_cost(
    int *netlist,
    int *locations,
    int n
) {
    int total_cost = 0;
    for (int i = 0; i < n; i++) {
        int a = netlist[i * 2];
        int b = netlist[i * 2 + 1];

        int ax = locations[a * 2];
        int ay = locations[a * 2 + 1];
        int bx = locations[b * 2];
        int by = locations[b * 2 + 1];

        int dx = ax - bx;
        if (dx < 0) dx = -dx;
        int dy = ay - by;
        if (dy < 0) dy = -dy;

        total_cost += dx + dy;
    }
    return total_cost;
}
