#!/usr/bin/env python3
"""One renderer for partial observations and validated CI bundles."""
import copy
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import shutil
import tempfile
import threading
import unittest
from http.server import ThreadingHTTPServer
from html.parser import HTMLParser
from urllib.request import urlopen
from urllib.error import HTTPError
from unittest.mock import patch

import accuracy_live as live
import accuracy_site as site
import accuracy_test_support as fixtures
import model_profiles as models

ROOT = Path(__file__).resolve().parents[1]


class LiveReportTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.tmp.cleanup)
        cls.folder = Path(cls.tmp.name)
        fixture = fixtures.Evidence(ROOT, cls.folder / "evidence")
        cls.source = fixture.source
        cls.report = fixtures.compact_report(fixture)
        report = cls.report
        profile = report["workload_manifest"]["profile"]
        profile_hash = hashlib.sha256(models.encode(profile)).hexdigest()
        report["head"]["files"][models.PATH] = profile_hash
        report["model_profiles"] = {
            role: dict(commit=report["head"]["commit"] if role == "head" else report["base_commit"],
                       path=models.PATH, sha256=profile_hash, profile=copy.deepcopy(profile))
            for role in ("head", "base")}
        cls.reference = site.throughput.load_reference(ROOT)
        for name in [*site.throughput.MEASUREMENT_INPUTS, site.throughput.ELF, site.throughput.REFERENCE]:
            path = cls.source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, path)
            report["head"]["files"][name] = site.digest(path)
        report["throughput_simulation"] = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * 1024, cycles=100000) for c in cls.reference["cases"]]
        raw = cls.folder / "report.json"
        site.write(raw, report)
        raw.with_suffix('.md').write_text('Synthetic report transport fixture')
        cls.bundle = cls.folder / "bundle"
        site.bundle(raw, cls.source, cls.bundle, report["head"]["commit"])

    def snapshot(self, complete=False):
        report = copy.deepcopy(self.report)
        report["artifact_status"] = "partial"
        if not complete:
            report["cases"] = report["cases"][:30]
        return dict(kind=live.KIND, report=report,
                    progress=dict(state="complete" if complete else "running", completed_cases=len(report["cases"]),
                                  required_cases=264, completed_shards=132 if complete else 15, required_shards=132,
                                  active_cases=[] if complete else [report["workload_manifest"]["cases"][30]["id"]], errors=[],
                                  updated_at=datetime.now(timezone.utc).isoformat()),
                    throughput_reference=self.reference,
                    throughput_provenance=dict(commit=report["head"]["commit"], binary_sha256=report["head_binary_sha256"], model_profile=report["model_profiles"]["head"]))

    def view(self, value, bundle=None):
        path = self.folder / self.id().split('.')[-1] / 'snapshot.json'
        site.write(path, value)
        return live.SnapshotView(path, bundle)

    def test_live_progress_follows_title_and_does_not_claim_unvalidated_completion(self):
        value = self.snapshot()
        progress = value["progress"]
        entry = dict(commit=self.report["head"]["commit"])
        page = site.detail(self.report, entry, live=progress)
        self.assertLess(page.index('</header>'), page.index('id="case-progress"'))
        self.assertLess(page.index('id="case-progress"'), page.index('<nav'))
        self.assertIn('role="progressbar"', page)
        self.assertIn('aria-valuenow="30" aria-valuemax="264"', page)
        self.assertIn('aria-valuetext="Completed: 30; In progress: 1; Not started: 233"', page)
        for segment in ("completed", "active", "remaining"):
            self.assertIn(f'data-progress-state="{segment}"', page)
        self.assertIn('30 / 264 cases · 11.4%', page)
        self.assertNotIn('Active cases:', page)
        self.assertIn('active_cases', progress)
        static = site.detail(self.report, entry)
        self.assertNotIn('id="case-progress"', static)
        for rendered in (page, static):
            for removed in ("Live snapshot — not a publishable report.",
                            "Partial results are not a final accuracy conclusion",
                            "Hardware accuracy: unverified", "Core identity and instantaneous frequency",
                            "Report artifact", "Full Evaluation", "Usable benchmark references",
                            '<div class="cards">'):
                self.assertNotIn(removed, rendered)
            self.assertIn("Evidence and provenance", rendered)
        self.assertEqual(self.report["accuracy_status"], "unverified")
        for state, validated, expected in (("failed", False, "Failed"),
                                            ("complete", False, "Cases finished · validation pending"),
                                            ("complete", True, "Complete validated bundle.")):
            with self.subTest(state=state, validated=validated):
                current = dict(progress, state=state, completed_cases=264,
                               completed_shards=132, active_cases=[], validated_bundle=validated)
                if state == "failed":
                    current["errors"] = ["bundle <validation> failed"]
                rendered = site.detail(self.report, entry, live=current)
                self.assertIn(expected, rendered)
                self.assertIn('264 / 264 cases · 100.0%', rendered)
                if not validated:
                    self.assertNotIn('Complete validated bundle.', rendered)
                if state == "failed":
                    self.assertIn('bundle &lt;validation&gt; failed', rendered)

    def test_progress_counts_use_unique_active_cases_and_validate_disjointness(self):
        value = self.snapshot()
        progress = value["progress"]
        progress["active_cases"] *= 3
        with self.assertRaises(ValueError):
            live.validate_snapshot(value)
        rendered = site.live_progress(progress)
        self.assertIn('Completed: 30; In progress: 1; Not started: 233', rendered)
        idle = dict(progress, active_cases=[])
        self.assertIn('Completed: 30; In progress: 0; Not started: 234', site.live_progress(idle))
        for active in ([value["report"]["cases"][0]["id"]], ["unknown-case"]):
            invalid = copy.deepcopy(value)
            invalid["progress"]["active_cases"] = active
            with self.assertRaises(ValueError):
                live.validate_snapshot(invalid)
        for completed, required in ((265, 264), (-1, 264), (0, 0)):
            with self.assertRaises(ValueError):
                site.live_progress(dict(idle, completed_cases=completed, required_cases=required))
        with self.assertRaises(ValueError):
            site.live_progress(dict(progress, completed_cases=264))

    def test_memory_snapshot_and_complete_bundle_share_bound_evidence(self):
        value = self.snapshot()
        report, suite = value['report'], site.memory
        reference = suite.load_reference(ROOT)
        for name in [*suite.INPUTS, suite.REFERENCE]:
            path = self.source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, path)
            report['head']['files'][name] = suite.digest(path)
        rows = [dict(id=c['id'], instructions=c['instructions_per_pass'] * c['passes'], cycles=1000000)
                for c in reference['cases']]
        report['memory_simulation'] = dict(base=copy.deepcopy(rows), head=copy.deepcopy(rows))
        report['memory_base_large_image_support'] = True
        value['memory_reference_text'] = (ROOT / suite.REFERENCE).read_text()
        provenance = {key: report[key] for key in ('base_commit', 'head_binary_sha256', 'base_binary_sha256', 'model_profiles')}
        provenance['head_commit'] = report['head']['commit']
        value['memory_provenance'] = copy.deepcopy(provenance)
        live.validate_snapshot(value)
        rendered, assets = self.view(value).load()
        self.assertEqual(assets['memory.json'], (ROOT / suite.REFERENCE).read_bytes())
        text = rendered.decode()
        self.assertLess(text.index('Memory latency, parallelism and bandwidth'), text.index('<details id="memory-diagnostics-table">'))
        self.assertIn('data-tip=', text)
        for mutate in (lambda v: v.pop('memory_reference_text'),
                       lambda v: v['report']['memory_simulation'].pop('base'),
                       lambda v: v['report'].pop('memory_base_large_image_support'),
                       lambda v: v['report'].update(memory_base_large_image_support=False),
                       lambda v: v['memory_provenance'].update(head_commit='0' * 40),
                       lambda v: v['memory_provenance'].update(base_binary_sha256='0' * 64),
                       lambda v: v.update(memory_reference_text=v['memory_reference_text'] + ' '),
                       lambda v: v['report']['head']['files'].pop(suite.ELF)):
            bad = copy.deepcopy(value); mutate(bad)
            with self.assertRaises((ValueError, KeyError)): live.validate_snapshot(bad)
        complete = copy.deepcopy(value)
        complete['report']['cases'] = copy.deepcopy(self.report['cases'])
        complete['progress'].update(state='complete', completed_cases=264, completed_shards=132, active_cases=[])
        report = copy.deepcopy(complete['report']); report['artifact_status'] = 'pass'
        folder = self.folder / 'memory-complete'
        folder.mkdir()
        raw = folder / 'report.json'; site.write(raw, report)
        raw.with_suffix('.md').write_text('Synthetic memory transport; not hardware performance evidence')
        bundle = folder / 'bundle'
        site.bundle(raw, self.source, bundle, report['head']['commit'])
        rendered, assets = self.view(complete, bundle).load()
        self.assertIn(b'Complete validated bundle.', rendered)
        self.assertEqual(assets['memory.json'], (ROOT / suite.REFERENCE).read_bytes())
        # Adding diagnostics must leave nonempty application statistics unchanged.
        self.assertEqual(site.benchmark_summary(report), site.benchmark_summary(self.report))
        self.assertEqual(site.overview_charts([c for c in report['cases'] if not site.is_micro_case(c)]),
                         site.overview_charts([c for c in self.report['cases'] if not site.is_micro_case(c)]))
        for label, mutate in [('missing', lambda r: r.pop('memory_simulation')),
                              ('partial', lambda r: r['memory_simulation']['base'].pop()),
                              ('missing-capability', lambda r: r.pop('memory_base_large_image_support')),
                              ('wrong-capability', lambda r: r.update(memory_base_large_image_support=False))]:
            bad = copy.deepcopy(report); mutate(bad)
            bad_raw = folder / (label + '.json'); site.write(bad_raw, bad)
            bad_raw.with_suffix('.md').write_text('Invalid synthetic memory evidence')
            with self.assertRaises(ValueError):
                site.bundle(bad_raw, self.source, folder / label, report['head']['commit'])
        (bundle / 'memory.json').write_bytes((ROOT / suite.REFERENCE).read_bytes() + b' ')
        with self.assertRaises(ValueError): site.validate_bundle(bundle, report['head']['commit'])
        (bundle / 'memory.json').unlink()
        with self.assertRaises((ValueError, OSError)): site.validate_bundle(bundle, report['head']['commit'])
        # The historical bundle has no memory declaration and remains readable.
        historical, _ = site.validate_bundle(self.bundle, self.report['head']['commit'])
        self.assertNotIn('memory_simulation', historical)

    def test_forwarding_snapshot_requires_both_revision_provenance_and_one_renderer(self):
        value=self.snapshot()
        report=value['report']
        suite=site.forwarding
        reference=suite.load_reference(ROOT)
        for name in [*suite.MEASUREMENT_INPUTS,suite.ELF,suite.REFERENCE]:
            report['head']['files'][name]=suite.digest(ROOT/name)
        rows=[dict(id=c['id'],instructions=c['instructions_per_iteration']*1024,cycles=100000,updates=65536)
              for c in reference['cases']]
        report['forwarding_simulation']=dict(base=copy.deepcopy(rows),head=copy.deepcopy(rows))
        value['forwarding_reference_text']=(ROOT/suite.REFERENCE).read_text()
        provenance={key:report[key] for key in ('base_commit','head_binary_sha256','base_binary_sha256','model_profiles')}
        provenance['head_commit']=report['head']['commit']
        value['forwarding_provenance']=copy.deepcopy(provenance)
        live.validate_snapshot(value)
        rendered,assets=self.view(value).load()
        self.assertIn('forwarding.json',assets)
        self.assertEqual(hashlib.sha256(assets['forwarding.json']).hexdigest(), report['head']['files'][suite.REFERENCE])
        def tamper_native(v):
            changed=json.loads(v['forwarding_reference_text'])
            for key in ('p10_seconds','median_seconds','p90_seconds'):
                changed['cases'][0][key] *= 2
            v['forwarding_reference_text']=json.dumps(changed)
        page=rendered.decode();micro=page.split('<section id="microbenchmarks">')[1]
        self.assertLess(micro.index('64-bit GPR dependencies'),micro.index('<details id="throughput-table">'))
        self.assertIn('<details id="forwarding-table">',micro)
        for mutate in (tamper_native, lambda v:v.pop('forwarding_reference_text'),
                       lambda v:v['report']['forwarding_simulation'].pop('base'),
                       lambda v:v['forwarding_provenance'].update(base_binary_sha256='0'*64),
                       lambda v:v['forwarding_provenance'].update(head_commit='0'*40),
                       lambda v:v['report']['head']['files'].pop(suite.ELF),
                       lambda v:v['report']['head']['files'].pop(suite.REFERENCE),
                       lambda v:v.update(forwarding_reference_text=v['forwarding_reference_text'].replace('median_seconds', 'altered_seconds'))):
            bad=copy.deepcopy(value);mutate(bad)
            with self.assertRaises((ValueError,KeyError)):
                live.validate_snapshot(bad)

        # The completed CI bundle must retain the same renderer and exact asset.
        complete=copy.deepcopy(value)
        complete['report']['cases']=copy.deepcopy(self.report['cases'])
        complete['progress'].update(state='complete',completed_cases=264,completed_shards=132,active_cases=[])
        for name in [*suite.INPUTS,suite.REFERENCE]:
            target=self.source/name;target.parent.mkdir(parents=True,exist_ok=True)
            shutil.copyfile(ROOT/name,target)
        final=copy.deepcopy(complete['report']);final['artifact_status']='pass'
        folder=self.folder/'forwarding-complete';folder.mkdir()
        raw=folder/'report.json';site.write(raw,final);raw.with_suffix('.md').write_text('Synthetic complete fixture')
        bundle=folder/'bundle';site.bundle(raw,self.source,bundle,final['head']['commit'])
        rendered,assets=self.view(complete,bundle).load()
        self.assertIn(b'Complete validated bundle.',rendered)
        self.assertEqual(assets['forwarding.json'],(ROOT/suite.REFERENCE).read_bytes())
        self.assertIn(b'<details id="forwarding-table">',rendered)

    def test_scheduling_snapshot_requires_both_revision_provenance_and_one_renderer(self):
        value=self.snapshot()
        report=value['report']
        suite=site.scheduling
        reference=suite.load_reference(ROOT)
        for name in [*suite.MEASUREMENT_INPUTS,suite.ELF,suite.REFERENCE]:
            report['head']['files'][name]=suite.digest(ROOT/name)
        rows=[dict(id=c['id'],instructions=c['instructions_per_iteration']*1024,cycles=100000)
              for c in reference['cases']]
        report['scheduling_simulation']=dict(base=copy.deepcopy(rows),head=copy.deepcopy(rows))
        value['scheduling_reference_text']=(ROOT/suite.REFERENCE).read_text()
        provenance={key:report[key] for key in ('base_commit','head_binary_sha256','base_binary_sha256','model_profiles')}
        provenance['head_commit']=report['head']['commit']
        value['scheduling_provenance']=copy.deepcopy(provenance)
        live.validate_snapshot(value)
        rendered,assets=self.view(value).load()
        self.assertIn('scheduling.json',assets)
        self.assertEqual(hashlib.sha256(assets['scheduling.json']).hexdigest(), report['head']['files'][suite.REFERENCE])
        def tamper_native(v):
            changed=json.loads(v['scheduling_reference_text'])
            for key in ('p10_seconds','median_seconds','p90_seconds'):
                changed['cases'][0][key] *= 2
            v['scheduling_reference_text']=json.dumps(changed)
        page=rendered.decode();micro=page.split('<section id="microbenchmarks">')[1]
        self.assertLess(micro.index('Execution-unit scheduling probes'),micro.index('<details id="throughput-table">'))
        self.assertIn('<details id="scheduling-table">',micro)
        for mutate in (tamper_native, lambda v:v.pop('scheduling_reference_text'),
                       lambda v:v['report']['scheduling_simulation'].pop('base'),
                       lambda v:v['scheduling_provenance'].update(base_binary_sha256='0'*64),
                       lambda v:v['scheduling_provenance'].update(head_commit='0'*40),
                       lambda v:v['report']['head']['files'].pop(suite.ELF),
                       lambda v:v['report']['head']['files'].pop(suite.REFERENCE),
                       lambda v:v.update(scheduling_reference_text=v['scheduling_reference_text'].replace('median_seconds', 'altered_seconds'))):
            bad=copy.deepcopy(value);mutate(bad)
            with self.assertRaises((ValueError,KeyError)):
                live.validate_snapshot(bad)

        # The completed CI bundle must retain the same renderer and exact asset.
        complete=copy.deepcopy(value)
        complete['report']['cases']=copy.deepcopy(self.report['cases'])
        complete['progress'].update(state='complete',completed_cases=264,completed_shards=132,active_cases=[])
        for name in [*suite.INPUTS,suite.REFERENCE]:
            target=self.source/name;target.parent.mkdir(parents=True,exist_ok=True)
            shutil.copyfile(ROOT/name,target)
        final=copy.deepcopy(complete['report']);final['artifact_status']='pass'
        folder=self.folder/'scheduling-complete';folder.mkdir()
        raw=folder/'report.json';site.write(raw,final);raw.with_suffix('.md').write_text('Synthetic complete fixture')
        bundle=folder/'bundle';site.bundle(raw,self.source,bundle,final['head']['commit'])
        rendered,assets=self.view(complete,bundle).load()
        self.assertIn(b'Complete validated bundle.',rendered)
        self.assertEqual(assets['scheduling.json'],(ROOT/suite.REFERENCE).read_bytes())
        self.assertIn(b'<details id="scheduling-table">',rendered)

    def test_shared_complete_layout_metrics_and_seventeen_probes(self):
        value = self.snapshot(True)
        page, _ = self.view(value).load()
        page = page.decode()
        static = site.detail(self.report, dict(commit=self.report["head"]["commit"]), preview=True, throughput_reference=self.reference)
        def content(text):
            return text[text.index('<h2 id="overall">'):text.index('<h2>Evidence and provenance')]
        self.assertEqual(content(page), content(static))
        self.assertIn('Base and head comparison', page)
        self.assertNotIn('sessionStorage', static)
        self.assertNotIn('async function refresh', static)
        for case in self.reference['cases']:
            self.assertIn('<td>' + case['id'] + '</td>', page)
        self.assertLess(page.index('Overall benchmark accuracy'), page.index('Benchmark timing curves'))
        self.assertLess(page.index('Benchmark timing curves'), page.index('Benchmark timing table'))
        self.assertLess(page.index('Benchmark timing table'), page.index('<section id="microbenchmarks">'))
        class TableDisclosureParser(HTMLParser):
            def __init__(self):
                super().__init__()
                self.stack, self.disclosures, self.table_owners = [], {}, []
            def handle_starttag(self, tag, attrs):
                attrs = dict(attrs)
                if tag == "details":
                    ident = attrs.get("id")
                    self.stack.append(ident)
                    if ident:
                        self.disclosures[ident] = attrs
                elif tag == "table":
                    self.table_owners.append(tuple(self.stack))
            def handle_endtag(self, tag):
                if tag == "details":
                    self.stack.pop()
        for rendered in (page, static):
            parsed = TableDisclosureParser()
            parsed.feed(rendered)
            for ident in ("benchmark-table", "throughput-table", "microbenchmark-table"):
                self.assertIn(ident, parsed.disclosures)
                self.assertNotIn("open", parsed.disclosures[ident])
                self.assertIn((ident,), parsed.table_owners)
            self.assertIn((), parsed.table_owners)  # Compact base/head summary stays visible.
            micro = rendered[rendered.index('<section id="microbenchmarks">'):]
            self.assertLess(micro.index('<svg'), micro.index('<details'))
            self.assertIn('Instruction throughput table · 17 probes', micro)
            self.assertIn('Whole-program microbenchmark timing table · 18 cases', micro)

    def test_historical_frontend_table_remains_renderable_and_collapsed(self):
        reference = site.read(ROOT / site.FRONTEND_REFERENCE)
        rendered = site.detail(self.report, dict(commit=self.report["head"]["commit"]), frontend=reference)
        self.assertIn('<details id="frontend-table"><summary>Frontend throughput table · 2 probes</summary>', rendered)
        self.assertIn('href="frontend.json"', rendered)
        micro = rendered[rendered.index('<section id="microbenchmarks">'):]
        self.assertLess(micro.index('<svg'), micro.index('<details id="frontend-table">'))

    def test_failed_ci_hides_summary_while_live_partial_shows_it(self):
        value = self.snapshot()
        failed = copy.deepcopy(value["report"])
        failed["artifact_status"] = "fail"
        rendered = site.detail(failed, dict(commit=failed["head"]["commit"]))
        self.assertNotIn("Application timing error across", rendered)
        partial, _ = self.view(value).load()
        self.assertIn(b"Application timing error across", partial)
        self.assertIn(b"maximum", partial)

    def test_completed_assets_drop_snapshot_only_downloads(self):
        manifest = site.read(self.bundle / "bundle.json")
        manifest["files"].pop("throughput.json")
        # Exercise the transport boundary with a validator-approved asset list
        # that lacks the optional throughput attachment.
        with patch.object(site, "validate_bundle", return_value=(self.report, manifest)):
            rendered, assets = self.view(self.snapshot(True), self.bundle).load()
        self.assertIn(b"Complete validated bundle", rendered)
        self.assertEqual(set(assets), {"snapshot.json", "bundle.json", *manifest["files"]})
        self.assertNotIn("throughput.json", assets)
        self.assertNotIn(b'href="throughput.json"', rendered)

    def test_partial_never_publishable_and_no_missing_downloads(self):
        value = self.snapshot()
        view = self.view(value)
        page, assets = view.load()
        self.assertIn(b'id="case-progress"', page)
        self.assertIn(('Measurement completed (UTC): ' + value['report']['reference']['completed_at_utc']).encode(), page)
        self.assertNotIn(b'href="report.json"', page)
        self.assertNotIn(b'href="report.md"', page)
        self.assertNotIn(b'href="bundle.json"', page)
        self.assertEqual(set(assets), {'snapshot.json', 'throughput.json'})
        for value in (value['report'], value):
            with self.assertRaises(ValueError):
                site.check_report(value, self.report['head']['commit'])
        bad = self.snapshot(); bad['report']['artifact_status'] = 'pass'
        with self.assertRaises(ValueError):
            live.validate_snapshot(bad)

    def test_transition_requires_validated_matching_bundle(self):
        view = self.view(self.snapshot(), self.bundle)
        partial, _ = view.load()
        self.assertIn(b'setTimeout', partial)
        self.assertIn(b"fetch(location.href, {cache:'no-store'", partial)
        self.assertNotIn(b'location.reload', partial)
        self.assertNotIn(b'sessionStorage', partial)
        self.assertIn(b'data-live-state="polling"', partial)
        site.write(view.snapshot, self.snapshot(True))
        completed, assets = view.load()
        self.assertIn(b'Complete validated bundle', completed)
        self.assertIn(b'href="bundle.json"', completed)
        self.assertNotIn(b'setTimeout', completed)
        self.assertIn(b'data-live-state="validated"', completed)
        def identity(page):
            return page.split(b'data-report-identity="')[1].split(b'"')[0]
        self.assertEqual(identity(partial), identity(completed))
        self.assertIn('report.json', assets)
        waiting, _ = self.view(self.snapshot(True)).load()
        self.assertIn(b'setTimeout', waiting)
        broken, _ = self.view(self.snapshot(True), self.folder / 'missing').load()
        self.assertIn(b'Final bundle validation failed', broken)
        self.assertNotIn(b'Complete validated bundle', broken)
        self.assertIn(b'setTimeout', broken)

    def test_refresh_identity_is_stable_across_progress_but_not_other_campaigns(self):
        entry = dict(commit=self.report['head']['commit'])
        value = self.snapshot()
        def identity(report, progress):
            page = site.detail(report, entry, live=progress)
            return page.split('data-report-identity="')[1].split('"')[0]
        initial = identity(value['report'], value['progress'])
        complete = self.snapshot(True)
        self.assertEqual(initial, identity(complete['report'], complete['progress']))
        for field in ('reference_sha256', 'base_commit', 'head_binary_sha256'):
            changed = copy.deepcopy(value['report'])
            changed[field] = 'different'
            self.assertNotEqual(initial, identity(changed, value['progress']))

    def test_ranked_point_focus_identity_survives_inserted_cases(self):
        class Points(HTMLParser):
            def __init__(self):
                super().__init__()
                self.points = {}
            def handle_starttag(self, tag, attrs):
                attrs = dict(attrs)
                if tag == 'circle':
                    self.points[attrs['data-point-key']] = attrs['cx']
        def render(values):
            parser = Points()
            parser.feed(site.chart('S-curve: predicted time relative to measured time',
                                  [('head', values, '#087e8b')], 'rank', 'ratio'))
            return parser.points
        before = render([(1, 1, 'rank 1 · atax-1 · measured 10 ms')])
        after = render([(1, 0, 'rank 1 · bicg-1 · measured 10 ms'),
                        (2, 1, 'rank 2 · atax-1 · measured 10 ms')])
        self.assertEqual(set(before) & set(after), set(before))
        key = next(iter(before))
        self.assertNotEqual(before[key], after[key])

    def test_same_commit_bundle_must_match_campaign_identity(self):
        for field in ("head_binary_sha256", "base_binary_sha256", "reference_sha256", "model_profiles", "workload_manifest"):
            changed = copy.deepcopy(self.report)
            changed[field] = "changed"
            with self.subTest(field=field), patch.object(site, "validate_bundle", return_value=(changed, {})):
                page, assets = self.view(self.snapshot(True), self.bundle).load()
                self.assertIn(b'Final bundle validation failed', page)
                self.assertNotIn('report.json', assets)

    def test_last_valid_snapshot_staleness_and_failed_producer(self):
        view = self.view(self.snapshot())
        view.load()
        view.snapshot.write_text('{')
        page, _ = view.load()
        self.assertIn(b'last validated snapshot', page)
        self.assertIn(b'setTimeout', page)
        for malformed in ([], dict(kind=live.KIND, report=[], progress={}),
                          dict(kind=live.KIND, report={"head": None}, progress={})):
            site.write(view.snapshot, malformed)
            self.assertIn(b'last validated snapshot', view.load()[0])
        value = self.snapshot(); value['progress']['updated_at'] = '2000-01-01T00:00:00+00:00'
        site.write(view.snapshot, value)
        self.assertIn(b'progress may be stale', view.load()[0])
        value['progress'].update(state='failed', errors=['Runner stopped'])
        site.write(view.snapshot, value)
        page, _ = view.load()
        self.assertIn(b'Runner stopped', page)
        self.assertIn(b'<strong>Failed</strong>', page)

    def test_comparison_uses_intersection_and_shared_tie_statistics(self):
        rows = [dict(id='app-'+str(i), reference_quality='usable', reference_user_seconds=x,
                     head=dict(predicted_user_seconds=y, error_percent=10), base=dict(predicted_user_seconds=x, error_percent=0))
                for i, (x,y) in enumerate([(1,2),(2,1),(3,3),(4,3)])]
        rows.append(dict(id='missing-base', reference_quality='usable', reference_user_seconds=10, head=dict(predicted_user_seconds=1,error_percent=900)))
        table = site.comparison_table(rows)
        self.assertIn('same 4 qualified', table)
        expected = site.correlation_statistics([(r['reference_user_seconds'],r['head']['predicted_user_seconds'],r['id']) for r in rows[:4]])
        self.assertIn(expected['tau'], table)
        self.assertIn(expected['pair'], table)
        self.assertIn('5 qualified cases', site.overview_charts(rows))

    def test_server_routes_only_available_assets(self):
        view = self.view(self.snapshot())
        server = ThreadingHTTPServer(('127.0.0.1', 0), live.handler(view))
        worker = threading.Thread(target=server.serve_forever, daemon=True); worker.start()
        self.addCleanup(server.server_close); self.addCleanup(server.shutdown)
        url = 'http://127.0.0.1:' + str(server.server_port)
        self.assertIn(b'Whole-program user time', urlopen(url).read())
        self.assertEqual(json.loads(urlopen(url+'/throughput.json').read()), self.reference)
        for path in ('/report.json', '/bundle.json', '/../report.json', '/%2e%2e/report.json', '/model-profile.json'):
            with self.assertRaises(HTTPError) as error:
                urlopen(url+path)
            self.assertEqual(error.exception.code, 404)
            error.exception.close()


if __name__ == '__main__':
    unittest.main()
