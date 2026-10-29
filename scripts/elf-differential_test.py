#!/usr/bin/env python3
"""Prove differential gate failures cannot silently turn into success."""
import copy
import base64
import math
import struct
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("elf_gate", Path(__file__).with_name("elf-differential.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


class GateTests(unittest.TestCase):
    def test_clean_gate(self):
        gate.require_clean_state({"git_dirty": False, "git_sha": "abc"})
        for state in ({"git_dirty": True, "git_sha": "abc"}, {"git_dirty": False, "git_sha": ""}):
            with self.assertRaises(ValueError):
                gate.require_clean_state(state)

    def test_required_inventory(self):
        root = gate.ROOT / "tests/elf"
        manifest = json.loads((root / "manifest.json").read_text())
        gate.inventory(manifest, root)
        for mutation in (manifest["cases"][:-1], [], manifest["cases"] + [manifest["cases"][0]]):
            bad = dict(manifest, cases=mutation)
            with self.assertRaises(ValueError):
                gate.inventory(bad, root)

    def test_missing_skipped_failed(self):
        rows = [{"name": name, "status": "pass"} for name in gate.REQUIRED]
        gate.require_complete(rows)
        for status in ("skip", "fail", "pending", None):
            bad = copy.deepcopy(rows)
            bad[0]["status"] = status
            with self.assertRaises(ValueError):
                gate.require_complete(bad)
        with self.assertRaises(ValueError):
            gate.require_complete(rows[:-1])

    def test_reference_and_output_must_match(self):
        manifest = json.loads((gate.ROOT / "tests/elf/manifest.json").read_text())
        case = manifest["cases"][0]
        p = {"termination": "exit", "exit_status": 0, "instructions": 51,
             "stdout_base64": "aGVsbG8sIEVMRgo=", "stderr_base64": "c3RkZXJyCg=="}
        reference = dict(p, instructions=None)
        gate.validate(case, p, reference)
        for field, value in (("exit_status", 1), ("termination", "time_limit"),
                             ("stdout_base64", ""), ("stderr_base64", "")):
            bad = dict(reference)
            bad[field] = value
            with self.assertRaises(ValueError):
                gate.validate(case, p, bad)
        with self.assertRaises(ValueError):
            gate.validate(case, dict(p, instructions=None), reference)

    def test_numerical_oracle(self):
        manifest = json.loads((gate.ROOT / "tests/elf/manifest.json").read_text())
        case = next(c for c in manifest["cases"] if c["name"] == "compute")
        p = {"termination": "exit", "exit_status": 0, "instructions": 100,
             "stdout_base64": base64.b64encode(bytes.fromhex(case["stdout_hex"])).decode(), "stderr_base64": ""}
        gate.validate(case, p, p)
        def altered(value):
            b = struct.pack("<Qdddd", 96, 3.25, .875, -2**-54, value)
            return dict(p, stdout_base64=base64.b64encode(b).decode())
        gate.validate(case, altered(math.nextafter(.1, 1)), p)
        for value in (math.nan, math.inf, -math.inf, .11):
            with self.assertRaises(ValueError):
                gate.validate(case, altered(value), p)
            with self.assertRaises(ValueError):
                gate.validate(case, p, altered(value))
        for count in (True, 0, case["max_instructions"] + 1):
            with self.assertRaises(ValueError):
                gate.validate(case, dict(p, instructions=count), p)


if __name__ == "__main__":
    unittest.main()
