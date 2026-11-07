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
        self.assertIn(b'Live snapshot', page)
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
        self.assertIn(b'sessionStorage.setItem', partial)
        self.assertIn(b'details[id][open]', partial)
        site.write(view.snapshot, self.snapshot(True))
        completed, assets = view.load()
        self.assertIn(b'Complete validated bundle', completed)
        self.assertIn(b'href="bundle.json"', completed)
        self.assertNotIn(b'setTimeout', completed)
        self.assertIn(b'sessionStorage.getItem', completed)
        self.assertIn(b'sessionStorage.removeItem', completed)
        self.assertIn('report.json', assets)
        waiting, _ = self.view(self.snapshot(True)).load()
        self.assertIn(b'setTimeout', waiting)
        broken, _ = self.view(self.snapshot(True), self.folder / 'missing').load()
        self.assertIn(b'Final bundle validation failed', broken)
        self.assertNotIn(b'Complete validated bundle', broken)
        self.assertIn(b'setTimeout', broken)

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
