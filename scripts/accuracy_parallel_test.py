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

    def test_memory_only_roundtrip_with_real_source_metadata(self):
        # Only simulator execution is synthetic. Git commits, source hashes,
        # reference, model declarations and the complete 140-point transport are real.
        source = self.root / "memory-source"
        shutil.copytree(self.fixture.source, source)
        for name in [*memory.INPUTS, memory.REFERENCE, models.PATH,
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
             patch.object(parallel.compute, 'base_capability', side_effect=lambda rev: original_capability(rev, source)), \
             patch.object(parallel.compute, 'base_declares_cohort', side_effect=lambda rev: original_cohort(rev, source)), \
             contextlib.redirect_stdout(io.StringIO()):
            state = user.source_state()
            self.assertFalse(state['dirty'])
            plan = copy.deepcopy(self.plan)
            plan.update(head=state, base_commit=base, base_capability=original_capability(base, source),
                        model_profiles=models.resolve(source, state, base),
                        memory=parallel.memory_plan(state, len(plan['shards']), base))
            parallel.write(self.prepared / 'plan.json', plan)
            def simulate(binary, root, case, revision, profile):
                samples = self.memory_samples(binary, root, case, revision, profile)
                for sample in samples: sample['binary_sha256'] = state['files'][memory.case_elf(case)]
                return samples
            results = self.root / 'memory-only-shards'
            with patch.object(memory, 'measure_case', side_effect=simulate):
                for index in range(len(plan['memory']['shards'])):
                    self.assertEqual(parallel.run_memory_shard(self.prepared, '123', 1, index,
                                     results / str(index) / 'memory-shard.json'), 0)
            output = self.root / 'memory-only.json'
            with patch.object(memory, 'measure_case', side_effect=AssertionError('aggregate reran simulation')):
                self.assertEqual(parallel.aggregate_memory(self.prepared, results, '123', 1, output), 0)
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

    def test_aggregation_requires_frontend_probe_when_source_declares_it(self):
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
                status = parallel.aggregate(self.prepared, results, "123", 1, output)
            measure.assert_called_once()
            report = parallel.read(output)
            self.assertEqual(status, int(error is not None))
            if error:
                self.assertEqual(report["artifact_status"], "fail")
                self.assertIn("not executable", report["error"])
            else:
                self.assertEqual(report["frontend_simulation"], rows)

    def test_aggregation_requires_throughput_suite_when_source_declares_it(self):
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
                status = parallel.aggregate(self.prepared, results, "123", 1, output)
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

    def test_aggregation_runs_forwarding_on_both_revision_binaries(self):
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
                status=parallel.aggregate(self.prepared,results,'123',1,output)
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

    def test_aggregation_runs_capacity_on_both_revision_binaries(self):
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
                status=parallel.aggregate(self.prepared,results,'123',1,output)
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
