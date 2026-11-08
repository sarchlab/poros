#!/usr/bin/env python3
"""Import the six audited 16-pass Streamcluster measurements, without executing code.

The original capture's metadata remains historical. Only the six replacement
rows and their explicitly uncommitted experiment provenance are added. Raw
captures, Mach-O binaries and output blocks stay outside the repository.
"""
import argparse
from collections import Counter
from datetime import datetime
import hashlib
import json
from pathlib import Path
import statistics
import struct
import subprocess

import compute_usertime as compute
import macho_reference as macho
import performance_reference as performance
import workload_contract as contract


def validate_samples(raw, inputs):
    require = macho.require
    configs = {c["id"]: c for c in raw["configs"]}
    require(raw["configs"] == inputs["configs"] and len(configs) == len(raw["configs"]) == 18, "unexpected focused capture inventory")
    require(Counter((s["id"], s["trial"]) for s in raw["samples"])
            == Counter((ident, trial) for ident in configs for trial in range(21))
            and Counter(s["id"] for s in raw["warmups"]) == Counter(configs.keys()), "incomplete focused capture census")
    started, ended = (datetime.fromisoformat(raw[k]) for k in ("started_at", "completed_at"))
    require(started.tzinfo is not None and ended.tzinfo is not None and ended >= started, "invalid capture interval")
    require(all(s.get("trial") == -1 for s in raw["warmups"]), "invalid warmup trial identity")
    for s in raw["samples"] + raw["warmups"]:
        c = configs[s["id"]]; expected = inputs["expected"][str(c["n"])]; trial = s["trial"]
        require(type(trial) is int and -1 <= trial < 21 and s["batch"] == (trial//7 if trial >= 0 else -1)
                and s["n"] == c["n"] and s["k"] == c["k"] and s["exit"] == 0
                and s["all_repetitions_match"] is True and s["output_block_sha256"] == expected["sha256"]
                and s["output_bytes"] == expected["bytes"]*c["k"]
                and started.timestamp() <= s["unix_time"] <= ended.timestamp(), "invalid capture sample/output")
        require(all(type(s[k]) in (int, float) and macho.math.isfinite(s[k]) and s[k] >= 0
                    for k in ("user_seconds", "system_seconds", "wall_seconds")), "invalid timing sample")
    return configs


def export(root, evidence, blocks, output):
    root, evidence, blocks, output = map(Path, (root, evidence, blocks, output))
    require = macho.require
    require(not output.exists(), "never overwrite measurement evidence")
    raw = compute.read(evidence / "focused.json")
    inputs = compute.read(evidence / "inputs.json")
    audits = compute.read(evidence / "evidence/instruction-audit.json")
    require(raw.get("protocol") == "streamcluster-scaling-experiment-v1" and raw.get("status") == "pass",
            "incomplete Streamcluster capture")
    require(raw.get("hardware") == "Apple M2" and raw.get("page_size") == 16384
            and raw.get("platform", "").startswith("macOS-") and raw.get("core_identity") == "unobserved"
            and raw.get("frequency") == "unobserved", "invalid native capture environment")
    require(raw.get("inputs_sha256") == macho.digest(evidence / "inputs.json")
            and raw.get("collector_sha256") == macho.digest(evidence / "capture.py"), "capture source identity mismatch")
    base = raw["source_commit"]
    head = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
    performance.require_collection_ancestor(root, base, head)
    def committed(path):
        return subprocess.check_output(["git", "-C", str(root), "show", base+":"+path])
    previous_bytes = committed(performance.PATH)
    previous = json.loads(previous_bytes)
    require("streamcluster_replacement" not in previous, "replacement requires original acquisition reference")
    micro = json.loads(committed(contract.PERFORMANCE_MANIFEST))
    program = json.loads(committed(compute.PERFORMANCE_MANIFEST))
    build = json.loads(committed("tests/benchmarks/provenance.json"))
    original = compute.expanded_manifest(micro, compute.contract_data(program, build, micro["profile"], compute.PROTOCOL))
    performance.validate(previous, original)
    require(previous["protocol"] == performance.NATIVE_PROTOCOL, "replacement requires native acquisition")
    declared = performance.inventory(root)
    old_cases = {c["id"]: c for c in original["cases"]}
    for c in declared["cases"]:
        if c["workload"] != "streamcluster":
            require(c == old_cases[c["id"]], "unrelated execution input changed")
    ids = [f"streamcluster-{i}" for i in range(1, 7)]
    selected = [f"focused-n{n}-k16" for n in macho.STREAMCLUSTER_SIZES]
    configs = validate_samples(raw, inputs)
    require(len(audits) == 5 and {a["function"] for a in audits}
            == {"argument", "compute_distances", "workload", "guest_main", "repeat_main"}
            and all(a.get("match") is True and a.get("diffs") == [] and type(a.get("elf_count")) is int
                    and a["elf_count"] > 0 and a["elf_count"] == a.get("macho_count") for a in audits),
            "incomplete computation/repeat instruction audit")
    imported = {"computation.S": "assembly/streamcluster-linux.s", "repeat.S": "source/repeat-linux.S",
                "start.S": "source/repeat-start.S", "repeat.c": "source/repeat.c"}
    source_hashes = {}
    for name, source in imported.items():
        path = macho.REPEAT_PREFIX+name
        source_hashes[path] = macho.digest(evidence/source)
        require(source_hashes[path] == macho.digest(root/path), "frozen measured input changed")
    native_sha = macho.digest(evidence/"bin/streamcluster-repeat")
    elf_sha = macho.digest(evidence/"bin/streamcluster-repeat.elf")
    require(elf_sha == macho.digest(root/"tests/benchmarks/streamcluster-repeat.elf"), "measured ELF changed")
    rows, proofs = {}, []
    cases = {c["id"]: c for c in declared["cases"]}
    for ident, capture_id, n in zip(ids, selected, macho.STREAMCLUSTER_SIZES):
        c = configs[capture_id]
        require(c["n"] == n and c["k"] == 16 and c["candidate"] is True
                and c.get("qos") == "inherited" and c["binary_sha256"] == native_sha, "selected native executable changed")
        block = (blocks/f"{n}.bin").read_bytes()
        expected = inputs["expected"][str(n)]
        require(len(block) == n*8 == expected["bytes"] and hashlib.sha256(block).hexdigest() == expected["sha256"],
                "output block differs from complete captured native outputs")
        require(all(macho.math.isfinite(v[0]) for v in struct.iter_unpack("<d", block)), "nonfinite output block")
        digest = hashlib.sha256()
        for _ in range(16):
            digest.update(block)
        sha = digest.hexdigest()
        samples = [s for s in raw["samples"] if s["id"] == capture_id]
        row = performance.summarize(cases[ident], samples, sha, n*8*16, "shape_finite")
        batches = [statistics.median(s["user_seconds"] for s in samples if s["batch"] == b) for b in range(3)]
        row.update(batch_median_user_seconds=batches, batch_median_spread=(max(batches)-min(batches))/row["median_user_seconds"])
        require(row["quality"] == "usable", "selected replacement did not meet unchanged quality policy")
        rows[ident] = row
        proofs.append(dict(id=ident, block_sha256=expected["sha256"], block_bytes=len(block),
                           output_sha256=sha, output_bytes=n*8*16))
    result = previous.copy()
    result["cases"] = [rows.get(r["id"], r) for r in previous["cases"]]
    result["historical_input_sources"] = previous["input_sources"]
    active = dict(declared["compute"]["build"]["files"])
    active.update({contract.PERFORMANCE_MANIFEST: macho.digest(root/contract.PERFORMANCE_MANIFEST),
                   "tests/benchmarks/provenance.json": macho.digest(root/"tests/benchmarks/provenance.json")})
    active.update({"tests/usertime/"+k: v for k,v in declared["original"]["sources"].items()})
    active.update({"tests/usertime/"+c["file"]: c["sha256"] for c in declared["original"]["cases"]})
    result.update(input_sources=active, inventory_sha256=compute.object_digest(declared),
                  micro_manifest_sha256=active[contract.PERFORMANCE_MANIFEST],
                  build_manifest_sha256=active["tests/benchmarks/provenance.json"])
    result["streamcluster_replacement"] = dict(
        case_ids=ids, base_commit=base, source_origin="local-uncommitted-experiment", collection_dirty=True,
        samples=126, warmups=6, audited_functions=5, passes=16,
        started_at_utc=raw["started_at"], completed_at_utc=raw["completed_at"], page_size=16384, hardware="Apple M2",
        metric="child_user_cpu_seconds", timer="wait4.ru_utime", startup_included=True,
        core_identity="unknown", instantaneous_frequency="unknown", input_sources=source_hashes,
        capture_sha256=macho.digest(evidence/"focused.json"), inputs_sha256=raw["inputs_sha256"],
        collector_sha256=raw["collector_sha256"], audit_sha256=macho.digest(evidence/"evidence/instruction-audit.json"),
        elf_sha256=elf_sha, native_sha256=native_sha,
        matched_assembly_sha256=compute.object_digest({n:macho.digest(evidence/n) for n in ("assembly/streamcluster-matched.S", "source/repeat-mac.S")}),
        output_proof_sha256=compute.object_digest(proofs), original_reference_sha256=hashlib.sha256(previous_bytes).hexdigest(),
        rows_sha256=compute.object_digest(list(rows.values())),
        retained_rows_sha256=compute.object_digest([r for r in result["cases"] if r["id"] not in ids]))
    performance.validate(result, declared)
    compute.write(output, result)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--output-blocks", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    export(args.root, args.evidence, args.output_blocks, args.output)
