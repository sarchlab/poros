#!/usr/bin/env python3
"""Record exact source/build inputs only after all ELF compilations succeed."""
import hashlib
import json
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parents[2]
cc = sys.argv[1]
sys.path.insert(0, str(root / "scripts"))
from workload_contract import mapped_image_bytes
performance_path = root / "tests/benchmarks/performance.json"
performance = json.loads(performance_path.read_text())
for workload in performance["workloads"]:
    workload["mapped_image_bytes"] = mapped_image_bytes(root / workload["binary"])
performance_path.write_text(json.dumps(performance, indent=2) + "\n")
manifest = json.loads((root / "tests/benchmarks/manifest.json").read_text())
files = {"tests/benchmarks/manifest.json", "tests/benchmarks/performance.json", "tests/benchmarks/runtime.h",
         "tests/benchmarks/build.sh", "tests/benchmarks/provenance.py",
         "tests/usertime/start.S", "tests/usertime/layout.ld"}
files.update(str(p.relative_to(root)) for p in (root / "tests/benchmarks/support").iterdir() if p.suffix in (".c", ".elf"))
for workload in manifest["workloads"]:
    files.update(workload[key] for key in ("source", "kernel", "binary"))
compiler = subprocess.check_output([cc, "--version"], text=True).splitlines()[0]
linker = subprocess.check_output([cc, "-Wl,--version"], text=True).splitlines()[0]
record = {"schema": 1, "compiler": compiler, "linker": linker,
          "libraries": {lib: hashlib.sha256(pathlib.Path(subprocess.check_output([cc, "-print-file-name=" + lib], text=True).strip()).read_bytes()).hexdigest() for lib in ("libc.a", "libgcc.a")},
          "files": {name: hashlib.sha256((root / name).read_bytes()).hexdigest()
                    for name in sorted(files)}}
(root / "tests/benchmarks/provenance.json").write_text(json.dumps(record, indent=2) + "\n")
