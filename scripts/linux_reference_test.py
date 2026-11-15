#!/usr/bin/env python3
"""External Grace observations are bound to the exact simulated ELF layout."""
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import subprocess
import unittest
from unittest.mock import patch

import accuracy_live as live
import accuracy_progress as progress
import accuracy_site as site
import accuracy_test_support as fixtures
import compute_usertime as compute
import devices
import linux_reference as linux
import model_profiles
import performance_compare as comparison
import performance_reference as performance
import usertime as user
import workload_contract as contract

ROOT = Path(__file__).resolve().parents[1]


class LinuxReferenceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.declared, cls.reference = performance.load(ROOT, require_native=True, path=linux.REFERENCE,
                                                      hardware=linux.HARDWARE)
        cls.canonical, cls.m2 = performance.load(ROOT, require_native=True)
        cls.summary = compute.read(ROOT / linux.SUMMARY)

    def test_all_measured_binaries_and_inputs_are_explicit_and_m2_is_unchanged(self):
        self.assertEqual(len(self.declared["cases"]), 264)
        self.assertEqual(len(self.declared["binary_bindings"]), 44)
        self.assertNotIn("binary_bindings", self.canonical)
        self.assertEqual(self.declared["original"], self.canonical["original"])
        self.assertEqual(self.declared["compute"], self.canonical["compute"])
        for case, original, observed in zip(self.declared["cases"], self.canonical["cases"], self.reference["cases"]):
            name = original["file"] if "outputs" in original else "tests/usertime/" + original["file"]
            self.assertEqual(case["canonical_file"], name)
            self.assertEqual(case["sha256"], self.summary["binaries"][name])
            self.assertEqual(contract.digest(ROOT / case["file"]), case["sha256"])
            self.assertTrue(case["file"].startswith("tests/grace/"))
            self.assertEqual({k: case[k] for k in original if k not in ("file", "sha256", "mapped_image_bytes")},
                             {k: original[k] for k in original if k not in ("file", "sha256", "mapped_image_bytes")})
            self.assertEqual(observed["input_sha256"], compute.object_digest(case))
        declaration = devices.load()
        grace = devices.select(declaration, "grace")
        self.assertEqual(grace["parameter_source"], "m2")
        self.assertEqual(devices.load_profile(grace), devices.load_profile(devices.select(declaration, "m2")))
        self.assertEqual(grace["references"], {"application": linux.REFERENCE})
        repeat = next(c for c in self.declared["cases"] if c["id"] == "streamcluster-6")
        self.assertEqual(contract.instruction_budget(repeat), 6_000_000_000)
        self.assertEqual(contract.execution_timeout_seconds(repeat), 5400)

    def test_quality_and_external_capture_scope_are_preserved(self):
        self.assertEqual(self.reference["quality_counts"], {"usable": 255, "noisy": 9})
        apps = [r for r in self.reference["cases"] if not comparison.is_micro_case(r)]
        self.assertEqual((len(apps), sum(r["quality"] == "usable" for r in apps)), (246, 237))
        for raw, row in zip(self.summary["cases"], self.reference["cases"]):
            self.assertEqual({k: row[k] for k in raw}, raw)
        self.assertEqual(self.reference["source_summary"]["sha256"], contract.digest(ROOT / linux.SUMMARY))
        self.assertIn("have not been independently audited", self.reference["reference_scope"])
        self.assertNotIn("collection_dirty", self.reference)
        self.assertNotIn("oracle_commit", self.reference)
        self.assertEqual(self.reference["collector_sources"], {})

    def test_wrong_device_hash_quality_and_layout_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "hardware mismatch"):
            performance.validate(self.reference, self.declared, "Apple M2")
        mutations = [lambda r: r.update(inventory_sha256="0" * 64),
                     lambda r: r.update(source_inventory_sha256="0" * 64),
                     lambda r: r["native_environment"].update(page_size=4096),
                     lambda r: r["native_environment"].pop("governor"),
                     lambda r: r["binary_bindings"][next(iter(r["binary_bindings"]))].update(sha256="0" * 64),
                     lambda r: r["cases"][0].update(quality="noisy"),
                     lambda r: r["cases"][0].update(sample_count=True),
                     lambda r: r["cases"][0].update(input_sha256="0" * 64),
                     lambda r: r["cases"].pop(),
                     lambda r: r["quality_counts"].update(usable=256)]
        for mutate in mutations:
            bad = copy.deepcopy(self.reference)
            mutate(bad)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                performance.validate(bad, self.declared, linux.HARDWARE)
        for change in (dict(file="tests/usertime/integer.elf"), dict(args=["program", "1"]),
                       dict(canonical_file="tests/usertime/memory.elf")):
            bad = copy.deepcopy(self.declared)
            bad["cases"][0].update(change)
            with self.assertRaises(ValueError):
                compute.validate_expanded(bad)

    def test_case_labels_cannot_hide_changed_arguments_at_collection_commit(self):
        inputs = copy.deepcopy(self.reference["original_input_sources"])
        linux.verify_original_inputs(ROOT, self.summary["source_commit"], inputs)
        # A manifest can retain all case labels while changing argv or outputs.
        # Its exact Git bytes, rather than those labels, bind the acquisition.
        changed = copy.deepcopy(self.canonical["compute"]["program_manifest"])
        changed["workloads"][0]["cases"][0]["args"][0] = "1"
        inputs[compute.PERFORMANCE_MANIFEST] = hashlib.sha256(json.dumps(changed).encode()).hexdigest()
        with self.assertRaisesRegex(ValueError, "arguments/sources differ"):
            linux.verify_original_inputs(ROOT, self.summary["source_commit"], inputs)

    def test_external_summary_mutations_and_actual_elf_mutation_fail(self):
        outputs = {r["id"]: r for r in self.m2["cases"]}
        for mutate in (lambda r: r["cases"][0].update(output_sha256="0" * 64),
                       lambda r: r["cases"][0].update(sample_count=20),
                       lambda r: r["cases"].reverse(),
                       lambda r: r["environment"].update(page_size=4096),
                       lambda r: r["binaries"].pop(next(iter(r["binaries"]))),
                       lambda r: r.update(source_commit="--invalid")):
            bad = copy.deepcopy(self.summary)
            mutate(bad)
            with self.assertRaises(ValueError):
                linux.validate_summary(bad, self.canonical, outputs)
        with tempfile.TemporaryDirectory() as folder:
            # Exactly one retained binary is changed; binding rejects it before
            # a simulator can run or a native timing can be consumed.
            source = next(iter(self.summary["binaries"]))
            path = Path(folder) / "tests/grace" / source.removeprefix("tests/")
            path.parent.mkdir(parents=True)
            path.write_bytes(b"wrong ELF")
            with self.assertRaisesRegex(ValueError, "measured Grace ELF hash mismatch"):
                linux.bindings(folder, self.summary)

    def test_future_m2_timing_changes_do_not_change_grace_observations(self):
        original = compute.read
        def read(path):
            if Path(path) == ROOT / performance.PATH:
                raise AssertionError("Grace depends on the active M2 timing file")
            return original(path)
        with patch.object(compute, "read", side_effect=read):
            self.assertEqual(linux.derive(ROOT), self.reference)

    def test_historical_derivation_reads_requested_commit_not_current_checkout(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            def git(*args):
                return subprocess.check_output(["git", "-C", folder, *args], text=True, stderr=subprocess.DEVNULL).strip()
            git("init")
            git("config", "user.name", "Test")
            git("config", "user.email", "unit@example.invalid")
            (root / "proof").write_text("historical Grace source")
            git("add", "proof")
            git("commit", "-m", "Measured Grace report source")
            commit = git("rev-parse", "HEAD")
            (root / "proof").write_text("later, unrelated working-tree state")
            def derive(tree, *, git_root):
                self.assertEqual(Path(git_root), root)
                return (Path(tree) / "proof").read_text()
            with patch.object(linux, "derive", side_effect=derive):
                self.assertEqual(linux.derive_at(root, commit), "historical Grace source")
            self.assertEqual((root / "proof").read_text(), "later, unrelated working-tree state")

    def report(self):
        """Synthetic counters exercise archive/rendering; never timing evidence."""
        files = dict(self.reference["input_sources"])
        files[linux.REFERENCE] = contract.digest(ROOT / linux.REFERENCE)
        report = dict(schema=4, artifact_status="pass", accuracy_status="unverified", formula=user.FORMULA,
                      workload_manifest=copy.deepcopy(self.declared), head=dict(commit="a" * 40, dirty=False, files=files),
                      base_commit="b" * 40, base_capability=compute.base_capability(self.summary["source_commit"], ROOT),
                      head_binary_sha256="1" * 64, base_binary_sha256="2" * 64, base_status="per_case_resource_limit",
                      reference_sha256=contract.digest(ROOT / linux.REFERENCE),
                      reference={k: v for k, v in self.reference.items() if k not in ("cases", "input_sources", "collector_sources")},
                      cases=[])
        report["device"] = fixtures.device_selection("grace", report["head"]["commit"], files, report["base_commit"])
        profile = compute.read(ROOT / model_profiles.PATH)
        files[model_profiles.PATH] = contract.digest(ROOT / model_profiles.PATH)
        report["model_profiles"] = {
            role: dict(commit=report["head"]["commit"] if role == "head" else report["base_commit"],
                       path=model_profiles.PATH, sha256=files[model_profiles.PATH], profile=copy.deepcopy(profile))
            for role in ("head", "base")}
        for case, native in zip(self.declared["cases"], self.reference["cases"]):
            row = dict(id=case["id"], split=case["split"], mapped_image_bytes=case["mapped_image_bytes"],
                       base_status="available", status="unverified", functional_status="pass",
                       reference_quality=native["quality"], reference_user_seconds=native["median_user_seconds"],
                       reference_p10_seconds=native["p10_user_seconds"], reference_p90_seconds=native["p90_user_seconds"])
            for role in ("head", "base"):
                seconds = 6980000 / case["frequency_hz"]
                row[role] = dict(predicted_user_seconds=seconds, cycles=6980000, instructions=1000, **profile)
                if native["quality"] == "usable":
                    row[role].update(error_percent=user.error_percent(seconds, native["median_user_seconds"]),
                                     direction="slower" if seconds > native["median_user_seconds"] else "faster")
            if native["quality"] == "usable":
                row["error_change_percentage_points"] = 0
            report["cases"].append(row)
        report["summary"] = compute.summaries(report["cases"], True)
        return report

    def test_bundle_live_and_per_benchmark_curves_use_grace_data_and_label_m2_parameters(self):
        report = self.report()
        references = {"reference.json": (ROOT / linux.REFERENCE).read_bytes()}
        site.validate_compact_evidence(report, references)
        self.assertEqual(site.benchmark_summary(report)["comparable"], 237)
        self.assertTrue(all("error_percent" not in row["head"] for row in report["cases"] if row["reference_quality"] == "noisy"))
        state = dict(state="complete", completed_cases=264, required_cases=264,
                     completed_shards=1, required_shards=1, active_cases=[], errors=[], updated_at="2026-11-15T00:00:00Z")
        with patch.object(progress, "reporting_identity", return_value={}):
            snapshot = progress.snapshot(report, ROOT, state)
        live.validate_snapshot(snapshot)
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            compute.write(root / "report.json", report)
            (root / "report.md").write_text("Synthetic transport test, not a simulator result.\n")
            site.bundle(root / "report.json", ROOT, root / "bundle", report["head"]["commit"])
            retained, _ = site.validate_bundle(root / "bundle", report["head"]["commit"])
            self.assertEqual(retained, report)
            page = (root / "bundle/index.html").read_text()
            for text in ("NVIDIA Grace", "Apple M2 model profile (placeholder;", "Native Linux reference",
                         "64 KiB pages", "255 usable and 9 noisy", "raw captures", "Benchmark timing curves", "nussinov"):
                self.assertIn(text, page)
            self.assertNotIn("native macOS thread CPU time", page)


if __name__ == "__main__":
    unittest.main()
