#!/usr/bin/env python3
import base64
import importlib.util
import json
import copy
import pathlib
import struct
import subprocess
import unittest

spec = importlib.util.spec_from_file_location("differential", pathlib.Path(__file__).with_name("benchmark-differential.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class DifferentialTest(unittest.TestCase):
    def test_complete_bytes_and_termination(self):
        case = {"outputs": [{"type": "double", "count": 2}, {"type": "int", "count": 1}]}
        expected = struct.pack("<ddi", 1, 2, 3)
        def result(data=expected, **override):
            fields = dict(termination="exit", exit_status=0, stderr_base64="", stdout_base64=base64.b64encode(data).decode())
            fields.update(override)
            return subprocess.CompletedProcess([], 0, json.dumps(fields).encode(), b"")
        module.check_result(case, result(), expected)
        for corrupted in [expected[:-1], expected+b"x", struct.pack("<ddi", 1, 3, 3), struct.pack("<ddi", 1, float("nan"), 3)]:
            with self.assertRaises(ValueError):
                module.check_result(case, result(corrupted), expected)
        for override in [dict(termination="instruction_limit"), dict(exit_status=2), dict(stderr_base64="eA==")]:
            with self.assertRaises(ValueError):
                module.check_result(case, result(**override), expected)

    def test_inventory_and_source_binding(self):
        root = pathlib.Path(__file__).resolve().parents[1]
        manifest = json.loads((root / "tests/benchmarks/manifest.json").read_text())
        provenance = json.loads((root / "tests/benchmarks/provenance.json").read_text())
        module.validate_inventory(root, manifest, provenance)
        missing = copy.deepcopy(manifest)
        missing["workloads"].pop()
        with self.assertRaises(ValueError):
            module.validate_inventory(root, missing, provenance)
        duplicate = copy.deepcopy(manifest)
        duplicate["workloads"][0]["cases"][1]["args"] = duplicate["workloads"][0]["cases"][0]["args"]
        with self.assertRaises(ValueError):
            module.validate_inventory(root, duplicate, provenance)
        for broken in ({}, {"tests/benchmarks/atax.c": "0" * 64}):
            with self.assertRaises(ValueError):
                module.validate_inventory(root, manifest, {**provenance, "files": broken})
        stale = copy.deepcopy(provenance)
        stale["files"]["tests/benchmarks/atax.elf"] = "0" * 64
        with self.assertRaises(ValueError):
            module.validate_inventory(root, manifest, stale)

    def test_stale_simulator_build_rejected(self):
        state = {"git_sha": "current", "git_dirty": False}
        valid = {"build": {"vcs.revision": "current", "vcs.modified": "false"}}
        module.check_build(valid, state)
        for build in ({}, {"vcs.revision": "old", "vcs.modified": "false"}, {"vcs.revision": "current", "vcs.modified": "true"}):
            with self.assertRaises(ValueError):
                module.check_build({"build": build}, state)


if __name__ == "__main__":
    unittest.main()
