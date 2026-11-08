#!/usr/bin/env python3
"""Scheduling evidence shares transport checks without joining application stats."""
import copy
import hashlib
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import scheduling_microbenchmark as suite
import throughput_microbenchmark as throughput
import accuracy_site as site
import accuracy_test_support as fixtures
import compute_usertime as compute


class SchedulingTests(unittest.TestCase):
    def setUp(self):
        self.reference = suite.load_reference(suite.ROOT)
        self.cases = suite.cases()
        self.rows = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * 1024, cycles=100000) for c in self.cases]
        self.comparison = {role: copy.deepcopy(self.rows) for role in ("base", "head")}

    def test_suite_is_explicit_and_does_not_change_old_reference(self):
        old = throughput.load_reference(suite.ROOT)
        self.assertEqual(len(old["cases"]), 17)
        self.assertEqual(len(self.cases), 4)
        self.assertNotEqual(suite.SUITE.protocol, throughput.THROUGHPUT.protocol)
        self.assertEqual(suite.expected(self.cases[0], 2), 780)
        self.assertEqual(suite.expected(self.cases[1], 2), 1508)
        self.assertEqual(suite.expected(self.cases[2], 2), 262)
        self.assertEqual(suite.expected(self.cases[3], 2), 506)
        with self.assertRaises(ValueError):
            throughput.validate_reference(self.reference)
        for mutation in (lambda r: r["cases"].pop(), lambda r: r.update(protocol=throughput.PROTOCOL),
                         lambda r: r["sources"].pop(suite.ELF), lambda r: r["cases"][0].update(median_seconds=float("nan"))):
            bad = copy.deepcopy(self.reference)
            mutation(bad)
            with self.assertRaises(ValueError):
                suite.validate_reference(bad)

    def test_comparison_requires_both_roles_and_exact_instruction_counts(self):
        suite.validate_comparison(self.comparison, self.cases)
        for mutation in (lambda r: r.pop("base"), lambda r: r["base"].pop(),
                         lambda r: r["head"][0].update(instructions=1)):
            bad = copy.deepcopy(self.comparison)
            mutation(bad)
            with self.assertRaises(ValueError):
                suite.validate_comparison(bad, self.cases)

    def test_panels_group_by_ids_and_reject_missing_patterns(self):
        reference = copy.deepcopy(self.reference)
        # A self-consistent synthetic reference may order its cases differently;
        # integer and FP charts must still select their named patterns.
        order = (2, 0, 3, 1)
        reference["cases"] = [reference["cases"][i] for i in order]
        lines = reference["definition"].splitlines()
        reference["definition"] = "\n".join(lines[i] for i in order) + "\n"
        reference["sources"][suite.DIRECTORY + "cases.def"] = hashlib.sha256(reference["definition"].encode()).hexdigest()
        simulations = {role: [self.rows[i] for i in order] for role in ("base", "head")}
        profiles = {role: {"configuration": {"freq": 3490000000}} for role in ("base", "head")}
        with patch.object(site, "chart", return_value="chart") as chart:
            site.scheduling_panels(reference, simulations, profiles)
        for call, names in zip(chart.call_args_list, (("int_chain", "int_interleaved"), ("fp_chain", "fp_interleaved"))):
            points = call.args[1][0][1]
            self.assertEqual([p[2].split(":")[0] for p in points], list(names))
        reference["cases"].pop()
        reference["definition"] = "\n".join(reference["definition"].splitlines()[:-1]) + "\n"
        reference["sources"][suite.DIRECTORY + "cases.def"] = hashlib.sha256(reference["definition"].encode()).hexdigest()
        for rows in simulations.values():
            rows.pop()
        with self.assertRaisesRegex(ValueError, "four declared dependency patterns"):
            site.scheduling_panels(reference, simulations, profiles)

    def test_bundle_and_archive_keep_scheduling_separate_and_fail_closed(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            fixture = fixtures.Evidence(suite.ROOT, root / "fixture")
            report = fixtures.compact_report(fixture)
            before = site.benchmark_summary(report)
            for name in [*suite.INPUTS, suite.REFERENCE]:
                target = fixture.source / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(suite.ROOT / name, target)
                report["head"]["files"][name] = suite.digest(target)
            report["scheduling_simulation"] = self.comparison
            raw = root / "report.json"
            compute.write(raw, report)
            raw.with_suffix(".md").write_text("Synthetic unit application data")
            bundle = root / "bundle"
            site.bundle(raw, fixture.source, bundle, fixtures.SHA)
            valid, manifest = site.validate_bundle(bundle, fixtures.SHA)
            self.assertEqual(before, site.benchmark_summary(valid))
            page = (bundle / "index.html").read_text()
            overall, micro = page.split('<section id="microbenchmarks">')
            self.assertNotIn("Execution-unit scheduling probes", overall)
            self.assertIn("Execution-unit scheduling probes", micro)
            self.assertIn("ns/iteration", micro)
            self.assertIn('<details id="scheduling-table">', micro)
            self.assertLess(micro.index("Execution-unit scheduling probes"), micro.index('id="scheduling-table"'))
            self.assertIn("scheduling.json", manifest["files"])
            url = "https://github.com/sarchlab/poros/actions/runs/1/attempts/1"
            metadata = dict(commit=fixtures.SHA, kind="main", conclusion="success", run_id=1, run_attempt=1,
                            run_url=url, evaluation_conclusion="success", evaluation_run_id=1,
                            evaluation_run_attempt=1, evaluation_run_url=url)
            pages = root / "pages"
            site.archive(pages, bundle, metadata, [fixtures.SHA])
            archived = (pages / "calibration/runs" / fixtures.SHA / "index.html").read_text()
            self.assertIn("Execution-unit scheduling probes", archived)
            self.assertIn('<details id="scheduling-table">', archived)

            for mutate in (lambda r: r.pop("scheduling_simulation"),
                           lambda r: r["scheduling_simulation"].pop("base"),
                           lambda r: r["head"]["files"].update({suite.DIRECTORY + "loops.S": "0" * 64})):
                bad = copy.deepcopy(report)
                mutate(bad)
                compute.write(bundle / "report.json", bad)
                manifest["files"]["report.json"] = suite.digest(bundle / "report.json")
                compute.write(bundle / "bundle.json", manifest)
                with self.assertRaises(ValueError):
                    site.validate_bundle(bundle, fixtures.SHA)


if __name__ == "__main__":
    unittest.main()
