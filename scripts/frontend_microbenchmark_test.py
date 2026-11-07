#!/usr/bin/env python3
"""Frontend diagnostics reject incorrect execution identity and loop counts."""
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import frontend_microbenchmark as probe


class FrontendProbeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        elf = self.root / probe.ELF
        elf.parent.mkdir(parents=True)
        elf.write_bytes(b"unit fixture")
        self.digest = hashlib.sha256(elf.read_bytes()).hexdigest()
        self.revision = "a" * 40
        self.profile = dict(configuration={"FetchWidth": 3}, cache_config={"L1Latency": 5})

    def result(self, command):
        loop, length = command[-2:]
        instructions = int(length) * probe.LOOP_INSTRUCTIONS + 12
        return dict(termination="exit", exit_status=0, binary_sha256=self.digest,
                    args=["program", loop, length], stderr_base64="", stdout_base64="",
                    build={"vcs.revision": self.revision, "vcs.modified": "false"},
                    instructions=instructions,
                    timing=dict(retired_instructions=instructions,
                                user_cycles=int(length) * 352256 + 20, **self.profile))

    def run_probe(self, mutation=None):
        def execute(command, **kwargs):
            result = self.result(command)
            if mutation:
                mutation(result)
            return SimpleNamespace(stdout=json.dumps(result).encode())
        with patch.object(probe.subprocess, "run", side_effect=execute):
            return probe.measure(self.root / "head", self.root, self.revision, self.profile)

    def test_difference_removes_startup_and_validates_both_loops(self):
        rows = self.run_probe()
        self.assertEqual(rows, {loop: dict(instructions=probe.LOOP_INSTRUCTIONS, cycles=352256)
                                for loop in ("nop", "add")})
        probe.validate(rows)

    def test_rejects_bad_identity_execution_and_counter_evidence(self):
        mutations = [lambda r: r.update(exit_status=1),
                     lambda r: r.update(binary_sha256="0" * 64),
                     lambda r: r.update(args=["program", "wrong", "1"]),
                     lambda r: r.update(stdout_base64="eA=="),
                     lambda r: r["build"].update({"vcs.revision": "b" * 40}),
                     lambda r: r["build"].update({"vcs.modified": "true"}),
                     lambda r: r["timing"].update(configuration={"FetchWidth": 8}),
                     lambda r: r["timing"].update(retired_instructions=0),
                     lambda r: r.update(instructions=0, timing={**r["timing"], "retired_instructions": 0}),
                     lambda r: r["timing"].update(user_cycles=1)]
        for mutation in mutations:
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                self.run_probe(mutation)

    def test_failed_process_and_downloaded_binary_permissions_fail_closed(self):
        head = self.root / "head"
        head.write_text("#!/bin/sh\nexit 1\n")
        head.chmod(0o644)
        with self.assertRaises(PermissionError):
            probe.measure(head, self.root, self.revision, self.profile)
        head.chmod(0o755)
        with self.assertRaises(subprocess.CalledProcessError):
            probe.measure(head, self.root, self.revision, self.profile)

    def test_serialized_results_require_both_positive_loop_measurements(self):
        rows = self.run_probe()
        for mutation in (lambda r: r.pop("add"),
                         lambda r: r["nop"].update(instructions=0),
                         lambda r: r["add"].update(cycles=0),
                         lambda r: r["add"].update(cycles=True)):
            bad = copy.deepcopy(rows)
            mutation(bad)
            with self.assertRaises(ValueError):
                probe.validate(bad)


if __name__ == "__main__":
    unittest.main()
