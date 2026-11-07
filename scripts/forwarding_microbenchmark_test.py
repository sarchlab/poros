#!/usr/bin/env python3
"""Reject incomplete/misattributed dependency-probe evidence."""
import copy
import json
from pathlib import Path
import shutil
import tempfile
import unittest

import forwarding_microbenchmark as suite


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


if __name__ == '__main__':
    unittest.main()
