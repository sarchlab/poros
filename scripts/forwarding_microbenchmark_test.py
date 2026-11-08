#!/usr/bin/env python3
"""Reject incomplete/misattributed dependency-probe evidence."""
import copy
import json
from pathlib import Path
import shutil
import tempfile
import unittest

import forwarding_microbenchmark as suite
import accuracy_site as site
import accuracy_test_support as fixtures
import compute_usertime as compute


class ForwardingTests(unittest.TestCase):
    def setUp(self):
        self.reference = suite.load_reference(suite.ROOT)
        self.cases = suite.cases()
        self.rows = [dict(id=c['id'], instructions=c['instructions_per_iteration']*1024,
                          cycles=100000, updates=65536) for c in self.cases]

    def test_source_bound_reference_and_historical_collector(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            for name in [*suite.INPUTS, suite.REFERENCE]:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(suite.ROOT / name, target)
            for name in suite.COLLECTOR_SOURCES:
                (root/name).write_bytes(b'new collector version')
            self.assertEqual(suite.load_reference(root), self.reference)
            for name in suite.MEASUREMENT_INPUTS:
                saved = (root/name).read_bytes()
                (root/name).write_bytes(saved+b'\n')
                with self.subTest(name=name), self.assertRaises(ValueError):
                    suite.load_reference(root)
                (root/name).write_bytes(saved)
            self.assertEqual(json.loads((root/suite.REFERENCE).read_text()), self.reference)

    def test_corrupted_native_or_simulation_evidence_fails_closed(self):
        for mutate in (lambda r:r['cases'].pop(), lambda r:r['cases'].reverse(),
                       lambda r:r['cases'][0].update(median_seconds=float('nan')),
                       lambda r:r['cases'][0].update(p10_seconds=-1),
                       lambda r:r['sources'].pop(suite.ELF),
                       lambda r:r.update(timer='wall time'),
                       lambda r:r.update(definition=r['definition']+'\n')):
            bad=copy.deepcopy(self.reference);mutate(bad)
            with self.assertRaises((ValueError, KeyError)):
                suite.validate_reference(bad)
        suite.validate_simulation(self.rows,self.cases)
        for mutate in (lambda r:r.pop(),lambda r:r.reverse(),lambda r:r[0].update(cycles=True),
                       lambda r:r[0].update(updates=1),lambda r:r[0].update(instructions=1)):
            bad=copy.deepcopy(self.rows);mutate(bad)
            with self.assertRaises(ValueError):
                suite.validate_simulation(bad,self.cases)

    def test_checksum_wrap_and_width_boundaries(self):
        select=next(c for c in self.cases if c['kind']=='select')
        self.assertEqual([suite.expected(select,n) for n in (1023,1024,1025)], [65472,0,64])
        word=next(c for c in self.cases if c['kind']=='gpr32')
        self.assertEqual(suite.expected(word,1<<26),1)
        fp=next(c for c in self.cases if c['kind']=='fp')
        self.assertEqual(suite.expected(fp,3),97)


    def test_comparison_requires_both_revision_roles(self):
        comparison = dict(base=self.rows, head=self.rows)
        suite.validate_comparison(comparison, self.cases)
        for value in ({'head':self.rows}, {'base':self.rows}, dict(comparison, extra=self.rows)):
            with self.assertRaises(ValueError):
                suite.validate_comparison(value,self.cases)

    def test_bundle_plots_are_separate_ordered_and_source_bound(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder)
            fixture=fixtures.Evidence(suite.ROOT,root/'fixture')
            report=fixtures.compact_report(fixture)
            before=site.benchmark_summary(report)
            for name in [*suite.INPUTS,suite.REFERENCE]:
                target=fixture.source/name;target.parent.mkdir(parents=True,exist_ok=True)
                shutil.copyfile(suite.ROOT/name,target)
                report['head']['files'][name]=suite.digest(target)
            report['forwarding_simulation']=dict(base=self.rows,head=self.rows)
            raw=root/'report.json';compute.write(raw,report)
            raw.with_suffix('.md').write_text('Synthetic application fixture')
            bundle=root/'bundle';site.bundle(raw,fixture.source,bundle,fixtures.SHA)
            valid,manifest=site.validate_bundle(bundle,fixtures.SHA)
            self.assertEqual(site.benchmark_summary(valid),before)
            page=(bundle/'index.html').read_text()
            overall,micro=page.split('<section id="microbenchmarks">')
            self.assertNotIn('gpr64_same',overall)
            self.assertIn('forwarding.json',manifest['files'])
            self.assertIn('<details id="forwarding-table">',micro)
            self.assertNotIn('<details id="forwarding-table" open',micro)
            self.assertLess(micro.index('64-bit GPR dependencies'),micro.index('<details id="forwarding-table">'))
            self.assertIn('data-tip=',micro)
            self.assertIn('P10/median/P90',micro)
            self.assertIn('base 0.',micro)
            for case in self.cases:self.assertIn(case['id'],micro)
            url='https://github.com/sarchlab/poros/actions/runs/1/attempts/1'
            metadata=dict(commit=fixtures.SHA,kind='main',conclusion='success',run_id=1,run_attempt=1,
                          run_url=url,evaluation_conclusion='success',evaluation_run_id=1,
                          evaluation_run_attempt=1,evaluation_run_url=url)
            pages=root/'pages';site.archive(pages,bundle,metadata,[fixtures.SHA])
            archived=(pages/'calibration/runs'/fixtures.SHA/'index.html').read_text()
            self.assertIn('<details id="forwarding-table">',archived)
            self.assertIn('64-bit GPR dependencies',archived)
            for mutate in (lambda r:r.pop('forwarding_simulation'),
                           lambda r:r['forwarding_simulation'].pop('base'),
                           lambda r:r['forwarding_simulation']['head'].pop(),
                           lambda r:r['head']['files'].pop(suite.ELF),
                           lambda r:r['head']['files'].update({suite.DIRECTORY+'loops.S':'0'*64})):
                bad=copy.deepcopy(report);mutate(bad);compute.write(bundle/'report.json',bad)
                manifest['files']['report.json']=suite.digest(bundle/'report.json')
                compute.write(bundle/'bundle.json',manifest)
                with self.assertRaises((ValueError,KeyError)):
                    site.validate_bundle(bundle,fixtures.SHA)


if __name__ == '__main__':
    unittest.main()
