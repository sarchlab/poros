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
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import accuracy_parallel as parallel
import accuracy_site as site
import usertime as user
import accuracy_test_support as fixtures

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
        cls.state.update(commit=HEAD, dirty=False)
        for module in (parallel, user, user.native):
            patcher = patch.object(module, "ROOT", cls.fixture.source)
            patcher.start()
            cls.addClassCleanup(patcher.stop)
        git_patch = patch.object(parallel, "git", return_value=HEAD)
        git_patch.start()
        cls.addClassCleanup(git_patch.stop)
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

    def test_plan_covers_all_264_once(self):
        ids = [ident for shard in self.plan["shards"] for ident in shard]
        self.assertEqual(len(ids), 264)
        self.assertEqual(len(set(ids)), 264)
        self.assertEqual([len(s) for s in self.plan["shards"]], [33] * 8)
        self.assertEqual(self.plan["shards"], parallel.partition(self.cases, 8))
        for n in (0, -1, True, 265):
            with self.assertRaises(ValueError):
                parallel.partition(self.cases, n)

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


if __name__ == "__main__":
    unittest.main()
