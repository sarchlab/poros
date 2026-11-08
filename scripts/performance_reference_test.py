#!/usr/bin/env python3
"""Compact plot data keeps quality and complete-output validation strict."""
import copy
import json
from pathlib import Path
import tempfile
import shutil
import subprocess
import io
import tarfile
import publish_accuracy as publisher
import unittest

import accuracy_test_support as fixtures
import accuracy_site as site
import compute_usertime as compute
import performance_reference as performance
import usertime as user

ROOT = Path(__file__).resolve().parents[1]


class CompactReferenceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.temp.cleanup)
        cls.fixture = fixtures.Evidence(ROOT, Path(cls.temp.name), native=False)
        cls.declared, cls.reference = performance.load(cls.fixture.source)

    def test_collection_ancestry_survives_only_history_preserving_merge(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def git(*args):
                return subprocess.check_output(["git", "-C", str(root), *args], text=True, stderr=subprocess.DEVNULL).strip()
            git("init")
            git("config", "user.name", "Unit test")
            git("config", "user.email", "unit@example.invalid")
            (root / "source").write_text("measured source")
            git("add", ".")
            git("commit", "-m", "Actual collection source")
            collection = git("rev-parse", "HEAD")
            git("commit", "--allow-empty", "-m", "Reference data")
            target = git("rev-parse", "HEAD")
            performance.require_collection_ancestor(root, collection, target)
            # Identical files do not make a rewritten commit the measured commit.
            git("checkout", "--orphan", "squashed")
            git("commit", "-m", "Squashed source")
            squashed = git("rev-parse", "HEAD")
            with self.assertRaisesRegex(ValueError, "not an ancestor.*--merge"):
                performance.require_collection_ancestor(root, collection, squashed)
            with self.assertRaisesRegex(ValueError, "unavailable.*--merge"):
                performance.require_collection_ancestor(root, "0" * 40, target)


    def test_exact_264_summary_has_no_raw_observations_or_witnesses(self):
        self.assertEqual(len(self.reference["cases"]), 264)
        self.assertEqual(len(self.declared["workloads"]), 44)
        encoded = json.dumps(self.reference)
        self.assertNotIn('"samples"', encoded)
        self.assertNotIn("stdout_base64", encoded)
        self.assertNotIn("warmup_sample", encoded)
        self.assertLess(len(encoded), 300000)

    def test_quality_rules_unchanged(self):
        row = copy.deepcopy(self.reference["cases"][0])
        for values in ([0]*21, [.00001]*21, [.0005]*10+[.001]+[.002]*10, [.001]*21):
            samples = [dict(user_seconds=value) for value in values]
            summary = performance.summarize(self.declared["cases"][0], samples, row["output_sha256"], row["output_bytes"], "analytic")
            self.assertEqual(summary["quality"], compute.native.quality(samples))

    def test_mutated_stats_identities_and_environment_fail_closed(self):
        mutations = [lambda r: r["cases"].pop(), lambda r: r["cases"].reverse(),
                     lambda r: r["cases"][0].update(sample_count=True),
                     lambda r: r["cases"][0].update(output_bytes=True),
                     lambda r: r["cases"][0].update(nonpositive_samples=1),
                     lambda r: r["cases"][0].update(p10_user_seconds=.002),
                     lambda r: r["cases"][0].update(median_user_seconds=float("nan")),
                     lambda r: r["cases"][0].update(quality="noisy"),
                     lambda r: r["cases"][0].update(input_sha256="0"*64),
                     lambda r: r.update(completed_at_utc="not a timestamp"),
                     lambda r: r.update(completed_at_utc="2020-01-01T00:00:00Z"),
                     lambda r: r.update(environment="native"),
                     lambda r: r["collector_sources"].pop(next(iter(r["collector_sources"]))),
                     lambda r: r["input_sources"].update({next(iter(r["input_sources"])):"0"*64})]
        for mutate in mutations:
            with self.subTest(mutation=mutate):
                bad = copy.deepcopy(self.reference)
                mutate(bad)
                with self.assertRaises(ValueError):
                    performance.validate(bad, self.declared)

    def test_complete_output_digest_checks_every_byte(self):
        import hashlib
        raw = b"complete output including tail"
        expected = user.OutputIdentity(len(raw), hashlib.sha256(raw).hexdigest())
        expected.check(raw)
        for bad in (raw[:-1], raw[:-1]+b"X", raw+b"X"):
            with self.assertRaises(ValueError):
                expected.check(bad)

    def test_schema4_bundle_recomputes_plotted_metrics_and_remains_small(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            report = fixtures.compact_report(self.fixture)
            compute.write(root / "report.json", report)
            (root / "report.md").write_text("Synthetic unit report\n")
            site.bundle(root / "report.json", self.fixture.source, root / "bundle", fixtures.SHA)
            validated, manifest = site.validate_bundle(root / "bundle", fixtures.SHA)
            self.assertEqual(validated, report)
            self.assertEqual(set(manifest["files"]), {"report.json", "report.md", "reference.json"})
            self.assertIn("Plotting data", (root / "bundle/index.html").read_text())
            for mutate in (lambda r: r["cases"][0]["head"].update(error_percent=99),
                           lambda r: r["cases"][0].update(reference_p90_seconds=.1),
                           lambda r: r["summary"]["calibration"].update(base_comparable=0),
                           lambda r: r["cases"][0].update(base_status="image_limit_unavailable")):
                bad=copy.deepcopy(report);mutate(bad)
                compute.write(root / "bundle/report.json", bad)
                manifest["files"]["report.json"]=site.digest(root / "bundle/report.json")
                compute.write(root / "bundle/bundle.json", manifest)
                with self.assertRaises(ValueError):
                    site.validate_bundle(root / "bundle", fixtures.SHA)

    def test_publisher_binds_compact_collector_to_real_collection_git_objects(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);source=root/"source"
            shutil.copytree(self.fixture.source,source)
            target = source / site.FRONTEND_REFERENCE
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / site.FRONTEND_REFERENCE, target)
            capability=self.fixture.report["base_capability"]
            for name,content in (("pkg/loader/elf.go",capability["loader_source"]),("pkg/linuxuser/run.go",capability["runtime_source"])):
                path=source/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_text(content)
            def git(*args):
                return subprocess.check_output(["git","-C",str(source),*args],text=True,stderr=subprocess.DEVNULL).strip()
            git("init");git("config","user.name","Unit test");git("config","user.email","unit@example.invalid")
            git("add",".");git("commit","-m","Collection source")
            collection=git("rev-parse","HEAD")
            reference=copy.deepcopy(self.reference)
            reference.update(collection_commit=collection,oracle_commit=collection)
            for forged in (False,True):
                if forged:
                    reference["collector_sources"]["scripts/native_usertime.py"]="0"*64
                compute.write(source/performance.PATH,reference)
                git("add",".");git("commit","-m","Compact measurement")
                commit=git("rev-parse","HEAD")
                report=fixtures.compact_report(self.fixture)
                report.update(base_commit=collection,reference_sha256=site.digest(source/performance.PATH),reference={k:v for k,v in reference.items() if k not in ("cases","input_sources","collector_sources")})
                report["head"]=dict(commit=commit,dirty=False,files={name:site.digest(source/name) for name in git("ls-files").splitlines()})
                raw=root/("forged.json" if forged else "valid.json")
                compute.write(raw,report);raw.with_suffix(".md").write_text("Unit report")
                bundle=root/("forged" if forged else "valid")
                site.bundle(raw,source,bundle,commit)
                if forged:
                    with self.assertRaisesRegex(ValueError,"collection commit"):
                        publisher.verify_source(source,bundle,commit)
                else:
                    publisher.verify_source(source,bundle,commit)
                    page = (bundle / "index.html").read_text()
                    overall, micro = page.split('<section id="microbenchmarks">')
                    self.assertIn("Front-end throughput on Apple M2", micro)
                    self.assertNotIn("<figcaption>Integer</figcaption>", overall)
                    self.assertIn("<figcaption>MVT</figcaption>", overall)
                    (bundle / "frontend.json").write_text("{}")
                    with self.assertRaisesRegex(ValueError, "frontend diagnostic"):
                        site.validate_bundle(bundle, commit)

    def test_historical_compute_packaging_selects_original_reference_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);source=root/"source";source.mkdir()
            archive=subprocess.check_output(["git","-C",str(ROOT),"archive","ec5df8efdd80fd02a0db7bd80e9ae413c509852b"])
            with tarfile.open(fileobj=io.BytesIO(archive)) as tree:
                tree.extractall(source,filter="data")
            destination=root/"evidence"
            paths=site.copy_compute_evidence(source,destination,compute.LEGACY_PROTOCOL)
            self.assertEqual(set(paths),compute.evidence_paths(destination))
            self.assertEqual((destination/"reference/reference.json").read_bytes(),(source/"evaluation/usertime/compute-linuxvm-v1/reference.json").read_bytes())
            self.assertEqual(compute.validate_reference(destination/"reference",destination/"source")["protocol"],compute.LEGACY_PROTOCOL)
            with self.assertRaisesRegex(ValueError,"unsupported"):
                site.copy_compute_evidence(source,root/"unknown","unknown")


if __name__ == "__main__":
    unittest.main()
