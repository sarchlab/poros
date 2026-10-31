#include "../../parsec/kernels/canneal.c"
#include "runtime.h"

static int netlist[16384];
static int locations[1024];
static int cost[1];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int n;
  if (!argument(argv[1], 1, 8192, &n))
    return 2;
  int m;
  if (!argument(argv[2], 2, 512, &m))
    return 2;
  for (int i = 0; i < n * 2; i++)
    netlist[i] = i % m;
  for (int i = 0; i < m; i++) {
    locations[2 * i] = (i * 37) % 1000;
    locations[2 * i + 1] = (i * 73) % 1000;
  }
  cost[0] = routing_cost(netlist, locations, n);
  if (output(cost, (1) * sizeof(int)))
    return 3;
  return 0;
}
