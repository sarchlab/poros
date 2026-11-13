#!/usr/bin/env python3
"""Import audited native Mach-O captures as compact plotting references.

Raw captures, executables and original experiment scripts remain local. Their
hashes are evidence identities, not a claim that the collector was committed at
collection_commit. Only the frozen ELF inputs are bound to that Git commit.
"""
import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
import math
from pathlib import Path
import re
import statistics
import subprocess

import compute_usertime as compute
import performance_reference as performance
import workload_contract as contract

COLLECTORS = {"build.py", "musl.py", "audit.py", "capture.py", "smoke.py"}
SHA = re.compile(r"[0-9a-f]{64}")
SCOPE = ("Audited ELF-matched computation, with x18 register renaming where required; "
         "same linked musl math. Native startup, argument/output adapters and address "
         "layout differ. Whole-child user time includes startup; no offset subtraction.")


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def require(condition, message):
    if not condition:
        raise ValueError(message)


def valid_sha(value):
    return isinstance(value, str) and SHA.fullmatch(value) is not None


def source_name(workload):
    if workload in ("integer", "branch", "memory"):
        return "tests/usertime/program.c"
    if workload == "mvt":
        return "tests/usertime/mvt.c"
    return f"tests/benchmarks/{workload}.c"


def validate_metadata(reference, declared, hardware="Apple M2"):
    """Check a native Mach-O capture. `hardware` is the device the caller compares it with."""
    require(reference.get("hardware") == hardware,
            f"native reference was measured on {reference.get('hardware')!r}, not {hardware!r}")
    require(reference.get("environment") == "native" and reference.get("executable_format") == "Mach-O"
            and reference.get("machine") == "arm64" and reference.get("page_size") == 16384
            and reference.get("platform", "").startswith("macOS-")
            and "container_image" not in reference, "invalid native Mach-O environment")
    require(reference.get("reference_scope") == SCOPE and reference.get("startup_included") is True,
            "invalid native measurement scope")
    require(reference.get("collector_origin") == "local-uncommitted-experiment"
            and set(reference.get("collector_sources", {})) == COLLECTORS,
            "invalid native collector provenance")
    proof = reference.get("capture_evidence", {})
    require(set(proof) == {"capture_sha256", "build_sha256", "audit_sha256", "smoke_sha256", "oracle_reference_sha256",
                          "samples", "warmups", "audited_functions"}, "incomplete native evidence identity")
    require(all(valid_sha(proof[k]) for k in proof if k.endswith("sha256"))
            and type(proof["samples"]) is int and proof["samples"] == 5544
            and type(proof["warmups"]) is int and proof["warmups"] == 264
            and type(proof["audited_functions"]) is int and proof["audited_functions"] == 175,
            "invalid native capture/audit coverage")
    programs = reference.get("native_programs", {})
    names = {c["workload"] for c in declared["cases"]}
    require(set(programs) == names, "missing/extra native programs")
    for name, program in programs.items():
        require(program.get("source") == source_name(name)
                and program.get("source_sha256") == reference.get("input_sources", {}).get(source_name(name)),
                "native program source differs from ELF input source")
        for key in ("binary_sha256", "matched_assembly_sha256", "linux_assembly_sha256", "audit_sha256"):
            require(valid_sha(program.get(key)), "missing native program identity")
        rename = program.get("reserved_register_rename")
        require(rename is None or (type(rename) is int and rename == (29 if name == "adi" else 28)),
                "invalid reserved-register adaptation")
        require(type(program.get("audited_functions")) is int and program["audited_functions"] > 0,
                "missing native program audit")
    require(sum(p["audited_functions"] for p in programs.values()) == proof["audited_functions"],
            "native per-program audit coverage mismatch")
    validate_streamcluster_replacement(reference, declared, hardware)
    for row in reference["cases"]:
        batches = row.get("batch_median_user_seconds")
        require(isinstance(batches, list) and len(batches) == 3
                and all(type(v) in (int, float) and math.isfinite(v) and v > 0 for v in batches),
                "invalid native chronological batch statistics")
        median = row.get("median_user_seconds")
        spread = row.get("batch_median_spread")
        require(type(median) in (int, float) and math.isfinite(median) and median > 0
                and type(spread) in (int, float) and math.isfinite(spread)
                and math.isclose(spread, (max(batches)-min(batches))/median, rel_tol=1e-12, abs_tol=1e-12),
                "native batch spread mismatch")


STREAMCLUSTER_SIZES = (160000, 200000, 250000, 315000, 397000, 500000)
REPEAT_FILES = ("computation.S", "repeat.S", "start.S", "repeat.c")
REPEAT_PREFIX = "tests/benchmarks/streamcluster-repeat/"


def validate_streamcluster_replacement(reference, declared, hardware="Apple M2"):
    replacement = reference.get("streamcluster_replacement")
    historical = reference.get("historical_input_sources")
    if replacement is None:
        require(historical is None, "historical inputs require explicit replacement provenance")
        require(not any(c["file"] == "tests/benchmarks/streamcluster-repeat.elf" for c in declared["cases"]),
                "repeat workload requires explicit replacement acquisition")
        return
    require(reference.get("protocol") == performance.NATIVE_PROTOCOL,
            "replacement requires native reference")
    ids = [f"streamcluster-{i}" for i in range(1, 7)]
    require(replacement.get("case_ids") == ids and replacement.get("source_origin") == "local-uncommitted-experiment"
            and replacement.get("collection_dirty") is True
            and re.fullmatch(r"[0-9a-f]{40}", replacement.get("base_commit", "")),
            "invalid Streamcluster acquisition identity")
    require(isinstance(historical, dict) and historical and all(valid_sha(v) for v in historical.values()),
            "missing historical input bindings")
    for name in historical:
        compute.relative(name)
    require(replacement.get("samples") == 126 and replacement.get("warmups") == 6
            and replacement.get("audited_functions") == 5 and replacement.get("passes") == 16
            and replacement.get("page_size") == 16384 and replacement.get("hardware") == hardware
            and replacement.get("metric") == "child_user_cpu_seconds"
            and replacement.get("timer") == "wait4.ru_utime"
            and replacement.get("startup_included") is True
            and replacement.get("core_identity") == "unknown" and replacement.get("instantaneous_frequency") == "unknown",
            "invalid Streamcluster measurement contract")
    for key in ("capture_sha256", "inputs_sha256", "collector_sha256", "audit_sha256", "elf_sha256",
                "native_sha256", "matched_assembly_sha256", "output_proof_sha256", "original_reference_sha256"):
        require(valid_sha(replacement.get(key)), "missing Streamcluster evidence identity")
    try:
        start = datetime.fromisoformat(replacement["started_at_utc"])
        end = datetime.fromisoformat(replacement["completed_at_utc"])
    except (KeyError, ValueError):
        raise ValueError("invalid Streamcluster acquisition timestamps") from None
    require(start.tzinfo is not None and end.tzinfo is not None and end >= start,
            "invalid Streamcluster acquisition timestamps")
    sources = replacement.get("input_sources", {})
    expected_names = {REPEAT_PREFIX+n for n in REPEAT_FILES}
    require(set(sources) == expected_names and all(valid_sha(v) for v in sources.values())
            and all(reference["input_sources"].get(k) == v for k, v in sources.items()),
            "Streamcluster frozen input binding mismatch")
    require(sources[REPEAT_PREFIX+"computation.S"] == reference["native_programs"]["streamcluster"]["linux_assembly_sha256"],
            "Streamcluster computation assembly changed")
    selected = [r for r in reference["cases"] if r["id"] in ids]
    retained = [r for r in reference["cases"] if r["id"] not in ids]
    require(compute.object_digest(selected) == replacement.get("rows_sha256")
            and compute.object_digest(retained) == replacement.get("retained_rows_sha256"),
            "replacement/retained reference rows changed")
    cases = {c["id"]: c for c in declared["cases"]}
    for ident, n in zip(ids, STREAMCLUSTER_SIZES):
        c = cases[ident]
        require(c["args"] == ["program", str(n), "32", "16"]
                and c["sha256"] == replacement["elf_sha256"]
                and c["file"] == "tests/benchmarks/streamcluster-repeat.elf"
                and compute.expected_output_bytes(c) == n*32*4,
                "Streamcluster repeat execution identity mismatch")


def export(root, evidence, output):
    root, evidence = Path(root), Path(evidence)
    require(not Path(output).exists(), "never overwrite compact measurement data")
    declared = performance.inventory(root)
    raw = compute.read(evidence / "capture.json")
    build = compute.read(evidence / "evidence/build.json")
    audits = compute.read(evidence / "evidence/instruction-audit.json")
    smoke = compute.read(evidence / "evidence/smoke.json")
    require(raw.get("schema") == 1 and raw.get("protocol") == "matched-macho-native-reference-v1"
            and raw.get("status") == "pass" and raw.get("completed_passes") == 21
            and raw.get("source_dirty") is False, "incomplete native capture")
    collection = raw.get("source_head", "")
    head = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
    performance.require_collection_ancestor(root, collection, head)
    original = subprocess.check_output(["git", "-C", str(root), "show", collection + ":" + performance.PATH])
    require(hashlib.sha256(original).hexdigest() == raw.get("old_reference_sha256"), "ELF oracle reference changed")
    oracle = performance.validate(json.loads(original), declared)
    expected = {c["id"]: c for c in oracle["cases"]}
    require(raw.get("build_manifest_sha256") == digest(evidence / "evidence/build.json")
            and raw.get("audit_sha256") == digest(evidence / "evidence/instruction-audit.json")
            and raw.get("collector_sha256") == digest(evidence / "capture.py"), "native capture evidence hash mismatch")
    require(raw.get("case_inventory_sha256") == hashlib.sha256(json.dumps(declared["cases"], sort_keys=True).encode()).hexdigest(),
            "native capture inventory differs from simulated ELF inputs")
    started = datetime.fromisoformat(raw["started_at_utc"])
    ended = datetime.fromisoformat(raw["completed_at_utc"])
    require(started.tzinfo is not None and ended.tzinfo is not None and ended >= started, "invalid native capture timestamps")
    inputs = oracle["input_sources"]
    for name, sha in inputs.items():
        committed = subprocess.check_output(["git", "-C", str(root), "show", collection + ":" + name])
        require(digest(root / name) == sha == hashlib.sha256(committed).hexdigest(), "frozen ELF input changed: " + name)
    names = {c["workload"] for c in declared["cases"]}
    require(set(build["workloads"]) == names, "native build inventory mismatch")
    require(len(audits) == 175 and len({(a["workload"], a["function"]) for a in audits}) == 175,
            "incomplete/duplicate instruction audit")
    require(all(a.get("workload") in names and a.get("pass") is True and a.get("diffs") == []
                and type(a.get("elf_count")) is int and a["elf_count"] > 0 and a["elf_count"] == a.get("mac_count")
                and type(a.get("register_renamed_instructions")) is int
                and 0 <= a["register_renamed_instructions"] <= a["elf_count"] for a in audits),
            "instruction audit failed")
    programs = {}
    for name, b in build["workloads"].items():
        source = source_name(name)
        require(b["source_sha256"] == inputs[source] and Path(b["source"]).as_posix().endswith("/"+source),
                "native build/source mismatch")
        require(digest(evidence / f"bin/{name}-matched") == b["binary_sha256"]
                and digest(evidence / f"assembly/{name}-matched.S") == b["matched_assembly_sha256"],
                "measured native executable/assembly changed")
        native_audits = [a for a in audits if a["workload"] == name]
        assembly = evidence / f"assembly/{name}-linux.s"
        functions = set(re.findall(r"\.type\s+([\w.]+), %function", assembly.read_text()))
        require(functions and functions <= {a["function"] for a in native_audits}, "incomplete computation function audit")
        programs[name] = dict(source=source, source_sha256=b["source_sha256"],
                              binary_sha256=b["binary_sha256"], matched_assembly_sha256=b["matched_assembly_sha256"],
                              linux_assembly_sha256=digest(assembly), reserved_register_rename=b["reserved_register_rename"],
                              audited_functions=len(native_audits), audit_sha256=compute.object_digest(native_audits))
    require(len(smoke) == 264 and {s["id"] for s in smoke} == set(expected), "native smoke inventory mismatch")
    for s in smoke:
        e = expected[s["id"]]
        require(s.get("pass_") is True and s.get("exit") == 0 and s.get("stderr") == ""
                and s.get("sha256") == e["output_sha256"] and s.get("bytes") == e["output_bytes"], "native smoke output mismatch")
    samples, warmups = raw["samples"], raw["warmups"]
    require(len(samples) == 5544 and Counter((s["id"], s["trial"]) for s in samples)
            == Counter((ident, trial) for ident in expected for trial in range(21)), "missing/duplicate native trials")
    require(len(warmups) == 264 and Counter(s["id"] for s in warmups) == Counter(expected.keys()), "missing/duplicate native warmups")
    for records, warmup in ((samples, False), (warmups, True)):
        for s in records:
            e = expected[s["id"]]
            require(type(s["trial"]) is int and (s["trial"] == -1 if warmup else 0 <= s["trial"] < 21)
                    and s["batch"] == (-1 if warmup else s["trial"]//7), "invalid trial/batch identity")
            require(s["exit"] == 0 and s["output_sha256"] == e["output_sha256"] and s["output_bytes"] == e["output_bytes"],
                    "native complete output mismatch")
            require(all(type(s[k]) in (int, float) and math.isfinite(s[k]) and s[k] >= 0
                        for k in ("user_seconds", "system_seconds", "wall_seconds")), "invalid native CPU sample")
            require(started.timestamp() <= s["unix_time"] <= ended.timestamp(), "sample outside capture interval")
    rows = []
    for c in declared["cases"]:
        ss = [s for s in samples if s["id"] == c["id"]]
        e = expected[c["id"]]
        row = performance.summarize(c, ss, e["output_sha256"], e["output_bytes"], e["numerical_check"])
        batches = [statistics.median(s["user_seconds"] for s in ss if s["batch"] == b) for b in range(3)]
        row.update(batch_median_user_seconds=batches, batch_median_spread=(max(batches)-min(batches))/row["median_user_seconds"])
        rows.append(row)
    result = {k: oracle[k] for k in ("schema", "quality_policy", "micro_manifest_sha256", "build_manifest_sha256",
                                    "numerical_oracle_sha256", "input_sources", "inventory_sha256")}
    result.update(protocol=performance.NATIVE_PROTOCOL, measurement_id="m2-native-macho-20261107", collection_commit=collection,
                  oracle_commit=collection, collection_dirty=False, collector_origin="local-uncommitted-experiment",
                  collector_sources={n: digest(evidence/n) for n in sorted(COLLECTORS)},
                  environment=raw["environment"], executable_format=raw["executable_format"], machine="arm64",
                  page_size=raw["page_size"], hardware=raw["hardware"], platform=raw["platform"],
                  core_identity="unknown", instantaneous_frequency=raw["instantaneous_frequency"],
                  metric=raw["metric"], timer=raw["timer"], reference_scope=SCOPE, startup_included=True,
                  started_at_utc=started.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                  completed_at_utc=ended.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                  capture_evidence=dict(capture_sha256=digest(evidence/"capture.json"), build_sha256=raw["build_manifest_sha256"],
                      audit_sha256=raw["audit_sha256"], smoke_sha256=digest(evidence/"evidence/smoke.json"),
                      oracle_reference_sha256=raw["old_reference_sha256"], samples=len(samples), warmups=len(warmups), audited_functions=len(audits)),
                  native_programs=programs, cases=rows)
    performance.validate(result, declared)
    compute.write(output, result)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--evidence", type=Path, required=True, help="directory containing capture.json, source, assembly, bin and evidence")
    parser.add_argument("--output", type=Path, required=True, help="new compact reference file; existing files are never overwritten")
    args = parser.parse_args()
    export(args.root, args.evidence, args.output)
