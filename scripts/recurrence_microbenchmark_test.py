#!/usr/bin/env python3
"""Reject incomplete, reordered or misattributed recurrence-probe evidence."""
import copy
import unittest

import recurrence_microbenchmark as suite


def emulate(case, iterations):
    """Step each recurrence literally; the closed forms must agree."""
    mask, trips = (1 << 32) - 1, 64 * iterations
    kind, name = case["kind"], case["id"]
    if kind == "count64":
        return trips + 1
    if kind == "half":
        return int(1.0 + 0.5 * trips)
    if kind in ("select_max", "select_low", "select_shift", "select_inc"):
        value, candidate, total = (1, 1, 0) if kind in ("select_max", "select_inc") else (0, 5000 if kind == "select_shift" else 100, 0)
        limit = {"select_low": 16 if "ri16" in name else 15, "select_shift": 4096}.get(kind)
        for _ in range(trips):
            total += value
            candidate = (candidate + 1) & mask
            if kind == "select_max":
                value = value if value >= candidate else candidate
            elif kind == "select_inc":
                value = value if value >= candidate else candidate + 1
            else:
                value = candidate if value < limit else 0
        return total + value
    if kind == "mix":
        source = iterations - 1
        return 4 * max(source, 0) + (source + 1) + (source + 2)
    if kind in ("max_const", "max_chg"):
        step, candidate, stored, total = (0, 1, 5, 0) if kind == "max_const" else (1, 5, 5, 0)
        for _ in range(trips):
            candidate += step
            stored = candidate if candidate >= stored else stored
            total += candidate
        return total + stored
    if kind == "sum64":
        return trips
    if kind in ("count128", "mid", "far"):
        return {"count128": 1, "mid": 2, "far": 3}[kind] * ((2 * trips if kind == "count128" else trips) + 1)
    if kind == "trips":
        return iterations
    if kind == "tight_max":
        return trips + 5
    reset, total = (4096, 0) if kind == "kernel_const" else (0, 0)
    for _ in range(iterations):
        stored = reset
        for trip in range(1024):
            candidate = trip + 1
            stored = candidate if candidate >= stored else stored
        total += stored
    return total


def capture(declared, blocks=4, target=24000000, *, system="linux", counters="perf_event_user", away=(), stop="complete"):
    """A collector-shaped capture from exact per-update costs (0.3 ns per cycle)."""
    cost = {case["id"]: 1.0 + index % 3 for index, case in enumerate(declared)}
    lines, chosen, clock = [], [], 0
    def record(phase, block, slot, probe, iterations):
        nonlocal clock
        case = declared[probe]
        cycles = round(iterations * case["updates_per_iteration"] * cost[case["id"]]) + 9000
        ns = round(cycles * 0.3)
        left = (block, slot) in away
        counted = counters != "none"
        fields = [phase, block, slot, probe, iterations, ns, ns + 40, clock, suite.expected(case, iterations),
                  1 if left and system == "linux" else 4, 4, 0, 0, cycles if counted else 0,
                  iterations * case["instructions_per_iteration"] + 8582 if counted else 0,
                  cycles if system == "darwin" and not left else 0, 0]
        clock += ns
        lines.append("R " + " ".join(map(str, fields)))
        return ns
    for probe, case in enumerate(declared):
        record("W", -1, 0, probe, -(-suite.PILOT_INSTRUCTIONS // case["instructions_per_iteration"]))
    for probe, case in enumerate(declared):
        base = -(-suite.PILOT_INSTRUCTIONS // case["instructions_per_iteration"])
        fastest = min(record("P", -1, step, probe, base << step) / (base << step) for step in range(3))
        chosen.append((base, (int(target / fastest) // 8 + 1) * 8))
    for block in range(blocks):
        for slot, probe in enumerate(suite.block_order(block, len(declared))):
            for step in range(4):
                record("M", block, slot, probe, chosen[probe][1] << ((step in (1, 2)) != bool(block % 2)))
    header = dict(protocol=suite.NATIVE_PROTOCOL, clock="measured", probes=len(declared), blocks_requested=blocks,
                  blocks_completed=blocks, target_ns=target, deadline_s=150, pilot_instructions=suite.PILOT_INSTRUCTIONS,
                  stop=stop, iteration_limited=0, start_realtime_ns=1794312000000000000, elapsed_ns=clock)
    environment = dict(os=system, counters=counters, pinned_cpu=4 if system == "linux" else -1)
    return "\n".join([*(f"H {k} {v}" for k, v in header.items()), *(f"E {k} {v}" for k, v in environment.items()),
                      *(f"F {b} 3504000 3504000" for b in range(blocks)),
                      *(f"N {i} {a} {b}" for i, (a, b) in enumerate(chosen)), *lines]) + "\n", cost


class RecurrenceTests(unittest.TestCase):
    def setUp(self):
        self.cases = suite.cases()
        span = suite.ITERATIONS[1] - suite.ITERATIONS[0]
        self.rows = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * span, cycles=100000,
                          updates=c["updates_per_iteration"] * span) for c in self.cases]

    def summary(self, text):
        return suite.summarize_capture(suite.parse_capture(text, self.cases), self.cases)

    def test_declared_cases_and_analytic_outputs(self):
        self.assertEqual(len(self.cases), 36)
        self.assertEqual(self.cases[0]["id"], suite.CLOCK)
        for case in self.cases:
            for iterations in (1, 2, 5):
                with self.subTest(case=case["id"], iterations=iterations):
                    self.assertEqual(suite.expected(case, iterations), emulate(case, iterations))
            self.assertLess(suite.expected(case, suite.MAX_ITERATIONS), 1 << 64)
        # Every 32-bit recurrence value stays below 2^31 at the trip bound.
        self.assertLess(64 * suite.MAX_ITERATIONS + 5000, 1 << 31)
        for definition in ("", "PROBE(clock_add, 66, 64, unknown)\n", "PROBE(other, 66, 64, count64)\n",
                           "PROBE(clock_add, 66, 64, count64)\nPROBE(clock_add, 66, 64, count64)\n"):
            with self.assertRaises(ValueError):
                suite.parse_cases(definition)

    def test_schedule_is_balanced_and_mirrored(self):
        probes = len(self.cases)
        for block in range(0, 32, 2):
            first, second = suite.block_order(block, probes), suite.block_order(block + 1, probes)
            self.assertEqual(first, second[::-1])
            self.assertEqual(sorted(first[1:-1]), list(range(1, probes)))
            self.assertEqual((first[0], first[-1]), (0, 0))
        neighbours = {tuple(suite.block_order(block, probes)[1:3]) for block in range(0, 16, 2)}
        self.assertEqual(len(neighbours), 8)

    def test_capture_reproduces_costs_and_rejects_altered_evidence(self):
        text, cost = capture(self.cases)
        rows, campaign = self.summary(text)
        self.assertEqual((campaign["blocks"], campaign["reported_khz_maximum"]), (4, 3504000))
        for row in rows:
            self.assertAlmostEqual(row["median_cycles_per_update"], cost[row["id"]], places=6)
            self.assertAlmostEqual(row["median_clock_ratio"], cost[row["id"]], places=3)
            self.assertEqual((row["quality"], row["blocks"], row["excluded_blocks"]), ("usable", 4, 0))
            # The pilot rate includes the fixed call overhead, so N can fall a
            # few microseconds short of the 24 ms target; 20 ms is the floor.
            self.assertGreaterEqual(row["shortest_interval_seconds"], 0.0239)
        lines = text.splitlines()
        main = next(i for i, line in enumerate(lines) if line.startswith("R M"))
        def swapped(values):
            values[main], values[main + 4] = values[main + 4], values[main]
        def field(position, value):
            def change(values):
                parts = values[main].split()
                parts[position] = str(value)
                values[main] = " ".join(parts)
            return change
        selected = next(i for i, line in enumerate(lines) if line.startswith("N 3 "))
        def trip_count(values):
            parts = values[selected].split()
            values[selected] = " ".join([*parts[:3], str(int(parts[3]) + 8)])
        for mutate in (swapped, field(5, 1), field(9, 7), field(14, 0), field(15, 1), field(6, 0), trip_count,
                       lambda values: values.pop(main), lambda values: values.append("X 1"),
                       lambda values: values.remove("F 3 3504000 3504000"),
                       lambda values: values.remove("E counters perf_event_user"),
                       lambda values: values.__setitem__(0, "H protocol other")):
            changed = list(lines)
            mutate(changed)
            with self.assertRaises((ValueError, KeyError, IndexError)):
                suite.parse_capture("\n".join(changed), self.cases)

    def test_uncounted_capture_reports_the_inferred_clock_ratio(self):
        text, cost = capture(self.cases, counters="none")
        rows, campaign = self.summary(text)
        self.assertNotIn("counter_cycles_per_cpu_second", campaign)
        for row in rows:
            self.assertAlmostEqual(row["median_cycles_per_update"], row["median_clock_ratio"])
            self.assertAlmostEqual(row["median_cycles_per_update"], cost[row["id"]], places=3)
        reference = suite.reference_from_capture(text, self.cases, {name: "0" * 64 for name in suite.INPUTS} | {
            suite.DIRECTORY + "cases.def": suite.digest(suite.ROOT / suite.DIRECTORY / "cases.def")}, "1" * 64, "gcc", "test")
        self.assertEqual(reference["cycle_scale"], "time ratio to clock_add (inferred, not counted)")

    def test_visits_off_the_performance_core_are_counted_not_mixed(self):
        slot = suite.block_order(1, len(self.cases)).index(5)
        every = {(block, slot) for block in range(4) for slot in range(len(self.cases) + 1)}
        for system in ("linux", "darwin"):
            with self.subTest(system=system):
                rows, _ = self.summary(capture(self.cases, system=system, counters="perf_event_user" if system == "linux" else "proc_pid_rusage", away={(1, slot)})[0])
                self.assertEqual((rows[5]["blocks"], rows[5]["excluded_blocks"], rows[5]["quality"]), (3, 1, "usable"))
                self.assertEqual((rows[6]["blocks"], rows[6]["excluded_blocks"]), (4, 0))
                rows, campaign = self.summary(capture(self.cases, system=system, counters="perf_event_user" if system == "linux" else "proc_pid_rusage", away=every)[0])
                self.assertEqual({row["quality"] for row in rows}, {"invalid"})
                self.assertEqual(campaign["performance_core_interval_share"], 0)

    def test_reference_and_simulation_evidence_fail_closed(self):
        text, _ = capture(self.cases)
        sources = {name: suite.digest(suite.ROOT / name) for name in suite.INPUTS}
        reference = suite.reference_from_capture(text, self.cases, sources, "1" * 64, "gcc", "test host cpu4")
        suite.validate_reference(reference, {name: suite.digest(suite.ROOT / name) for name in [*suite.MEASUREMENT_INPUTS, suite.ELF]})
        self.assertEqual((reference["os"], reference["core_identity"]), ("linux", "pinned CPU 4; departures counted"))
        for mutate in (lambda r: r["cases"].pop(), lambda r: r["cases"].reverse(),
                       lambda r: r["cases"][0].update(median_cycles_per_update=float("nan")),
                       lambda r: r["cases"][0].update(p10_cycles_per_update=-1),
                       lambda r: r["cases"][0].update(blocks=3), lambda r: r["cases"][0].update(quality="fine"),
                       lambda r: r["campaign"].update(blocks=3), lambda r: r["campaign"].update(stop="interval-limit"),
                       lambda r: r["sources"].pop(suite.ELF), lambda r: r.update(timer="wall time"),
                       lambda r: r.update(cycle_scale="assumed frequency"), lambda r: r.update(os="other"),
                       lambda r: r.update(definition=r["definition"] + "\n")):
            bad = copy.deepcopy(reference)
            mutate(bad)
            with self.assertRaises((ValueError, KeyError)):
                suite.validate_reference(bad)
        with self.assertRaises(ValueError):
            suite.reference_from_capture(text.replace("H clock measured", "H clock synthetic"), self.cases, sources, "1" * 64, "gcc", "test")
        suite.validate_simulation(self.rows, self.cases)
        suite.validate_comparison(dict(base=self.rows, head=self.rows), self.cases)
        for mutate in (lambda r: r.pop(), lambda r: r.reverse(), lambda r: r[0].update(cycles=True),
                       lambda r: r[0].update(updates=1), lambda r: r[0].update(instructions=1)):
            bad = copy.deepcopy(self.rows)
            mutate(bad)
            with self.assertRaises(ValueError):
                suite.validate_simulation(bad, self.cases)
        for value in ({"head": self.rows}, {"base": self.rows}, dict(base=self.rows, head=self.rows, extra=self.rows)):
            with self.assertRaises(ValueError):
                suite.validate_comparison(value, self.cases)


if __name__ == "__main__":
    unittest.main()
