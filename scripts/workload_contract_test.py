#!/usr/bin/env python3
"""Manifest completeness, independent arithmetic and all-output validation."""

import copy
import json
import math
from pathlib import Path
import struct
import unittest

import workload_contract as contract

ROOT = Path(__file__).resolve().parents[1]


class WorkloadContractTest(unittest.TestCase):
    def test_inventory_has_four_six_point_sweeps(self):
        manifest = contract.load_manifest(ROOT)
        self.assertEqual(len(manifest["cases"]), 24)
        self.assertEqual(
            [g["id"] for g in manifest["workloads"]],
            ["integer", "memory", "branch", "mvt"],
        )
        self.assertEqual(
            [
                sum(c["split"] == s for c in manifest["cases"])
                for s in ("calibration", "held_out")
            ],
            [12, 12],
        )
        self.assertEqual(
            manifest["workloads"][3]["points"], [8, 63, 120, 129, 400, 1536]
        )

    def test_reject_incomplete_or_mislabeled_sweeps(self):
        manifest = contract.load_manifest(ROOT)
        mutations = [
            lambda m: m["cases"].pop(),
            lambda m: m["workloads"].pop(),
            lambda m: m["workloads"][0]["points"].pop(),
            lambda m: m["workloads"][0]["points"].__setitem__(1, 500000),
            lambda m: m["cases"][0]["args"].append("extra"),
            lambda m: m["cases"][0].update(split="held_out"),
            lambda m: m["cases"][6].update(point=64),
            lambda m: m["workloads"][1].update(iterations=4000000),
            lambda m: m["workloads"][3]["points"].__setitem__(5, 1537),
            lambda m: m["sources"].update({"../escape": "0" * 64}),
        ]
        for mutation in mutations:
            m = copy.deepcopy(manifest)
            mutation(m)
            with self.assertRaises(ValueError):
                contract.validate_manifest(m)

    def test_scalar_closed_forms_against_small_iterative_reference(self):
        for n in (1, 2, 3, 31, 64, 65, 131):
            value = 1
            for _ in range(n):
                value = (value * 1664525 + 1013904223) & 0xFFFFFFFF
            self.assertEqual(
                contract.scalar_reference("integer", n), value.to_bytes(8, "little")
            )
            for count in (1, 2, 8, 64):
                data = list(range(count))
                value = 1
                for i in range(n):
                    j = i % count
                    data[j] += i
                    value += data[j]
                self.assertEqual(
                    contract.scalar_reference("memory", n, count),
                    value.to_bytes(8, "little"),
                )

    def test_mvt_all_elements_and_finiteness(self):
        case = next(
            c for c in contract.load_manifest(ROOT)["cases"] if c["id"] == "mvt-63"
        )
        golden = contract.expected(case)
        contract.check_output(case, golden)
        for position in (0, 62, 63, 125):
            for bad in (math.nan, math.inf, -math.inf, 1000000.0):
                altered = bytearray(golden)
                struct.pack_into("<d", altered, position * 8, bad)
                with self.assertRaises(ValueError):
                    contract.check_output(case, altered)
        for altered in (golden[:-8], golden + b"12345678"):
            with self.assertRaises(ValueError):
                contract.check_output(case, altered)
        # Exact integer oracle compared to independently initialized matrix products.
        n = 8
        want = []
        for half in (0, 1):
            for i in range(n):
                want.append(
                    (i + half) % n / n
                    + math.fsum(
                        (i * j % n / n) * ((j + 3 + half) % n / n) for j in range(n)
                    )
                )
        self.assertEqual(contract.mvt_reference(n), tuple(want))

    def test_legacy_inventory_is_read_only_compatibility(self):
        m = {"schema": 1, "cases": [{"id": i} for i in contract.LEGACY_IDS]}
        contract.validate_manifest(m)
        m["cases"].pop()
        with self.assertRaises(ValueError):
            contract.validate_manifest(m)


if __name__ == "__main__":
    unittest.main()
