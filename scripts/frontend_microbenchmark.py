"""Measure the steady-state simulated IPC of the two fixed assembly loops."""
import hashlib
import json
from pathlib import Path
import subprocess

ELF = "evaluation/microbenchmarks/frontend.elf"
LOOP_INSTRUCTIONS = 258 * 4096


def measure(binary, root, revision, profile):
    elf = Path(root) / ELF
    digest = hashlib.sha256(elf.read_bytes()).hexdigest()
    rows = {}
    for loop in ("nop", "add"):
        samples = []
        for length in ("1", "2"):
            args = [str(binary), "elf", "--mode", "ooo", "--max-instructions", "2200000",
                    "--timeout", "90s", str(elf), loop, length]
            p = subprocess.run(args, capture_output=True, check=True, timeout=100)
            data = json.loads(p.stdout)
            timing = data["timing"]
            if (data["termination"] != "exit" or data["exit_status"] != 0 or data["binary_sha256"] != digest
                    or data["args"] != ["program", loop, length] or data["stderr_base64"] or data["stdout_base64"]
                    or data["build"].get("vcs.revision") != revision or data["build"].get("vcs.modified") != "false"
                    or data["instructions"] != timing["retired_instructions"]
                    or timing["configuration"] != profile["configuration"] or timing["cache_config"] != profile["cache_config"]):
                raise ValueError("incorrect frontend probe execution: " + loop)
            samples.append((data["instructions"], timing["user_cycles"]))
        inst = samples[1][0] - samples[0][0]
        cycles = samples[1][1] - samples[0][1]
        if inst != LOOP_INSTRUCTIONS or cycles <= 0:
            raise ValueError("incorrect frontend loop counters: " + loop)
        rows[loop] = dict(instructions=inst, cycles=cycles)
    return rows


def validate(rows):
    if not isinstance(rows, dict) or set(rows) != {"nop", "add"}:
        raise ValueError("missing frontend probe results")
    for row in rows.values():
        if row["instructions"] != LOOP_INSTRUCTIONS or type(row["cycles"]) is not int or row["cycles"] < 1:
            raise ValueError("invalid frontend probe counters")
