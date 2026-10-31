#!/usr/bin/env python3
"""Refresh source/ELF identities without changing either inventory's coordinates."""
import hashlib
import json
from pathlib import Path
import sys
import subprocess

root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(root / "scripts"))
from workload_contract import mapped_image_bytes

compiler = subprocess.check_output([sys.argv[1], "--version"], text=True).splitlines()[0]
folder = root / "tests/usertime"
for filename in ("manifest.json", "performance.json"):
    path = folder / filename
    manifest = json.loads(path.read_text())
    manifest["compiler"] = compiler
    manifest["sources"]["provenance.py"] = ""
    manifest["sources"] = {name: hashlib.sha256((folder / name).read_bytes()).hexdigest() for name in manifest["sources"]}
    for case in manifest["cases"]:
        case["sha256"] = hashlib.sha256((folder / case["file"]).read_bytes()).hexdigest()
        if filename == "performance.json":
            case["mapped_image_bytes"] = mapped_image_bytes(folder / case["file"])
    path.write_text(json.dumps(manifest, indent=2) + "\n")
