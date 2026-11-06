#!/usr/bin/env python3
"""Native child user-CPU measurements and frozen base/head accuracy reports.

No simulator-host elapsed time or kernel CPU time is used as guest user time.
"""

import base64
import hashlib
import json
import math
import os
from pathlib import Path
import platform
import statistics
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
from workload_contract import case_ids, check_output, digest, expected, load_manifest, manifest_path


def inventory():
    return load_manifest(ROOT, performance=True)


def child_usage(executable, argv, env, timeout=10):
    """Direct child only, no descendants; wait4 gives per-child accounting.

    stdout/stderr are files to avoid pipe-capacity deadlocks. This runner is for
    trusted benchmarks, not an untrusted-program sandbox.
    """
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        p = subprocess.Popen(
            argv, executable=str(executable), env=env, stdout=out, stderr=err
        )
        deadline = time.monotonic() + timeout
        while True:
            pid, status, usage = os.wait4(p.pid, os.WNOHANG)
            if pid:
                p.returncode = os.waitstatus_to_exitcode(status)
                break
            if time.monotonic() >= deadline:
                p.kill()
                _, status, usage = os.wait4(p.pid, 0)
                p.returncode = os.waitstatus_to_exitcode(status)
                raise ValueError("native child timeout")
            time.sleep(0.001)
        out.seek(0)
        err.seek(0)
        return dict(
            user_seconds=usage.ru_utime,
            system_seconds=usage.ru_stime,
            exit_status=p.returncode,
            stdout_base64=base64.b64encode(out.read()).decode(),
            stderr_base64=base64.b64encode(err.read()).decode(),
        )


def quality(samples):
    values = [s["user_seconds"] for s in samples]
    if not values or any(not math.isfinite(v) or v <= 0 for v in values):
        return "invalid_nonpositive"
    median = statistics.median(values)
    if median < 0.0001:
        return "below_precision_floor"
    ordered = sorted(values)
    if (
        ordered[int(0.9 * (len(values) - 1))] - ordered[int(0.1 * (len(values) - 1))]
    ) / median > 0.2:
        return "noisy"
    return "usable"


def validate_capture_metadata(reference):
    """Strict metadata for the explicit performance-v1 micro cohort."""
    import re
    from datetime import datetime
    if (reference.get("schema") != 2 or reference.get("metric") != "child_user_cpu_seconds"
            or reference.get("timer") != "wait4.ru_utime" or reference.get("environment") != "virtualized"
            or reference.get("machine") not in ("aarch64", "arm64")
            or reference.get("collection_dirty") is not False
            or not re.fullmatch(r"[0-9a-f]{40}", reference.get("collection_commit", ""))
            or not re.fullmatch(r"sha256:[0-9a-f]{64}", reference.get("container_image", ""))
            or reference.get("retained_samples_per_case") != 21
            or reference.get("precision_floor_seconds") != .0001
            or reference.get("quality_policy") != {"nonpositive": "reject-any", "precision_floor_seconds": .0001, "relative_p10_p90_limit": .2}):
        raise ValueError("performance capture metadata mismatch")
    try:
        start = datetime.strptime(reference["started_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
        end = datetime.strptime(reference["completed_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
        if end < start:
            raise ValueError("completion predates capture")
    except (KeyError, ValueError):
        raise ValueError("invalid capture timestamps") from None


def capture(args):
    started = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    m = inventory()
    if platform.system() != "Linux" or platform.machine() not in ("aarch64", "arm64"):
        raise ValueError(
            "same-ELF capture requires native Linux ARM64; QEMU is not a timing reference"
        )
    if args.samples != 21:
        raise ValueError("performance capture requires exactly 21 retained samples")
    git = ["git", "-c", "safe.directory=" + str(ROOT), "-C", str(ROOT)]
    commit = subprocess.check_output([*git, "rev-parse", "HEAD"], text=True).strip()
    if subprocess.check_output([*git, "status", "--porcelain"]):
        raise ValueError("formal capture requires clean committed source")
    if Path(args.output).exists():
        raise ValueError("never overwrite formal raw measurements")
    import re
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", os.environ.get("POROS_REFERENCE_IMAGE", "")):
        raise ValueError("formal capture requires exact native container image")
    report = dict(
        schema=2,
        collection_commit=commit,
        collection_dirty=False,
        retained_samples_per_case=21,
        quality_policy={"nonpositive": "reject-any", "precision_floor_seconds": .0001, "relative_p10_p90_limit": .2},
        started_at_utc=started,
        measurement_id=args.measurement_id,
        metric="child_user_cpu_seconds",
        environment=args.environment,
        hardware=args.hardware,
        container_image=os.environ.get("POROS_REFERENCE_IMAGE", "unknown"),
        compiler=m["compiler"],
        source_hashes=m["sources"],
        platform=platform.platform(),
        machine=platform.machine(),
        kernel=platform.release(),
        core_identity="unknown",
        instantaneous_frequency="unknown",
        timer="wait4.ru_utime",
        precision_floor_seconds=0.0001,
        precision_floor_basis="conservative protocol policy, not measured timer resolution",
        warmup="one complete process run; OS page cache not flushed",
        harness_sha256=digest(__file__),
        harness_path="scripts/native_usertime.py",
        support_sha256={
            "workload_contract.py": digest(
                Path(__file__).with_name("workload_contract.py")
            )
        },
        manifest_sha256=digest(ROOT / manifest_path(m)),
        cases=[],
    )
    Path(args.output).parent.mkdir(parents=True, exist_ok=True)
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n")
    for c in m["cases"]:
        executable = ROOT / "tests/usertime" / c["file"]
        samples = []
        golden = expected(c)
        time.sleep(1)
        for i in range(args.samples + 1):
            sample = child_usage(
                executable, c["args"], dict(x.split("=", 1) for x in c["env"])
            )
            if sample["exit_status"] != 0 or sample["stderr_base64"]:
                raise ValueError("native correctness failed: " + c["id"])
            check_output(c, base64.b64decode(sample["stdout_base64"]), golden)
            if i:
                samples.append(sample)
            else:
                warmup_sample = sample
        report["cases"].append(
            dict(
                c,
                warmup_sample=warmup_sample,
                samples=samples,
                median_user_seconds=statistics.median(
                    s["user_seconds"] for s in samples
                ),
                quality=quality(samples),
            )
        )
        Path(args.output).write_text(json.dumps(report, indent=2) + "\n")
        print(c["id"], report["cases"][-1]["quality"], flush=True)
    report["completed_at_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n")


def measure(args):
    started = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    if args.samples < 5 or not args.program:
        raise ValueError("need program and at least5samples")
    program = args.program[1:] if args.program[0] == "--" else args.program
    executable = Path(program[0]).resolve()
    samples = [child_usage(executable, program, {}) for _ in range(args.samples)]
    if any(s["exit_status"] != 0 for s in samples):
        raise ValueError("native program failed")
    r = dict(
        schema=1,
        measurement_id=args.measurement_id,
        started_at_utc=started,
        completed_at_utc=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        harness_sha256=digest(__file__),
        harness_path="scripts/native_usertime.py",
        precision_floor_seconds=0.0001,
        precision_floor_basis="conservative protocol policy, not measured timer resolution",
        timer="wait4.ru_utime",
        kernel=platform.release(),
        machine=platform.machine(),
        core_identity="unknown",
        instantaneous_frequency="unknown",
        metric="child_user_cpu_seconds",
        platform=platform.platform(),
        environment=args.environment,
        hardware=args.hardware,
        binary_sha256=digest(executable),
        args=program,
        env=[],
        samples=samples,
        quality=quality(samples),
        accuracy_status="unverified_different_binary",
        scope="direct child entire user CPU time; no grandchildren; no same-ELF claim",
    )
    r["build_info"] = (
        json.loads(Path(args.build_info).read_text())
        if args.build_info
        else {"status": "unknown"}
    )
    Path(args.output).write_text(json.dumps(r, indent=2) + "\n")
