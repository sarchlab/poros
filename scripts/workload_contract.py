"""Versioned workload inventories and independent output checks.

Schema 1 is read-only compatibility for already-published six-case evidence.
Schema 2 declares four six-point sweeps; the manifest owns their coordinates.
"""

import functools
import hashlib
import json
import math
from pathlib import Path
import re
import struct

LEGACY_IDS = tuple(
    f"{w}-{n}" for w in ("integer", "memory", "branch") for n in (2000000, 8000000)
)
ROLES = {
    "integer": "calibration",
    "memory": "held_out",
    "branch": "held_out",
    "mvt": "calibration",
}
SHA256 = re.compile(r"[0-9a-f]{64}")
NAME = re.compile(r"[A-Za-z0-9_][A-Za-z0-9_.-]*")
MVT_REL_TOL = 1e-12
MVT_ABS_TOL = 1e-11


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def case_ids(manifest):
    return tuple(c["id"] for c in manifest["cases"])


def validate_manifest(m):
    if m.get("schema") == 1:
        if case_ids(m) != LEGACY_IDS:
            raise ValueError("missing required legacy user-time cases")
        return m
    if m.get("schema") != 2 or m.get("suite") != "whole-program-six-point-v2":
        raise ValueError("unsupported workload manifest")
    if m.get("reference_file") != "m2-linuxvm-user-v2.json":
        raise ValueError("invalid reference file")
    groups = m["workloads"]
    if [g["id"] for g in groups] != list(ROLES):
        raise ValueError("missing/reordered required workload groups")
    expected = []
    for g in groups:
        name = g["id"]
        if g["split"] != ROLES[name]:
            raise ValueError("workload split changed")
        points = g["points"]
        if (
            len(points) != 6
            or any(type(n) is not int or n < 1 for n in points)
            or points != sorted(set(points))
        ):
            raise ValueError("each sweep needs six distinct increasing positive points")
        if g["axis"] != (
            "working_set_bytes"
            if name == "memory"
            else "dimension" if name == "mvt" else "iterations"
        ):
            raise ValueError("invalid sweep axis")
        if name == "memory" and (
            g.get("iterations") != 4194304
            or any(n < 8 or n > 33554432 or n & (n - 1) for n in points)
        ):
            raise ValueError("invalid memory sweep")
        if name == "mvt" and any(n > 1536 for n in points):
            raise ValueError("MVT dimension out of range")
        if name in ("integer", "branch") and any(n > 10000000 for n in points):
            raise ValueError("iteration limit exceeded")
        for n in points:
            expected.append((g, n))
    if len(m["cases"]) != len(expected):
        raise ValueError("missing required user-time cases")
    for c, (g, n) in zip(m["cases"], expected):
        args = (
            ["program", str(g["iterations"]), str(n)]
            if g["id"] == "memory"
            else ["program", str(n)]
        )
        wanted = dict(
            id=f'{g["id"]}-{n}',
            workload=g["id"],
            point=n,
            axis=g["axis"],
            split=g["split"],
            args=args,
            env=[],
            file=g["id"] + ".elf",
            model="ooo",
            frequency_hz=3490000000,
        )
        if any(c.get(k) != v for k, v in wanted.items()):
            raise ValueError(
                "case disagrees with declared sweep: " + c.get("id", "missing")
            )
        if not SHA256.fullmatch(c.get("sha256", "")):
            raise ValueError("invalid ELF digest")
    if not m.get("sources") or any(
        not NAME.fullmatch(k) or not SHA256.fullmatch(v)
        for k, v in m["sources"].items()
    ):
        raise ValueError("invalid source manifest")
    return m


def load_manifest(root):
    folder = Path(root) / "tests/usertime"
    m = validate_manifest(json.loads((folder / "manifest.json").read_text()))
    for name, sha in m["sources"].items():
        if digest(folder / name) != sha:
            raise ValueError("source hash mismatch: " + name)
    for c in m["cases"]:
        if digest(folder / c["file"]) != c["sha256"]:
            raise ValueError("ELF hash mismatch: " + c["id"])
    return m


@functools.lru_cache(maxsize=64)
def mvt_reference(n):
    # Exact integer numerators for A*y, converted once. This does not duplicate
    # the guest's sequential floating-point dot-product implementation.
    first, second = [], []
    for i in range(n):
        p = sum(((i * j) % n) * ((j + 3) % n) for j in range(n))
        q = sum(((i * j) % n) * ((j + 4) % n) for j in range(n))
        first.append((i * n + p) / (n * n))
        second.append((((i + 1) % n) * n + q) / (n * n))
    return tuple(first + second)


@functools.lru_cache(maxsize=64)
def scalar_reference(kind, n, count=8192):
    value = 1
    if kind == "memory":
        # Sum each cell's arithmetic progression, independently of the guest loop.
        q, r = divmod(n, count)

        def contribution(k, cells, sum_indices):
            return (
                sum_indices * k * (k + 3) // 2
                + count * cells * k * (k - 1) * (k + 1) // 6
            )

        value += contribution(q + 1, r, r * (r - 1) // 2)
        value += contribution(q, count - r, count * (count - 1) // 2 - r * (r - 1) // 2)
    elif kind == "integer":
        a, b, power_a, power_b = 1, 0, 1664525, 1013904223
        while n:
            if n & 1:
                a, b = a * power_a & 0xFFFFFFFF, (b * power_a + power_b) & 0xFFFFFFFF
            power_a, power_b = (
                power_a * power_a & 0xFFFFFFFF,
                (power_b * (power_a + 1)) & 0xFFFFFFFF,
            )
            n >>= 1
        value = (a + b) & 0xFFFFFFFF
    elif kind == "branch":
        for _ in range(n):
            value = (value * 1664525 + 1013904223) & 0xFFFFFFFF
            value = value ^ 0x1234 if value & 0x10000 else value + 7
    else:
        raise ValueError("unknown output protocol")
    return (value & 0xFFFFFFFFFFFFFFFF).to_bytes(8, "little")


def expected(case):
    kind = case.get("workload", case["id"].split("-")[0])
    n = int(case["args"][1])
    if kind == "mvt":
        return struct.pack("<" + "d" * (2 * n), *mvt_reference(n))
    count = (
        int(case["args"][2]) // 8
        if kind == "memory" and len(case["args"]) == 3
        else 8192
    )
    return scalar_reference(kind, n, count)


def check_output(case, actual, golden=None):
    golden = expected(case) if golden is None else golden
    if case.get("workload") != "mvt":
        if actual != golden:
            raise ValueError("scalar result mismatch: " + case["id"])
        return
    n = int(case["args"][1])
    if len(actual) != 16 * n or len(golden) != len(actual):
        raise ValueError("MVT output length mismatch")
    for i, (got, want) in enumerate(
        zip(
            struct.unpack("<" + "d" * (2 * n), actual),
            struct.unpack("<" + "d" * (2 * n), golden),
        )
    ):
        if not math.isfinite(got) or not math.isclose(
            got, want, rel_tol=MVT_REL_TOL, abs_tol=MVT_ABS_TOL
        ):
            raise ValueError(f"MVT output element {i} mismatch: {got} versus {want}")
