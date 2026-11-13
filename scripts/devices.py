#!/usr/bin/env python3
"""Modeled devices: the hardware a run describes, whose parameters it carries
and which native references exist for it.

The simulator calls a device a target (`poros elf --target`). These scripts
say device, because their target is a Git revision. A revision or
report without a declaration predates device selection and describes DEFAULT.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]
PATH = "evaluation/devices.json"
DEFAULT = "m2"
# Reference kinds: whole-program user time, then one per diagnostic suite.
KINDS = ("application", "throughput", "forwarding", "scheduling", "capacity", "recurrence", "memory")
UNMEASURED = "unmeasured"
BASE_UNAVAILABLE = "device_unavailable"
NAME = re.compile(r"[a-z0-9][a-z0-9-]*")


def encode(declaration):
    return (json.dumps(declaration, sort_keys=True, indent=2) + "\n").encode()


def check(declaration):
    if (not isinstance(declaration, dict) or set(declaration) != {"schema", "default", "devices"}
            or declaration["schema"] != 1 or declaration["default"] != DEFAULT
            or not isinstance(declaration["devices"], list)):
        raise ValueError("invalid device declaration")
    entries = declaration["devices"]
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != {"id", "hardware", "parameter_source", "references"}:
            raise ValueError("invalid device entry")
        if (not isinstance(entry["id"], str) or not NAME.fullmatch(entry["id"])
                or not isinstance(entry["hardware"], str) or not entry["hardware"].strip()
                or not isinstance(entry["references"], dict) or set(entry["references"]) - set(KINDS)):
            raise ValueError("invalid device identity or reference kinds")
        for path in entry["references"].values():
            if (not isinstance(path, str) or not path or path.startswith("/") or "\\" in path
                    or any(part in ("", ".", "..") for part in path.split("/"))):
                raise ValueError("unsafe device reference path")
    ids = [entry["id"] for entry in entries]
    if len(set(ids)) != len(ids) or DEFAULT not in ids or len({entry["hardware"] for entry in entries}) != len(ids):
        raise ValueError("devices need distinct identities and hardware, including " + DEFAULT)
    own = {entry["id"] for entry in entries if entry["parameter_source"] == entry["id"]}
    if any(entry["parameter_source"] not in own for entry in entries):
        raise ValueError("a device's parameters must come from a device that has its own")
    return declaration


def select(declaration, ident):
    """The declared device, or an error naming the valid ones; never another device."""
    for entry in declaration["devices"]:
        if entry["id"] == ident:
            return entry
    raise ValueError(f"unknown device {ident!r} (declared: {', '.join(e['id'] for e in declaration['devices'])})")


def load(root=ROOT):
    raw = (Path(root) / PATH).read_bytes()
    declaration = check(json.loads(raw))
    if raw != encode(declaration):
        raise ValueError("device declaration must use canonical JSON")
    return declaration


def revision(root, commit):
    """The declaration at an exact revision; None when the revision predates it."""
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("device declaration requires full revision")
    git = ["git", "-C", str(root)]
    if PATH not in subprocess.check_output([*git, "ls-tree", "-r", "--name-only", commit], text=True).splitlines():
        return None
    raw = subprocess.check_output([*git, "show", commit + ":" + PATH])
    declaration = check(json.loads(raw))
    if raw != encode(declaration):
        raise ValueError("device declaration must use canonical JSON")
    return dict(commit=commit, path=PATH, sha256=hashlib.sha256(raw).hexdigest(), declaration=declaration)


def resolve(root, state, base, requested=None):
    """Bind a requested device to the head and base revisions. None means legacy DEFAULT."""
    if PATH not in state["files"]:
        if requested not in (None, DEFAULT):
            raise ValueError(f"source predates device declarations; it models only {DEFAULT}, not {requested!r}")
        return None
    head = revision(root, state["commit"])
    if head is None or head["sha256"] != state["files"][PATH]:
        raise ValueError("working device declaration differs from committed source")
    ident = head["declaration"]["default"] if requested is None else requested
    select(head["declaration"], ident)
    chosen = dict(id=ident, head=head, base=revision(root, base))
    require_shared_parameters(chosen)
    return chosen


def declared_at(chosen, role):
    # A failure written before the declarations were read names only the device.
    if chosen is None or chosen.get(role) is None:
        return None
    return next((e for e in chosen[role]["declaration"]["devices"] if e["id"] == chosen["id"]), None)


def require_shared_parameters(chosen):
    # Plans and reports resolve only DEFAULT's model profile and the workload
    # contract's clock. A device with parameters of its own declares them in
    # profile_path(), but accuracy campaigns and reports cannot bind them yet.
    for role in ("head", "base"):
        declared = declared_at(chosen, role)
        if declared is not None and declared["parameter_source"] != DEFAULT:
            raise ValueError(f"device {declared['id']!r} declares its own parameters; no per-device model profile is resolved yet")


def profile_path(declared):
    """The model profile a device's executions report: DEFAULT's for a device
    that carries DEFAULT's parameters, otherwise its parameter source's own."""
    source = declared["parameter_source"]
    return "evaluation/model-profile.json" if source == DEFAULT else f"evaluation/model-profile-{source}.json"


def load_profile(declared, root=ROOT):
    return json.loads((Path(root) / profile_path(declared)).read_text())


def at_profile_clock(case, profile):
    """A whole-program case converted at the profile's clock instead of the
    workload contract's assumed clock (identical for DEFAULT's profile)."""
    return dict(case, frequency_hz=profile["configuration"]["freq"])


def ident(value):
    return value["device"]["id"] if "device" in value else DEFAULT


def entry(value, role="head"):
    """A plan's or report's device as declared at a role's revision, if declared there."""
    return declared_at(value.get("device"), role)


def available(value, role):
    """Whether a role's simulator models the device. An undeclared base revision is DEFAULT only."""
    if "device" not in value or role != "base":
        return True
    if value["device"].get("base") is None:
        return value["device"]["id"] == DEFAULT
    return entry(value, "base") is not None


def identity(value, role):
    """What a role's simulator must report; None for a revision that predates targets."""
    declared = entry(value, "base" if role == "base" else "head")
    return None if declared is None else target(declared)


def target(declared):
    """A declared device in the simulator's own terms."""
    return dict(name=declared["id"], hardware=declared["hardware"], parameter_source=declared["parameter_source"])


def options(value, role):
    """Keyword arguments for execution and validation helpers of one role."""
    device = identity(value, role)
    return {} if device is None else {"device": device}


def arguments(device):
    return [] if device is None else ["--target", device["name"]]


def check_timing(timing, device):
    if device is not None and timing.get("target") != device:
        raise ValueError(f"simulated device mismatch: requested {device['name']!r}, simulator reported {timing.get('target')!r}")


def measured(value, kind):
    """Whether a native reference of this kind exists for the device. Legacy values are DEFAULT's."""
    declared = entry(value)
    return True if declared is None else kind in declared["references"]


def require_reference(value, kind, reference):
    """A native reference describes only the hardware it was measured on."""
    declared = entry(value)
    if declared is not None and reference.get("hardware") != declared["hardware"]:
        raise ValueError(f"{kind} reference was measured on {reference.get('hardware')!r}, not {declared['hardware']!r}")


def bind_reference(value, kind, path, reference):
    """Accept a loaded native reference for a plan's or report's device.

    The device must declare exactly this file for the kind, and the file must
    have been measured on the device's hardware. Values without a declaration
    are the default device with its original files."""
    declared = entry(value)
    if declared is not None and declared["references"].get(kind) != path:
        raise ValueError(f"device {declared['id']} declares its {kind} reference at {declared['references'].get(kind)}, not {path}")
    require_reference(value, kind, reference)


def reference_status(declared, kinds=KINDS):
    present = [kind for kind in kinds if kind in declared["references"]]
    return "measured" if len(present) == len(kinds) else "partial" if present else UNMEASURED


def parameter_text(declared, declaration):
    source = select(declaration, declared["parameter_source"])
    if source is declared:
        return f"{declared['hardware']} model profile"
    return f"{source['hardware']} model profile (placeholder; {declared['hardware']} parameters not established)"


def validate(report):
    """Offline archive validation. Publication additionally checks both Git revisions."""
    chosen = report.get("device")
    if PATH not in report["head"]["files"]:
        if chosen is not None:
            raise ValueError("device declaration absent from tested source")
        return None
    if not isinstance(chosen, dict) or set(chosen) != {"id", "head", "base"} or chosen["head"] is None:
        raise ValueError("missing device selection")
    for role, commit in (("head", report["head"]["commit"]), ("base", report["base_commit"])):
        item = chosen[role]
        if item is None:
            continue
        if (not isinstance(item, dict) or set(item) != {"commit", "path", "sha256", "declaration"}
                or item["commit"] != commit or item["path"] != PATH
                or hashlib.sha256(encode(check(item["declaration"]))).hexdigest() != item["sha256"]):
            raise ValueError(role + " device declaration identity mismatch")
    if chosen["head"]["sha256"] != report["head"]["files"][PATH]:
        raise ValueError("head device declaration differs from tested source")
    declared = select(chosen["head"]["declaration"], chosen["id"])
    require_shared_parameters(chosen)
    return declared


def verify_source(root, report):
    """Check a report's device against both exact Git revisions.

    A report that names no device and lists no declaration claims to predate
    device selection. That holds only if its head revision has no declaration;
    leaving both out of a report of a later revision is not a way to be DEFAULT."""
    declared = validate(report)
    if declared is None:
        if revision(root, report["head"]["commit"]) is not None:
            raise ValueError("report names no device, but its head revision declares devices")
        return None
    for role in ("head", "base"):
        commit = report["head"]["commit"] if role == "head" else report["base_commit"]
        if report["device"][role] != revision(root, commit):
            raise ValueError(role + " device declaration differs from exact Git revision")
    return declared


def smoke(binary):
    """Run the real simulator as every declared device, end to end.

    Each device executes a complete program and one point of every diagnostic
    suite through the executors and validators an accuracy campaign uses. Names
    the simulator does not build must fail, and the default command line must
    still be the default device."""
    import capacity_microbenchmark as capacity
    import forwarding_microbenchmark as forwarding
    import memory_microbenchmark as memory
    import recurrence_microbenchmark as recurrence
    import scheduling_microbenchmark as scheduling
    import throughput_microbenchmark as throughput
    import usertime as user
    import workload_contract as contract

    declaration = load()

    def run(*args):
        return subprocess.run([str(binary), *args], capture_output=True, text=True, timeout=120)

    listed = run("targets")
    if (listed.returncode != 0 or listed.stderr
            or json.loads(listed.stdout) != dict(default=declaration["default"], targets=[target(d) for d in declaration["devices"]])):
        raise ValueError("simulator targets differ from " + PATH)
    hello = str(ROOT / "tests/elf/hello.elf")
    rejections = [("elf", "--mode", mode, "--target", name, hello) for mode in ("functional", "inorder", "ooo") for name in ("m3", "M2", "")]
    rejections.append(("run", "--root", str(ROOT), "--target", "m3", "integer", "1024"))
    for args in rejections:
        rejected = run(*args)
        if rejected.returncode != 2 or rejected.stdout or "unknown target" not in rejected.stderr:
            raise ValueError("unknown target was not rejected: " + " ".join(args[:-1]))
    print("unknown targets rejected in every mode", flush=True)
    for mode in ("functional", "inorder", "ooo"):
        legacy, explicit = run("elf", "--mode", mode, hello), run("elf", "--mode", mode, "--target", DEFAULT, hello)
        if legacy.returncode != 0 or legacy.stderr or explicit.stderr or legacy.stdout != explicit.stdout:
            raise ValueError("command line without a target is no longer the default device: " + mode)
    print("command line without a target is byte-identical to --target " + DEFAULT, flush=True)

    contract_case = next(c for c in contract.load_manifest(ROOT)["cases"] if c["id"] == "mvt-63")
    probes = [(suite, suite.cases(ROOT)[0]) for suite in (throughput, forwarding, scheduling, capacity, recurrence)]
    cycles = {}
    for declared in declaration["devices"]:
        device = target(declared)
        profile = load_profile(declared)
        case = at_profile_clock(contract_case, profile)
        functional = user.execute(binary, case, mode="functional", device=device)
        timed = user.execute(binary, case, expected_profile=profile, device=device)
        user.validate_timing(timed, case)
        if any(functional[key] != timed[key] for key in ("instructions", "stdout_base64")):
            raise ValueError("functional/timing mismatch on " + declared["id"])
        samples = [timed]
        for suite, probe in probes:
            samples += suite.measure_case(binary, ROOT, probe, 1, None, profile, device=device)
        samples += memory.measure_case(binary, ROOT, memory.cases(ROOT)[0], None, profile, device=device)
        cycles[declared["id"]] = [sample["timing"]["user_cycles"] for sample in samples]
        # The same execution must not pass as any other device's.
        for other in declaration["devices"]:
            if other is not declared:
                try:
                    user.validate_execution(timed, at_profile_clock(contract_case, load_profile(other)),
                                            expected_profile=load_profile(other), device=target(other))
                except ValueError:
                    continue
                raise ValueError(f"{declared['id']} execution validated as {other['id']}")
        print(f"{declared['id']}: {declared['hardware']}; parameters: {parameter_text(declared, declaration)}; "
              f"native references: {reference_status(declared)}; {len(samples)} validated executions; "
              f"{case['id']} {timed['timing']['user_cycles']} cycles", flush=True)
    # A device that names another as its parameter source carries that device's
    # configuration, so it must reproduce its cycles until it gets its own.
    for declared in declaration["devices"]:
        if cycles[declared["id"]] != cycles[declared["parameter_source"]]:
            raise ValueError(declared["id"] + " differs from its declared parameter source " + declared["parameter_source"])
    print("devices sharing a parameter source reproduce its cycles", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("list")
    smoke_parser = commands.add_parser("smoke")
    smoke_parser.add_argument("--poros", required=True, type=Path)
    args = parser.parse_args()
    if args.command == "smoke":
        smoke(args.poros.resolve())
        return
    declaration = load()
    for declared in declaration["devices"]:
        print(f"{declared['id']}\t{declared['hardware']}\tparameters: {parameter_text(declared, declaration)}"
              f"\treferences: {reference_status(declared)}")


if __name__ == "__main__":
    main()
