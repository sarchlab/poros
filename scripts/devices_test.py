#!/usr/bin/env python3
"""Device selection: declarations, reference ownership, reports, archive and publication.

Simulator metrics here are synthetic transport fixtures, never timing evidence.
The real simulator runs as every device in `devices.py smoke` and the Go tests.
"""
import collections
import copy
from datetime import datetime, timezone
import hashlib
import html.parser
from http.server import ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import statistics
import subprocess
import tempfile
import threading
import unittest
from unittest.mock import patch
import urllib.error
import urllib.request

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


class ReportFixture(unittest.TestCase):
    """One synthetic 264-case report and its source, built once for every class that uses it."""
    built = None

    @classmethod
    def setUpClass(cls):
        if ReportFixture.built is None:
            temp = tempfile.TemporaryDirectory()
            unittest.addModuleCleanup(temp.cleanup)
            folder = Path(temp.name)
            fixture = fixtures.Evidence(ROOT, folder / "evidence")
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
                    path = fixture.source / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(ROOT / name, path)
                    report["head"]["files"][name] = site.digest(path)
            report["throughput_simulation"] = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * 1024, cycles=100000)
                                               for c in site.throughput.cases(ROOT)]
            rows = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * 1024, cycles=200000, updates=65536)
                    for c in site.forwarding.cases(ROOT)]
            report["forwarding_simulation"] = dict(base=copy.deepcopy(rows), head=rows)
            ReportFixture.built = (folder, fixture.source, report)
        cls.folder, cls.source, cls.legacy = ReportFixture.built
        cls.commit = cls.legacy["head"]["commit"]

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

    def references(self, report, source=None):
        source = self.source if source is None else source
        return {name: (source / path).read_bytes() for name, path in site.reference_files(report).items()}

    def bundle(self, report, name, source=None):
        raw = self.root / name / "report.json"
        site.write(raw, report)
        raw.with_suffix(".md").write_text("Synthetic transport fixture\n")
        output = self.root / name / "bundle"
        site.bundle(raw, self.source if source is None else source, output, report["head"]["commit"])
        return output

    def metadata(self, kind="main", device=None, run=1, commit=None):
        url = f"https://github.com/sarchlab/poros/actions/runs/{run}/attempts/1"
        return dict(commit=commit or self.commit, kind=kind, conclusion="success", evaluation_conclusion="success",
                    evaluation_run_id=run, evaluation_run_attempt=1, evaluation_run_url=url,
                    run_id=run, run_attempt=1, run_url=url,
                    **({"device": device} if device else {}),
                    **({"release": {"id": 7, "tag": "v1.0"}} if kind == "release" else {}))


class ReportTests(ReportFixture):
    """The synthetic report viewed as each kind of device."""

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
        # The oracle statement matches what is done: the whole file is validated,
        # its timing fields included, and only the output identities are used.
        self.assertEqual(report["output_oracle"]["scope"], site.OUTPUT_ORACLE_SCOPE)
        self.assertIn("the committed file is validated as a whole, native timing fields included; this device uses only its "
                      "complete-output identities", page)
        self.assertIn("none of its timings is shown or compared.", page)
        self.assertNotIn("no native timing from this file is read", page)
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
        # A device names its own application reference file, so a path the
        # tested source does not have supplies no bytes to accept.
        moved = redeclare(m2, lambda d: d["devices"][0]["references"].update(application="evaluation/usertime/other.json"))
        self.assertEqual(site.reference_files(moved)["reference.json"], "evaluation/usertime/other.json")
        with self.assertRaisesRegex(ValueError, "report reference bytes differ from tested source: reference.json"):
            site.validate_compact_evidence(moved, self.references(m2))
        # Suite references are still the default device's files only.
        suite = redeclare(m2, lambda d: d["devices"][0]["references"].update(throughput="evaluation/microbenchmarks/throughput/other.json"))
        with self.assertRaisesRegex(ValueError, "declares its throughput reference at evaluation/microbenchmarks/throughput/other.json"):
            site.validate_compact_evidence(suite, self.references(m2))
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


SYNTHETIC_B = "evaluation/usertime/synthetic-b.json"
# Commits other than the fixture's own: one that predates devices, and two more declared ones.
LEGACY, QUIET, LATER = "c" * 40, "d" * 40, "e" * 40


def at_commit(report, commit):
    """The same synthetic report as evidence of another commit."""
    report = copy.deepcopy(report)
    report["head"]["commit"] = commit
    report["model_profiles"]["head"]["commit"] = commit
    if "device" in report:
        report["device"]["head"]["commit"] = commit
    return report


def view(page, name):
    """One view of a commit report, as text."""
    start = page.index(f'<section class="view" id="{name}"')
    end = page.find('<section class="view"', start + 1)
    return page[start:end if end != -1 else page.index("<footer>")]


class Page(html.parser.HTMLParser):
    """What a page names and what it refers to."""
    def __init__(self, text):
        super().__init__()
        self.ids, self.links, self.labels = [], [], []
        self.feed(text)

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if "id" in attrs:
            self.ids.append(attrs["id"])
        if tag == "a" and "href" in attrs:
            self.links.append(attrs["href"])
        for name in ("aria-labelledby", "aria-describedby"):
            self.labels += attrs.get(name, "").split()


def check_site(test, pages):
    """Follow every local link of a staged site as a browser would.

    Each page names its elements uniquely. Each link reaches a file and, with a
    fragment, an element of that file. Returns the parsed pages by path."""
    parsed = {path.resolve(): Page(path.read_text()) for path in pages.rglob("*.html")}
    for path, page in parsed.items():
        test.assertEqual([name for name, count in collections.Counter(page.ids).items() if count > 1], [], path)
        test.assertLessEqual(set(page.labels), set(page.ids), path)
        for link in page.links:
            if link.startswith("https://"):
                continue
            location, _, fragment = link.partition("#")
            target = (path.parent / location).resolve() if location else path
            target = target / "index.html" if target.is_dir() else target
            test.assertTrue(target.is_file(), f"{path}: {link}")
            if fragment:
                test.assertIn(fragment, parsed[target].ids, f"{path}: {link}")
    return parsed


class CommitReportTests(ReportFixture):
    """A commit's one report: devices that arrive separately, pooled by observation.

    The fixture commit declares devices the repository does not: Apple M2 and a
    second measured device with its own reference file, one unmeasured device,
    and one that reports late or fails. The qualified observations are, in
    measured -> predicted milliseconds:

        m2     first 1.0 -> 1.1 (10%),  second 2.0 -> 2.6 (30%)
        syn-b  first 1.5 -> 3.0 (100%), three others 4 -> 6, 8 -> 12, 16 -> 24 (50% each)

    `first` is the same input on both devices. The base revision predicts only
    m2: first 1.2 (20%) and second 2.2 (10%)."""

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.declared = dict(schema=1, default="m2", devices=[
            copy.deepcopy(devices.select(devices.load(), "m2")),
            dict(id="syn-b", hardware="Synthetic device B", parameter_source="m2", references=dict(application=SYNTHETIC_B)),
            dict(id="syn-c", hardware="Synthetic device C", parameter_source="m2", references={}),
            dict(id="syn-d", hardware="Synthetic device D", parameter_source="m2", references={})])
        applications = [row for row in cls.legacy["cases"] if not site.is_micro_case(row)]
        cls.first, cls.second = [row["id"] for row in applications if "base" in row][:2]
        cls.others = [row["id"] for row in applications if "base" not in row][:3]
        # Case -> (measured ms, head cycles, base cycles) at 3.49 GHz.
        observations = {
            "m2": {cls.first: (1.0, 3839000, 4188000), cls.second: (2.0, 9074000, 7678000)},
            "syn-b": {cls.first: (1.5, 10470000, None),
                      **dict(zip(cls.others, ((4.0, 20940000, None), (8.0, 41880000, None), (16.0, 83760000, None))))}}
        cls.world = cls.measured_source("pooled-source", observations)

    @classmethod
    def measured_source(cls, name, observations):
        """A source tree in which each measured device has exactly these qualified observations.

        Every other application reference is too noisy to qualify."""
        source = cls.folder / name
        shutil.copytree(cls.source, source)
        original = site.read(cls.source / site.performance.PATH)
        native = {}
        for ident, path in (("m2", site.performance.PATH), ("syn-b", SYNTHETIC_B)):
            native[ident] = fixtures.device_reference(original, devices.select(cls.declared, ident)["hardware"],
                                                      {case: ms / 1000 for case, (ms, _, _) in observations[ident].items()})
            site.write(source / path, native[ident])
        return source, native, observations

    def device_report(self, ident, commit=None, world=None):
        """One device's report of a commit that carries the fixture declaration."""
        source, native, observations = world or self.world
        start = copy.deepcopy(self.legacy)
        start["head"]["files"].update({path: site.digest(source / path) for path in (site.performance.PATH, SYNTHETIC_B)})
        report = fixtures.as_device(start, ident, declaration=self.declared)
        if ident in native:
            cycles = {(case, role): value for case, (_, head, base) in observations[ident].items()
                      for role, value in (("head", head), ("base", base)) if value is not None}
            report = fixtures.with_reference(report, native[ident], site.application_reference(report)[0], cycles)
        if not devices.available(report, "base"):
            report["forwarding_simulation"]["base"] = []
        definitions = site.diagnostic_definitions(report, source)
        if definitions:
            report["diagnostic_definitions"] = definitions
        return report if commit is None else at_commit(report, commit)

    def publish(self, pages, report, name, *, run=1, world=None, repository=None, **changes):
        device = site.report_device(report)
        bundle = self.bundle(report, name, self.source if "device" not in report else (world or self.world)[0])
        metadata = self.metadata(device=None if device == "m2" else device, run=run, commit=report["head"]["commit"])
        return site.archive(pages, bundle, dict(metadata, **changes), self.order, source=repository)

    def publish_failure(self, pages, commit, device, name, *, run=1, repository=None):
        raw = self.root / name
        site.failure(raw, commit, "simulation failed", device=device)
        site.bundle(raw / "report.json", self.source, raw / "bundle", commit)
        metadata = self.metadata(device=None if device in (None, "m2") else device, run=run, commit=commit)
        return site.archive(pages, raw / "bundle", dict(metadata, conclusion="failure"), self.order, source=repository)

    def setUp(self):
        super().setUp()
        self.pages = self.root / "pages"
        self.pages.mkdir()
        self.order = [self.commit, LATER, QUIET, LEGACY]

    def folder_of(self, commit):
        return self.pages / "calibration/runs" / commit

    def page(self, commit=None):
        return (self.folder_of(commit or self.commit) / "index.html").read_text()

    def home(self):
        return (self.pages / "calibration/index.html").read_text()

    def staged(self):
        """The fixture commit with both measured devices and the unmeasured one reported."""
        for run, ident in enumerate(("m2", "syn-b", "syn-c"), 1):
            index = self.publish(self.pages, self.device_report(ident), ident, run=run)
        return index

    def test_fixture_reports_are_valid_evidence_of_their_own_devices(self):
        source = self.world[0]
        for ident, files in (("m2", ["forwarding.json", "reference.json", "throughput.json"]), ("syn-b", ["reference.json"]),
                             ("syn-c", []), ("syn-d", [])):
            with self.subTest(device=ident):
                report = self.device_report(ident)
                references = self.references(report, source)
                site.validate_compact_evidence(report, references)
                self.assertEqual(sorted(references), files)
        # Each measured device is compared with its own file, measured on its own hardware.
        b = self.device_report("syn-b")
        self.assertEqual(site.reference_files(b)["reference.json"], SYNTHETIC_B)
        self.assertEqual(json.loads(self.references(b, source)["reference.json"])["hardware"], "Synthetic device B")
        m2_bytes = self.references(self.device_report("m2"), source)["reference.json"]
        with self.assertRaisesRegex(ValueError, "report reference bytes differ from tested source"):
            site.validate_compact_evidence(b, {"reference.json": m2_bytes})
        # Pointing syn-b at the M2 file is refused because of where it was measured.
        borrowed = copy.deepcopy(b)
        declaration = borrowed["device"]["head"]["declaration"]
        declaration["devices"][1]["references"]["application"] = site.performance.PATH
        borrowed["device"]["head"]["sha256"] = hashlib.sha256(devices.encode(declaration)).hexdigest()
        borrowed["head"]["files"][devices.PATH] = borrowed["device"]["head"]["sha256"]
        with self.assertRaisesRegex(ValueError, "application reference was measured on 'Apple M2', not 'Synthetic device B'"):
            site.validate_compact_evidence(borrowed, {"reference.json": m2_bytes})

    def test_pooled_statistics_cover_every_observation_and_are_not_device_averages(self):
        entry = self.staged()["entries"][0]
        summary, overview = entry["benchmark_summary"], entry["overview"]
        # Errors 10, 30, 100, 50, 50, 50: mean 290 / 6, median 50, maximum 100.
        self.assertEqual((summary["required"], summary["comparable"]), (492, 6))
        self.assertAlmostEqual(summary["head"]["mean_error_percent"], 290 / 6)
        self.assertAlmostEqual(overview["median_error_percent"], 50)
        self.assertAlmostEqual(summary["head"]["maximum_error_percent"], 100)
        # The devices' own means are 20 and 62.5. Their average, 41.25, is not the commit's mean.
        own = [entry["devices"][ident]["benchmark_summary"]["head"]["mean_error_percent"] for ident in ("m2", "syn-b")]
        self.assertAlmostEqual(own[0], 20)
        self.assertAlmostEqual(own[1], 62.5)
        self.assertEqual(overview["contributors"], ["m2", "syn-b"])
        self.assertEqual(overview["devices"], {
            "m2": dict(state="report", required=246, reported=246, referenced=True, qualified=2),
            "syn-b": dict(state="report", required=246, reported=246, referenced=True, qualified=4),
            "syn-c": dict(state="report", required=246, reported=246, referenced=False, qualified=None),
            "syn-d": dict(state="pending")})
        page = self.page()
        text = view(page, "overview")
        self.assertIn("across 6 qualified observations: mean 48.3333%, median 50.0000%, maximum 100.0000%.", text)
        self.assertNotIn("41.25", text)
        # By measured time the observations are 1.0->1.1, 1.5->3.0, 2.0->2.6, 4->6, 8->12 and 16->24 ms.
        # Of the 15 pairs one is out of order, and it pairs two devices: 1.5->3.0 with 2.0->2.6.
        self.assertIn("Kendall’s τ-b = 0.8667", text)  # (14 - 1) / 15
        self.assertIn("Pairs ordered correctly: 93.33% (14/15)", text)
        pearson = statistics.correlation([1.0, 1.5, 2.0, 4.0, 8.0, 16.0], [1.1, 3.0, 2.6, 6.0, 12.0, 24.0])
        self.assertIn(f"Pearson correlation (r) = {pearson:.4f}", text)
        self.assertIn("6 qualified cases.", text)
        self.assertIn("Absolute error across 6 qualified cases", text)
        # Within each device every pair is in order, so averaging device results would report 1.
        for ident in ("m2", "syn-b"):
            self.assertIn("Kendall’s τ-b = 1.0000", view(page, "device-" + ident))
        self.assertIn("across 2 qualified", view(page, "device-m2"))
        # The percentile chart ranks the pooled errors: the 3rd of 6 is 50%, the 6th is 100%.
        self.assertIn("P50: <strong>50.00%</strong>", text)
        self.assertIn("P95: <strong>100.00%</strong>", text)
        # All three charts plot the six observations, each named by its device.
        labels = re.findall(r'data-tip="([^"]+)"', text)
        self.assertEqual(len(labels), 18)
        self.assertTrue(all(re.match(r"head: (rank \d · )?(m2|syn-b)/", label) for label in labels), labels)
        self.assertIn(f"rank 1 · m2/{self.first} · measured 1.0000 ms · predicted 1.1000 ms · 1.100× slower", text)
        self.assertIn(f"rank 6 · syn-b/{self.first} · measured 1.5000 ms · predicted 3.0000 ms · 2.000× slower", text)
        for ident in ("m2", "syn-b", "syn-c"):
            self.assertNotRegex(view(page, "device-" + ident), r'data-tip="[^"]*(m2|syn-b|syn-c)/')
        # Base and head are compared on the observations both predict: the two of m2.
        self.assertIn("Both rows use the same 2 qualified pooled observations with base and head predictions.", text)
        self.assertIn("<tr><td>base</td><td>15.0000%</td><td>15.0000%</td><td>1.0000</td><td>1.0000</td><td>100.00% (1/1)</td></tr>", text)
        self.assertIn("<tr><td>head</td><td>20.0000%</td><td>20.0000%</td><td>1.0000</td><td>1.0000</td><td>100.00% (1/1)</td></tr>", text)
        self.assertIn("6 qualified observations from 2 of 4 devices: m2 2 of 246, syn-b 4 of 246.", text)
        self.assertIn("Not pooled: syn-c (no native application reference: predictions only); syn-d (no report yet).", text)
        self.assertIn('id="pooled-partial"', text)
        self.assertNotIn('id="pooled-undefined"', text)
        rows = dict(re.findall(r'<tr data-row-key="([^"]+)">(.*?)</tr>', text))
        self.assertEqual(list(rows), ["m2", "syn-b", "syn-c", "syn-d"])
        for ident, cells in (
                ("m2", ("<td>Apple M2</td><td>Apple M2 model profile</td><td>measured</td>", ">report</span>", "2 qualified of 246")),
                ("syn-b", ("<td>Synthetic device B</td><td>Apple M2 model profile (placeholder; Synthetic device B parameters not established)</td>"
                           "<td>partial: application</td>", ">report</span>", "4 qualified of 246")),
                ("syn-c", ("<td>Synthetic device C</td>", "<td>unmeasured</td>", ">report</span>", "246 of 246 predicted; no native reference")),
                ("syn-d", ("<td>Synthetic device D</td>", "<td>unmeasured</td>", ">pending</span>", "<td>—</td>"))):
            for cell in cells:
                self.assertIn(cell, rows[ident], ident)
        # The landing page states the pooled mean for the commit, and nothing per device.
        self.assertIn("<td>48.3333% (6/492)</td>", self.home())

    def test_commit_without_a_qualified_observation_is_undefined_not_zero(self):
        # Only the unmeasured device has reported.
        index = self.publish(self.pages, self.device_report("syn-c"), "syn-c")
        entry = index["entries"][0]
        self.assertEqual(entry["benchmark_summary"], {})
        self.assertEqual((entry["overview"]["contributors"], entry["overview"]["median_error_percent"]), ([], None))
        self.assertIsNone(entry["series"])
        text = view(self.page(), "overview")
        self.assertIn('id="pooled-empty"', text)
        self.assertIn("No qualified observation at this commit.", text)
        self.assertIn("Not pooled: m2 (no report yet); syn-b (no report yet); syn-c (no native application reference: "
                      "predictions only); syn-d (no report yet).", text)
        for absent in ("0.00", "Kendall", "Pearson", "S-curve", "Base and head comparison", 'id="pooled-error"', "data-tip"):
            self.assertNotIn(absent, text)
        self.assertIn("<td>— (—/—)</td>", self.home())
        self.assertNotIn("Series ", self.home())
        # A measured device whose every reference is too noisy still has nothing to pool.
        one = (self.first, (1.5, 10470000, None))
        world = self.measured_source("quiet-source", {"m2": {}, "syn-b": dict([one])})
        index = self.publish(self.pages, self.device_report("m2", QUIET, world), "noisy", run=2, world=world)
        entry = next(e for e in index["entries"] if e["commit"] == QUIET)
        self.assertEqual(entry["benchmark_summary"], dict(required=246, comparable=0, head_comparable=0, base_comparable=0))
        self.assertEqual(entry["overview"]["devices"]["m2"]["qualified"], 0)
        text = view(self.page(QUIET), "overview")
        self.assertIn('id="pooled-empty"', text)
        self.assertIn("The reported devices that have a native application reference have no qualified application case.", text)
        self.assertIn("0 qualified of 246", text)
        self.assertNotIn("0.00", text)
        self.assertIn("<td>— (0/246)</td>", self.home())
        # One observation has an error, and no correlation or ordering.
        index = self.publish(self.pages, self.device_report("syn-b", QUIET, world), "single", run=3, world=world)
        entry = next(e for e in index["entries"] if e["commit"] == QUIET)
        self.assertEqual(entry["benchmark_summary"]["comparable"], 1)
        self.assertAlmostEqual(entry["benchmark_summary"]["head"]["mean_error_percent"], 100)
        text = view(self.page(QUIET), "overview")
        self.assertIn("across 1 qualified observations: mean 100.0000%, median 100.0000%, maximum 100.0000%.", text)
        self.assertIn('id="pooled-undefined"', text)
        self.assertIn("Undefined for these observations, shown as —: Pearson r, Kendall τ-b, the share of correctly ordered pairs.", text)
        self.assertIn("Kendall’s τ-b = —", text)
        self.assertIn("Pearson correlation (r) = —", text)
        check_site(self, self.pages)

    def test_devices_arrive_one_at_a_time_in_the_one_report_of_their_commit(self):
        folder, index_path = self.folder_of(self.commit), self.pages / "calibration/index.json"

        def arrived(status, coverage, states):
            """One row and one report for the commit, whatever has arrived, with every link working."""
            index, home, page = site.read(index_path), self.home(), self.page()
            self.assertEqual([entry["commit"] for entry in index["entries"]], [self.commit])
            self.assertEqual(index["entries"][0]["artifact_status"], status)
            self.assertEqual(home.count("<tr><td><a href="), 1)
            self.assertIn(f'<tr><td><a href="runs/{self.commit}/">{self.commit[:12]}</a></td><td>success</td>'
                          f'<td><span class="badge {status}">{status}</span></td><td>{coverage}</td>', home)
            self.assertEqual(re.findall(r'<a href="#device-([^"]+)">\1 · (\w+)</a>', page), states)
            self.assertEqual(site.read(folder / "commit.json")["artifact_status"], status)
            self.assertEqual(sorted(str(path.relative_to(self.pages)) for path in self.pages.rglob("index.html")),
                             ["calibration/index.html", f"calibration/runs/{self.commit}/index.html", "index.html"])
            check_site(self, self.pages)
            return index["entries"][0], page

        def kept(*names):
            return {name: (folder / name).read_bytes() for name in names}

        # First arrival: the default device. Its files sit where published reports always had them.
        m2 = self.device_report("m2")
        self.publish(self.pages, m2, "m2")
        entry, page = arrived("partial", "1 of 4 reported",
                              [("m2", "report"), ("syn-b", "pending"), ("syn-c", "pending"), ("syn-d", "pending")])
        self.assertEqual(sorted(path.name for path in folder.iterdir()),
                         ["bundle.json", "commit.json", "devices.json", "forwarding.json", "index.html", "metadata.json",
                          "reference.json", "report.json", "report.md", "throughput.json"])
        self.assertIn("2 qualified observations from 1 of 4 devices: m2 2 of 246.", page)
        self.assertIn("mean 20.0000%, median 20.0000%, maximum 30.0000%", view(page, "overview"))
        self.assertIn("No report has been published for this device at this commit.", view(page, "device-syn-b"))
        self.assertEqual(site.read(index_path)["latest_success"], self.commit)
        default = kept("report.json", "bundle.json", "reference.json")

        # Second arrival: another device joins the same report and leaves the first one's files alone.
        proof = "https://github.com/sarchlab/poros/actions/runs/20/attempts/1"
        self.publish(self.pages, self.device_report("syn-b"), "syn-b", run=2, evaluation_run_id=20, evaluation_run_url=proof)
        entry, page = arrived("partial", "2 of 4 reported",
                              [("m2", "report"), ("syn-b", "report"), ("syn-c", "pending"), ("syn-d", "pending")])
        self.assertEqual(kept(*default), default)
        # Each device keeps the Evaluation that produced its own evidence; the commit shows the newest.
        self.assertEqual([entry["devices"]["m2"]["evaluation_run_id"], entry["devices"]["syn-b"]["evaluation_run_id"],
                          entry["evaluation_run_id"]], [1, 20, 20])
        self.assertIn(f'<a href="{proof}">Full Evaluation proof</a>', view(page, "device-syn-b"))
        self.assertNotIn("/runs/20/", view(page, "device-m2"))
        self.assertEqual(site.read(folder / "metadata.json")["evaluation_run_id"], 1)
        self.assertEqual(sorted(path.name for path in (folder / "devices/syn-b").iterdir()),
                         ["bundle.json", "metadata.json", "reference.json", "report.json", "report.md"])
        self.assertEqual(site.read(folder / "devices/syn-b/metadata.json")["device"], "syn-b")
        self.assertEqual(site.read(folder / "metadata.json")["device"], "m2")
        self.assertIn("mean 48.3333%, median 50.0000%, maximum 100.0000%", view(page, "overview"))
        second = kept("devices/syn-b/report.json", "devices/syn-b/bundle.json", "devices/syn-b/reference.json")

        # A repeated arrival keeps the report already retained and records the newer run.
        again = self.device_report("m2")
        again["head_binary_sha256"] = "9" * 64
        self.publish(self.pages, again, "m2-again", run=3)
        entry, page = arrived("partial", "2 of 4 reported",
                              [("m2", "report"), ("syn-b", "report"), ("syn-c", "pending"), ("syn-d", "pending")])
        self.assertEqual((kept(*default), kept(*second)), (default, second))
        self.assertEqual((entry["devices"]["m2"]["run_id"], entry["devices"]["syn-b"]["run_id"]), (3, 2))

        # One device fails. The commit's report says so; its siblings and the pooled numbers stay.
        self.publish_failure(self.pages, self.commit, "syn-d", "syn-d-failed", run=4)
        entry, page = arrived("fail", "2 of 4 reported, 1 failed",
                              [("m2", "report"), ("syn-b", "report"), ("syn-c", "pending"), ("syn-d", "failed")])
        self.assertEqual((kept(*default), kept(*second)), (default, second))
        self.assertIn("simulation failed", view(page, "device-syn-d"))
        self.assertNotIn("simulation failed", view(page, "device-m2") + view(page, "device-syn-b"))
        self.assertIn("mean 48.3333%, median 50.0000%, maximum 100.0000%", view(page, "overview"))
        self.assertIn("Not pooled: syn-c (no report yet); syn-d (failed evidence).", view(page, "overview"))
        self.assertEqual(entry["overview"]["devices"]["syn-d"], dict(state="failed"))
        self.assertIsNone(site.read(index_path)["latest_success"])
        self.assertEqual(site.read(folder / "commit.json")["devices"]["syn-d"],
                         dict(state="failed", artifact_status="fail", bundle="devices/syn-d/bundle.json",
                              bundle_sha256=site.digest(folder / "devices/syn-d/bundle.json")))

        # Publication is refused, and nothing is stored, when the metadata names another device,
        # when the device is not one this commit declares, or when a report of this commit carries
        # another declaration, such as the repository's present one.
        before = index_path.read_bytes()
        b = self.bundle(self.device_report("syn-b"), "syn-b-relabelled", self.world[0])
        for device in (None, "syn-c"):
            with self.assertRaisesRegex(ValueError, "publication names device"):
                site.archive(self.pages, b, self.metadata(device=device, run=5), self.order)
        with self.assertRaisesRegex(ValueError, "unknown device 'm1'"):
            self.publish_failure(self.pages, self.commit, "m1", "undeclared", run=5)
        with self.assertRaisesRegex(ValueError, "conflicting device declarations for one commit"):
            self.publish(self.pages, self.report("m1"), "other-declaration", run=5)
        self.assertEqual(index_path.read_bytes(), before)
        self.assertFalse((folder / "devices/m1").exists())

        # The failed device reports, replacing its failure; then the last device arrives as a release.
        self.publish(self.pages, self.device_report("syn-d"), "syn-d", run=6)
        entry, page = arrived("partial", "3 of 4 reported",
                              [("m2", "report"), ("syn-b", "report"), ("syn-c", "pending"), ("syn-d", "report")])
        self.assertNotIn("simulation failed", page)
        self.publish(self.pages, self.device_report("syn-c"), "syn-c", run=7, kind="release", release={"id": 7, "tag": "v1.0"})
        entry, page = arrived("pass", "4 of 4 reported",
                              [("m2", "report"), ("syn-b", "report"), ("syn-c", "report"), ("syn-d", "report")])
        self.assertEqual((kept(*default), kept(*second)), (default, second))
        self.assertNotIn('id="pooled-partial"', page)
        self.assertEqual(entry["releases"], [{"id": 7, "tag": "v1.0"}])
        self.assertIn("Pinned by release: v1.0", page)
        self.assertIn("<td>v1.0</td>", self.home())
        self.assertEqual(site.read(index_path)["latest_success"], self.commit)
        self.assertEqual(sorted(path.name for path in (folder / "devices").iterdir()), ["syn-b", "syn-c", "syn-d"])
        # The landing page is the list of commits. It has no catalog of devices or hardware.
        home = self.home()
        for absent in ("syn-b", "Synthetic device", "Apple M", "data-row-key", "devices/"):
            self.assertNotIn(absent, home)

    def test_default_device_arriving_last_keeps_its_siblings(self):
        # The default device's bundle shares the commit directory with the commit's own files
        # and with the other devices' directories. Replacing it must remove none of them.
        folder = self.folder_of(self.commit)
        self.publish(self.pages, self.device_report("syn-b"), "syn-b")
        self.publish(self.pages, self.device_report("syn-c"), "syn-c", run=2)
        def siblings():
            # metadata.json also records the commit's latest evaluation, which later publications update.
            return {str(path.relative_to(folder)): b"" if path.name == "metadata.json" else path.read_bytes()
                    for path in sorted((folder / "devices").rglob("*")) if path.is_file()}
        kept = siblings()
        self.assertEqual(sorted(kept), [f"devices/{ident}/{name}" for ident, names in (
            ("syn-b", ("bundle.json", "metadata.json", "reference.json", "report.json", "report.md")),
            ("syn-c", ("bundle.json", "metadata.json", "report.json", "report.md"))) for name in names])
        self.assertEqual(sorted(path.name for path in folder.iterdir()), ["commit.json", "devices", "devices.json", "index.html"])
        self.assertIn("4 qualified observations from 1 of 4 devices: syn-b 4 of 246.", self.page())
        index = self.publish_failure(self.pages, self.commit, None, "m2-failed", run=3)
        self.assertEqual((index["entries"][0]["artifact_status"], siblings()), ("fail", kept))
        self.assertEqual(sorted(path.name for path in folder.iterdir()),
                         ["bundle.json", "commit.json", "devices", "devices.json", "index.html", "metadata.json", "report.json", "report.md"])
        index = self.publish(self.pages, self.device_report("m2"), "m2", run=4)
        entry = index["entries"][0]
        self.assertEqual((entry["artifact_status"], siblings()), ("partial", kept))
        self.assertEqual(list(entry["devices"]), ["syn-b", "syn-c", "m2"])
        # The report still lists devices in declared order, not in order of arrival.
        self.assertEqual(re.findall(r'<a href="#device-([^"]+)">\1 · (\w+)</a>', self.page()),
                         [("m2", "report"), ("syn-b", "report"), ("syn-c", "report"), ("syn-d", "pending")])
        self.assertIn("6 qualified observations from 2 of 4 devices: m2 2 of 246, syn-b 4 of 246.", self.page())
        self.assertEqual(site.validate_bundle(folder, self.commit)[0], self.device_report("m2"))
        check_site(self, self.pages)

    def test_a_commit_is_evicted_or_pinned_with_all_its_devices(self):
        for run, ident in enumerate(("m2", "syn-b"), 1):
            self.publish(self.pages, self.device_report(ident, LATER), "later-" + ident, run=run)
        # A release pins its commit through whichever device published it.
        self.publish(self.pages, self.device_report("syn-b"), "pinned", run=3, kind="release", release={"id": 7, "tag": "v1.0"})
        self.assertTrue((self.folder_of(LATER) / "devices/syn-b/report.json").is_file())
        newest = self.bundle(at_commit(self.legacy, LEGACY), "newest")
        index = site.archive(self.pages, newest, self.metadata(run=4, commit=LEGACY), [LEGACY, LATER, self.commit], keep=1)
        self.assertEqual([entry["commit"] for entry in index["entries"]], [LEGACY, self.commit])
        self.assertFalse(self.folder_of(LATER).exists())
        pinned = index["entries"][1]
        self.assertEqual((pinned["releases"], pinned["main"], list(pinned["devices"])), ([{"id": 7, "tag": "v1.0"}], False, ["syn-b"]))
        self.assertTrue((self.folder_of(self.commit) / "devices/syn-b/report.json").is_file())
        # The pin belongs to the commit: another device of it cannot move the release elsewhere.
        with self.assertRaisesRegex(ValueError, "a permanently pinned release cannot move to another commit"):
            self.publish(self.pages, self.device_report("m2", LATER), "moved", run=5, kind="release", release={"id": 7, "tag": "v1.0"})
        index = self.publish(self.pages, self.device_report("m2"), "pinned-m2", run=6, kind="release", release={"id": 7, "tag": "v1.0"})
        pinned = next(entry for entry in index["entries"] if entry["commit"] == self.commit)
        self.assertEqual((pinned["releases"], list(pinned["devices"])), ([{"id": 7, "tag": "v1.0"}], ["syn-b", "m2"]))
        check_site(self, self.pages)

    def test_each_commit_shows_the_devices_it_declared_itself(self):
        reports = [at_commit(self.legacy, LEGACY), self.device_report("m2"), self.device_report("syn-b")]
        # Nothing here may be taken from the devices the reporting checkout declares today.
        with patch.object(devices, "load", side_effect=AssertionError("read the current device catalog")):
            for run, (report, name) in enumerate(zip(reports, ("legacy", "m2", "syn-b")), 1):
                index = self.publish(self.pages, report, name, run=run)
            old, new = self.folder_of(LEGACY), self.folder_of(self.commit)
            self.assertEqual(index["schema"], 2)
            self.assertEqual([entry["commit"] for entry in index["entries"]], [self.commit, LEGACY])
            declared, legacy = index["entries"]
            # The commit that predates declarations is the default device alone.
            self.assertEqual(legacy["declaration"], dict(status="legacy", origin="report"))
            self.assertEqual((list(legacy["devices"]), legacy["artifact_status"]), (["m2"], "pass"))
            self.assertEqual(site.read(old / "commit.json")["declaration"], legacy["declaration"])
            self.assertFalse((old / "devices.json").exists())
            page = self.page(LEGACY)
            self.assertEqual(re.findall(r'<a href="#device-([^"]+)">', page.split("</nav>")[0]), ["m2"])
            self.assertIn("This revision predates device declarations.", page)
            self.assertIn('<td>Apple M2</td><td>m2 (implied)</td><td>measured (legacy evidence)</td>', page)
            self.assertIn("246 qualified observations from 1 of 1 devices: m2 246 of 246.", page)
            for absent in ("syn-b", "Synthetic device", "m1", "Apple M1", "grace", "m5-max", 'id="pooled-partial"', "devices.json"):
                self.assertNotIn(absent, page)
            self.assertIn("<td>1 of 1 reported</td>", self.home())
            # The declared commit lists its own four, in declared order, from its retained copy.
            self.assertEqual(re.findall(r'<a href="#device-([^"]+)">', self.page().split("</nav>")[0]),
                             ["m2", "syn-b", "syn-c", "syn-d"])
            raw = (new / "devices.json").read_bytes()
            self.assertEqual(raw, devices.encode(self.declared))
            record = site.read(new / "commit.json")["declaration"]
            self.assertEqual(record, dict(status="declared", origin="report", path=devices.PATH,
                                          sha256=hashlib.sha256(raw).hexdigest(), declaration=self.declared))
            self.assertEqual(record["sha256"], reports[1]["head"]["files"][devices.PATH])
            self.assertEqual(declared["declaration"], record)
            self.assertIn(f'SHA-256 <code>{record["sha256"]}</code> (<a href="devices.json">retained copy</a>)', self.page())
            for ident in ("m1", "grace", "m5-max"):
                self.assertNotIn(ident, view(self.page(), "overview"))
            # A device the old revision could not model is refused there, not filed as the default.
            with self.assertRaisesRegex(ValueError, "predates device declarations and models only m2, not 'syn-b'"):
                self.publish_failure(self.pages, LEGACY, "syn-b", "impossible", run=4)
            # Later publications re-render every commit from what it retains: the old page does not change.
            before = self.page(LEGACY)
            self.publish(self.pages, self.device_report("syn-c"), "syn-c", run=5)
            self.assertEqual(self.page(LEGACY), before)
            check_site(self, self.pages)
            # A retained declaration that no longer matches its digest stops publication.
            index_path = self.pages / "calibration/index.json"
            good = index_path.read_bytes()
            tampered = json.loads(good)
            tampered["entries"][0]["declaration"]["declaration"]["devices"].append(
                dict(id="m1", hardware="Apple M1", parameter_source="m2", references={}))
            site.write(index_path, tampered)
            with self.assertRaisesRegex(ValueError, "retained device declaration does not match its recorded digest"):
                self.publish(self.pages, self.device_report("syn-d"), "syn-d", run=6)
            index_path.write_bytes(good)

    def test_archive_published_before_devices_becomes_default_device_reports(self):
        # What the publisher wrote before commits had devices: schema 1, one report per commit.
        legacy = at_commit(self.legacy, LEGACY)
        bundle = self.bundle(legacy, "legacy")
        old = self.folder_of(LEGACY)
        shutil.copytree(bundle, old)
        (old / "index.html").write_text("the page this commit was published with")
        manifest = site.read(bundle / "bundle.json")
        metadata = self.metadata(run=1, commit=LEGACY)
        entry = dict({key: value for key, value in metadata.items() if key != "kind"}, main=True, releases=[{"id": 3, "tag": "v0.9"}],
                     artifact_status="pass", accuracy_status="unverified", series=site.series_key(legacy), summary=legacy["summary"],
                     benchmark_summary=site.benchmark_summary(legacy), evidence_run_url=metadata["run_url"],
                     evidence_files=sorted(manifest["files"]), measurement_completed_at="2026-01-01T00:10:00Z")
        site.write(self.pages / "calibration/index.json", dict(schema=1, main_head=LEGACY, latest_success=LEGACY, entries=[entry]))
        published = {name: (old / name).read_bytes() for name in [*manifest["files"], "bundle.json"]}
        index = self.publish(self.pages, self.device_report("m2"), "m2", run=2)
        self.assertEqual(index["schema"], 2)
        migrated = next(e for e in index["entries"] if e["commit"] == LEGACY)
        self.assertEqual((list(migrated["devices"]), migrated["declaration"]), (["m2"], dict(status="legacy", origin="report")))
        self.assertEqual((migrated["releases"], migrated["main"], migrated["evaluation_conclusion"]), ([{"id": 3, "tag": "v0.9"}], True, "success"))
        self.assertEqual(migrated["devices"]["m2"]["summary"], legacy["summary"])
        self.assertEqual(migrated["benchmark_summary"], site.benchmark_summary(legacy))
        self.assertEqual(migrated["series"], entry["series"])  # The trend continues the published series.
        # The same address still opens the commit; its files are the ones published.
        self.assertEqual({name: (old / name).read_bytes() for name in published}, published)
        page = self.page(LEGACY)
        self.assertIn("Pinned by release: v0.9", page)
        self.assertIn('<section class="view" id="device-m2"', page)
        # Links into the report as it was published land in the default device's view.
        parsed = check_site(self, self.pages)[(old / "index.html").resolve()]
        m2 = view(page, "device-m2")
        for name in ("overall", "benchmark-table", "microbenchmarks", "throughput-table"):
            self.assertIn(name, parsed.ids)
            self.assertIn(f'id="{name}"', m2)

    def test_failed_evidence_alone_leaves_the_devices_unknown_until_the_source_is_read(self):
        repository = self.root / "repository"
        repository.mkdir()
        def git(*args):
            return subprocess.check_output(["git", "-C", str(repository), *args], text=True, stderr=subprocess.DEVNULL).strip()
        def commit(message):
            git("add", ".")
            git("commit", "--quiet", "-m", message)
            return git("rev-parse", "HEAD")
        git("init", "--quiet")
        git("config", "user.name", "Test")
        git("config", "user.email", "test@example.org")
        (repository / "README").write_text("before devices")
        before = commit("no declaration")
        (repository / devices.PATH).parent.mkdir(parents=True)
        (repository / devices.PATH).write_bytes(devices.encode(self.declared))
        after = commit("declare devices")
        self.order = [after, before]
        folder = self.folder_of(after)
        # A failure names a device and proves nothing about what its commit declares.
        self.publish_failure(self.pages, after, None, "failed")
        index = self.publish_failure(self.pages, after, "m1", "failed-m1", run=2)
        entry = index["entries"][0]
        self.assertEqual((entry["declaration"], entry["artifact_status"], list(entry["devices"])), (dict(status="unknown"), "fail", ["m2", "m1"]))
        page = self.page(after)
        self.assertIn("Supported devices are unknown for this commit.", page)
        self.assertEqual(re.findall(r'<a href="#device-([^"]+)">', page.split("</nav>")[0]), ["m1", "m2"])
        rows = dict(re.findall(r'<tr data-row-key="([^"]+)">(.*?)</tr>', view(page, "overview")))
        for ident in ("m1", "m2"):
            self.assertIn('<td>unknown</td><td>unknown</td><td>unknown</td><td><span class="badge fail">failed</span></td><td>—</td>', rows[ident])
        for absent in ("syn-b", "Apple M", "pending", "devices.json"):
            self.assertNotIn(absent, page)
        self.assertIn('id="pooled-empty"', page)
        self.assertIn("<td>declaration unknown; 2 failed</td>", self.home())
        self.assertIn("<td>— (—/—)</td>", self.home())
        self.assertFalse((folder / "devices.json").exists())
        self.assertEqual(site.read(folder / "commit.json")["declaration"], dict(status="unknown"))
        check_site(self, self.pages)
        # With the repository at hand the declaration is read at exactly that commit. The attempt
        # for a device the commit does not declare is dropped; the declared ones wait for reports.
        index = self.publish_failure(self.pages, after, None, "failed-again", run=3, repository=repository)
        entry = index["entries"][0]
        self.assertEqual(entry["declaration"], dict(status="declared", origin="source", path=devices.PATH,
                                                    sha256=hashlib.sha256(devices.encode(self.declared)).hexdigest(),
                                                    declaration=self.declared))
        self.assertEqual(list(entry["devices"]), ["m2"])
        self.assertFalse((folder / "devices/m1").exists())
        page = self.page(after)
        self.assertEqual(re.findall(r'<a href="#device-([^"]+)">\1 · (\w+)</a>', page),
                         [("m2", "failed"), ("syn-b", "pending"), ("syn-c", "pending"), ("syn-d", "pending")])
        self.assertIn("read from the repository at this commit", page)
        self.assertEqual((folder / "devices.json").read_bytes(), devices.encode(self.declared))
        self.assertIn("<td>0 of 4 reported, 1 failed</td>", self.home())
        with self.assertRaisesRegex(ValueError, "unknown device 'm1'"):
            self.publish_failure(self.pages, after, "m1", "undeclared", run=4, repository=repository)
        # The same repository says the earlier commit predates declarations.
        index = self.publish_failure(self.pages, before, None, "failed-old", run=5, repository=repository)
        old = next(e for e in index["entries"] if e["commit"] == before)
        self.assertEqual(old["declaration"], dict(status="legacy", origin="source"))
        page = self.page(before)
        self.assertIn("This revision predates device declarations.", page)
        self.assertIn("<td>not recorded</td><td>m2 (implied)</td><td>unknown without a report</td>", page)
        with self.assertRaisesRegex(ValueError, "predates device declarations and models only m2, not 'syn-b'"):
            self.publish_failure(self.pages, before, "syn-b", "impossible", run=6, repository=repository)
        # The retained record stands on later publications that have no repository.
        index = self.publish_failure(self.pages, after, "syn-c", "failed-c", run=7)
        self.assertEqual(index["entries"][0]["declaration"]["origin"], "source")
        self.assertEqual(list(index["entries"][0]["devices"]), ["m2", "syn-c"])
        check_site(self, self.pages)

    def test_device_views_share_no_element_link_or_data(self):
        self.staged()
        self.publish_failure(self.pages, self.commit, "syn-d", "syn-d-failed", run=4)
        folder, page = self.folder_of(self.commit), self.page()
        parsed = check_site(self, self.pages)[(folder / "index.html").resolve()]
        # One document: the views are sections of it, reached by fragment links and not by scripts.
        self.assertEqual([name for name in parsed.ids if name == "overview" or re.fullmatch(r"device-[a-z0-9-]+?", name) and
                          f'<section class="view" id="{name}"' in page],
                         ["overview", "device-m2", "device-syn-b", "device-syn-c", "device-syn-d"])
        navigation = page[page.index('<nav class="views"'):page.index("</nav>")]
        self.assertEqual(re.findall(r'href="([^"]+)"', navigation),
                         ["#overview", "#device-m2", "#device-syn-b", "#device-syn-c", "#device-syn-d"])
        self.assertIn("@supports selector(:has(*))", page)
        for name in ("overview", "device-m2", "device-syn-b", "device-syn-c", "device-syn-d"):
            self.assertIn(f'.report:has(#{name}:target,#{name} :target) .views a[href="#{name}"]', page)
        self.assertNotIn("<script src", page)
        # The default device keeps the element names its reports were published with.
        sections = {ident: view(page, "device-" + ident) for ident in ("m2", "syn-b", "syn-c", "syn-d")}
        # No name of its own starts like a view's, so no device's view can take one.
        self.assertIn('id="overall"', sections["m2"])
        self.assertIn('id="device-syn-b_overall"', sections["syn-b"])
        self.assertNotIn('id="overall"', sections["syn-b"])
        self.assertEqual([name for name in re.findall(r' id="([^"]+)"', sections["m2"]) if name.startswith(("device-", "view-"))],
                         ["device-m2", "view-m2-title"])
        hardware = {"m2": "Apple M2", "syn-b": "Synthetic device B", "syn-c": "Synthetic device C", "syn-d": "Synthetic device D"}
        for ident, section in sections.items():
            own = "" if ident == "m2" else f"devices/{ident}/"
            self.assertIn(f'data-device="{ident}"', section)
            self.assertIn(hardware[ident], section)
            for other, name in hardware.items():
                if other not in (ident, "m2"):  # Every device here still borrows the M2 parameters, and says so.
                    self.assertNotIn(name, section, (ident, other))
            for name in re.findall(r' id="([^"]+)"', section):
                self.assertTrue(ident == "m2" or name in (f"device-{ident}", f"view-{ident}-title") or name.startswith(f"device-{ident}_"),
                                (ident, name))
            for link in re.findall(r' href="([^"]+)"', section):
                if link.startswith("#"):
                    self.assertTrue(link == "#overview" or link.startswith(f"#device-{ident}_") or ident == "m2" and not link.startswith("#device-"),
                                    (ident, link))
                elif not link.startswith("https://"):
                    self.assertTrue(link.startswith(own) and (own or not link.startswith("devices/")), (ident, link))
                    self.assertTrue((folder / link).is_file(), (ident, link))
        # Each view offers its own device's evidence, kept once, in that device's bundle.
        self.assertIn('href="reference.json"', sections["m2"])
        self.assertIn('href="devices/syn-b/reference.json"', sections["syn-b"])
        self.assertNotIn("reference.json", sections["syn-c"])
        self.assertEqual(site.read(folder / "reference.json")["hardware"], "Apple M2")
        self.assertEqual(site.read(folder / "devices/syn-b/reference.json")["hardware"], "Synthetic device B")
        for ident in ("m2", "syn-b", "syn-c"):
            bundle = site.bundle_dir(self.pages, self.commit, ident)
            report, _ = site.validate_bundle(bundle, self.commit)
            self.assertEqual(site.report_device(report), ident)
            self.assertEqual(report, self.device_report(ident))
        self.assertEqual(sorted(path.name for path in (folder / "devices/syn-c").iterdir()),
                         ["bundle.json", "metadata.json", "report.json", "report.md"])
        # The commit record points at the bundles by digest. It repeats none of their rows.
        record = site.read(folder / "commit.json")
        self.assertEqual(record["devices"]["syn-b"], dict(state="report", artifact_status="pass", bundle="devices/syn-b/bundle.json",
                                                          bundle_sha256=site.digest(folder / "devices/syn-b/bundle.json")))
        self.assertEqual(record["devices"]["m2"]["bundle"], "bundle.json")
        self.assertNotIn("cases", json.dumps(record))
        self.assertLess(len(json.dumps(record)), 20000)
        # The unmeasured device makes no accuracy statement, and the failed one has no value to show.
        for absent in ("Kendall", "Pearson", "Mean error"):
            self.assertNotIn(absent, sections["syn-c"])
        self.assertIn("simulation failed", sections["syn-d"])
        self.assertNotIn("data-tip", sections["syn-d"])
        self.assertNotRegex(sections["syn-d"], r"\d%")
        # A reused default-device bundle is that device's files only: not the commit's page or its siblings.
        with patch.object(site, "git", return_value=self.commit):
            source = self.root / "checkout"
            source.mkdir()
            record = backfill.route(source, self.pages, self.root / "reuse", dict(kind="release", commit=self.commit))
        self.assertEqual(record["route"], "reuse")
        self.assertEqual(sorted(path.name for path in (self.root / "reuse/evidence").iterdir()),
                         ["bundle.json", "forwarding.json", "metadata.json", "reference.json", "report.json", "report.md", "throughput.json"])
        site.validate_bundle(self.root / "reuse/evidence", self.commit)

    def test_live_commit_view_is_the_same_report_while_devices_run(self):
        source = self.world[0]
        state = dict(state="running", completed_cases=264, required_cases=264, completed_shards=131, required_shards=132,
                     active_cases=[], errors=[], updated_at=datetime.now(timezone.utc).isoformat())
        paths = {}
        for ident in ("m2", "syn-b"):
            paths[ident] = self.root / (ident + ".json")
            site.write(paths[ident], progress.snapshot(self.device_report(ident), source, state))
        rendered, assets = live.CommitView([paths["syn-b"], paths["m2"]]).load()
        page = rendered.decode()
        self.assertEqual(sorted(assets), ["devices/syn-b/reference.json", "devices/syn-b/snapshot.json", "forwarding.json",
                                          "reference.json", "snapshot.json", "throughput.json"])
        self.assertEqual(json.loads(assets["devices/syn-b/reference.json"])["hardware"], "Synthetic device B")
        self.assertEqual(json.loads(assets["reference.json"])["hardware"], "Apple M2")
        # Declared order, whatever order the snapshots were given in; devices without one are pending.
        self.assertEqual(re.findall(r'<a href="#device-([^"]+)">\1 · (\w+)</a>', page),
                         [("m2", "running"), ("syn-b", "running"), ("syn-c", "pending"), ("syn-d", "pending")])
        text = view(page, "overview")
        self.assertIn("Live commit view · Not a published result", page)
        self.assertIn("across 6 qualified observations: mean 48.3333%, median 50.0000%, maximum 100.0000%.", text)
        self.assertIn("Kendall’s τ-b = 0.8667", text)
        self.assertIn('id="pooled-partial"', text)
        self.assertIn("location.reload()", page)
        for absent in ('href="devices.json"', "commit.json", "Report history"):  # Nothing is retained for a live view.
            self.assertNotIn(absent, page)
        parsed = Page(page)
        self.assertEqual([name for name, count in collections.Counter(parsed.ids).items() if count > 1], [])
        for link in parsed.links:
            if link.startswith("#"):
                self.assertIn(link[1:], parsed.ids)
            elif not link.startswith("https://"):
                self.assertIn(link, assets)
        # The loopback server serves that page and each device's assets under the linked names.
        def fetch(snapshots, path):
            server = ThreadingHTTPServer(("127.0.0.1", 0), live.handler(live.CommitView(snapshots)))
            worker = threading.Thread(target=server.handle_request)
            with patch.object(live.BaseHTTPRequestHandler, "log_message"):  # No request log among the test results.
                worker.start()
                try:
                    with urllib.request.urlopen(f"http://127.0.0.1:{server.server_port}{path}", timeout=60) as response:
                        return response.status, response.read()
                except urllib.error.HTTPError as error:
                    return error.code, error.read()
                finally:
                    worker.join()
                    server.server_close()
        both = [paths["m2"], paths["syn-b"]]
        status, served = fetch(both, "/")
        self.assertEqual((status, view(served.decode(), "overview")), (200, text))
        self.assertEqual(fetch(both, "/devices/syn-b/reference.json"), (200, assets["devices/syn-b/reference.json"]))
        self.assertEqual(fetch(both, "/reference.json"), (200, assets["reference.json"]))
        self.assertEqual(fetch(both, "/devices/syn-c/snapshot.json")[0], 404)
        status, refused = fetch([paths["m2"], paths["m2"]], "/")
        self.assertEqual(status, 503)
        self.assertIn("one snapshot per device, all of one commit", refused.decode())
        # One snapshot per device, all of one commit, and only devices that commit declares.
        other = self.root / "other.json"
        site.write(other, progress.snapshot(self.device_report("syn-c", LATER), source, state))
        with self.assertRaisesRegex(ValueError, "one snapshot per device, all of one commit"):
            live.CommitView([paths["m2"], other]).load()
        with self.assertRaisesRegex(ValueError, "one snapshot per device, all of one commit"):
            live.CommitView([paths["m2"], paths["m2"]]).load()
        site.write(other, progress.snapshot(self.report("m1"), self.source, state))
        with self.assertRaisesRegex(ValueError, "conflicting device declarations for one commit"):
            live.CommitView([paths["m2"], other]).load()
        with self.assertRaisesRegex(ValueError, "a final bundle belongs to one device"):
            live.serve([paths["m2"], paths["syn-b"]], self.root / "bundle", 0)
        # A failed device is shown as failed beside the one still running.
        site.write(paths["syn-b"], progress.snapshot(self.device_report("syn-b"), source, dict(state, state="failed", errors=["shard 3 failed"])))
        page = live.CommitView([paths["m2"], paths["syn-b"]]).load()[0].decode()
        self.assertIn('<a href="#device-syn-b">syn-b · failed</a>', page)
        self.assertIn("shard 3 failed", view(page, "device-syn-b"))
        self.assertNotIn("shard 3 failed", view(page, "device-m2"))
        # A device whose campaign completed is a report; the page stops reloading once none is running.
        done = dict(state, state="complete", completed_shards=132)
        site.write(paths["syn-b"], progress.snapshot(self.device_report("syn-b"), source, done))
        page = live.CommitView([paths["m2"], paths["syn-b"]]).load()[0].decode()
        self.assertEqual(re.findall(r'<a href="#device-([^"]+)">\1 · (\w+)</a>', page)[:2], [("m2", "running"), ("syn-b", "report")])
        self.assertIn("location.reload()", page)
        site.write(paths["m2"], progress.snapshot(self.device_report("m2"), source, done))
        page = live.CommitView([paths["m2"], paths["syn-b"]]).load()[0].decode()
        self.assertEqual(re.findall(r'<a href="#device-([^"]+)">\1 · (\w+)</a>', page)[:2], [("m2", "report"), ("syn-b", "report")])
        self.assertNotIn("location.reload()", page)
        self.assertIn('id="pooled-partial"', page)  # Two declared devices still have no snapshot.

    def test_staged_site_can_be_walked_from_history_to_commit_to_device(self):
        """The preview a reader clicks through: history, then a commit, then its devices."""
        self.staged()
        self.publish_failure(self.pages, self.commit, "syn-d", "syn-d-failed", run=4)
        self.publish(self.pages, at_commit(self.legacy, LEGACY), "legacy", run=5)
        self.publish(self.pages, self.device_report("syn-c", LATER), "later", run=6)
        self.publish_failure(self.pages, QUIET, "m1", "quiet", run=7)
        parsed = check_site(self, self.pages)
        home = parsed[(self.pages / "calibration/index.html").resolve()]
        # History: one link per commit row, each to that commit's one report.
        rows = [link for link in home.links if link.startswith("runs/")]
        self.assertEqual(sorted(set(rows)), sorted(f"runs/{commit}/" for commit in self.order))
        self.assertEqual(len(rows), len(self.order) + 1)  # The latest successful report is also linked above the table.
        # Commit: the overview links each device view, and each view links back.
        report = parsed[(self.folder_of(self.commit) / "index.html").resolve()]
        for ident in ("m2", "syn-b", "syn-c", "syn-d"):
            self.assertGreaterEqual(report.links.count("#device-" + ident), 2)  # Navigation and the device table.
        self.assertGreaterEqual(report.links.count("#overview"), 5)
        self.assertIn("../../index.html", report.links)
        # A copied link to a device view, or to something inside it, names an element of this same file.
        for fragment in ("device-syn-b", "device-syn-b_overall", "overall", "pooled-accuracy", "commit-devices"):
            self.assertIn(fragment, report.ids)
        statuses = dict(re.findall(r'<tr><td><a href="runs/([0-9a-f]{40})/">.*?<span class="badge (\w+)">', self.home()))
        self.assertEqual(statuses, {self.commit: "fail", LATER: "partial", QUIET: "fail", LEGACY: "pass"})
        # For a browser: keep this site, built by the real archive and renderer from synthetic values, where asked.
        if os.environ.get("POROS_DEVICE_PAGES_PREVIEW"):
            shutil.copytree(self.pages, os.environ["POROS_DEVICE_PAGES_PREVIEW"], dirs_exist_ok=True)


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
            (archived / "proof").write_text("m2 bundle")
            with patch.object(site, "reusable_bundle", return_value=True):
                record = backfill.route(self.source, self.root / "pages", self.root / "m1", dict(target, kind="release"))
                self.assertEqual(record["route"], "parallel")
                own = archived / "devices/m1"
                own.mkdir(parents=True)
                (own / "proof").write_text("m1 bundle")
                record = backfill.route(self.source, self.root / "pages", self.root / "reuse", dict(target, kind="release"))
                self.assertEqual(record["route"], "reuse")
                self.assertEqual((self.root / "reuse/evidence/proof").read_text(), "m1 bundle")
                # Reusing the default device's bundle takes none of its siblings along.
                default = dict(kind="release", commit=self.commit)
                self.assertEqual(backfill.route(self.source, self.root / "pages", self.root / "default", default)["route"], "reuse")
                self.assertEqual([path.name for path in (self.root / "default/evidence").iterdir()], ["proof"])

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
