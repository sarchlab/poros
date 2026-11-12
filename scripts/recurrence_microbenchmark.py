#!/usr/bin/env python3
"""Compare/select and memory-carried recurrence probes: functional CI, native reference, simulation."""
from report_records import covered_cases

import argparse
import base64
from datetime import datetime, timezone
import hashlib
import json
import math
from pathlib import Path
import re
import statistics
import struct
import subprocess

import devices

ROOT = Path(__file__).resolve().parents[1]
DIRECTORY = "evaluation/microbenchmarks/recurrence/"
ELF = DIRECTORY + "recurrence.elf"
# A separately identified native Linux cohort; no Mach-O reference is replaced.
REFERENCE = DIRECTORY + "m2-linux.json"
MEASUREMENT_INPUTS = [DIRECTORY + name for name in ("cases.def", "loops.S", "driver.h", "native.c")]
# Recorded collection provenance, not prerequisites for reusing native measurements.
COLLECTOR_SOURCES = [DIRECTORY + name for name in ("linux.c", "entry.S", "build.sh", "recurrence.elf")]
COLLECTOR_SOURCES += ["scripts/recurrence_microbenchmark.py", "tests/usertime/layout.ld"]
INPUTS = MEASUREMENT_INPUTS + COLLECTOR_SOURCES
PROTOCOL = "assembly-recurrence-v1"
NATIVE_PROTOCOL = "recurrence-native-v1"
ITERATIONS = (128, 256)
MAX_INSTRUCTIONS = 4000000
MAX_ITERATIONS = 8388608
PILOT_INSTRUCTIONS = 16000000
CLOCK = "clock_add"
KINDS = ("count64", "half", "select_max", "select_low", "select_shift", "select_inc", "select_other", "select_hold",
         "mix", "max_const", "max_chg", "sum64", "count128", "mid", "far", "trips", "tight_max", "kernel_const", "kernel_chg")
QUALITIES = ("usable", "noisy", "invalid")
NOISY_SPREAD = 0.20
# Harness, kernel-entry and interrupt instructions counted around one interval.
COUNTER_OVERHEAD = 2000000
# How a capture's cycles per update were obtained. Only the first two are counted
# by hardware; the last is a time ratio to the interleaved one-cycle ADD chain.
CYCLE_SCALES = {"proc_pid_rusage": "kernel cycle counter (proc_pid_rusage ri_cycles)",
                "perf_event_user": "user-mode cycle counter (perf_event)",
                "none": "time ratio to clock_add (inferred, not counted)"}
NATIVE_COMPILERS = {"darwin": ["clang", "-O2", "-arch", "arm64"],
                    "linux": ["aarch64-linux-musl-gcc", "-O2", "-static", "-s", "-Wl,--build-id=none,-z,max-page-size=16384"]}


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def cases(root=ROOT):
    return parse_cases((Path(root) / DIRECTORY / "cases.def").read_text())


def parse_cases(definition):
    result = []
    for line in definition.splitlines():
        match = re.fullmatch(r"PROBE\(([a-z0-9_]+), ([0-9]+), ([0-9]+), ([a-z0-9_]+)\)", line)
        if not match or match.group(4) not in KINDS:
            raise ValueError("invalid recurrence case definition")
        name, count, updates, kind = match.groups()
        result.append(dict(id=name, instructions_per_iteration=int(count), updates_per_iteration=int(updates), kind=kind))
    if not result or len({c["id"] for c in result}) != len(result) or result[0]["id"] != CLOCK:
        raise ValueError("empty/duplicate recurrence cases or missing leading clock")
    return result


def expected(case, iterations):
    """Analytic outputs derived from the recurrences, independent of the assembly."""
    n, t = iterations, 64 * iterations
    return {
        "count64": t + 1,
        "half": 32 * n + 1,
        "select_max": t * (t + 1) // 2 + t + 1,
        "select_low": 50 * t + t * t // 4,
        "select_shift": 2500 * t + t * t // 4,
        "select_inc": t * (t + 1) // 2 + t // 2 + t + 1,
        "select_other": (1 << 29) * t,
        "select_hold": 101 * t,
        "mix": 6 * n - 3,
        "max_const": t + 5,
        "max_chg": 5 * t + t * (t + 1) // 2 + 5 + t,
        "sum64": t,
        "count128": 2 * t + 1,
        "mid": 2 * (t + 1),
        "far": 3 * (t + 1),
        "trips": n,
        "tight_max": t + 5,
        "kernel_const": 4096 * n,
        "kernel_chg": 1024 * n,
    }[case["kind"]]


def execute(binary, root, case, index, iterations, mode, revision=None, profile=None, device=None):
    args = [str(binary), "elf", "--mode", mode, *devices.arguments(device), "--max-instructions", str(MAX_INSTRUCTIONS), "--timeout", "90s",
            str(Path(root) / ELF), str(index), str(iterations)]
    process = subprocess.run(args, capture_output=True, check=True, timeout=100)
    row = json.loads(process.stdout)
    if process.stderr:
        raise ValueError("probe process wrote stderr")
    return validate_execution(row, root, case, index, iterations, mode, revision, profile, device=device)


def validate_execution(row, root, case, index, iterations, mode, revision=None, profile=None, device=None):
    if (row["termination"] != "exit" or row["exit_status"] != 0 or row["stderr_base64"]
            or row["binary_sha256"] != digest(Path(root) / ELF)
            or row["args"] != ["program", str(index), str(iterations)] or row["env"]
            or base64.b64decode(row["stdout_base64"], validate=True) != struct.pack("<Q", expected(case, iterations))
            or type(row["instructions"]) is not int or not 0 < row["instructions"] <= MAX_INSTRUCTIONS):
        raise ValueError("invalid recurrence execution: " + case["id"])
    if revision is not None and (row["build"].get("vcs.revision") != revision or row["build"].get("vcs.modified") != "false"):
        raise ValueError("recurrence simulator revision mismatch")
    if mode == "ooo":
        timing = row["timing"]
        if (timing["retired_instructions"] != row["instructions"] or type(timing["user_cycles"]) is not int
                or timing["user_cycles"] <= 0 or (profile is not None and any(timing[k] != profile[k] for k in ("configuration", "cache_config")))):
            raise ValueError("invalid recurrence timing/profile")
        devices.check_timing(timing, device)
    elif "timing" in row:
        raise ValueError("functional probe unexpectedly timed")
    return row


def smoke(binary, root=ROOT):
    for index, case in enumerate(cases(root), 1):
        samples = []
        for iterations in (16, 32):
            functional = execute(binary, root, case, index, iterations, "functional")
            timed = execute(binary, root, case, index, iterations, "ooo")
            if functional["instructions"] != timed["instructions"]:
                raise ValueError("functional/timing mismatch")
            samples.append(timed["instructions"])
        if samples[1] - samples[0] != 16 * case["instructions_per_iteration"]:
            raise ValueError("incorrect smoke loop instruction count")
        print(case["id"], "functional + ooo checksum passed", flush=True)


def summarize_samples(case, samples):
    if len(samples) != 2:
        raise ValueError("recurrence short/long execution pair missing")
    row = dict(id=case["id"], instructions=samples[1]["instructions"] - samples[0]["instructions"],
               cycles=samples[1]["timing"]["user_cycles"] - samples[0]["timing"]["user_cycles"],
               updates=case["updates_per_iteration"] * (ITERATIONS[1] - ITERATIONS[0]))
    validate_simulation([row], [case])
    return row


def measure_case(binary, root, case, index, revision, profile, *, checkpoint=None, device=None):
    samples = []
    # Passed only when a device is requested, like every later execution option.
    options = {} if device is None else {"device": device}
    for n in ITERATIONS:
        collect = lambda: execute(binary, root, case, index, n, "ooo", revision, profile, **options)
        validate = lambda row: validate_execution(row, root, case, index, n, "ooo", revision, profile, **options)
        samples.append(checkpoint(case, index, n, collect, validate) if checkpoint else collect())
    summarize_samples(case, samples)
    return samples


def measure(binary, root, revision, profile, *, checkpoint=None, progress=None, device=None):
    declared = cases(root)
    rows = []
    for index, case in enumerate(declared, 1):
        if progress:
            progress(rows, case["id"])
        samples = measure_case(binary, root, case, index, revision, profile, checkpoint=checkpoint,
                               **({} if device is None else {"device": device}))
        rows.append(summarize_samples(case, samples))
        validate_simulation(rows, declared, partial=True)
        if progress:
            progress(rows, None)
    validate_simulation(rows, declared)
    return rows


def validate_simulation(rows, declared, *, partial=False):
    declared = covered_cases(rows, declared, partial=partial)
    span = ITERATIONS[1] - ITERATIONS[0]
    for row, case in zip(rows, declared):
        if (type(row.get("instructions")) is not int or row["instructions"] != span * case["instructions_per_iteration"]
                or type(row.get("cycles")) is not int or row["cycles"] <= 0
                or type(row.get("updates")) is not int or row["updates"] != span * case["updates_per_iteration"]):
            raise ValueError("invalid recurrence counters")


def declared(files):
    return any(name in files for name in [*MEASUREMENT_INPUTS, ELF, REFERENCE])


def validate_comparison(simulations, declared_cases, *, partial=False):
    if not isinstance(simulations, dict) or set(simulations) != {"base", "head"}:
        raise ValueError("recurrence requires both base and head simulation")
    for rows in simulations.values():
        validate_simulation(rows, declared_cases, partial=partial)


def rank(values, quantile):
    """Nearest lower rank, the repository's nine-sample P10/P90 convention."""
    ordered = sorted(values)
    return ordered[int(quantile * (len(ordered) - 1))]


def block_order(block, probes):
    """Predeclared visit order; an independent restatement of native.c."""
    others, pair = probes - 1, block // 2
    stride = found = 0
    while found <= pair:
        stride += 1
        found += math.gcd(stride, others) == 1
    order = [1 + (5 * pair + k * stride) % others for k in range(others)]
    return [0, *(reversed(order) if block % 2 else order), 0]


def counter_group_ran(environment):
    """A perf group that was ever descheduled undercounts silently, so the
    collector reports the group's enabled and running times; they must agree."""
    times = [str(environment.get(key, "")) for key in ("counter_enabled_ns", "counter_running_ns")]
    return environment.get("counters") != "perf_event_user" or (all(time.isdigit() for time in times) and 0 < int(times[0]) == int(times[1]))


def parse_capture(text, declared):
    """Parse and independently check one native capture; nothing is repaired."""
    header, environment, frequency, chosen, records = {}, {}, {}, {}, []
    for line in text.splitlines():
        fields = line.split()
        if fields[:1] == ["H"] and len(fields) == 3:
            header[fields[1]] = fields[2]
        elif fields[:1] == ["E"] and len(fields) == 3:
            environment[fields[1]] = fields[2]
        elif fields[:1] == ["F"] and len(fields) == 4:
            frequency[int(fields[1])] = (int(fields[2]), int(fields[3]))
        elif fields[:1] == ["N"] and len(fields) == 4:
            chosen[int(fields[1])] = (int(fields[2]), int(fields[3]))
        elif fields[:1] == ["R"] and len(fields) == 18:
            keys = ("block", "slot", "probe", "iterations", "cpu_ns", "wall_ns", "start_ns", "output", "cpu_before",
                    "cpu_after", "involuntary", "voluntary", "cycles", "instructions", "performance_cycles", "performance_instructions")
            records.append(dict(phase=fields[1], **{key: int(value) for key, value in zip(keys, fields[2:])}))
        else:
            raise ValueError("unrecognized native capture line")
    probes = len(declared)
    synthetic = header.get("clock") == "synthetic"
    if (header.get("protocol") != NATIVE_PROTOCOL or header.get("clock") not in ("measured", "synthetic")
            or int(header["probes"]) != probes or set(chosen) != set(range(probes))):
        raise ValueError("native capture protocol/case coverage mismatch")
    if synthetic:
        environment = dict(os="synthetic", counters="proc_pid_rusage", pinned_cpu="-1")
    if environment.get("os") not in ("darwin", "linux", "synthetic") or environment.get("counters") not in CYCLE_SCALES:
        raise ValueError("native capture environment missing")
    if not counter_group_ran(environment):
        raise ValueError("native capture counter group was multiplexed or its times are missing")
    target, blocks = int(header["target_ns"]), int(header["blocks_completed"])
    if int(header["pilot_instructions"]) != PILOT_INSTRUCTIONS:
        raise ValueError("native capture pilot length mismatch")
    counted = environment["counters"] != "none"
    for record in records:
        case = declared[record["probe"]]
        if record["cpu_ns"] <= 0 or record["wall_ns"] <= 0 or record["output"] != expected(case, record["iterations"]):
            raise ValueError("native interval output/timer failed: " + case["id"])
        # A hardware counter must see the loop itself: the declared count plus
        # harness, kernel-entry and interrupt instructions, never fewer.
        body = record["iterations"] * case["instructions_per_iteration"]
        if counted and (record["cycles"] <= 0 or not body <= record["instructions"] <= body + body // 20 + COUNTER_OVERHEAD):
            raise ValueError("native interval hardware counters disagree with the executed loop: " + case["id"])
        # macOS counts performance-core cycles separately; a pinned Linux
        # thread is on its performance core when it never left the pinned CPU.
        if environment["os"] == "linux":
            record["performance"] = record["cpu_before"] == record["cpu_after"] == int(environment["pinned_cpu"]) >= 0
        else:
            record["performance"] = record["performance_cycles"] == record["cycles"]
    pilots = [r for r in records if r["phase"] == "P"]
    warmups = [r for r in records if r["phase"] == "W"]
    if [r["probe"] for r in warmups] != list(range(probes)) or [(r["probe"], r["slot"]) for r in pilots] != [(i, s) for i in range(probes) for s in range(3)]:
        raise ValueError("native warmup/pilot schedule mismatch")
    for index, case in enumerate(declared):
        base = 2 if synthetic else -(-PILOT_INSTRUCTIONS // case["instructions_per_iteration"])
        mine = [r for r in pilots if r["probe"] == index]
        fastest = min(r["cpu_ns"] / r["iterations"] for r in mine)
        selected = (int(target / fastest) // 8 + 1) * 8
        limited = 2 * selected > MAX_ITERATIONS
        if ([r["iterations"] for r in mine] != [base, 2 * base, 4 * base] or chosen[index][0] != base
                or chosen[index][1] != (MAX_ITERATIONS // 2 if limited else selected)):
            raise ValueError("native duration pilot did not select the declared trip count: " + case["id"])
    main = [r for r in records if r["phase"] == "M"]
    wanted = []
    for block in range(blocks if header["stop"] == "complete" else blocks + 1):
        for slot, probe in enumerate(block_order(block, probes)):
            for step in range(4):
                twice = (step in (1, 2)) != bool(block % 2)
                wanted.append((block, slot, probe, chosen[probe][1] << twice))
    observed = [(r["block"], r["slot"], r["probe"], r["iterations"]) for r in main]
    if observed != wanted[:len(observed)] or (header["stop"] == "complete" and len(observed) != len(wanted)):
        raise ValueError("native main schedule differs from the predeclared order")
    if not synthetic and set(frequency) != set(range(blocks)):
        raise ValueError("native capture frequency observations incomplete")
    return dict(header=header, environment=environment, frequency=frequency, chosen=chosen, records=records)


def summarize_capture(capture, declared):
    """Per-block N/2N differences over every completed mirrored block pair.

    The reference describes the performance core. A visit that left it is a
    different machine, so it is counted and left out of the percentiles; the
    all-visit median stays beside them for comparison."""
    header, chosen, records = capture["header"], capture["chosen"], capture["records"]
    counted = capture["environment"]["counters"] != "none"
    blocks, probes = int(header["blocks_completed"]) // 2 * 2, len(declared)
    visits = {}
    for record in records:
        if record["phase"] == "M" and record["block"] < blocks:
            visits.setdefault((record["block"], record["slot"]), []).append(record)
    per_case = {index: [] for index in range(probes)}
    for block in range(blocks):
        samples = {}
        for slot, probe in enumerate(block_order(block, probes)):
            group = visits[block, slot]
            n = chosen[probe][1]
            short = [r for r in group if r["iterations"] == n]
            long = [r for r in group if r["iterations"] == 2 * n]
            updates = n * declared[probe]["updates_per_iteration"]
            difference = lambda key: (sum(r[key] for r in long) - sum(r[key] for r in short)) / 2 / updates
            samples.setdefault(probe, []).append(dict(
                counted=difference("cycles"), ns=difference("cpu_ns"),
                direct=sum(r["cpu_ns"] / r["iterations"] for r in group) / 4 / declared[probe]["updates_per_iteration"],
                shortest_ns=min(r["cpu_ns"] for r in short), performance=all(r["performance"] for r in group)))
        # The clock is visited first and last; its mean is this block's time scale.
        clock_ns = statistics.fmean(s["ns"] for s in samples[0])
        for probe, values in samples.items():
            sample = {key: statistics.fmean(v[key] for v in values) for key in ("counted", "ns", "direct")}
            sample.update(shortest_ns=min(v["shortest_ns"] for v in values), clock_ratio=sample["ns"] / clock_ns,
                          performance=all(v["performance"] for v in values))
            sample["cycles"] = sample["counted"] if counted else sample["clock_ratio"]
            per_case[probe].append(sample)
    rows = []
    for index, case in enumerate(declared):
        everything = per_case[index]
        performance = [s for s in everything if s["performance"]]
        samples = performance or everything
        cycles = [s["cycles"] for s in samples]
        ns = [s["ns"] for s in samples]
        excluded = len(everything) - len(performance)
        median = statistics.median(cycles)
        # A nonpositive median is already invalid; its spread stays finite so
        # the row can still be recorded and shown.
        spread = (rank(cycles, 0.9) - rank(cycles, 0.1)) / abs(median) if median else 0.0
        if not performance or min(cycles) <= 0 or min(ns) <= 0:
            quality = "invalid"
        else:
            quality = "noisy" if spread > NOISY_SPREAD or 4 * excluded > len(everything) else "usable"
        rows.append(dict(case, iterations=chosen[index][1], blocks=len(performance), excluded_blocks=excluded,
                         p10_cycles_per_update=rank(cycles, 0.1), median_cycles_per_update=median,
                         p90_cycles_per_update=rank(cycles, 0.9),
                         all_blocks_median_cycles_per_update=statistics.median(s["cycles"] for s in everything),
                         p10_ns_per_update=rank(ns, 0.1), median_ns_per_update=statistics.median(ns),
                         p90_ns_per_update=rank(ns, 0.9),
                         direct_median_ns_per_update=statistics.median(s["direct"] for s in samples),
                         median_clock_ratio=statistics.median(s["clock_ratio"] for s in samples),
                         shortest_interval_seconds=min(s["shortest_ns"] for s in samples) / 1e9,
                         relative_spread=spread, quality=quality))
    main = [r for r in records if r["phase"] == "M" and r["block"] < blocks]
    body = sum(r["iterations"] * declared[r["probe"]]["instructions_per_iteration"] for r in main)
    observed = [khz for block in range(blocks) for khz in capture["frequency"].get(block, ()) if khz > 0]
    campaign = dict(blocks=blocks, blocks_completed=int(header["blocks_completed"]), stop=header["stop"],
                    target_seconds=int(header["target_ns"]) / 1e9,
                    main_intervals=len(main), timed_cpu_seconds=sum(r["cpu_ns"] for r in records) / 1e9,
                    elapsed_seconds=int(header["elapsed_ns"]) / 1e9,
                    involuntary_context_switches=sum(r["involuntary"] for r in main),
                    cpu_numbers=sorted({r[key] for r in main for key in ("cpu_before", "cpu_after")}),
                    maximum_wall_over_cpu=max(r["wall_ns"] / r["cpu_ns"] for r in main),
                    performance_core_interval_share=sum(r["performance"] for r in main) / len(main),
                    iteration_limited=int(header["iteration_limited"]))
    if counted:
        campaign.update(counted_over_declared_instructions=sum(r["instructions"] for r in main) / body,
                        counter_cycles_per_cpu_second=sum(r["cycles"] for r in main) / sum(r["cpu_ns"] for r in main) * 1e9)
    if observed:
        # Operating-system frequency reports at block boundaries, not a measurement.
        campaign.update(reported_khz_minimum=min(observed), reported_khz_maximum=max(observed))
    return rows, campaign


def reference_from_capture(text, declared, sources, native_binary_sha256, compiler, host):
    """A reference only from a complete measured capture of the declared cases."""
    capture = parse_capture(text, declared)
    header, environment = capture["header"], capture["environment"]
    if header["clock"] != "measured":
        raise ValueError("native capture did not use the measured clocks")
    rows, campaign = summarize_capture(capture, declared)
    reference = dict(protocol=PROTOCOL, hardware="Apple M2", os=environment["os"], host=host,
                     collected_at_utc=datetime.fromtimestamp(int(header["start_realtime_ns"]) / 1e9, timezone.utc).isoformat(),
                     timer="CLOCK_THREAD_CPUTIME_ID", cycle_scale=CYCLE_SCALES[environment["counters"]],
                     core_identity=("pinned CPU %s; departures counted" % environment["pinned_cpu"] if environment["os"] == "linux"
                                    else "unpinned; performance-core cycle share recorded"),
                     instantaneous_frequency="unknown", environment={k: v for k, v in environment.items() if k != "os"},
                     compiler=compiler, native_binary_sha256=native_binary_sha256,
                     capture_sha256=hashlib.sha256(text.encode()).hexdigest(), sources=sources,
                     definition=(ROOT / DIRECTORY / "cases.def").read_text(), campaign=campaign, cases=rows)
    return validate_reference(reference)


def validate_reference(reference, files=None):
    if (reference.get("protocol") != PROTOCOL or reference.get("timer") != "CLOCK_THREAD_CPUTIME_ID"
            or reference.get("cycle_scale") not in CYCLE_SCALES.values() or reference.get("os") not in ("darwin", "linux")
            or reference.get("instantaneous_frequency") != "unknown" or reference.get("hardware") != "Apple M2"
            or not isinstance(reference.get("environment"), dict) or not counter_group_ran(reference["environment"])
            or any(not isinstance(reference.get(key), str) or not reference[key] for key in ("core_identity", "host", "compiler"))):
        raise ValueError("invalid recurrence reference protocol/environment")
    if (not re.fullmatch(r"[0-9a-f]{64}", reference.get("native_binary_sha256", ""))
            or not re.fullmatch(r"[0-9a-f]{64}", reference.get("capture_sha256", ""))):
        raise ValueError("missing native build/capture identity")
    try:
        timestamp = datetime.fromisoformat(reference["collected_at_utc"])
        if timestamp.utcoffset() is None or timestamp.utcoffset().total_seconds() != 0:
            raise ValueError("non-UTC measurement time")
    except (KeyError, TypeError, ValueError):
        raise ValueError("invalid native collection timestamp") from None
    sources = reference.get("sources", {})
    if set(sources) != set(INPUTS) or any(not re.fullmatch(r"[0-9a-f]{64}", sha) for sha in sources.values()):
        raise ValueError("incomplete recurrence provenance")
    if files is not None and any(files.get(name) != sources[name] for name in MEASUREMENT_INPUTS):
        raise ValueError("recurrence reference differs from source")
    if files is not None and not re.fullmatch(r"[0-9a-f]{64}", files.get(ELF, "")):
        raise ValueError("recurrence ELF absent from tested source")
    definition = reference.get("definition", "")
    if hashlib.sha256(definition.encode()).hexdigest() != sources[DIRECTORY + "cases.def"]:
        raise ValueError("recurrence case definition digest mismatch")
    campaign, rows = reference.get("campaign", {}), reference.get("cases", [])
    blocks = campaign.get("blocks")
    if (type(blocks) is not int or blocks < 2 or blocks % 2 or campaign.get("stop") not in ("complete", "deadline")
            or type(campaign.get("involuntary_context_switches")) is not int
            or not isinstance(campaign.get("cpu_numbers"), list)
            or any(type(campaign.get(key)) not in (int, float) or not math.isfinite(campaign[key]) or campaign[key] <= 0
                   for key in ("target_seconds", "timed_cpu_seconds", "maximum_wall_over_cpu", "performance_core_interval_share"))):
        raise ValueError("invalid recurrence campaign summary")
    keys = ("id", "instructions_per_iteration", "updates_per_iteration", "kind")
    if [{k: row.get(k) for k in keys} for row in rows] != parse_cases(definition):
        raise ValueError("recurrence native case coverage mismatch")
    for row in rows:
        ns = [row.get(k + "_ns_per_update") for k in ("p10", "median", "p90")]
        cycles = [row.get(k + "_cycles_per_update") for k in ("p10", "median", "p90")]
        numbers = [*ns, *cycles, *(row.get(k) for k in ("relative_spread", "direct_median_ns_per_update", "median_clock_ratio",
                                                         "all_blocks_median_cycles_per_update", "shortest_interval_seconds"))]
        if (any(type(v) not in (int, float) or not math.isfinite(v) for v in numbers) or ns != sorted(ns) or cycles != sorted(cycles)
                or row.get("quality") not in QUALITIES or (row["quality"] != "invalid" and min(ns[0], cycles[0]) <= 0)
                or type(row.get("iterations")) is not int or not 0 < 2 * row["iterations"] <= MAX_ITERATIONS
                or type(row.get("blocks")) is not int or type(row.get("excluded_blocks")) is not int
                or min(row["blocks"], row["excluded_blocks"]) < 0 or row["blocks"] + row["excluded_blocks"] != blocks
                or (row["blocks"] == 0 and row["quality"] != "invalid")):
            raise ValueError("invalid recurrence native case")
    return reference


def load_reference(root):
    root = Path(root)
    reference = json.loads((root / REFERENCE).read_text())
    validate_reference(reference, {name: digest(root / name) for name in [*MEASUREMENT_INPUTS, ELF]})
    keys = ("id", "instructions_per_iteration", "updates_per_iteration", "kind")
    if [{k: row[k] for k in keys} for row in reference["cases"]] != cases(root):
        raise ValueError("recurrence manifest differs from reference")
    return reference


def build_native(directory, target="darwin"):
    """The same probe bodies as the simulated ELF; Linux output is static with 16 KiB-compatible segments."""
    binary = Path(directory) / ("native" if target == "darwin" else "native-linux")
    subprocess.run([*NATIVE_COMPILERS[target], str(ROOT / DIRECTORY / "native.c"),
                    str(ROOT / DIRECTORY / "loops.S"), "-o", str(binary)], check=True)
    return binary


def native_check(binary):
    """Untimed functional run of every probe and of the collection schedule.

    The dry capture uses the collector's synthetic clock, so no measurement is taken."""
    declared = cases()
    for index, case in enumerate(declared, 1):
        for iterations in (3, 16):
            output = subprocess.check_output([str(binary), "run", str(index), str(iterations)], text=True)
            if int(output) != expected(case, iterations):
                raise ValueError("native functional output mismatch: " + case["id"])
    text = subprocess.check_output([str(binary), "dry", "4", "2000", "60"], text=True)
    capture = parse_capture(text, declared)
    if capture["header"]["clock"] != "synthetic" or capture["header"]["stop"] != "complete":
        raise ValueError("native dry schedule did not complete")
    summarize_capture(capture, declared)
    return text


def summarize(capture, output, native_binary, compiler, host):
    """Reference from the collector's retained output and the binary that produced it."""
    text = Path(capture).read_text()
    reference = reference_from_capture(text, cases(), {name: digest(ROOT / name) for name in INPUTS},
                                       digest(native_binary), compiler, host)
    Path(output).write_text(json.dumps(reference, indent=2) + "\n")
    for row in reference["cases"]:
        print(row["id"], f'{row["median_cycles_per_update"]:.3f}', f'{row["relative_spread"]:.2%}', row["quality"], flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    smoke_parser = commands.add_parser("smoke")
    smoke_parser.add_argument("--poros", required=True, type=Path)
    check_parser = commands.add_parser("native-check", help="untimed functional native run")
    check_parser.add_argument("--directory", required=True, type=Path, help="existing directory for the native build")
    build_parser = commands.add_parser("build-linux", help="cross-build the static native Linux collector")
    build_parser.add_argument("--directory", required=True, type=Path)
    summarize_parser = commands.add_parser("summarize", help="reference from a retained capture")
    summarize_parser.add_argument("--capture", required=True, type=Path)
    summarize_parser.add_argument("--native-binary", required=True, type=Path)
    summarize_parser.add_argument("--compiler", required=True)
    summarize_parser.add_argument("--host", required=True)
    summarize_parser.add_argument("--output", type=Path, default=ROOT / REFERENCE)
    args = parser.parse_args()
    if args.command == "smoke":
        smoke(args.poros)
    elif args.command == "native-check":
        native_check(build_native(args.directory))
    elif args.command == "build-linux":
        print(digest(build_native(args.directory, "linux")))
    else:
        summarize(args.capture, args.output, args.native_binary, args.compiler, args.host)


if __name__ == "__main__":
    main()
