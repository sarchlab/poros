"""Independent guest instruction counts for deterministic whole-program inputs."""
import hashlib
import json
import re
from pathlib import Path


PATH = "evaluation/usertime/functional-witness.json"


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def engine_key(files):
    # The functional CLI calls linuxuser directly. Exclude the timing-only
    # packages, but include the CLI, dependencies and new Go packages by default.
    return digest({name: value for name, value in files.items()
                   if name in ("go.mod", "go.sum") or (
                       name.endswith((".go", ".s", ".c", ".h")) and not name.endswith("_test.go")
                       and not name.startswith(("pkg/cpu/", "pkg/elfsim/", "pkg/mem/")))})


def input_key(case, output):
    return digest([case["sha256"], case["args"], case["env"], output.length, output.sha256])


def matches(root, state, cases, outputs):
    if PATH not in state["files"]:
        return {}
    raw = (Path(root) / PATH).read_bytes()
    return match_bytes(raw, state, cases, outputs)


def match_bytes(raw, state, cases, outputs):
    if hashlib.sha256(raw).hexdigest() != state["files"][PATH]:
        raise ValueError("functional witness file changed")
    evidence = json.loads(raw)
    if evidence["schema"] != 1:
        raise ValueError("unknown functional witness schema")
    if (not re.fullmatch(r"[0-9a-f]{40}", evidence["commit"])
            or not re.fullmatch(r"[0-9a-f]{64}", evidence["binary_sha256"])
            or not evidence["run_id"]):
        raise ValueError("invalid functional witness provenance")
    if evidence["engine_sha256"] != engine_key(state["files"]):
        return {}
    found = {}
    for case in cases:
        ident = case["id"]
        row = evidence["cases"].get(ident)
        if row is None or row["input_sha256"] != input_key(case, outputs[ident]):
            continue
        count = row["instructions"]
        if type(count) is not int or count <= 0:
            raise ValueError(f"{ident}: invalid functional witness instruction count")
        found[ident] = {"instructions": count, "commit": evidence["commit"],
                        "binary_sha256": evidence["binary_sha256"], "run_id": evidence["run_id"]}
    return found


def check(case, result, witness):
    if result["instructions"] != witness["instructions"]:
        raise ValueError(f"{case['id']}: functional witness expected {witness['instructions']} "
                         f"instructions, timing executed {result['instructions']}")
