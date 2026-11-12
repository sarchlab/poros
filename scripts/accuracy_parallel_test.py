#!/usr/bin/env python3
"""Shard transport tests. Synthetic simulator metrics are never timing evidence."""
import base64
import contextlib
import copy
import io
import json
import math
from pathlib import Path
import tempfile
import struct
import shutil
import subprocess
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import accuracy_parallel as parallel
import accuracy_site as site
import usertime as user
import accuracy_test_support as fixtures
import frontend_microbenchmark as probe
import throughput_microbenchmark as throughput
import forwarding_microbenchmark as forwarding
import capacity_microbenchmark as capacity
import model_profiles as models
import memory_microbenchmark as memory
import hashlib

HEAD = "a" * 40
BASE = "ec5df8efdd80fd02a0db7bd80e9ae413c509852b"


class ParallelAccuracyTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # Small synthetic evidence exercises all264 transport identities without
        # copying hundreds of MiB of production numerical witnesses per mutation.
        temporary = tempfile.TemporaryDirectory()
        cls.addClassCleanup(temporary.cleanup)
        cls.fixture = fixtures.Evidence(parallel.ROOT, Path(temporary.name))
        original_validate = parallel.compute.validate_reference
        validation = patch.object(parallel.compute, "validate_reference", side_effect=lambda folder, root=None: original_validate(folder, cls.fixture.source if root is None else root))
        validation.start()
        cls.addClassCleanup(validation.stop)
        cls.state = user.source_state()
        cls.state["files"] = {k: v for k, v in cls.state["files"].items() if (cls.fixture.source/k).is_file()}
        cls.state.update(commit=HEAD, dirty=False)
        for module in (parallel, user, user.native):
            patcher = patch.object(module, "ROOT", cls.fixture.source)
            patcher.start()
            cls.addClassCleanup(patcher.stop)
        git_patch = patch.object(parallel, "git", return_value=HEAD)
        git_patch.start()
        cls.addClassCleanup(git_patch.stop)
        # Fixture workloads live outside Git; capability evidence is read from
        # the real repository, while execution/source manifests remain synthetic.
        for name in ("base_capability", "base_declares_cohort"):
            original = getattr(parallel.compute, name)
            patcher = patch.object(parallel.compute, name,
                side_effect=lambda revision, root=None, original=original: original(revision, root) if root is not None and (Path(root)/".git").exists() else original(revision))
            patcher.start(); cls.addClassCleanup(patcher.stop)
        cls.cases, cls.witnesses = parallel.inputs()
        original = parallel.read(cls.fixture.source / "evaluation/usertime/m2-linuxvm-performance-v1.json")
        cls.outputs = {r["id"]: base64.b64decode(r["samples"][0]["stdout_base64"]) for r in original["cases"]}
        cls.outputs.update({case["id"]: parallel.compute.witness(cls.fixture.folder, case, row) for case, row in zip(cls.fixture.declared["cases"], cls.fixture.reference["cases"])})
        cls.profile = user.inventory()["profile"]

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.prepared = self.root / "prepared"
        self.prepared.mkdir()
        for name in ("head", "base"):
            (self.prepared / name).write_text("synthetic unit binary " + name)
        self.plan = dict(schema=1, run_id="123", prepared_attempt=1,
                         head=self.state, base_commit=BASE, compute_base_available=True, base_capability=parallel.compute.base_capability(BASE),
                         binaries={name: user.digest(self.prepared / name) for name in ("head", "base")},
                         references={"performance264": user.digest(parallel.ROOT / parallel.REFERENCE)},
                         cases=self.cases, shards=parallel.partition(self.cases, 8))
        parallel.write(self.prepared / "plan.json", self.plan)
        self.state_patch = patch.object(user, "source_state", return_value=self.state)
        self.state_patch.start()
        self.addCleanup(self.state_patch.stop)

    def result(self, case, role):
        result = dict(binary_sha256=case["sha256"], args=case["args"], env=case["env"],
                      termination="exit", exit_status=0, stderr_base64="", instructions=100,
                      stdout_base64=base64.b64encode(self.outputs[case["id"]]).decode(),
                      build={"vcs.revision": BASE if role == "base" else HEAD, "vcs.modified": "false"})
        if role != "functional":
            result["timing"] = dict(model=case["model"], assumed_hz=case["frequency_hz"],
                                    retired_instructions=100, user_cycles=200, cpi=2,
                                    predicted_user_seconds=200/case["frequency_hz"],
                                    cache={name: 0 for name in ("L1Hits", "L1Misses", "L2Hits", "L2Misses")},
                                    branch_predictions=0, branch_direction_hits=0, **self.profile)
        return result

    def shards(self):
        cases = {c["id"]: c for c in self.cases}
        return [dict(schema=1, plan_id=parallel.identity(self.plan), run_id="123", attempt=1,
                     shard=i, status="pass", errors=[], started_at=1, completed_at=2,
                     records=[dict(case_id=ident, executions={role: self.result(cases[ident], role)
                               for role in parallel.roles(self.plan, cases[ident], self.witnesses)})
                              for ident in ids]) for i, ids in enumerate(self.plan["shards"])]

    def write_shards(self, shards):
        root = self.root / "shards"
        for i, shard in enumerate(shards):
            parallel.write(root / str(i) / "shard.json", shard)
        return root

    def serial_compare(self, plan, results, output, **options):
        selected = parallel.select_shards(plan, [parallel.read(p) for p in results.rglob('shard.json')], 1)
        executor, _ = parallel.recorded_executor(plan, selected, self.witnesses, self.prepared)
        args = SimpleNamespace(head=str(self.prepared/'head'), base=str(self.prepared/'base'),
            base_unavailable=False, base_commit=plan['base_commit'],
            reference=str(self.fixture.source/parallel.REFERENCE), output=str(output))
        return user.compare(args, executor, **options)

    def declarations(self):
        profile_hash = hashlib.sha256(models.encode(self.profile)).hexdigest()
        self.plan["head"] = copy.deepcopy(self.plan["head"])
        self.plan["head"]["files"] = {name: user.digest(self.fixture.source / name) for name in self.plan["head"]["files"]}
        self.plan["head"]["files"][models.PATH] = profile_hash
        self.plan["model_profiles"] = {
            "head": dict(commit=HEAD, path=models.PATH, sha256=profile_hash, profile=copy.deepcopy(self.profile)),
            "base": models.revision(Path(__file__).resolve().parents[1], BASE)}
        parallel.write(self.prepared / "plan.json", self.plan)

    @contextlib.contextmanager
    def device_campaign(self, ident, base_declared=False):
        """This fixture's plan as a campaign for one declared device.

        The fixture source is not a Git repository, so the declarations a real
        prepare reads from the head and base revisions are supplied directly."""
        if "model_profiles" not in self.plan:
            self.declarations()
        chosen = fixtures.device_selection(ident, HEAD, self.plan["head"]["files"], BASE if base_declared else None)
        self.plan["device"] = chosen
        parallel.write(self.prepared / "plan.json", self.plan)
        def resolve(root, state, base, requested=None):
            return copy.deepcopy(chosen) if requested in (None, ident) else fixtures.device_selection(requested, HEAD, {})
        with patch.object(user, "source_state", return_value=self.plan["head"]), \
             patch.object(models, "resolve", return_value=self.plan["model_profiles"]), \
             patch.object(parallel.devices, "resolve", side_effect=resolve), \
             contextlib.redirect_stdout(io.StringIO()):
            yield

    def device_result(self, case, role):
        """What a simulator given this campaign's target reports for a role."""
        result = self.result(case, role)
        target = parallel.devices.identity(self.plan, role)
        if role != "functional" and target is not None:
            result["timing"]["target"] = target
        return result

    def device_shards(self):
        shards = self.shards()
        cases = {c["id"]: c for c in self.cases}
        for shard in shards:
            for record in shard["records"]:
                record["executions"] = {role: self.device_result(cases[record["case_id"]], role) for role in record["executions"]}
        return shards

    def test_unmeasured_device_campaign_runs_all_264_cases_and_reports_no_reference(self):
        output = self.root / "m1/report.json"
        with self.device_campaign("m1"):
            self.assertEqual(parallel.load_plan(self.prepared, "123", 1)[0], self.plan)
            shards = self.device_shards()
            # Every case is executed functionally and timed; the base revision cannot model M1.
            self.assertEqual({tuple(record["executions"]) for shard in shards for record in shard["records"]}, {("functional", "head")})
            results = self.write_shards(shards)
            self.assertEqual(parallel.aggregate(self.prepared, results, "123", 1, output, device="m1"), 0)
            report = parallel.read(output)
            self.assertEqual((report["artifact_status"], len(report["cases"]), report["device"]["id"]), ("pass", 264, "m1"))
            self.assertNotIn("reference", report)
            self.assertNotIn("reference_sha256", report)
            self.assertEqual(report["output_oracle"]["sha256"], self.plan["references"]["performance264"])
            for row in report["cases"]:
                self.assertEqual(sorted(row), ["base_status", "functional_status", "head", "id", "mapped_image_bytes", "reason",
                                               "reference_quality", "split", "status"])
                self.assertEqual((row["reference_quality"], row["base_status"], row["functional_status"]),
                                 ("unmeasured", "device_unavailable", "pass"))
                self.assertEqual(sorted(row["head"]), ["cache_config", "configuration", "cycles", "instructions", "predicted_user_seconds"])
                self.assertIn("No native Apple M1 reference", row["reason"])
            self.assertTrue(all(split == dict(required=split["required"], comparable=0, head_comparable=0, base_comparable=0)
                                for split in report["summary"].values()))
            markdown = output.with_suffix(".md").read_text()
            self.assertIn("Device: `m1` (Apple M1); parameters: Apple M2 model profile (placeholder; Apple M1 parameters not established); native references: unmeasured.", markdown)
            self.assertIn("| integer-1000000 | unmeasured | unmeasured | unavailable |", markdown)
            site.bundle(output, self.fixture.source, self.root / "m1/bundle", HEAD)
            retained, manifest = site.validate_bundle(self.root / "m1/bundle", HEAD)
            self.assertEqual((retained, sorted(manifest["files"])), (report, ["report.json", "report.md"]))
            # The caller's device must be the plan's; neither is silently preferred.
            self.assertEqual(parallel.aggregate(self.prepared, results, "123", 1, output, device="m2"), 1)
            failed = parallel.read(output)
            self.assertIn("requested device 'm2' differs from prepared plan device 'm1'", failed["error"])
            self.assertEqual(site.report_device(failed), "m2")
            # A default-device execution is not evidence for M1, and neither is a base run.
            for label, mutate, reason in (
                    ("default target", lambda r: r["executions"]["head"]["timing"]["target"].update(name="m2", hardware="Apple M2"), "simulated device mismatch"),
                    ("no target", lambda r: r["executions"]["head"]["timing"].pop("target"), "simulated device mismatch"),
                    ("base run", lambda r: r["executions"].update(base=self.result(self.cases[0], "base")), "missing/extra execution roles")):
                with self.subTest(label):
                    altered = copy.deepcopy(shards)
                    mutate(next(r for r in altered[0]["records"] if r["case_id"] == self.cases[0]["id"]))
                    self.assertEqual(parallel.aggregate(self.prepared, self.write_shards(altered), "123", 1, output, device="m1"), 1)
                    failed = parallel.read(output)
                    self.assertIn(reason, failed["error"])
                    self.assertEqual(site.report_device(failed), "m1")

    def test_declared_default_device_matches_the_legacy_report_and_requires_its_target(self):
        legacy_output, output = self.root / "legacy/report.json", self.root / "m2/report.json"
        self.declarations()
        with patch.object(user, "source_state", return_value=self.plan["head"]), \
             patch.object(models, "resolve", return_value=self.plan["model_profiles"]), \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(parallel.aggregate(self.prepared, self.write_shards(self.shards()), "123", 1, legacy_output), 0)
        legacy = parallel.read(legacy_output)
        self.assertNotIn("device", legacy)
        with self.device_campaign("m2"):
            shards = self.device_shards()
            case = self.cases[30]
            record = next(r for shard in shards for r in shard["records"] if r["case_id"] == case["id"])
            # The base revision predates targets: it ran, and reported, as it always has.
            self.assertEqual(record["executions"]["head"]["timing"]["target"], dict(name="m2", hardware="Apple M2", parameter_source="m2"))
            self.assertNotIn("target", record["executions"]["base"]["timing"])
            self.assertEqual(parallel.aggregate(self.prepared, self.write_shards(shards), "123", 1, output), 0)
            report = parallel.read(output)
            self.assertEqual(report["device"]["id"], "m2")
            for key in ("cases", "summary", "reference", "reference_sha256", "workload_manifest", "base_capability"):
                self.assertEqual(report[key], legacy[key], key)
            self.assertNotIn("output_oracle", report)
            del record["executions"]["head"]["timing"]["target"]
            self.assertEqual(parallel.aggregate(self.prepared, self.write_shards(shards), "123", 1, output), 1)
            self.assertIn("simulated device mismatch: requested 'm2'", parallel.read(output)["error"])

    def test_shards_request_the_plan_device_from_each_simulator_that_models_it(self):
        for ident, base_declared, base_runs in (("grace", False, False), ("grace", True, True), ("m2", False, True)):
            with self.subTest(device=ident, base_declared=base_declared), self.device_campaign(ident, base_declared):
                requested = []
                def live(binary, case, mode=None, expected_identity=None, expected_profile=None, device=None):
                    role = "functional" if mode == "functional" else Path(binary).name
                    requested.append((role, device))
                    return self.device_result(case, role)
                output = self.root / ident / str(base_declared) / "shard.json"
                with patch.object(user, "execute", side_effect=live):
                    self.assertEqual(parallel.run_shard(self.prepared, "123", 1, 0, output), 0)
                head = parallel.devices.identity(self.plan, "head")
                self.assertEqual(head["name"], ident)
                self.assertEqual({device["name"] for role, device in requested if role != "base"}, {ident})
                bases = [device for role, device in requested if role == "base"]
                self.assertEqual(bool(bases), base_runs)
                # A base that declares the device is asked for it; one that predates
                # targets is run exactly as before, with no target at all.
                self.assertEqual(set(map(repr, bases)), {repr(head if base_declared else None)} if base_runs else set())

    def test_plan_device_must_be_the_one_its_source_resolves(self):
        with self.device_campaign("m1"):
            parallel.load_plan(self.prepared, "123", 1)
            shards = self.device_shards()
            differs = "plan device declaration differs from source"
            for change, reason in ((lambda p: p.pop("device"), differs),
                                   (lambda p: p["device"].update(base=copy.deepcopy(p["device"]["head"])), differs),
                                   (lambda p: p["device"].update(id="m3"), "unknown device 'm3'")):
                altered = copy.deepcopy(self.plan)
                change(altered)
                parallel.write(self.prepared / "plan.json", altered)
                with self.assertRaisesRegex(ValueError, reason):
                    parallel.load_plan(self.prepared, "123", 1)
            # Renaming the device makes a valid plan for that device, and a
            # different plan: the shards executed as M1 do not belong to it.
            other = copy.deepcopy(self.plan)
            other["device"]["id"] = "grace"
            parallel.write(self.prepared / "plan.json", other)
            self.assertEqual(parallel.load_plan(self.prepared, "123", 1)[0], other)
            self.assertNotEqual(parallel.identity(other), parallel.identity(self.plan))
            with self.assertRaisesRegex(ValueError, "foreign shard identity"):
                parallel.select_shards(other, shards, 1)

    def test_failure_after_a_prepared_plan_is_filed_under_the_plan_device(self):
        output = self.root / "failed/report.json"
        def failed(reason, filed, **options):
            """Aggregation fails for this reason, and its report is filed under this device (None: unnamed)."""
            self.assertEqual(parallel.aggregate(self.prepared, results, "123", 1, output, **options), 1)
            report = parallel.read(output)
            self.assertEqual(report["artifact_status"], "fail")
            self.assertIn(reason, report["error"])
            self.assertEqual(report.get("device"), None if filed is None else dict(id=filed))
            self.assertEqual(site.report_device(report), filed or "m2")
        with self.device_campaign("m1"):
            results = self.write_shards(self.device_shards())
            # The workflow's aggregate job repeats no device: the plan prepared for M1 says whose failure it is.
            failed("upstream shard jobs did not succeed: failure", "m1", shard_status="failure")
            (results / "3/shard.json").unlink()
            failed("missing", "m1")
            failed("missing", "m1", device="m1")
            # The same through the command line, and the failure is publishable as M1's.
            argv = ["accuracy_parallel.py", "aggregate", "--prepared", str(self.prepared), "--run-id", "123", "--attempt", "1",
                    "--results", str(results), "--output", str(output), "--shard-status", "cancelled"]
            with patch("sys.argv", argv):
                self.assertEqual(parallel.main(), 1)
            report = parallel.read(output)
            self.assertEqual((report["error"], report["device"]), ("Parallel accuracy aggregation failed: upstream shard jobs did not succeed: cancelled", dict(id="m1")))
            site.bundle(output, self.fixture.source, self.root / "bundle", HEAD)
            self.assertEqual(site.report_device(site.validate_bundle(self.root / "bundle", HEAD)[0]), "m1")
            # A caller that asks for another device is told so, under the device it asked for.
            failed("requested device 'grace' differs from prepared plan device 'm1'", "grace", device="grace", shard_status="failure")
            # Without a readable plan only the caller can name the device; unnamed, it is the default as before.
            (self.prepared / "plan.json").write_text("{}")
            failed("upstream shard jobs did not succeed: failure", None, shard_status="failure")
            failed("schema", None)
            failed("schema", "m1", device="m1")
        # A plan without a device stays an unlabelled, default-device failure.
        parallel.write(self.prepared / "plan.json", {key: value for key, value in self.plan.items() if key not in ("device", "model_profiles")})
        results = self.root / "missing-results"
        with patch.object(parallel.devices, "resolve", return_value=None), patch.object(models, "resolve", return_value=None), \
             patch.object(user, "source_state", return_value=self.plan["head"]):
            failed("upstream shard jobs did not succeed: failure", None, shard_status="failure")

    def test_probe_progress_of_another_device_is_not_imported(self):
        import accuracy_progress as progress
        results = self.root / "no-shards"
        results.mkdir()
        def exporter(**options):
            return progress.SnapshotExporter(self.prepared, self.fixture.source, results, "123", 1, **options)
        # Devices that share parameters differ in nothing but the device their snapshot names.
        probes = {}
        for ident in ("m2", "m1", "grace"):
            with self.device_campaign(ident):
                probes[ident] = self.root / (ident + "-probes.json")
                parallel.write(probes[ident], exporter().export())
        identity = ("head", "base_commit", "head_binary_sha256", "base_binary_sha256", "model_profiles")
        reports = {ident: parallel.read(path)["report"] for ident, path in probes.items()}
        self.assertTrue(all(reports[ident][key] == reports["m2"][key] for ident in reports for key in identity))
        for ident in probes:
            for other, path in probes.items():
                with self.subTest(campaign=ident, probes=other), self.device_campaign(ident):
                    if other == ident:
                        self.assertEqual(exporter(probe_progress=path).export()["report"]["device"]["id"], ident)
                    else:
                        with self.assertRaisesRegex(ValueError, "probe progress is for another modeled device"):
                            exporter(probe_progress=path).export()

    def test_compressed_partial_exporter_caches_validated_rows_and_rejects_mutation(self):
        import gzip
        import accuracy_progress as progress
        self.declarations()
        shards=self.shards()
        root=self.root/'delivered';folder=root/'0';folder.mkdir(parents=True)
        path=folder/'shard.json.gz'
        raw=json.dumps(shards[0]).encode()
        path.write_bytes(gzip.compress(raw))
        receipt=dict(shard_sha256=hashlib.sha256(raw).hexdigest(), head=HEAD, binaries=self.plan['binaries'],returncode=0)
        parallel.write(folder/'receipt.json',receipt)
        with patch.object(parallel,'load_plan',return_value=(self.plan,self.witnesses)):
            exporter=progress.SnapshotExporter(self.prepared,self.fixture.source,root,'123',1)
        value=exporter.export()
        self.assertEqual(value['progress']['completed_cases'],33)
        self.assertEqual(value['report']['head'],self.plan['head'])
        self.assertEqual(value['report']['cases'][0]['head']['instructions'],100)
        with patch.object(parallel,'recorded_executor',side_effect=AssertionError('raw revalidated')):
            self.assertEqual(exporter.export()['report'],value['report'])
        # In-flight gzip delivery has no receipt and must remain pending.
        pending=root/'1';pending.mkdir();(pending/'shard.json.gz').write_bytes(b'incomplete')
        self.assertEqual(exporter.export()['progress']['completed_shards'],1)
        status_path=self.root/'status.json'
        status=dict(head=HEAD,base=BASE,run_id='123',completed=[0],active={},failed=[dict(index=1,error='worker failed')],phase='failed')
        parallel.write(status_path,status);exporter.status=status_path
        failed=exporter.export()
        self.assertEqual(failed['progress']['state'],'failed')
        self.assertEqual(failed['progress']['errors'],['Shard 1: worker failed'])
        for invalid in ('-1','unknown','1000'):
            changed=copy.deepcopy(status);changed['active']={invalid:{}}
            parallel.write(status_path,changed)
            with self.assertRaisesRegex(ValueError,'index'):exporter.export()
        exporter.status=None
        path.write_bytes(gzip.compress(raw+b' '))
        with self.assertRaisesRegex(ValueError,'receipt'):
            exporter.export()
        path.write_bytes(gzip.compress(raw))
        duplicate=root/'duplicate';duplicate.mkdir()
        (duplicate/'shard.json.gz').write_bytes(gzip.compress(raw));parallel.write(duplicate/'receipt.json',receipt)
        with self.assertRaisesRegex(ValueError,'duplicate'):
            exporter.export()

    def test_aggregate_explicit_source_emits_validated_partial_progress(self):
        import accuracy_live as live
        self.declarations()
        results=self.write_shards(self.shards())
        output=self.root/'progress/report.json';progress=self.root/'snapshot.json'
        with patch.object(parallel,'load_plan',return_value=(self.plan,self.witnesses)), \
             patch.object(site,'git',return_value=HEAD), \
             patch.object(user,'source_state',return_value=self.plan['head']), \
             patch.object(models,'resolve',return_value=self.plan['model_profiles']), \
             patch.object(parallel.compute,'base_declares_cohort',return_value=True), \
             patch.object(parallel.compute,'base_capability',return_value=self.plan['base_capability']), \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(parallel.aggregate(self.prepared,results,'123',1,output,source=self.fixture.source,progress=progress),0)
        snapshot=live.validate_snapshot(parallel.read(progress))
        report=parallel.read(output)
        self.assertEqual(snapshot['report']['cases'],report['cases'])
        self.assertEqual(snapshot['report']['head'],self.plan['head'])
        self.assertEqual(snapshot['report']['artifact_status'],'partial')

    def test_progress_early_header_failure_and_late_bad_probe_preserve_evidence(self):
        import accuracy_live as live
        self.declarations()
        progress=self.root/'last-good.json';progress.write_text('previous validated snapshot')
        args=SimpleNamespace(head=str(self.prepared/'head'),base=str(self.prepared/'base'),base_unavailable=False,
            base_commit=BASE,reference=str(self.fixture.source/parallel.REFERENCE),output=str(self.root/'early.json'))
        with patch.object(user,'source_state',return_value=self.plan['head']), \
             patch.object(models,'resolve',side_effect=ValueError('bad model declaration')):
            self.assertEqual(user.compare(args,lambda *a,**kw: self.fail('executed before header'),progress_path=progress),1)
        self.assertEqual(parallel.read(args.output)['error'],'bad model declaration')
        self.assertEqual(progress.read_text(),'previous validated snapshot')
        for source in [*throughput.MEASUREMENT_INPUTS,throughput.ELF,throughput.REFERENCE]:
            target=self.fixture.source/source;target.parent.mkdir(parents=True,exist_ok=True)
            shutil.copyfile(Path(__file__).resolve().parents[1]/source,target)
            self.plan['head']['files'][source]=user.digest(target)
        parallel.write(self.prepared/'plan.json',self.plan)
        results=self.write_shards(self.shards())
        cases=throughput.cases(self.fixture.source)
        row=dict(id=cases[0]['id'],instructions=1024*cases[0]['instructions_per_iteration'],cycles=1000)
        def corrupt(*args,**options):
            options['progress']([row],cases[1]['id'])
            bad=dict(id=cases[1]['id'],instructions=1024*cases[1]['instructions_per_iteration'],cycles=-1)
            options['progress']([row,bad],None)
            self.fail('invalid progress accepted')
        output=self.root/'late/report.json'
        with patch.object(user,'source_state',return_value=self.plan['head']), \
             patch.object(models,'resolve',return_value=self.plan['model_profiles']), \
             patch.object(throughput,'measure',side_effect=corrupt), \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(self.serial_compare(self.plan,results,output,progress_path=progress),1)
        saved=live.validate_snapshot(parallel.read(progress))
        self.assertEqual(saved['report']['throughput_simulation'],[row])
        self.assertEqual(saved['progress']['state'],'failed')
        self.assertEqual(saved['report']['diagnostic_progress']['throughput']['state'],'failed')
        self.assertEqual(len(saved['report']['cases']),264)
        self.assertIn('counters',parallel.read(output)['error'])

    def test_plan_covers_all_264_once(self):
        ids = [ident for shard in self.plan["shards"] for ident in shard]
        self.assertEqual(len(ids), 264)
        self.assertEqual(len(set(ids)), 264)
        self.assertEqual([len(s) for s in self.plan["shards"]], [33] * 8)
        self.assertEqual(self.plan["shards"], parallel.partition(self.cases, 8))
        for n in (0, -1, True, 265):
            with self.assertRaises(ValueError):
                parallel.partition(self.cases, n)

    def test_aggregate_replays_diagnostics_and_never_falls_back_to_executor(self):
        from accuracy_diagnostics_test import synthetic_execution
        self.declarations()
        for name in (*throughput.MEASUREMENT_INPUTS,throughput.ELF,throughput.REFERENCE):
            target=self.fixture.source/name;target.parent.mkdir(parents=True,exist_ok=True)
            shutil.copyfile(Path(__file__).resolve().parents[1]/name,target)
            self.plan['head']['files'][name]=user.digest(target)
        parallel.write(self.prepared/'plan.json',self.plan)
        results=self.write_shards(self.shards())
        checkpoints=self.root/'diagnostics'
        saved=parallel.diagnostic_checkpoint(self.plan,self.prepared,checkpoints,'throughput','head')
        with patch.object(throughput,'execute',side_effect=synthetic_execution):
            expected=throughput.measure(self.prepared/'head',self.fixture.source,HEAD,
                self.plan['model_profiles']['head']['profile'],checkpoint=saved)
        output=self.root/'replayed/report.json'
        with patch.object(user,'source_state',return_value=self.plan['head']), \
             patch.object(models,'resolve',return_value=self.plan['model_profiles']), \
             patch.object(user,'execute',side_effect=AssertionError('aggregate executed application')), \
             patch.object(throughput,'execute',side_effect=AssertionError('aggregate executed diagnostic')), \
             patch.object(throughput,'measure',side_effect=AssertionError('aggregate serial fallback')), \
             patch.object(memory,'measure',side_effect=AssertionError('aggregate memory fallback')), \
             patch.object(probe,'measure',side_effect=AssertionError('aggregate frontend fallback')), \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(parallel.aggregate(self.prepared,results,'123',1,output),1)
            self.assertIn('requires --checkpoints with recorded diagnostic evidence',parallel.read(output)['error'])
            self.assertEqual(parallel.aggregate(self.prepared,results,'123',1,output,checkpoints=checkpoints),0)
            self.assertEqual(parallel.read(output)['throughput_simulation'],expected)
            next(checkpoints.glob('*.json')).unlink()
            self.assertEqual(parallel.aggregate(self.prepared,results,'123',1,output,checkpoints=checkpoints),1)
            self.assertIn('missing diagnostic checkpoint',parallel.read(output)['error'])

    def test_plan_rebinds_current_source_inputs_binaries_and_references(self):
        parallel.load_plan(self.prepared, "123", 1)
        mutations = [lambda p: p.update(run_id="other"), lambda p: p.update(prepared_attempt=2),
                     lambda p: p["head"].update(commit="b"*40),
                     lambda p: p["cases"][0]["args"].append("changed"),
                     lambda p: p["shards"][0].pop(),
                     lambda p: p["references"].update(original24="0"*64),
                     lambda p: p["binaries"].update(head="0"*64),
                     lambda p: p.update(compute_base_available=False)]
        for mutate in mutations:
            altered = copy.deepcopy(self.plan)
            mutate(altered)
            parallel.write(self.prepared / "plan.json", altered)
            with self.assertRaises((ValueError, KeyError)):
                parallel.load_plan(self.prepared, "123", 1)
        parallel.write(self.prepared / "plan.json", self.plan)
        (self.prepared / "head").write_text("tampered binary")
        with self.assertRaisesRegex(ValueError, "binary mismatch"):
            parallel.load_plan(self.prepared, "123", 1)

    def test_missing_duplicate_extra_and_foreign_shards_rejected(self):
        shards = self.shards()
        with self.assertRaisesRegex(ValueError, "missing required"):
            parallel.select_shards(self.plan, shards[:-1], 1)
        with self.assertRaisesRegex(ValueError, "duplicate shard"):
            parallel.select_shards(self.plan, shards + [shards[0]], 1)
        for change in (dict(shard=8), dict(plan_id="0"*64), dict(run_id="other"), dict(attempt=2), dict(attempt=0)):
            altered = copy.deepcopy(shards)
            altered[0].update(change)
            with self.assertRaises(ValueError):
                parallel.select_shards(self.plan, altered, 1)
        for mutate in (lambda r: r["records"].pop(), lambda r: r["records"].append(r["records"][0]),
                       lambda r: r["records"][0].update(case_id="unknown"),
                       lambda r: r.update(status="fail"), lambda r: r.pop("completed_at"),
                       lambda r: r["errors"].append("execution failed")):
            altered = copy.deepcopy(shards)
            mutate(altered[0])
            with self.assertRaises(ValueError):
                parallel.select_shards(self.plan, altered, 1)

    def test_retry_reuses_only_matching_plan_and_new_failure_supersedes_old_success(self):
        shards = self.shards()
        newer = copy.deepcopy(shards[0])
        newer["attempt"] = 2
        selected = parallel.select_shards(self.plan, shards+[newer], 2)
        self.assertEqual(selected[0]["attempt"], 2)
        self.assertEqual(selected[1]["attempt"], 1)
        newer["status"] = "fail"
        with self.assertRaisesRegex(ValueError, "failed or incomplete"):
            parallel.select_shards(self.plan, shards+[newer], 2)
        newer["status"] = "pass"
        newer["plan_id"] = "b"*64
        with self.assertRaisesRegex(ValueError, "foreign"):
            parallel.select_shards(self.plan, shards+[newer], 2)

    def test_recorded_execution_validates_full_output_build_profile_and_metrics(self):
        case = self.cases[24]
        good = self.result(case, "head")
        parallel.validate_result(good, self.plan, case, "head", self.witnesses[case["id"]])
        mutations = [lambda r: r.update(termination="time_limit"), lambda r: r.update(exit_status=1),
                     lambda r: r.update(stdout_base64=base64.b64encode(b"wrong").decode()),
                     lambda r: r.update(binary_sha256="0"*64), lambda r: r.update(args=["wrong"]),
                     lambda r: r.update(env=["changed=1"]), lambda r: r.update(instructions=0),
                     lambda r: r["build"].update(**{"vcs.revision": "b"*40}),
                     lambda r: r["build"].update(**{"vcs.modified": "true"}),
                     lambda r: r["timing"].update(predicted_user_seconds=math.nan),
                     lambda r: r["timing"].update(user_cycles=True),
                     lambda r: r["timing"].update(retired_instructions=99),
                     lambda r: r["timing"].update(configuration={}),
                     lambda r: r["timing"].update(branch_direction_hits=1)]
        for mutate in mutations:
            bad = copy.deepcopy(good)
            mutate(bad)
            with self.assertRaises(ValueError):
                parallel.validate_result(bad, self.plan, case, "head", self.witnesses[case["id"]])
        bad = self.result(case, "functional")
        bad["instructions"] = False
        with self.assertRaises(ValueError):
            parallel.validate_result(bad, self.plan, case, "functional", self.witnesses[case["id"]])
        with self.assertRaisesRegex(ValueError, "functional execution"):
            parallel.validate_result(good, self.plan, case, "functional", self.witnesses[case["id"]])

    def test_recorded_execution_roles_and_consumption_are_exact(self):
        selected = parallel.select_shards(self.plan, self.shards(), 1)
        executor, remaining = parallel.recorded_executor(self.plan, selected, self.witnesses, self.prepared)
        self.assertEqual(len(remaining), sum(len(parallel.roles(self.plan,c,self.witnesses)) for c in self.cases))
        executor(self.prepared / "head", self.cases[0], mode="functional", expected_identity=self.witnesses[self.cases[0]["id"]])
        with self.assertRaisesRegex(ValueError, "reused"):
            executor(self.prepared / "head", self.cases[0], mode="functional", expected_identity=self.witnesses[self.cases[0]["id"]])
        del selected[0]["records"][0]["executions"]["base"]
        with self.assertRaisesRegex(ValueError, "execution roles"):
            parallel.recorded_executor(self.plan, selected, self.witnesses, self.prepared)

    def test_prepare_rejects_missing_collection_before_build(self):
        destination = self.root / "rejected-prepare"
        with self.assertRaisesRegex(ValueError, "collection commit.*--merge"):
            parallel.prepare(BASE, destination, "123", 1, 8)
        self.assertFalse((destination / "head").exists())
        self.assertFalse((destination / "base").exists())


    def test_full_264_serial_and_recorded_report_paths_agree(self):
        args = SimpleNamespace(head=str(self.prepared / "head"), base=str(self.prepared / "base"),
                               base_unavailable=False, base_commit=BASE,
                               reference=str(parallel.ROOT / parallel.REFERENCE),
                               compute_reference=str(parallel.ROOT / parallel.compute.REFERENCE),
                               output=str(self.root / "serial.json"))
        def live(binary, case, mode=None, expected_output=None, expected_identity=None):
            role = "functional" if mode == "functional" else "base" if Path(binary).name == "base" else "head"
            return self.result(case, role)
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(user.compare(args, live), 0)
            self.assertEqual(parallel.aggregate(self.prepared, self.write_shards(self.shards()), "123", 1,
                                                self.root / "aggregate/report.json"), 0)
        serial = parallel.read(self.root / "serial.json")
        aggregated = parallel.read(self.root / "aggregate/report.json")
        self.assertEqual(serial, aggregated)
        site.check_report(aggregated, HEAD)

    @contextlib.contextmanager
    def memory_fixture(self):
        """Small, non-power-of-two and head-only large cases plus unassigned shards."""
        declared = memory.cases()
        cases = [declared[0], next(c for c in declared if c["id"] == "chase16-98304"), declared[-1]]
        state, plan = copy.deepcopy(self.state), copy.deepcopy(self.plan)
        profile_hash = hashlib.sha256(models.encode(self.profile)).hexdigest()
        state["files"].update({memory.ELF: "f" * 64, memory.REFERENCE: "e" * 64, memory.LARGE_ELF: "d" * 64,
                               models.PATH: profile_hash})
        declarations = {role: dict(commit=revision, path=models.PATH, sha256=profile_hash,
                                   profile=self.profile) for role, revision in (("head", HEAD), ("base", BASE))}
        plan.update(head=state, model_profiles=declarations,
                    memory=dict(elfs={memory.ELF: "f" * 64, memory.LARGE_ELF: "d" * 64},
                                base_large_image_support=False, reference_sha256="e" * 64,
                                cases=cases, shards=parallel.partition(cases, 3)))
        parallel.write(self.prepared / "plan.json", plan)
        with patch.object(user, "source_state", return_value=state), \
             patch.object(models, "resolve", return_value=declarations), \
             patch.object(memory, "cases", return_value=cases), \
             patch.object(memory, "supports_large_images", return_value=False), \
             patch.object(memory, "load_reference", return_value={"cases": cases}):
            yield plan

    def memory_samples(self, binary, root, case, revision, profile):
        samples = []
        hz = profile["configuration"]["freq"]
        for factor in (1, 2):
            passes = case["passes"] * factor
            instructions = 1000 + passes * case["instructions_per_pass"]
            cycles = 2000 + instructions
            samples.append(dict(termination="exit", exit_status=0, stderr_base64="",
                                binary_sha256=("d" if memory.case_elf(case) == memory.LARGE_ELF else "f") * 64,
                                max_image_bytes=case["max_image_bytes"], args=["program", *memory.arguments(case, passes)],
                                env=[], instructions=instructions,
                                stdout_base64=base64.b64encode(struct.pack("<Q", memory.expected(case, passes))).decode(),
                                build={"vcs.revision": revision, "vcs.modified": "false"},
                                timing=dict(model="ooo", user_cycles=cycles, retired_instructions=instructions,
                                            assumed_hz=hz, predicted_user_seconds=cycles / hz, **profile)))
        return samples

    def live_result(self, binary, case, mode=None, **options):
        role = "functional" if mode == "functional" else Path(binary).name
        return self.result(case, role)

    def test_active_shard_memory_assignments_are_unique_and_role_aware(self):
        import accuracy_progress as progress
        with self.memory_fixture() as plan:
            cases=plan['memory']['cases']
            report={'memory_simulation':{role:[dict(id=cases[0]['id'])] for role in ('head','base')}}
            progress.active_memory_progress(plan,report,[0,1,1,2,7])
            item=report['diagnostic_progress']['memory']
            self.assertEqual(item['state'],'running')
            self.assertEqual(item['active'],[dict(role='head',id=cases[1]['id']),dict(role='base',id=cases[1]['id']),dict(role='head',id=cases[2]['id'])])
            report['memory_simulation']={role:[dict(id=c['id']) for c in cases] for role in ('head','base')}
            progress.active_memory_progress(plan,report,[])
            self.assertEqual(report['diagnostic_progress']['memory'],dict(state='complete',active=[],errors=[]))

    def test_memory_runs_in_assigned_shards_and_aggregate_only_consumes_records(self):
        results, output = self.root / "memory-shards", self.root / "memory/report.json"
        with self.memory_fixture() as plan, contextlib.redirect_stdout(io.StringIO()):
            with patch.object(user, "execute", side_effect=self.live_result), \
                 patch.object(memory, "measure_case", side_effect=self.memory_samples) as execute_memory:
                for index in range(8):
                    self.assertEqual(parallel.run_shard(self.prepared, "123", 1, index,
                                                       results / str(index) / "shard.json"), 0)
            self.assertEqual(execute_memory.call_count, 5)  # Two base/head cases and one head-only case.
            shards = [parallel.read(results / str(i) / "shard.json") for i in range(8)]
            self.assertEqual([len(s["memory_records"]) for s in shards], [1, 1, 1, 0, 0, 0, 0, 0])
            expected = parallel.recorded_memory(plan, shards[:3], 1)
            with patch.object(user, "execute", side_effect=AssertionError("aggregate executed app")), \
                 patch.object(memory, "measure_case", side_effect=AssertionError("aggregate executed memory")), \
                 patch.object(memory, "measure", side_effect=AssertionError("aggregate used serial fallback")), \
                 patch.object(user, "compare", wraps=user.compare) as compare:
                self.assertEqual(parallel.aggregate(self.prepared, results, "123", 1, output), 0)
                self.assertEqual(compare.call_args.kwargs["memory_simulation"], expected)
                self.assertFalse(hasattr(compare.call_args.args[0], "memory_simulation"))
                report = parallel.read(output)
                self.assertEqual(report["artifact_status"], "pass")
                self.assertEqual(report["memory_simulation"], expected)
                self.assertIs(report["memory_base_large_image_support"], False)
                self.assertEqual(report["memory_simulation"]["base"][-1], memory.unavailable(plan["memory"]["cases"][-1]))
                self.assertEqual(len(report["cases"]), 264)
                for index, records, reason in ((0, [], "coverage"),
                                               (3, shards[0]["memory_records"], "unassigned")):
                    changed = copy.deepcopy(shards[index]); changed["memory_records"] = records
                    parallel.write(results / str(index) / "shard.json", changed)
                    self.assertEqual(parallel.aggregate(self.prepared, results, "123", 1, output), 1)
                    self.assertIn(reason, parallel.read(output)["error"])
                    parallel.write(results / str(index) / "shard.json", shards[index])

    def test_compare_memory_explicit_rows_fail_closed_and_serial_path_announces_work(self):
        args = SimpleNamespace(head=str(self.prepared / "head"), base=str(self.prepared / "base"),
                               base_unavailable=False, base_commit=BASE,
                               reference=str(parallel.ROOT / parallel.REFERENCE), output=str(self.root / "serial-memory.json"))
        with self.memory_fixture() as plan, contextlib.redirect_stdout(io.StringIO()) as notices:
            def measured(binary, root, revision, profile):
                return [memory.unavailable(c) if revision == BASE and c["max_image_bytes"] > 268435456 else
                        memory.summarize_samples(c, self.memory_samples(binary, root, c, revision, profile))
                        for c in plan["memory"]["cases"]]
            with patch.object(memory, "measure", side_effect=measured) as measure:
                self.assertEqual(user.compare(args, self.live_result, memory_simulation={}, memory_base_large_image_support=False), 1)
                measure.assert_not_called()
                self.assertIn("both base and head", parallel.read(args.output)["error"])
                self.assertEqual(user.compare(args, self.live_result), 0)
                self.assertEqual(measure.call_count, 2)
            self.assertIn("measuring the full memory suite serially", notices.getvalue())

    @contextlib.contextmanager
    def memory_only_campaign(self, declared=False):
        """A real Git source for a memory-only campaign.

        Only simulator execution is synthetic. Git commits, source hashes,
        reference, model declarations and the complete 140-point transport are
        real. With `declared` both revisions carry the repository's device
        declaration; without it the source predates devices."""
        source = self.root / "memory-source"
        shutil.copytree(self.fixture.source, source)
        for name in [*memory.INPUTS, memory.REFERENCE, models.PATH, *([parallel.devices.PATH] if declared else []),
                     "pkg/loader/elf.go", "pkg/linuxuser/run.go", "cmd/poros/elf.go"]:
            target = source / name; target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(memory.ROOT / name, target)
        def git(*args):
            return subprocess.check_output(['git', '-C', str(source), *args], stderr=subprocess.DEVNULL, text=True).strip()
        git('init'); git('config', 'user.name', 'Test'); git('config', 'user.email', 'test@example.org')
        cli = source / 'cmd/poros/elf.go'
        current_cli = cli.read_bytes()
        cli.write_bytes(subprocess.check_output(['git', '-C', str(memory.ROOT), 'show', BASE + ':cmd/poros/elf.go']))
        git('add', '.'); git('commit', '-m', 'fixture base without extended image option')
        base = git('rev-parse', 'HEAD')
        cli.write_bytes(current_cli)
        git('add', '.'); git('commit', '-m', 'fixture head with extended image option')
        real_source_state = self.state_patch.temp_original
        original_capability, original_cohort = parallel.compute.base_capability, parallel.compute.base_declares_cohort
        with patch.object(parallel, 'ROOT', source), patch.object(user, 'ROOT', source), \
             patch.object(user, 'source_state', side_effect=real_source_state), \
             patch.object(parallel.compute, 'base_capability', side_effect=lambda rev, root=None: original_capability(rev, source)), \
             patch.object(parallel.compute, 'base_declares_cohort', side_effect=lambda rev, root=None: original_cohort(rev, source)), \
             contextlib.redirect_stdout(io.StringIO()):
            state = user.source_state()
            self.assertFalse(state['dirty'])
            plan = copy.deepcopy(self.plan)
            plan.update(head=state, base_commit=base, base_capability=original_capability(base, source),
                        model_profiles=models.resolve(source, state, base))
            def prepare(device=None):
                """Bind the plan to a device as `prepare --device` does, and write it."""
                chosen = parallel.devices.resolve(source, state, base, device)
                plan.pop('device', None)
                if chosen is not None:
                    plan['device'] = chosen
                plan['memory'] = parallel.memory_plan(state, len(plan['shards']), base, value=plan)
                parallel.write(self.prepared / 'plan.json', plan)
                return plan
            def simulate(binary, root, case, revision, profile, device=None):
                samples = self.memory_samples(binary, root, case, revision, profile)
                for sample in samples:
                    sample['binary_sha256'] = state['files'][memory.case_elf(case)]
                    if device is not None:
                        sample['timing']['target'] = device
                return samples
            def run(results):
                with patch.object(memory, 'measure_case', side_effect=simulate):
                    for index in range(len(plan['memory']['shards'])):
                        self.assertEqual(parallel.run_memory_shard(self.prepared, '123', 1, index,
                                         results / str(index) / 'memory-shard.json'), 0)
            yield SimpleNamespace(source=source, state=state, base=base, prepare=prepare, run=run)

    def test_memory_only_roundtrip_with_real_source_metadata(self):
        with self.memory_only_campaign() as campaign:
            source, plan = campaign.source, campaign.prepare()
            self.assertNotIn('device', plan)
            results = self.root / 'memory-only-shards'
            campaign.run(results)
            output = self.root / 'memory-only.json'
            with patch.object(memory, 'measure_case', side_effect=AssertionError('aggregate reran simulation')):
                self.assertEqual(parallel.aggregate_memory(self.prepared, results, '123', 1, output), 0)
            self.assertNotIn('device', parallel.read(output))  # A source without devices is the default device, as always.
            preview = self.root / 'memory-only-preview'
            site.memory_preview(output, source, preview)
            rendered = (preview / 'index.html').read_text()
            self.assertIn('140 diagnostic points', rendered)
            self.assertIn('application accuracy was not rerun', rendered)
            self.assertEqual(parallel.read(preview / 'memory-report.json'), parallel.read(output))
            # A syntactically valid unsupported-capability report cannot disagree
            # with the supplied Git source (this base has no extended image flag).
            altered = parallel.read(output); altered['memory_base_large_image_support'] = True
            altered['memory_simulation']['base'] = copy.deepcopy(altered['memory_simulation']['head'])
            memory.validate_comparison(altered['memory_simulation'], plan['memory']['cases'], True)
            parallel.write(output, altered)
            with self.assertRaisesRegex(ValueError, 'base capability differs from source'):
                site.memory_preview(output, source, self.root / 'bad-preview')
            shard = results / '0/memory-shard.json'
            shard.rename(shard.with_name('wrong-name.json'))
            with self.assertRaisesRegex(ValueError, 'missing required memory shards'):
                parallel.aggregate_memory(self.prepared, results, '123', 1, output)

    def test_memory_only_artifact_belongs_to_the_device_that_owns_the_memory_reference(self):
        with self.memory_only_campaign(declared=True) as campaign:
            source, state, base = campaign.source, campaign.state, campaign.base
            plan = campaign.prepare()
            self.assertEqual((plan['device']['id'], plan['device']['base']['commit']), ('m2', base))
            results, output = self.root / 'memory-only-shards', self.root / 'memory-only.json'
            campaign.run(results)
            # Each role's simulator was asked for the device, and its executions say so.
            shard = parallel.read(results / '0/memory-shard.json')
            targets = {role: {sample['timing']['target']['name'] for sample in samples}
                       for record in shard['memory_records'] for role, samples in record['executions'].items()}
            self.assertEqual(targets, {'head': {'m2'}, 'base': {'m2'}})
            self.assertEqual(parallel.aggregate_memory(self.prepared, results, '123', 1, output), 0)
            report = parallel.read(output)
            self.assertEqual(report['device'], plan['device'])
            preview = self.root / 'memory-only-preview'
            site.memory_preview(output, source, preview)
            self.assertIn('140 diagnostic points', (preview / 'index.html').read_text())
            # The preview accepts the artifact only as that device's, at both revisions.
            def rejected(change, reason):
                altered = copy.deepcopy(report)
                change(altered)
                parallel.write(self.root / 'altered.json', altered)
                with self.assertRaisesRegex(ValueError, reason):
                    site.memory_preview(self.root / 'altered.json', source, self.root / 'bad-preview')
                self.assertFalse((self.root / 'bad-preview').exists())
            rejected(lambda r: r['device'].update(id='m1'), 'memory-only artifact is for a device without a native memory reference')
            rejected(lambda r: r.pop('device'), 'missing device selection')
            rejected(lambda r: r['device'].update(base=None), 'base device declaration differs from exact Git revision')
            # A device without a memory reference has no memory-only artifact. It is refused before anything runs.
            unmeasured = campaign.prepare('m1')
            self.assertNotIn('reference_sha256', unmeasured['memory'])
            with patch.object(memory, 'measure_case', side_effect=AssertionError('executed for a device without a reference')):
                for attempt in (lambda: parallel.run_memory_shard(self.prepared, '123', 1, 0, self.root / 'm1/memory-shard.json'),
                                lambda: parallel.aggregate_memory(self.prepared, results, '123', 1, self.root / 'm1.json')):
                    with self.assertRaisesRegex(ValueError, 'memory-only artifacts need a native memory reference; device m1 has none'):
                        attempt()
            self.assertFalse((self.root / 'm1.json').exists())
            # A declaration cannot hand the M2 memory file to other hardware, or name another file for
            # it. Both are refused while the plan is built, so before a shard or an aggregation exists.
            declaration = plan['device']['head']['declaration']
            def planned(change):
                changed = copy.deepcopy(declaration)
                change({entry['id']: entry for entry in changed['devices']})
                value = dict(device=fixtures.device_selection('m1', state['commit'], {}, declaration=changed))
                return parallel.memory_plan(state, len(plan['shards']), base, value=value)
            with self.assertRaisesRegex(ValueError, "memory reference was measured on 'Apple M2', not 'Apple M1'"):
                planned(lambda entries: entries['m1']['references'].update(memory=memory.REFERENCE))
            with self.assertRaisesRegex(ValueError, 'device m1 declares its memory reference at evaluation/microbenchmarks/memory/m1.json, not ' + memory.REFERENCE):
                planned(lambda entries: entries['m1']['references'].update(memory='evaluation/microbenchmarks/memory/m1.json'))

    def test_memory_aggregate_rejects_unsupported_shard_status_option(self):
        argv = ["accuracy_parallel.py", "memory-aggregate", "--prepared", "unused", "--run-id", "123",
                "--attempt", "1", "--results", "unused", "--output", "unused", "--shard-status", "failure"]
        with patch("sys.argv", argv), contextlib.redirect_stderr(io.StringIO()) as errors:
            with self.assertRaises(SystemExit) as failure:
                parallel.main()
        self.assertEqual(failure.exception.code, 2)
        self.assertIn("unrecognized arguments: --shard-status failure", errors.getvalue())

    def test_upstream_failure_cannot_reuse_old_success_and_failure_bundle_is_valid(self):
        output = self.root / "failed/report.json"
        with patch.object(parallel, "git", return_value=HEAD):
            self.assertEqual(parallel.aggregate(self.prepared, self.write_shards(self.shards()), "123", 2,
                                                output, shard_status="failure"), 1)
        report = parallel.read(output)
        self.assertEqual(report["artifact_status"], "fail")
        self.assertIn("upstream", report["error"])
        site.bundle(output, parallel.ROOT, self.root / "bundle", HEAD)
        result, _ = site.validate_bundle(self.root / "bundle", HEAD)
        self.assertEqual(result["artifact_status"], "fail")

    def test_serial_comparison_requires_frontend_probe_when_source_declares_it(self):
        state = copy.deepcopy(self.state)
        state["files"][probe.ELF] = "f" * 64
        plan = copy.deepcopy(self.plan)
        plan["head"] = state
        parallel.write(self.prepared / "plan.json", plan)
        shards = self.shards()
        for shard in shards:
            shard["plan_id"] = parallel.identity(plan)
        results = self.write_shards(shards)
        rows = {loop: dict(instructions=probe.LOOP_INSTRUCTIONS, cycles=352256) for loop in ("nop", "add")}
        output = self.root / "frontend/report.json"
        for error in (None, PermissionError("downloaded head is not executable")):
            with patch.object(user, "source_state", return_value=state), \
                 patch.object(probe, "measure", return_value=rows, side_effect=error) as measure, \
                 contextlib.redirect_stdout(io.StringIO()):
                status = self.serial_compare(plan, results, output)
            measure.assert_called_once()
            report = parallel.read(output)
            self.assertEqual(status, int(error is not None))
            if error:
                self.assertEqual(report["artifact_status"], "fail")
                self.assertIn("not executable", report["error"])
            else:
                self.assertEqual(report["frontend_simulation"], rows)

    def test_serial_comparison_requires_throughput_suite_when_source_declares_it(self):
        state = copy.deepcopy(self.state)
        state["files"][throughput.ELF] = "f" * 64
        state["files"][probe.ELF] = "e" * 64
        plan = copy.deepcopy(self.plan)
        plan["head"] = state
        parallel.write(self.prepared / "plan.json", plan)
        shards = self.shards()
        for shard in shards:
            shard["plan_id"] = parallel.identity(plan)
        results = self.write_shards(shards)
        rows = [dict(id=c["id"], instructions=c["instructions_per_iteration"]*1024, cycles=100000) for c in throughput.cases()]
        output = self.root / "throughput/report.json"
        for error in (None, PermissionError("downloaded head is not executable")):
            with patch.object(user, "source_state", return_value=state), \
                 patch.object(probe, "measure", side_effect=AssertionError("superseded probe ran")) as old_measure, \
                 patch.object(throughput, "load_reference", return_value={}), \
                 patch.object(throughput, "measure", return_value=rows, side_effect=error) as measure, \
                 contextlib.redirect_stdout(io.StringIO()):
                status = self.serial_compare(plan, results, output)
            measure.assert_called_once()
            old_measure.assert_not_called()
            report = parallel.read(output)
            self.assertEqual(status, int(error is not None))
            if error:
                self.assertEqual(report["artifact_status"], "fail")
                self.assertIn("not executable", report["error"])
            else:
                self.assertEqual(report["throughput_simulation"], rows)
                self.assertNotIn("frontend_simulation", report)

    def test_serial_comparison_runs_forwarding_on_both_revision_binaries(self):
        state=copy.deepcopy(self.state)
        state['files'][forwarding.ELF]='f'*64
        plan=copy.deepcopy(self.plan);plan['head']=state
        parallel.write(self.prepared/'plan.json',plan)
        shards=self.shards()
        for shard in shards:shard['plan_id']=parallel.identity(plan)
        results=self.write_shards(shards)
        rows=[dict(id=c['id'],instructions=c['instructions_per_iteration']*1024,cycles=100000,updates=65536)
              for c in forwarding.cases()]
        output=self.root/'forwarding/report.json'
        for error in (None,PermissionError('base forwarding execution unavailable')):
            with patch.object(user,'source_state',return_value=state), \
                 patch.object(forwarding,'load_reference',return_value={}), \
                 patch.object(forwarding,'measure',side_effect=[rows,error if error else rows]) as measure, \
                 contextlib.redirect_stdout(io.StringIO()):
                status=self.serial_compare(plan,results,output)
            self.assertEqual(measure.call_count,2)
            calls=measure.call_args_list
            self.assertEqual(calls[0].args[2],state['commit'])
            self.assertEqual(calls[1].args[2],plan['base_commit'])
            self.assertEqual(calls[0].args[3],self.profile)
            self.assertEqual(calls[1].args[3],self.profile)
            report=parallel.read(output)
            self.assertEqual(status,int(error is not None))
            if error:self.assertIn('unavailable',report['error'])
            else:self.assertEqual(report['forwarding_simulation'],dict(base=rows,head=rows))

    def test_distinct_revision_profiles_survive_all_264_replayed_cases(self):
        state = copy.deepcopy(self.state)
        head_profile = copy.deepcopy(self.profile)
        head_profile["configuration"]["fetch_width"] = 8
        head_hash = hashlib.sha256(models.encode(head_profile)).hexdigest()
        state["files"][models.PATH] = head_hash
        declarations = {"head": dict(commit=HEAD, path=models.PATH, sha256=head_hash, profile=head_profile),
                        "base": models.revision(Path(__file__).resolve().parents[1], BASE)}
        self.plan["head"] = state
        self.plan["model_profiles"] = declarations
        parallel.write(self.prepared / "plan.json", self.plan)
        shards = self.shards()
        for shard in shards:
            for record in shard["records"]:
                record["executions"]["head"]["timing"].update(copy.deepcopy(head_profile))
        results = self.write_shards(shards)
        output = self.root / "models/report.json"
        with patch.object(user, "source_state", return_value=state), \
             patch.object(models, "resolve", return_value=declarations), \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(parallel.aggregate(self.prepared, results, "123", 1, output), 0)
            report = parallel.read(output)
            self.assertEqual(len(report["cases"]), 264)
            self.assertEqual(report["model_profiles"], declarations)
            for row in report["cases"]:
                self.assertEqual(row["head"]["configuration"], head_profile["configuration"])
                if "base" in row:
                    self.assertEqual(row["base"]["configuration"], self.profile["configuration"])
            for role in ("head", "base"):
                altered = copy.deepcopy(shards)
                record = next(record for shard in altered for record in shard["records"] if role in record["executions"])
                record["executions"][role]["timing"]["configuration"]["fetch_width"] = 16
                self.write_shards(altered)
                self.assertNotEqual(parallel.aggregate(self.prepared, results, "123", 1, output), 0)
                self.assertIn("declared model profile", parallel.read(output)["error"])

    def test_shard_failure_retains_partial_records_and_continues_other_cases(self):
        ids = self.plan["shards"][0]
        def live(binary, case, mode=None, expected_output=None, expected_identity=None):
            if case["id"] == ids[0]:
                raise ValueError("deliberate execution failure")
            role = "functional" if mode == "functional" else "base" if Path(binary).name == "base" else "head"
            return self.result(case, role)
        with patch.object(user, "execute", side_effect=live), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(parallel.run_shard(self.prepared, "123", 1, 0, self.root / "shard.json"), 1)
        report = parallel.read(self.root / "shard.json")
        self.assertEqual(report["status"], "fail")
        self.assertEqual(len(report["records"]), 33)
        self.assertEqual(len(report["errors"]), 1)
        last = next(c for c in self.cases if c["id"] == report["records"][-1]["case_id"])
        self.assertEqual(set(report["records"][-1]["executions"]), set(parallel.roles(self.plan,last,self.witnesses)))

    def test_strict_json_rejects_nonfinite_values(self):
        path = self.root / "bad.json"
        path.write_text('{"metric": NaN}')
        with self.assertRaises(ValueError):
            parallel.read(path)

    def test_missing_prepare_or_download_yields_failure_report(self):
        for prepared, results in ((self.root / "missing-prepare", self.root / "missing-results"),
                                  (self.prepared, self.root / "missing-results")):
            output = self.root / "missing/report.json"
            with patch.object(parallel, "git", return_value=HEAD):
                self.assertEqual(parallel.aggregate(prepared, results, "123", 1, output), 1)
            report = parallel.read(output)
            self.assertEqual(report["artifact_status"], "fail")
            self.assertEqual(report["head_commit"], HEAD)

    def test_serial_comparison_runs_capacity_on_both_revision_binaries(self):
        state=copy.deepcopy(self.state)
        state['files'][capacity.ELF]='f'*64
        plan=copy.deepcopy(self.plan);plan['head']=state
        parallel.write(self.prepared/'plan.json',plan)
        shards=self.shards()
        for shard in shards:shard['plan_id']=parallel.identity(plan)
        results=self.write_shards(shards)
        rows=[dict(id=c['id'],instructions=c['instructions_per_iteration']*1024,cycles=100000,updates=65536)
              for c in capacity.cases()]
        output=self.root/'capacity/report.json'
        for error in (None,PermissionError('base capacity execution unavailable')):
            with patch.object(user,'source_state',return_value=state), \
                 patch.object(capacity,'load_reference',return_value={}), \
                 patch.object(capacity,'measure',side_effect=[rows,error if error else rows]) as measure, \
                 contextlib.redirect_stdout(io.StringIO()):
                status=self.serial_compare(plan,results,output)
            self.assertEqual(measure.call_count,2)
            calls=measure.call_args_list
            self.assertEqual(calls[0].args[2],state['commit'])
            self.assertEqual(calls[1].args[2],plan['base_commit'])
            self.assertEqual(calls[0].args[3],self.profile)
            self.assertEqual(calls[1].args[3],self.profile)
            report=parallel.read(output)
            self.assertEqual(status,int(error is not None))
            if error:self.assertIn('unavailable',report['error'])
            else:self.assertEqual(report['capacity_simulation'],dict(base=rows,head=rows))

if __name__ == "__main__":
    unittest.main()
