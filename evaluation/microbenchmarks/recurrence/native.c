// Native collector for the recurrence probes on macOS and Linux. One process
// runs a predeclared, block-balanced schedule and prints every interval after
// the last timer stop.
//   native run INDEX ITERATIONS                        untimed functional output
//   native collect BLOCKS TARGET_NS DEADLINE_S [CPU]   timed capture
//   native dry BLOCKS TARGET_NS DEADLINE_S             same schedule, synthetic clock
// CPU pins the Linux thread; macOS requests user-interactive QoS instead.
// See README.md for the protocol and scripts/recurrence_microbenchmark.py for
// the independent schedule and output checks.
#define _GNU_SOURCE
#include "driver.h"
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <sys/resource.h>
#include <time.h>
#include <unistd.h>
#ifdef __APPLE__
#include <libproc.h>
#include <pthread.h>
#include <pthread/qos.h>
#define WALL_CLOCK CLOCK_UPTIME_RAW
#else
#include <linux/perf_event.h>
#include <sched.h>
#include <sys/syscall.h>
#define WALL_CLOCK CLOCK_MONOTONIC_RAW
#endif

#define PILOT_INSTRUCTIONS 16000000UL
#define MAX_BLOCKS 32
#define MAX_INTERVAL_NS 500000000ULL
#define MAX_RECORDS (4 * NUM_PROBES + 4 * (NUM_PROBES + 1) * MAX_BLOCKS)

static const unsigned long instructions[] = {
#define PROBE(name, count, updates, kind) count,
#include "cases.def"
#undef PROBE
};

// Hardware counters for the measured thread. macOS reads the kernel's
// per-process accounting (one thread); Linux reads a user-mode perf group.
struct counters {
    uint64_t cycles, instructions, performance_cycles, performance_instructions;
    long involuntary, voluntary;
};
struct record {
    char phase;
    int block, slot, probe;
    unsigned long iterations, output;
    uint64_t cpu_ns, wall_ns, start_ns;
    long cpu_before, cpu_after;
    struct counters used;
};
static struct record records[MAX_RECORDS];
static unsigned long count;
static uint64_t origin;
static long frequency[MAX_BLOCKS][2];
static long pinned = -1;
// A dry run reads no clock and no counter: synthetic time advances with the
// declared instruction count, so its output is identical on every host.
static int synthetic;
static uint64_t synthetic_ns, synthetic_instructions;
static const char *counter_source = "none";
#ifndef __APPLE__
static int perf = -1;
// Equal enabled and running times show the group was never multiplexed.
static uint64_t perf_enabled_ns, perf_running_ns;
#endif

static uint64_t read_clock(clockid_t id) {
    struct timespec t;
    if (synthetic) return synthetic_ns;
    clock_gettime(id, &t);
    return (uint64_t)t.tv_sec * 1000000000ULL + (uint64_t)t.tv_nsec;
}

static long cpu_number(void) {
    if (synthetic) return -1;
#ifdef __APPLE__
    size_t cpu = 0;
    return pthread_cpu_number_np(&cpu) ? -1 : (long)cpu;
#else
    return sched_getcpu();
#endif
}

static void open_counters(void) {
#ifdef __APPLE__
    counter_source = "proc_pid_rusage";
#else
    struct perf_event_attr attr;
    memset(&attr, 0, sizeof attr);
    attr.type = PERF_TYPE_HARDWARE, attr.size = sizeof attr, attr.config = PERF_COUNT_HW_CPU_CYCLES;
    attr.exclude_kernel = 1, attr.exclude_hv = 1;
    attr.read_format = PERF_FORMAT_GROUP | PERF_FORMAT_TOTAL_TIME_ENABLED | PERF_FORMAT_TOTAL_TIME_RUNNING;
    perf = (int)syscall(SYS_perf_event_open, &attr, 0, -1, -1, 0);
    if (perf < 0) return;
    attr.config = PERF_COUNT_HW_INSTRUCTIONS;
    if (syscall(SYS_perf_event_open, &attr, 0, -1, perf, 0) < 0) {
        close(perf);
        perf = -1;
        return;
    }
    counter_source = "perf_event_user";
#endif
}

static int read_counters(struct counters *c) {
    struct rusage usage;
    memset(c, 0, sizeof *c);
    if (synthetic) {
        c->cycles = c->performance_cycles = synthetic_ns;
        c->instructions = c->performance_instructions = synthetic_instructions;
        return 0;
    }
#ifdef __APPLE__
    struct rusage_info_v6 info;
    if (proc_pid_rusage(getpid(), RUSAGE_INFO_V6, (rusage_info_t *)&info) || getrusage(RUSAGE_SELF, &usage)) return -1;
    c->cycles = info.ri_cycles, c->instructions = info.ri_instructions;
    c->performance_cycles = info.ri_pcycles, c->performance_instructions = info.ri_pinstructions;
#else
    uint64_t group[5];
    if (getrusage(RUSAGE_THREAD, &usage)) return -1;
    if (perf >= 0) {
        if (read(perf, group, sizeof group) != (ssize_t)sizeof group || group[0] != 2) return -1;
        perf_enabled_ns = group[1], perf_running_ns = group[2];
        c->cycles = group[3], c->instructions = group[4];
    }
#endif
    c->involuntary = usage.ru_nivcsw, c->voluntary = usage.ru_nvcsw;
    return 0;
}

// The first whitespace-delimited token of a small system file, or "unknown".
static const char *system_value(const char *format, long cpu) {
    static char path[160], value[96];
    FILE *file;
    snprintf(path, sizeof path, format, cpu);
    strcpy(value, "unknown");
    if ((file = fopen(path, "r"))) {
        if (fscanf(file, "%95s", value) != 1) strcpy(value, "unknown");
        fclose(file);
    }
    return value;
}

static long current_frequency(void) {
    long khz = -1;
    if (!synthetic && pinned >= 0)
        sscanf(system_value("/sys/devices/system/cpu/cpu%ld/cpufreq/scaling_cur_freq", pinned), "%ld", &khz);
    return khz;
}

// Initialization, accounting reads and bookkeeping stay outside both timers.
static struct record *interval(char phase, int block, int slot, int probe, unsigned long iterations) {
    struct record *r = &records[count++];
    struct counters before, after;
    initialize();
    r->phase = phase, r->block = block, r->slot = slot, r->probe = probe, r->iterations = iterations;
    if (read_counters(&before)) return 0;
    r->cpu_before = cpu_number();
    uint64_t wall = read_clock(WALL_CLOCK);
    uint64_t cpu = read_clock(CLOCK_THREAD_CPUTIME_ID);
    r->output = probes[probe](iterations, data.cells);
    synthetic_ns += iterations * instructions[probe] / 4 + 1;
    synthetic_instructions += iterations * instructions[probe];
    r->cpu_ns = read_clock(CLOCK_THREAD_CPUTIME_ID) - cpu;
    r->wall_ns = read_clock(WALL_CLOCK) - wall;
    r->cpu_after = cpu_number();
    if (read_counters(&after)) return 0;
    r->start_ns = wall - origin;
    r->used.cycles = after.cycles - before.cycles;
    r->used.instructions = after.instructions - before.instructions;
    r->used.performance_cycles = after.performance_cycles - before.performance_cycles;
    r->used.performance_instructions = after.performance_instructions - before.performance_instructions;
    r->used.involuntary = after.involuntary - before.involuntary;
    r->used.voluntary = after.voluntary - before.voluntary;
    return r;
}

static unsigned long gcd(unsigned long a, unsigned long b) {
    while (b) { unsigned long t = a % b; a = b; b = t; }
    return a;
}

// Blocks 2j and 2j+1 visit the non-clock probes in mirrored order. Pair j
// starts 5j probes later and steps by the (j+1)-th stride coprime with the
// probe count, so neighbours differ between pairs. Probe 0 is the clock.
static void block_order(int block, int *order) {
    unsigned long others = NUM_PROBES - 1, pair = (unsigned long)block / 2, stride = 0;
    for (unsigned long found = 0; found <= pair;)
        if (gcd(++stride, others) == 1) found++;
    for (unsigned long k = 0; k < others; k++) {
        unsigned long position = block % 2 ? others - 1 - k : k;
        order[position] = 1 + (int)((5 * pair + k * stride) % others);
    }
}

static void print_environment(void) {
#ifdef __APPLE__
    printf("E os darwin\n");
#else
    printf("E os linux\n");
    if (pinned >= 0) {
        printf("E midr %s\n", system_value("/sys/devices/system/cpu/cpu%ld/regs/identification/midr_el1", pinned));
        printf("E capacity %s\n", system_value("/sys/devices/system/cpu/cpu%ld/cpu_capacity", pinned));
        printf("E governor %s\n", system_value("/sys/devices/system/cpu/cpu%ld/cpufreq/scaling_governor", pinned));
        printf("E maximum_khz %s\n", system_value("/sys/devices/system/cpu/cpu%ld/cpufreq/cpuinfo_max_freq", pinned));
    }
    printf("E load_average %s\n", system_value("/proc/loadavg", 0));
    if (perf >= 0)
        printf("E counter_enabled_ns %llu\nE counter_running_ns %llu\n", (unsigned long long)perf_enabled_ns,
               (unsigned long long)perf_running_ns);
#endif
    printf("E counters %s\nE pinned_cpu %ld\n", counter_source, pinned);
}

static int collect(unsigned long blocks, unsigned long target_ns, unsigned long deadline_s) {
    static unsigned long pilot[NUM_PROBES], chosen[NUM_PROBES];
    static int order[NUM_PROBES];
    struct timespec utc = {0, 0};
    const char *stop = "complete";
    unsigned long completed = 0, limited = 0;
    if (!blocks || blocks > MAX_BLOCKS || blocks % 2 || !target_ns || target_ns > 100000000 || !deadline_s || deadline_s > 180)
        return 2;
    if (!synthetic && (target_ns < 1000000 || clock_gettime(CLOCK_REALTIME, &utc))) return 2;
    if (!synthetic) open_counters();
    origin = read_clock(WALL_CLOCK);
    for (unsigned long i = 0; i < NUM_PROBES; i++) {
        pilot[i] = synthetic ? 2 : (PILOT_INSTRUCTIONS + instructions[i] - 1) / instructions[i];
        if (!interval('W', -1, 0, (int)i, pilot[i])) return 4;
    }
    // Duration-only pilot: 1:2:4 trip counts, fastest observed rate selects N.
    for (unsigned long i = 0; i < NUM_PROBES; i++) {
        double fastest = 0;
        for (int step = 0; step < 3; step++) {
            struct record *r = interval('P', -1, step, (int)i, pilot[i] << step);
            if (!r) return 4;
            double rate = (double)r->cpu_ns / (double)r->iterations;
            if (!step || rate < fastest) fastest = rate;
        }
        if (fastest <= 0) return 4;
        chosen[i] = ((unsigned long)((double)target_ns / fastest) / 8 + 1) * 8;
        if (2 * chosen[i] > MAX_ITERATIONS) chosen[i] = MAX_ITERATIONS / 2, limited++;
    }
    for (unsigned long b = 0; b < blocks && stop[0] == 'c'; b++) {
        // A block is started only when one more of the estimated size fits.
        if (read_clock(WALL_CLOCK) - origin + 7 * (uint64_t)target_ns * (NUM_PROBES + 1) > (uint64_t)deadline_s * 1000000000ULL) {
            stop = "deadline";
            break;
        }
        block_order((int)b, order);
        frequency[b][0] = current_frequency();
        for (unsigned long slot = 0; slot <= NUM_PROBES && stop[0] == 'c'; slot++) {
            int probe = slot == 0 || slot == NUM_PROBES ? 0 : order[slot - 1];
            for (int step = 0; step < 4 && stop[0] == 'c'; step++) {
                // ABBA on even blocks, BAAB on odd blocks (A = N, B = 2N).
                int twice = (step == 1 || step == 2) ^ (int)(b % 2);
                struct record *r = interval('M', (int)b, (int)slot, probe, chosen[probe] << twice);
                if (!r) return 4;
                if (r->wall_ns > MAX_INTERVAL_NS) stop = "interval-limit";
            }
        }
        frequency[b][1] = current_frequency();
        if (stop[0] == 'c') completed++;
    }
    printf("H protocol recurrence-native-v1\nH clock %s\n", synthetic ? "synthetic" : "measured");
    printf("H probes %lu\nH blocks_requested %lu\nH blocks_completed %lu\n", (unsigned long)NUM_PROBES, blocks, completed);
    printf("H target_ns %lu\nH deadline_s %lu\nH pilot_instructions %lu\n", target_ns, deadline_s, PILOT_INSTRUCTIONS);
    printf("H stop %s\nH iteration_limited %lu\n", stop, limited);
    printf("H start_realtime_ns %lld\n", (long long)utc.tv_sec * 1000000000LL + utc.tv_nsec);
    printf("H elapsed_ns %llu\n", (unsigned long long)(read_clock(WALL_CLOCK) - origin));
    if (!synthetic) {
        print_environment();
        for (unsigned long b = 0; b < completed; b++) printf("F %lu %ld %ld\n", b, frequency[b][0], frequency[b][1]);
    }
    for (unsigned long i = 0; i < NUM_PROBES; i++) printf("N %lu %lu %lu\n", i, pilot[i], chosen[i]);
    for (unsigned long i = 0; i < count; i++) {
        struct record *r = &records[i];
        printf("R %c %d %d %d %lu %llu %llu %llu %lu %ld %ld %ld %ld %llu %llu %llu %llu\n", r->phase, r->block, r->slot,
               r->probe, r->iterations, (unsigned long long)r->cpu_ns, (unsigned long long)r->wall_ns,
               (unsigned long long)r->start_ns, r->output, r->cpu_before, r->cpu_after, r->used.involuntary, r->used.voluntary,
               (unsigned long long)r->used.cycles, (unsigned long long)r->used.instructions,
               (unsigned long long)r->used.performance_cycles, (unsigned long long)r->used.performance_instructions);
    }
    return stop[0] == 'c' ? 0 : 5;
}

// Campaign parameters have their own range; parse() bounds trip counts only.
static unsigned long parameter(const char *s) {
    unsigned long n = 0;
    if (!*s) return 0;
    while (*s) {
        if (*s < '0' || *s > '9' || n > 100000000) return 0;
        n = n * 10 + *s++ - '0';
    }
    return n;
}

int main(int argc, char **argv) {
    if (argc == 4 && argv[1][0] == 'r') {
        unsigned long index = parse(argv[2]), iterations = parse(argv[3]);
        if (!index || index > NUM_PROBES || !iterations) return 2;
        initialize();
        printf("%lu\n", probes[index - 1](iterations, data.cells));
        return 0;
    }
    synthetic = argc == 5 && argv[1][0] == 'd';
    if (!synthetic && !((argc == 5 || argc == 6) && argv[1][0] == 'c')) return 2;
#ifdef __APPLE__
    if (argc == 6 || (!synthetic && pthread_set_qos_class_self_np(QOS_CLASS_USER_INTERACTIVE, 0))) return 3;
#else
    if (argc == 6) {
        cpu_set_t set;
        // CPU 0 is spelled "0"; parameter() cannot distinguish it from an error.
        pinned = strcmp(argv[5], "0") ? (long)parameter(argv[5]) : 0;
        if ((pinned == 0 && strcmp(argv[5], "0")) || pinned >= CPU_SETSIZE) return 2;
        CPU_ZERO(&set);
        CPU_SET((int)pinned, &set);
        if (sched_setaffinity(0, sizeof set, &set)) return 3;
    }
#endif
    return collect(parameter(argv[2]), parameter(argv[3]), parameter(argv[4]));
}
