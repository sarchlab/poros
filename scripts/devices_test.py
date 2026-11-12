#!/usr/bin/env python3
"""Device selection: declarations, reference ownership, reports, archive and publication.

Simulator metrics here are synthetic transport fixtures, never timing evidence.
The real simulator runs as every device in `devices.py smoke` and the Go tests.
"""
import copy
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import accuracy_backfill as backfill
import accuracy_live as live
import accuracy_progress as progress
import accuracy_site as site
import accuracy_test_support as fixtures
import devices
import model_profiles as models
import probe_checkpoint as checkpoint
import publish_accuracy as publisher
import usertime as user

ROOT = Path(__file__).resolve().parents[1]
IDS = ["m2", "m1", "grace", "m5-max"]


def selection(ident, base_declared=False):
    return fixtures.device_selection(ident, "a" * 40, {}, "b" * 40 if base_declared else None)


class DeclarationTests(unittest.TestCase):
    def test_repository_declares_four_devices_and_only_m2_has_references(self):
        declaration = devices.load()
        self.assertEqual([d["id"] for d in declaration["devices"]], IDS)
        self.assertEqual(declaration["default"], devices.DEFAULT)
        self.assertEqual({d["id"]: devices.reference_status(d) for d in declaration["devices"]},
                         {"m2": "measured", "m1": "unmeasured", "grace": "unmeasured", "m5-max": "unmeasured"})
        # Placeholders say whose parameters they carry rather than claiming their own.
        self.assertEqual({d["parameter_source"] for d in declaration["devices"]}, {"m2"})
        self.assertEqual(devices.parameter_text(devices.select(declaration, "m2"), declaration), "Apple M2 model profile")
        self.assertEqual(devices.parameter_text(devices.select(declaration, "m1"), declaration),
                         "Apple M2 model profile (placeholder; Apple M1 parameters not established)")

    def test_declared_references_are_the_files_the_reports_bind(self):
        expected = {"application": site.performance.PATH,
                    **{name: suite.REFERENCE for name, suite in site.DIAGNOSTIC_SUITES.items()}}
        self.assertEqual(devices.select(devices.load(), "m2")["references"], expected)
        self.assertEqual(set(expected), set(devices.KINDS))
        for path in expected.values():
            self.assertEqual(json.loads((ROOT / path).read_text())["hardware"], "Apple M2", path)

    def test_unknown_device_is_an_error_that_names_the_valid_ones(self):
        declaration = devices.load()
        for name in ("m3", "M2", "", "m5", None):
            with self.assertRaisesRegex(ValueError, "unknown device.*m2, m1, grace, m5-max"):
                devices.select(declaration, name)

    def test_malformed_declarations_are_rejected(self):
        good = devices.load()
        def entry(value, name):
            return next(d for d in value["devices"] if d["id"] == name)
        mutations = [lambda v: v.update(schema=2), lambda v: v.update(default="m1"),
                     lambda v: v["devices"].remove(entry(v, "m2")),
                     lambda v: v["devices"].append(copy.deepcopy(entry(v, "m1"))),
                     lambda v: entry(v, "m1").update(hardware="Apple M2"),
                     lambda v: entry(v, "m1").update(hardware=" "),
                     lambda v: entry(v, "m1").update(id="M1"),
                     lambda v: entry(v, "m1").update(id="../m1"),
                     lambda v: entry(v, "m1").update(parameter_source="grace"),
                     lambda v: entry(v, "m1").update(parameter_source="m3"),
                     lambda v: entry(v, "m1")["references"].update(power="evaluation/power.json"),
                     lambda v: entry(v, "m1")["references"].update(application="../outside.json"),
                     lambda v: entry(v, "m1")["references"].update(application="/etc/passwd"),
                     lambda v: entry(v, "m1").update(note="extra"),
                     lambda v: entry(v, "m1").pop("references")]
        for mutate in mutations:
            bad = copy.deepcopy(good)
            mutate(bad)
            with self.assertRaises(ValueError):
                devices.check(bad)
        devices.check(copy.deepcopy(good))

    def test_workflow_device_choices_are_the_declared_devices(self):
        pages = (ROOT / ".github/workflows/accuracy-pages.yml").read_text()
        evaluation = (ROOT / ".github/workflows/evaluation.yml").read_text()
        self.assertEqual(re.findall(r"options: \[(.*)\]", pages), [", ".join(IDS)])
        self.assertEqual(re.findall(r"options: \[(.*)\]", evaluation), [", ".join(["none", *IDS])])
        shared = (ROOT / ".github/workflows/accuracy-evaluate.yml").read_text()
        # The device is its own input beside the Git target and base revisions.
        inputs = shared.split("jobs:", 1)[0]
        for name in ("target:", "base:", "device:"):
            self.assertIn("      " + name, inputs)
        self.assertIn("default: m2", inputs)
        # m2 keeps the published artifact names; other devices cannot collide with them.
        self.assertIn("DEVICE_SUFFIX: ${{ inputs.device != 'm2' && format('{0}-', inputs.device) || '' }}", shared)
        self.assertIn("name: user-time-accuracy-${{ env.DEVICE_SUFFIX }}${{ github.run_attempt }}", shared)
        # Targets that predate devices still receive the command line they understand.
        self.assertIn("DEVICE_ARGUMENT: ${{ inputs.device != 'm2' && format('--device {0}', inputs.device) || '' }}", shared)
        self.assertEqual(shared.count(" $DEVICE_ARGUMENT\n"), 3)
        self.assertNotIn('--device "', shared)
        self.assertIn("device: ${{ needs.resolve.outputs.device }}", pages)

    def test_roles_of_legacy_default_and_new_devices(self):
        legacy = {}
        self.assertEqual(devices.ident(legacy), "m2")
        for role in ("functional", "head", "base"):
            self.assertIsNone(devices.identity(legacy, role))
            self.assertEqual(devices.options(legacy, role), {})
        self.assertTrue(devices.available(legacy, "base"))
        self.assertTrue(all(devices.measured(legacy, kind) for kind in devices.KINDS))
        m2 = dict(device=selection("m2"))
        target = dict(name="m2", hardware="Apple M2", parameter_source="m2")
        self.assertEqual(devices.identity(m2, "head"), target)
        self.assertEqual(devices.identity(m2, "functional"), target)
        # A base revision that predates targets is the default device and is
        # run without a target argument, as it always was.
        self.assertIsNone(devices.identity(m2, "base"))
        self.assertTrue(devices.available(m2, "base"))
        self.assertEqual(devices.arguments(devices.identity(m2, "head")), ["--target", "m2"])
        self.assertEqual(devices.arguments(devices.identity(m2, "base")), [])
        m1 = dict(device=selection("m1"))
        self.assertFalse(devices.available(m1, "base"))
        self.assertFalse(any(devices.measured(m1, kind) for kind in devices.KINDS))
        self.assertEqual(devices.options(m1, "head"), {"device": dict(name="m1", hardware="Apple M1", parameter_source="m2")})
        paired = dict(device=selection("m1", base_declared=True))
        self.assertTrue(devices.available(paired, "base"))
        self.assertEqual(devices.identity(paired, "base"), devices.identity(paired, "head"))
        # A base that declares devices, but not this one, cannot model it either.
        partial = dict(device=selection("grace", base_declared=True))
        partial["device"]["base"]["declaration"]["devices"] = [
            d for d in partial["device"]["base"]["declaration"]["devices"] if d["id"] != "grace"]
        self.assertFalse(devices.available(partial, "base"))

    def test_execution_and_reference_must_belong_to_the_requested_device(self):
        m1 = dict(name="m1", hardware="Apple M1", parameter_source="m2")
        devices.check_timing(dict(target=m1), m1)
        devices.check_timing(dict(), None)  # A simulator that predates targets.
        for timing in (dict(), dict(target=dict(m1, name="m2")), dict(target=dict(m1, hardware="Apple M2")),
                       dict(target=dict(m1, parameter_source="m1"))):
            with self.assertRaisesRegex(ValueError, "simulated device mismatch"):
                devices.check_timing(timing, m1)
        reference = dict(hardware="Apple M2")
        devices.require_reference({}, "application", reference)
        devices.require_reference(dict(device=selection("m2")), "application", reference)
        for ident in ("m1", "grace", "m5-max"):
            with self.assertRaisesRegex(ValueError, "measured on 'Apple M2'"):
                devices.require_reference(dict(device=selection(ident)), "memory", reference)

    def test_checkpoint_of_one_device_cannot_be_replayed_as_another(self):
        with tempfile.TemporaryDirectory() as folder:
            binary = Path(folder) / "poros"
            binary.write_text("synthetic")
            arguments = (folder, "throughput", "head", dict(commit="a" * 40), binary, "a" * 40, {}, "f" * 64)
            legacy = checkpoint.Checkpoint(*arguments)
            self.assertNotIn("device", legacy.identity)  # Earlier keys stay exact.
            identities = {name: devices.identity(dict(device=selection(name)), "head") for name in IDS}
            saved = {name: checkpoint.Checkpoint(*arguments, device=identity) for name, identity in identities.items()}
            case = dict(id="probe")
            saved["m1"](case, 1, 1024, lambda: dict(cycles=7), lambda row: None)
            self.assertEqual(saved["m1"].read(case, 1, 1024, lambda row: None), dict(cycles=7))
            for other in (legacy, saved["m2"], saved["grace"], saved["m5-max"]):
                with self.assertRaisesRegex(ValueError, "checkpoint identity"):
                    other.read(case, 1, 1024, lambda row: None)


class RevisionTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.git("init", "--quiet")
        self.git("config", "user.name", "Test")
        self.git("config", "user.email", "test@example.org")
        (self.root / "README").write_text("before devices")
        self.before = self.commit("no declaration")
        target = self.root / devices.PATH
        target.parent.mkdir(parents=True)
        shutil.copyfile(ROOT / devices.PATH, target)
        self.after = self.commit("declare devices")
        self.state = dict(commit=self.after, files={devices.PATH: user.digest(target)})

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), *args], text=True, stderr=subprocess.DEVNULL).strip()

    def commit(self, message):
        self.git("add", ".")
        self.git("commit", "--quiet", "-m", message)
        return self.git("rev-parse", "HEAD")

    def test_declaration_is_bound_to_each_exact_revision(self):
        self.assertIsNone(devices.revision(self.root, self.before))
        head = devices.revision(self.root, self.after)
        self.assertEqual(head["declaration"], devices.load())
        self.assertEqual(head["sha256"], self.state["files"][devices.PATH])
        chosen = devices.resolve(self.root, self.state, self.before)
        self.assertEqual((chosen["id"], chosen["head"], chosen["base"]), ("m2", head, None))
        self.assertEqual(devices.resolve(self.root, self.state, self.after, "grace")["base"], dict(head, commit=self.after))
        for name in ("m3", "M1", ""):
            with self.assertRaisesRegex(ValueError, "unknown device"):
                devices.resolve(self.root, self.state, self.before, name)
        with self.assertRaisesRegex(ValueError, "differs from committed source"):
            devices.resolve(self.root, dict(self.state, files={devices.PATH: "0" * 64}), self.before)

    def test_source_without_declaration_models_only_the_default(self):
        legacy = dict(commit=self.before, files={})
        self.assertIsNone(devices.resolve(self.root, legacy, self.before))
        self.assertIsNone(devices.resolve(self.root, legacy, self.before, "m2"))
        for name in ("m1", "grace", "m5-max", "m3"):
            with self.assertRaisesRegex(ValueError, "predates device declarations"):
                devices.resolve(self.root, legacy, self.before, name)

    def test_publication_checks_both_revisions_and_canonical_bytes(self):
        report = dict(head=self.state, base_commit=self.before, device=devices.resolve(self.root, self.state, self.before, "m1"))
        self.assertEqual(devices.verify_source(self.root, report)["id"], "m1")
        claimed = copy.deepcopy(report)
        claimed["device"]["base"] = dict(report["device"]["head"], commit=self.before)
        devices.validate(claimed)  # Self-consistent offline; only Git shows the base never declared it.
        with self.assertRaisesRegex(ValueError, "base device declaration differs from exact Git revision"):
            devices.verify_source(self.root, claimed)
        (self.root / devices.PATH).write_text(json.dumps(devices.load()))
        loose = self.commit("non-canonical declaration")
        with self.assertRaisesRegex(ValueError, "canonical JSON"):
            devices.revision(self.root, loose)


class ReportTests(unittest.TestCase):
    """One synthetic 264-case report, viewed as each kind of device."""

    @classmethod
    def setUpClass(cls):
        temp = tempfile.TemporaryDirectory()
        cls.addClassCleanup(temp.cleanup)
        cls.folder = Path(temp.name)
        fixture = fixtures.Evidence(ROOT, cls.folder / "evidence")
        cls.source = fixture.source
        report = fixtures.compact_report(fixture)
        profile = report["workload_manifest"]["profile"]
        profile_hash = hashlib.sha256(models.encode(profile)).hexdigest()
        report["head"]["files"][models.PATH] = profile_hash
        report["model_profiles"] = {
            role: dict(commit=report["head"]["commit"] if role == "head" else report["base_commit"],
                       path=models.PATH, sha256=profile_hash, profile=copy.deepcopy(profile))
            for role in ("head", "base")}
        # One head-only suite and one paired suite stand for all diagnostics.
        for suite in (site.throughput, site.forwarding):
            for name in [*suite.MEASUREMENT_INPUTS, suite.ELF, suite.REFERENCE]:
                path = cls.source / name
                path.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / name, path)
                report["head"]["files"][name] = site.digest(path)
        report["throughput_simulation"] = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * 1024, cycles=100000)
                                           for c in site.throughput.cases(ROOT)]
        rows = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * 1024, cycles=200000, updates=65536)
                for c in site.forwarding.cases(ROOT)]
        report["forwarding_simulation"] = dict(base=copy.deepcopy(rows), head=rows)
        cls.legacy = report
        cls.commit = report["head"]["commit"]

    def setUp(self):
        temp = tempfile.TemporaryDirectory(dir=self.folder)
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)

    def report(self, ident, base_declared=False):
        report = fixtures.as_device(self.legacy, ident, base_declared=base_declared)
        definitions = site.diagnostic_definitions(report, self.source)
        if definitions:
            report["diagnostic_definitions"] = definitions
        if not devices.available(report, "base"):
            report["forwarding_simulation"]["base"] = []
        return report

    def references(self, report):
        return {name: (self.source / path).read_bytes() for name, path in site.reference_files(report).items()}

    def bundle(self, report, name):
        raw = self.root / name / "report.json"
        site.write(raw, report)
        raw.with_suffix(".md").write_text("Synthetic transport fixture\n")
        output = self.root / name / "bundle"
        site.bundle(raw, self.source, output, self.commit)
        return output

    def metadata(self, kind="main", device=None, run=1):
        url = f"https://github.com/sarchlab/poros/actions/runs/{run}/attempts/1"
        return dict(commit=self.commit, kind=kind, conclusion="success", evaluation_conclusion="success",
                    evaluation_run_id=run, evaluation_run_attempt=1, evaluation_run_url=url,
                    run_id=run, run_attempt=1, run_url=url,
                    **({"device": device} if device else {}),
                    **({"release": {"id": 7, "tag": "v1.0"}} if kind == "release" else {}))

    def test_every_device_report_validates_and_names_its_device(self):
        for ident, status in (("m2", "measured"), ("m1", "unmeasured"), ("grace", "unmeasured"), ("m5-max", "unmeasured")):
            with self.subTest(device=ident):
                report = self.report(ident)
                site.validate_compact_evidence(report, self.references(report))
                described = site.device_summary(report)
                self.assertEqual((described["id"], described["references"]), (ident, status))
                self.assertEqual(site.report_device(report), ident)
                self.assertEqual(len(report["cases"]), 264)
                self.assertEqual(sorted(self.references(report)),
                                 ["forwarding.json", "reference.json", "throughput.json"] if ident == "m2" else [])

    def test_unmeasured_report_shows_predictions_and_no_reference_metric(self):
        report = self.report("m1")
        bundle = self.bundle(report, "m1")
        self.assertEqual(set(site.read(bundle / "bundle.json")["files"]), {"report.json", "report.md"})
        retained, _ = site.validate_bundle(bundle, self.commit)
        self.assertEqual(retained, report)
        self.assertEqual(site.benchmark_summary(retained)["comparable"], 0)
        self.assertNotIn("head", site.benchmark_summary(retained))
        self.assertIsNone(site.measurement_completed(retained, bundle))
        page = (bundle / "index.html").read_text()
        for expected in ("<strong>Device: Apple M1</strong> (<code>m1</code>)",
                         "Apple M2 model profile (placeholder; Apple M1 parameters not established)",
                         "Native references: unmeasured", "No native Apple M1 reference.",
                         "Benchmark predictions", "Application cases simulated with checked output: 246 / 246",
                         "Also predicted by the base revision: 0", "Head cycles/instruction",
                         'id="throughput-simulated-table"', 'id="forwarding-simulated-table"',
                         "The base revision does not model this device.",
                         "no native measurement exists for this device", 'id="output-oracle"',
                         "294 / 294 points"):  # 264 programs, 17 throughput and 13 forwarding points.
            self.assertIn(expected, page)
        for ident in IDS:  # Every modeled device is listed on every report.
            self.assertIn(f'data-row-key="{ident}"', page)
        # Nothing that only a native reference can supply is rendered, as a
        # value or as an empty placeholder.
        for absent in ("Kendall", "Pearson", "Qualified reference vs. prediction", "S-curve", "Absolute error",
                       "<th>Reference</th>", "Head error", "Base error", "Mean error", "Native ns/", "Estimated native IPC",
                       'href="reference.json"', 'href="throughput.json"', 'href="forwarding.json"',
                       "Usable microbenchmark references", "Store/load dependency probes", "macOS",
                       ">reference</span>", ">base</span>", "No comparable points"):
            self.assertNotIn(absent, page)
        self.assertIn('<span style="color:#087e8b">head</span>', page)  # Predicted curves are still drawn.
        markdown = self.root / "m1/report.md"
        self.assertTrue(markdown.exists())

    def test_measured_m2_report_is_unchanged_except_for_naming_its_device(self):
        declared, legacy = self.report("m2"), copy.deepcopy(self.legacy)
        self.assertEqual({k: v for k, v in declared.items() if k not in ("device", "head")},
                         {k: v for k, v in legacy.items() if k != "head"})
        legacy_page = (self.bundle(legacy, "legacy") / "index.html").read_text()
        page = (self.bundle(declared, "m2") / "index.html").read_text()
        self.assertIn("implied: this report predates device selection", legacy_page)
        self.assertIn("<strong>Device: Apple M2</strong> (<code>m2</code>) · Parameters: Apple M2 model profile · Native references: measured", page)
        for shared in ("Kendall", "Qualified reference vs. prediction", "Store/load dependency probes",
                       'href="reference.json"', "Instruction throughput table"):
            self.assertIn(shared, legacy_page)
            self.assertIn(shared, page)
        self.assertNotIn("No native", page)
        self.assertEqual(set(site.read(self.root / "m2/bundle/bundle.json")["files"]),
                         set(site.read(self.root / "legacy/bundle/bundle.json")["files"]))

    def test_unmeasured_report_rejects_every_borrowed_reference_or_accuracy_value(self):
        good = self.report("m1")
        m2 = self.report("m2")
        site.validate_compact_evidence(good, {})
        def row(report):
            return report["cases"][30]
        borrowed, oracle = "unmeasured device row carries reference or accuracy values", "must name only its output oracle"
        base, identity = "unjustified per-case base result", "device declaration identity mismatch"
        # Each change must be refused for its own reason, not by an earlier check.
        mutations = {
            "reference time": (lambda r: row(r).update(reference_user_seconds=m2["cases"][30]["reference_user_seconds"]), borrowed),
            "reference spread": (lambda r: row(r).update(reference_p10_seconds=.001), borrowed),
            "reference origin": (lambda r: row(r).update(reference_origin=dict(acquisition="original")), borrowed),
            "reference quality": (lambda r: row(r).update(reference_quality="usable"), borrowed),
            "error": (lambda r: row(r)["head"].update(error_percent=10), borrowed),
            "direction": (lambda r: row(r)["head"].update(direction="slower"), borrowed),
            "error change": (lambda r: row(r).update(error_change_percentage_points=0), "unpaired error change"),
            "reference metadata": (lambda r: r.update(reference=m2["reference"]), oracle),
            "reference digest": (lambda r: r.update(reference_sha256=m2["reference_sha256"]), oracle),
            "no oracle": (lambda r: r.pop("output_oracle"), oracle),
            "other oracle": (lambda r: r["output_oracle"].update(sha256="0" * 64), oracle),
            "oracle scope": (lambda r: r["output_oracle"].update(scope="timings"), oracle),
            "base prediction": (lambda r: row(r).update(base=copy.deepcopy(row(r)["head"])), base),
            "base status": (lambda r: row(r).update(base_status="available"), base),
            "base diagnostic rows": (lambda r: r["forwarding_simulation"].update(base=copy.deepcopy(r["forwarding_simulation"]["head"])),
                                     "base rows for a device the base revision does not model"),
            "no definitions": (lambda r: r.pop("diagnostic_definitions"), "retained case definitions differ"),
            "missing definition": (lambda r: r["diagnostic_definitions"].pop("forwarding"), "retained case definitions differ"),
            "changed definition": (lambda r: r["diagnostic_definitions"].update(throughput=r["diagnostic_definitions"]["throughput"] + "PROBE(extra, 3, nop)\n"),
                                   "throughput case definition differs from tested source"),
            # As m2 the base revision is the default device, so its predictions are missing.
            "relabel as m2": (lambda r: r["device"].update(id="m2"), base),
            "unknown device": (lambda r: r["device"].update(id="m3"), "unknown device 'm3'"),
            "no device": (lambda r: r.pop("device"), "missing device selection"),
            "other declaration": (lambda r: r["device"]["head"]["declaration"]["devices"][1].update(hardware="Apple M2 Pro"), "head " + identity),
            "other commit": (lambda r: r["device"]["head"].update(commit="c" * 40), "head " + identity),
            "invented base": (lambda r: r["device"].update(base=dict(r["device"]["head"], commit="c" * 40)), "base " + identity),
        }
        for label, (mutate, reason) in mutations.items():
            with self.subTest(label):
                bad = copy.deepcopy(good)
                mutate(bad)
                with self.assertRaisesRegex(ValueError, reason):
                    site.validate_compact_evidence(bad, {})
        # With base predictions present, relabelling still fails: m2 rows need m2's reference.
        relabelled = self.report("m1", base_declared=True)
        relabelled["device"]["id"] = "m2"
        with self.assertRaisesRegex(ValueError, "invalid reference time"):
            site.validate_compact_evidence(relabelled, {})
        # M2's reference bytes are never acceptable evidence in an M1 report.
        for name, data in self.references(m2).items():
            with self.assertRaisesRegex(ValueError, "membership"):
                site.validate_compact_evidence(good, {name: data})
        bundle = self.bundle(good, "tampered")
        shutil.copyfile(self.source / site.performance.PATH, bundle / "reference.json")
        manifest = site.read(bundle / "bundle.json")
        manifest["files"]["reference.json"] = site.digest(bundle / "reference.json")
        site.write(bundle / "bundle.json", manifest)
        with self.assertRaisesRegex(ValueError, "membership"):
            site.validate_bundle(bundle, self.commit)

    def test_a_device_cannot_claim_another_hardwares_reference(self):
        m2 = self.report("m2", base_declared=True)
        def redeclare(report, mutate):
            report = copy.deepcopy(report)
            for role in ("head", "base"):
                if report["device"][role] is not None:
                    mutate(report["device"][role]["declaration"])
                    report["device"][role]["sha256"] = hashlib.sha256(devices.encode(report["device"][role]["declaration"])).hexdigest()
            report["head"]["files"][devices.PATH] = report["device"]["head"]["sha256"]
            return report
        # A self-consistent declaration that hands M1 the M2 files: the report
        # is then "measured", and fails because the files say Apple M2.
        borrowed = redeclare(m2, lambda d: d["devices"][1].update(references=copy.deepcopy(d["devices"][0]["references"])))
        borrowed["device"]["id"] = "m1"
        with self.assertRaisesRegex(ValueError, "application reference was measured on 'Apple M2', not 'Apple M1'"):
            site.validate_compact_evidence(borrowed, self.references(borrowed))
        only_suite = redeclare(self.report("m1"), lambda d: d["devices"][1]["references"].update(
            throughput=site.throughput.REFERENCE))
        only_suite["diagnostic_definitions"].pop("throughput")
        with self.assertRaisesRegex(ValueError, "throughput reference was measured on 'Apple M2', not 'Apple M1'"):
            site.validate_compact_evidence(only_suite, self.references(only_suite))
        renamed = redeclare(m2, lambda d: d["devices"][0].update(hardware="Apple M2 Ultra"))
        with self.assertRaisesRegex(ValueError, "measured on 'Apple M2', not 'Apple M2 Ultra'"):
            site.validate_compact_evidence(renamed, self.references(renamed))
        moved = redeclare(m2, lambda d: d["devices"][0]["references"].update(application="evaluation/usertime/other.json"))
        with self.assertRaisesRegex(ValueError, "declares its application reference at evaluation/usertime/other.json"):
            site.validate_compact_evidence(moved, self.references(m2))
        own = redeclare(self.report("m1"), lambda d: d["devices"][1].update(parameter_source="m1"))
        with self.assertRaisesRegex(ValueError, "declares its own parameters"):
            site.validate_compact_evidence(own, {})

    def test_base_that_models_the_device_is_paired_and_one_that_cannot_is_absent(self):
        alone, paired = self.report("grace"), self.report("grace", base_declared=True)
        site.validate_compact_evidence(alone, {})
        site.validate_compact_evidence(paired, {})
        self.assertTrue(all("base" not in row and row["base_status"] == devices.BASE_UNAVAILABLE for row in alone["cases"]))
        self.assertTrue(any("base" in row for row in paired["cases"]))
        self.assertTrue(all("error_change_percentage_points" not in row for row in paired["cases"]))
        self.assertEqual(site.diagnostic_roles(alone, "forwarding"), ("head",))
        self.assertEqual(site.diagnostic_roles(paired, "forwarding"), ("head", "base"))
        self.assertEqual(len(paired["forwarding_simulation"]["base"]), 13)
        totals = [site.overall_progress(report, {}) for report in (alone, paired)]
        self.assertEqual(totals, [dict(required=294, completed=294, active=0, remaining=0)] * 2)
        page = site.detail(paired, dict(commit=self.commit), preview=True)
        self.assertIn("<th>Base cycles</th><th>Base cycles/instruction</th><th>Head cycles</th>", page)
        self.assertIn('<span style="color:#71838d">base</span>', page)  # A base that models the device is drawn.
        self.assertNotIn(">reference</span>", page)
        self.assertNotIn("The base revision does not model this device.", page)
        stripped = copy.deepcopy(paired)
        stripped["forwarding_simulation"]["base"] = []
        with self.assertRaises(ValueError):
            site.validate_compact_evidence(stripped, {})

    def test_live_snapshot_of_unmeasured_device_carries_no_native_reference(self):
        report = self.report("m5-max")
        report["cases"] = report["cases"][:30]
        report["summary"] = site.compute.summaries(report["cases"], True)
        report["throughput_simulation"] = report["throughput_simulation"][:5]
        state = dict(state="running", completed_cases=30, required_cases=264, completed_shards=15, required_shards=132,
                     active_cases=[], errors=[], updated_at=datetime.now(timezone.utc).isoformat())
        value = progress.snapshot(report, self.source, state)
        self.assertEqual(sorted(value), ["kind", "progress", "report"])
        self.assertEqual(live.snapshot_references(value), {})
        measured = progress.snapshot(dict(self.report("m2"), cases=[], summary=site.compute.summaries([], True)), self.source,
                                     dict(state, completed_cases=0))
        for key in ("reference_text", "throughput_reference_text", "throughput_provenance", "forwarding_reference_text"):
            self.assertIn(key, measured)
            borrowed = dict(value, **{key: measured[key]})
            with self.assertRaisesRegex(ValueError, "its device does not have"):
                live.validate_snapshot(borrowed)
        path = self.root / "snapshot.json"
        site.write(path, value)
        rendered, assets = live.SnapshotView(path).load()
        self.assertEqual(sorted(assets), ["snapshot.json"])
        page = rendered.decode()
        self.assertIn("No native Apple M5 Max reference.", page)
        self.assertIn("<strong>Device: Apple M5 Max</strong> (<code>m5-max</code>)", page)
        applications = sum(not site.is_micro_case(row) for row in report["cases"])
        self.assertEqual(applications, 12)
        self.assertIn("Application cases simulated with checked output: 12 / 246", page)
        self.assertIn("head 5/17 executions complete. No native reference exists for this device", page)
        self.assertIn("Pending — not simulated yet", page)
        self.assertNotIn("Kendall", page)

    def test_archive_keeps_devices_apart_and_links_the_same_commit(self):
        pages = self.root / "pages"
        pages.mkdir()
        order = [self.commit]
        bundles = {ident: self.bundle(self.report(ident), ident) for ident in ("m2", "m1")}
        site.archive(pages, bundles["m2"], self.metadata(), order)
        default = pages / "calibration/runs" / self.commit
        original = (default / "report.json").read_bytes()
        self.assertIn("not published", (default / "index.html").read_text())
        for ident in IDS[1:]:  # Every device has a dashboard before it has a report.
            placeholder = (pages / "calibration/devices" / ident / "index.html").read_text()
            self.assertIn("None yet", placeholder)
            self.assertIn("No native", placeholder)
            self.assertNotIn('href="index.json"', placeholder)
        # The report decides its archive; metadata for another device is refused.
        for bundle, device in ((bundles["m1"], None), (bundles["m1"], "grace"), (bundles["m2"], "m1")):
            with self.assertRaisesRegex(ValueError, "publication names device"):
                site.archive(pages, bundle, self.metadata(device=device), order)
        site.archive(pages, bundles["m1"], self.metadata(device="m1", run=2), order)
        other = pages / "calibration/devices/m1/runs" / self.commit
        self.assertEqual((default / "report.json").read_bytes(), original)
        self.assertEqual(site.read(other / "report.json")["device"]["id"], "m1")
        self.assertEqual(sorted(p.name for p in other.iterdir()), ["bundle.json", "index.html", "metadata.json", "report.json", "report.md"])
        self.assertTrue((default / "reference.json").exists())
        indexes = {ident: site.read(site.archive_root(pages, ident) / "index.json") for ident in ("m2", "m1")}
        self.assertEqual([len(index["entries"]) for index in indexes.values()], [1, 1])
        self.assertNotIn("device", indexes["m2"]["entries"][0])
        self.assertEqual(indexes["m1"]["entries"][0]["device"], "m1")
        self.assertEqual(indexes["m1"]["entries"][0]["benchmark_summary"]["comparable"], 0)
        self.assertNotIn("device_links", indexes["m1"]["entries"][0])
        # Each page links the same commit under the other device, both ways.
        self.assertIn(f'<a href="../../devices/m1/runs/{self.commit}/">report</a>', (default / "index.html").read_text())
        self.assertIn(f'<a href="../../../../runs/{self.commit}/">report</a>', (other / "index.html").read_text())
        for source, link in ((default, f"../../devices/m1/runs/{self.commit}/"), (other, f"../../../../runs/{self.commit}/")):
            self.assertTrue((source / link / "report.json").resolve().is_file())
        home = (pages / "calibration/index.html").read_text()
        device_home = (pages / "calibration/devices/m1/index.html").read_text()
        for ident in IDS:
            self.assertIn(f'data-row-key="{ident}"', home)
            self.assertIn(f'data-row-key="{ident}"', device_home)
        self.assertIn('<a href="devices/m5-max/">open</a>', home)
        self.assertIn('<a href="../../">open</a>', device_home)
        self.assertIn('<a href="../grace/">open</a>', device_home)
        self.assertIn("Device: <strong>Apple M1 · m1</strong>", device_home)
        # A device without a reference has a count of simulated cases, and no
        # error column or trend to leave empty.
        self.assertIn("<th>Application cases simulated</th>", device_home)
        self.assertIn("<td>246</td>", device_home)
        for absent in ("Mean application error", "Benchmark accuracy trends", "Mean error (%)"):
            self.assertNotIn(absent, device_home)
            self.assertIn(absent, home)
        self.assertNotIn("No native", home)
        # The same release pins both devices; neither pin moves the other.
        for ident in ("m2", "m1"):
            site.archive(pages, bundles[ident], self.metadata("release", None if ident == "m2" else ident, run=3), order)
            index = site.read(site.archive_root(pages, ident) / "index.json")
            self.assertEqual(index["entries"][0]["releases"], [{"id": 7, "tag": "v1.0"}])
        # A failed run of a third device is filed under that device only.
        raw = self.root / "failed"
        site.failure(raw, self.commit, "simulation failed", device="grace")
        site.bundle(raw / "report.json", self.source, self.root / "failed-bundle", self.commit)
        with self.assertRaisesRegex(ValueError, "publication names device"):
            site.archive(pages, self.root / "failed-bundle", dict(self.metadata(run=4), conclusion="failure"), order)
        site.archive(pages, self.root / "failed-bundle", dict(self.metadata(device="grace", run=4), conclusion="failure"), order)
        failed = pages / "calibration/devices/grace/runs" / self.commit
        self.assertIn("simulation failed", (failed / "index.html").read_text())
        self.assertEqual((default / "report.json").read_bytes(), original)
        self.assertEqual(site.archived_devices(pages), ["m2", "grace", "m1", "m5-max"])
        with self.assertRaises(ValueError):
            site.failure(raw, self.commit, "unsafe", device="../m2")
        # For a browser check of the archive: keep this site, built by the real
        # archive and renderer from synthetic transport values, where asked.
        if os.environ.get("POROS_DEVICE_PAGES_PREVIEW"):
            shutil.copytree(pages, os.environ["POROS_DEVICE_PAGES_PREVIEW"], dirs_exist_ok=True)

    def test_publication_source_check_follows_the_device(self):
        report = self.report("m1")
        bundle = self.bundle(report, "publish")
        files = report["head"]["files"]
        with patch.object(site, "git", return_value="\n".join(files)), \
             patch.object(publisher.subprocess, "check_output", side_effect=lambda command, **options: self.committed(command, files)), \
             patch.object(site.compute, "base_capability", return_value=report["base_capability"]), \
             patch.object(site.model_profiles, "verify_source") as profiles, \
             patch.object(site.devices, "verify_source") as declared, \
             patch.object(site.performance, "require_collection_ancestor", side_effect=AssertionError("traced a reference the device lacks")):
            publisher.verify_source(self.source, bundle, self.commit)
        profiles.assert_called_once()
        declared.assert_called_once()

    def committed(self, command, files):
        """The fixture's stand-in for `git show commit:path`."""
        path = command[-1].split(":", 1)[1]
        if path == devices.PATH:
            return devices.encode(devices.load())
        if path == models.PATH:
            return models.encode(self.legacy["workload_manifest"]["profile"])
        return (self.source / path).read_bytes()


class PublicationTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.source = self.root / "source"
        self.source.mkdir()
        self.commit = "a" * 40

    def test_manual_publication_takes_a_device_beside_its_git_target(self):
        event = dict(repository=dict(full_name=publisher.REPO), inputs=dict(release_tag="", device="m1"))
        environment = dict(GITHUB_REF="refs/heads/main", GITHUB_SHA=self.commit)
        with patch.dict(os.environ, environment):
            self.assertEqual(publisher.assert_event(event, "workflow_dispatch"), dict(kind="main", commit=self.commit, device="m1"))
            for inputs in (dict(release_tag=""), dict(release_tag="", device="m2"), dict(release_tag="", device=""), None):
                self.assertEqual(publisher.assert_event(dict(event, inputs=inputs), "workflow_dispatch"),
                                 dict(kind="main", commit=self.commit))
            for name in ("m3", "M1", "../m1"):
                with self.assertRaisesRegex(ValueError, "unknown device"):
                    publisher.assert_event(dict(event, inputs=dict(device=name)), "workflow_dispatch")
            release = dict(kind="release", commit=self.commit, release=dict(id=7, tag="v1.0"))
            with patch.object(publisher, "published_release_target", return_value=dict(release)):
                self.assertEqual(publisher.assert_event(dict(event, inputs=dict(release_tag="v1.0", device="grace")), "workflow_dispatch"),
                                 dict(release, device="grace"))

    def test_backfill_routes_and_artifacts_are_per_device(self):
        self.assertEqual(backfill.evidence_prefix("parallel", "m2"), "user-time-accuracy")
        self.assertEqual(backfill.evidence_prefix("parallel", "m5-max"), "user-time-accuracy-m5-max")
        artifacts = [dict(name=name, expired=False) for name in
                     ("user-time-accuracy-1", "user-time-accuracy-m1-2", "user-time-accuracy-m5-max-1", "user-time-accuracy-m1-extra-1")]
        chosen = {device: publisher.choose_artifact(artifacts, 3, backfill.evidence_prefix("parallel", device))[1]
                  for device in ("m2", "m1", "m5-max", "grace")}
        self.assertEqual({device: artifact and artifact["name"] for device, artifact in chosen.items()},
                         {"m2": "user-time-accuracy-1", "m1": "user-time-accuracy-m1-2",
                          "m5-max": "user-time-accuracy-m5-max-1", "grace": None})
        target = dict(kind="main", commit=self.commit, device="m1")
        with patch.object(site, "git", return_value=self.commit):
            # A target revision that predates devices is never run as the default instead.
            with self.assertRaisesRegex(ValueError, "predates device declarations"):
                backfill.route(self.source, self.root / "pages", self.root / "old", target)
            (self.source / "evaluation").mkdir()
            shutil.copyfile(ROOT / devices.PATH, self.source / devices.PATH)
            with self.assertRaisesRegex(ValueError, "unknown device"):
                backfill.route(self.source, self.root / "pages", self.root / "unknown", dict(target, device="m3"))
            site.write(self.source / "tests/usertime/performance.json", dict(schema=2, suite="whole-program-performance-six-point-v1"))
            (self.source / "scripts").mkdir()
            (self.source / "scripts/accuracy_parallel.py").write_text("unit")
            # A retained default-device release is not evidence for another device.
            archived = self.root / "pages/calibration/runs" / self.commit
            archived.mkdir(parents=True)
            with patch.object(site, "reusable_bundle", return_value=True):
                record = backfill.route(self.source, self.root / "pages", self.root / "m1", dict(target, kind="release"))
                self.assertEqual(record["route"], "parallel")
                own = self.root / "pages/calibration/devices/m1/runs" / self.commit
                own.mkdir(parents=True)
                (own / "proof").write_text("m1 bundle")
                record = backfill.route(self.source, self.root / "pages", self.root / "reuse", dict(target, kind="release"))
            self.assertEqual(record["route"], "reuse")
            self.assertEqual((self.root / "reuse/evidence/proof").read_text(), "m1 bundle")

    def test_failed_backfill_is_filed_under_its_device_and_foreign_evidence_is_refused(self):
        target = dict(kind="main", commit=self.commit, device="grace")
        resolved = self.root / "resolved"
        site.write(resolved / "backfill.json", dict(schema=1, target=target, route="parallel", base="b" * 40))
        with patch.object(site, "git", return_value=self.commit):
            self.assertEqual(backfill.package(self.source, resolved, self.root / "missing", self.root / "failed", self.commit, "failure"), 1)
            report, _ = site.validate_bundle(self.root / "failed", self.commit)
            self.assertEqual((report["artifact_status"], site.report_device(report)), ("fail", "grace"))
            self.assertEqual(site.read(self.root / "failed/publication-target.json"), target)
            # Evidence produced for the default device cannot be packaged as grace's.
            other = self.root / "raw"
            site.failure(other, self.commit, "default-device failure")
            site.bundle(other / "report.json", self.source, self.root / "selected", self.commit)
            with self.assertRaisesRegex(ValueError, "another device"):
                backfill.package(self.source, resolved, self.root / "selected", self.root / "out", self.commit, "success")


if __name__ == "__main__":
    unittest.main()
