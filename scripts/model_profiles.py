"""Revision-bound timing configuration, separate from measured workload inputs."""
import hashlib
import json
import re
import subprocess

PATH = "evaluation/model-profile.json"
LEGACY_PATH = "tests/usertime/performance.json"
LEGACY_PATHS = (LEGACY_PATH, "tests/usertime/manifest.json")


def encode(profile):
    return (json.dumps(profile, sort_keys=True, indent=2) + "\n").encode()


def check_profile(profile):
    if not isinstance(profile, dict) or set(profile) != {"configuration", "cache_config"}:
        raise ValueError("invalid model profile")
    if not all(isinstance(profile[k], dict) and profile[k] for k in profile):
        raise ValueError("empty model configuration")
    # Reject JSON NaN/Infinity as well as malformed declarations.
    json.dumps(profile, allow_nan=False)
    return profile


def revision(root, commit):
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("model profile requires full revision")
    git = ["git", "-C", str(root)]
    paths = subprocess.check_output([*git, "ls-tree", "-r", "--name-only", commit], text=True).splitlines()
    path = next((name for name in (PATH, *LEGACY_PATHS) if name in paths), None)
    if path is None:
        raise ValueError("revision has no declared model profile")
    raw = subprocess.check_output([*git, "show", commit + ":" + path])
    data = json.loads(raw)
    profile = check_profile(data if path == PATH else data["profile"])
    if path == PATH and raw != encode(profile):
        raise ValueError("model profile declaration must use canonical JSON")
    return dict(commit=commit, path=path, sha256=hashlib.sha256(raw).hexdigest(), profile=profile)


def resolve(root, state, base):
    """Legacy reports keep their existing frozen-profile protocol unchanged."""
    if PATH not in state["files"]:
        return None
    result = {"head": revision(root, state["commit"]), "base": revision(root, base)}
    if result["head"]["sha256"] != state["files"][PATH]:
        raise ValueError("working model declaration differs from committed source")
    return result


def validate(report):
    """Offline archive validation. Publication additionally checks both Git revisions."""
    legacy = report.get("workload_manifest", {}).get("profile")
    declarations = report.get("model_profiles")
    if PATH not in report["head"]["files"]:
        if declarations is not None:
            raise ValueError("model profiles absent from tested source")
        if legacy is None:
            raise ValueError("missing legacy model profile")
        return {"head": legacy, "base": legacy}
    if not isinstance(declarations, dict) or set(declarations) != {"head", "base"}:
        raise ValueError("missing head/base model declarations")
    result = {}
    for role, commit in (("head", report["head"]["commit"]), ("base", report["base_commit"])):
        entry = declarations[role]
        if not isinstance(entry, dict):
            raise ValueError(role + " model declaration must be an object")
        if (entry.get("commit") != commit or entry.get("path") not in (PATH, *LEGACY_PATHS)
                or not re.fullmatch(r"[0-9a-f]{64}", entry.get("sha256", ""))):
            raise ValueError("model profile source identity mismatch")
        profile = check_profile(entry["profile"])
        if entry["path"] == PATH:
            if hashlib.sha256(encode(profile)).hexdigest() != entry["sha256"]:
                raise ValueError("model profile declaration hash mismatch")
        elif profile != legacy:
            raise ValueError("legacy model profile differs from frozen declaration")
        if role == "head" and (entry["path"] != PATH or entry["sha256"] != report["head"]["files"][PATH]):
            raise ValueError("head model declaration differs from tested source")
        result[role] = profile
    return result


def verify_source(root, report):
    profiles = validate(report)
    if "model_profiles" in report:
        for role, entry in report["model_profiles"].items():
            if entry != revision(root, entry["commit"]):
                raise ValueError(role + " model declaration differs from exact Git revision")
    return profiles
