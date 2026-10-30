#!/usr/bin/env python3
"""Build ordinary dynamically linked fixtures without changing Linux sources.

The runtime.h prefix (types and number parser) and workload C are shared. Only
its platform output function is replaced. Branch-arm compiler barriers preserve
an actual data-dependent branch; they emit no instructions or timing hooks.
"""
import hashlib
import json
import pathlib
import platform
import re
import shutil
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent
SOURCE = ROOT.parent / "usertime"
FLAGS = ["-target", "arm64-apple-macos15", "-O1", "-funsigned-char",
         "-ffp-contract=off", "-fno-vectorize", "-fno-slp-vectorize",
         "-fno-unroll-loops", "-fno-stack-protector"]

def main():
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        raise SystemExit("fixture build requires an ARM64 macOS SDK/toolchain")
    compiler = subprocess.check_output(["xcrun", "--find", "clang"], text=True).strip()
    sdk = subprocess.check_output(["xcrun", "--sdk", "macosx", "--show-sdk-path"], text=True).strip()
    metadata = {"sdk": sdk, "compiler": subprocess.check_output([compiler, "--version"], text=True),
                "flags": FLAGS, "sources": {}, "binaries": {}}
    for path in [SOURCE / "program.c", SOURCE / "mvt.c", SOURCE / "runtime.h",
                 ROOT / "main.c", ROOT / "runtime-output.h", pathlib.Path(__file__)]:
        metadata["sources"][str(path.relative_to(ROOT.parent.parent))] = hashlib.sha256(path.read_bytes()).hexdigest()
    with tempfile.TemporaryDirectory(prefix="poros-macho-build-") as directory:
        stage = pathlib.Path(directory)
        runtime = (SOURCE / "runtime.h").read_text()
        marker = "static int output("
        if runtime.count(marker) != 1:
            raise SystemExit("shared runtime output boundary changed")
        (stage / "runtime.h").write_text(runtime.split(marker)[0] + (ROOT / "runtime-output.h").read_text())
        program = (SOURCE / "program.c").read_text()
        branch = "if (value & 0x10000) value ^= 0x1234; else value += 7;"
        if program.count(branch) != 1:
            raise SystemExit("shared branch source changed")
        program = program.replace(branch, 'if (value & 0x10000) { value ^= 0x1234; asm volatile("// taken" : "+r"(value)); } else { value += 7; asm volatile("// not taken" : "+r"(value)); }')
        (stage / "program.c").write_text(program)
        shutil.copyfile(SOURCE / "mvt.c", stage / "mvt.c")
        for name, case in [("integer", 1), ("memory", 2), ("branch", 3), ("mvt", None)]:
            args = [] if case is None else [f"-DCASE={case}"]
            output = ROOT / f"{name}.macho"
            subprocess.run([compiler, "-isysroot", sdk, *FLAGS, *args, str(stage / ("mvt.c" if case is None else "program.c")), str(ROOT / "main.c"), "-o", str(output)], check=True)
            metadata["binaries"][output.name] = hashlib.sha256(output.read_bytes()).hexdigest()
            if name == "branch":
                disassembly = subprocess.check_output(["xcrun", "llvm-objdump", "-d", str(output)], text=True)
                body = disassembly.split("<_guest_main>:", 1)[1].split("<_main>:", 1)[0]
                guards = [line.strip() for line in body.splitlines()
                          if re.search(r"\btbn?z\s+w[0-9]+,\s*#(?:0x10|16),", line)]
                if len(guards) != 1:
                    raise ValueError("branch workload lost its bit-16 data-dependent branch")
                metadata["branch_guard"] = guards[0]
    (ROOT / "build.json").write_text(json.dumps(metadata, indent=2) + "\n")

if __name__ == "__main__":
    main()
