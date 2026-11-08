#!/usr/bin/env python3
"""Memory diagnostic contracts; synthetic timings below are not hardware evidence."""
import base64
import copy
import hashlib
import json
from pathlib import Path
import struct
import subprocess
import tempfile
import unittest

import memory_microbenchmark as memory
import accuracy_parallel as parallel
import accuracy_site as site
import performance_compare as comparison
import model_profiles


class MemoryTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cases = memory.cases()
        cls.reference = memory.load_reference()
        cls.profile = memory.read(memory.ROOT / model_profiles.PATH)

    def test_matrix_residency_and_load_counts(self):
        self.assertEqual(len(self.cases), 140)
        for size in (8192, 131072, 16777216, 67108864):
            rows = [c for c in self.cases if c['working_set_bytes'] == size]
            self.assertEqual(len(rows), 8)
            for case in rows[:5]:
                self.assertEqual(case['operations_per_pass'], size // 128)
                self.assertEqual((size // 128 // case['chains']) * case['chains'], size // 128)
            self.assertEqual(rows[-1]['effective_bytes_per_pass'], size)
            self.assertEqual(rows[-1]['operations_per_pass'] * 16, size)
        for case in self.cases:
            self.assertGreaterEqual(case['passes'] * (case['operations_per_pass'] if case['kind'] <= 5 else case['effective_bytes_per_pass']),
                                    65536 if case['kind'] <= 5 else 1048576)

    def test_expanded_matrix_validation_rejects_superseded_definition(self):
        definition = memory.read(memory.ROOT / memory.DEFINITION)
        self.assertEqual(sum(c['kind'] <= 5 for c in self.cases), 110)
        self.assertEqual(sum(c['kind'] > 5 for c in self.cases), 30)
        for case in self.cases:
            if case['working_set_bytes'] in (98304, 163840, 196608):
                self.assertEqual(case['passes'] & (case['passes'] - 1), 0)
                self.assertGreaterEqual(case['passes'] * case['operations_per_pass'], 65536)
            if case['working_set_bytes'] == 268435456:
                self.assertEqual(case['elf'], memory.LARGE_ELF)
                self.assertEqual(case['max_image_bytes'], 536870912)
        for mutate in (lambda d: d['chase_working_set_bytes'].append(98304),
                       lambda d: d['chase_working_set_bytes'].__setitem__(0, True),
                       lambda d: d['chase_working_set_bytes'].__setitem__(0, 8193),
                       lambda d: d['chase_working_set_bytes'].append(536870912),
                       lambda d: d['stream_working_set_bytes'].append(134217728),
                       lambda d: d['patterns'][0].update(chains=True)):
            bad = copy.deepcopy(definition); mutate(bad)
            with self.assertRaises(ValueError): memory.parse_cases(bad)
        old = dict(schema=1, working_set_bytes=definition['stream_working_set_bytes'], patterns=definition['patterns'])
        with self.assertRaises(ValueError): memory.parse_cases(old)

    def test_only_large_base_may_be_explicitly_unavailable(self):
        plan, shards = self.plan_and_shards()
        result = parallel.recorded_memory(plan, shards, 1)
        self.assertEqual(sum(r.get('status') == memory.UNSUPPORTED_IMAGE for r in result['base']), 5)
        self.assertTrue(all('cycles' in r for r in result['head']))
        for capability in (None, 0, 'false', True):
            with self.assertRaises(ValueError): memory.validate_comparison(result, self.cases, capability)
        for role, index in (('head', -1), ('base', 0)):
            bad = copy.deepcopy(result); bad[role][index] = memory.unavailable(self.cases[index])
            with self.assertRaises(ValueError): memory.validate_comparison(bad, self.cases, False)
        # A base that advertises support must supply genuine executions too.
        supported = copy.deepcopy(plan); supported['memory']['base_large_image_support'] = True
        updated = copy.deepcopy(shards)
        for shard in updated: shard['plan_id'] = parallel.identity(supported)
        with self.assertRaisesRegex(ValueError, 'role mismatch'):
            parallel.recorded_memory(supported, updated, 1)

    def test_source_bound_capacity_flag_probe(self):
        # Isolated Git history keeps this test independent of shallow CI clones.
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.DEVNULL, text=True).strip()
            git('init'); git('config', 'user.name', 'Test'); git('config', 'user.email', 'test@example.org')
            source = root / 'cmd/poros/elf.go'; source.parent.mkdir(parents=True)
            source.write_text('package main\n// max-image-bytes mentioned without flag registration\n')
            git('add', '.'); git('commit', '-m', 'old binary interface')
            old = git('rev-parse', 'HEAD')
            source.write_text('package main\nfunc flags() { fs.Uint64("max-image-bytes", 268435456, "budget") }\n')
            git('add', '.'); git('commit', '-m', 'bounded image option')
            current = git('rev-parse', 'HEAD')
            self.assertFalse(memory.supports_large_images(root, old))
            self.assertTrue(memory.supports_large_images(root, current))

    def test_image_budget_metadata_checks_default_and_large_executions(self):
        for case in (self.cases[0], self.cases[-1]):
            valid = self.sample(case, 1, 'head')
            memory.validate_execution(valid, case, case['passes'], 'f' * 64)
            changed = copy.deepcopy(valid)
            changed['max_image_bytes'] = 536870912 if case['max_image_bytes'] == 268435456 else 268435456
            with self.assertRaisesRegex(ValueError, 'capacity provenance'):
                memory.validate_execution(changed, case, case['passes'], 'f' * 64)
            del valid['max_image_bytes']
            if case['max_image_bytes'] == 268435456:
                memory.validate_execution(valid, case, case['passes'], 'f' * 64)  # Older base JSON.
            else:
                with self.assertRaisesRegex(ValueError, 'capacity provenance'):
                    memory.validate_execution(valid, case, case['passes'], 'f' * 64)

    def test_random_cycles_cover_every_node_without_overlap(self):
        for size in (8192, 131072, 1048576):
            permutation = memory.permutation(size)
            self.assertEqual(sorted(permutation), list(range(size // 128)))
            for chains in (1, 2, 4, 8, 16):
                length = len(permutation) // chains
                visited = set()
                for c in range(chains):
                    cycle = permutation[c * length:(c + 1) * length]
                    links = dict(zip(cycle, cycle[1:] + cycle[:1]))
                    node = cycle[0]
                    for _ in range(length):
                        self.assertNotIn(node, visited)
                        visited.add(node)
                        node = links[node]
                    self.assertEqual(node, cycle[0])
                self.assertEqual(len(visited), len(permutation))

    def test_independent_checksums_and_unsigned_read_wrap(self):
        self.assertEqual([memory.expected(c, 1) for c in self.cases[:5]], [4608, 4864, 11520, 29440, 67072])
        case = next(c for c in self.cases if c['id'] == 'read-67108864')
        count, passes = 67108864 // 8, 16777216
        self.assertEqual(memory.expected(case, passes), (count * (count + 1) // 2 * passes) % 2**64)
        self.assertNotEqual(memory.expected(case, passes), count * (count + 1) // 2 * passes)

    def test_native_reference_coverage_noise_and_source_binding(self):
        self.assertEqual(len(self.reference['cases']), 140)
        self.assertEqual(set(self.reference['instruction_audit']), set(memory.FUNCTIONS))
        self.assertLess((memory.ROOT / memory.REFERENCE).stat().st_size, 200000)
        for row in self.reference['cases']:
            quality = 'usable' if row['median_seconds'] >= .02 and row['relative_spread'] <= .10 else 'noisy'
            self.assertEqual(row['quality'], quality)
        mutations = [lambda r: r['cases'].pop(), lambda r: r['cases'].__setitem__(1, r['cases'][0]),
                     lambda r: r['cases'][0].update(median_seconds=float('nan')),
                     lambda r: r['cases'][0].update(relative_spread=1),
                     lambda r: r['cases'][0].update(native_passes=3),
                     lambda r: r['cases'][0].update(raw_samples_sha256='bad'),
                     lambda r: r['instruction_audit'].pop('chase16'),
                     lambda r: r['sources'].pop(memory.ELF),
                     lambda r: r.update(definition='{}'), lambda r: r.update(warmup_passes=0),
                     lambda r: r.update(instantaneous_frequency=3490000000)]
        for i, mutate in enumerate(mutations):
            bad = copy.deepcopy(self.reference); mutate(bad)
            with self.subTest(mutation=i), self.assertRaises((ValueError, KeyError)):
                memory.validate_reference(bad)
        files = {n: memory.digest(memory.ROOT / n) for n in [*memory.MEASUREMENT_INPUTS, memory.ELF, memory.LARGE_ELF]}
        for name in files:
            bad = dict(files); bad[name] = '0' * 64
            with self.subTest(source=name), self.assertRaises(ValueError):
                memory.validate_reference(self.reference, bad)

    def test_quality_rule_does_not_drop_noisy_samples(self):
        good = memory.distribution([.03] * 9)
        self.assertEqual(good['quality'], 'usable')
        noisy = memory.distribution([.03] * 6 + [.05] * 3)
        self.assertEqual(noisy['quality'], 'noisy')
        self.assertGreater(noisy['p90_seconds'], .04)
        self.assertEqual(memory.distribution([.001] * 9)['quality'], 'noisy')
        for values in ([.03] * 8, [.03] * 8 + [float('nan')], [.03] * 8 + [-1]):
            with self.assertRaises(ValueError): memory.distribution(values)

    def test_exact_function_bytes_match_reference_audit(self):
        bodies = memory.symbol_bytes(memory.ROOT / memory.ELF, memory.FUNCTIONS)
        self.assertEqual({n: dict(bytes=len(b), sha256=hashlib.sha256(b).hexdigest()) for n, b in bodies.items()},
                         self.reference['instruction_audit'])
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'changed.elf'
            raw = (memory.ROOT / memory.ELF).read_bytes()
            body = bodies['chase1']
            position = raw.index(body)
            path.write_bytes(raw[:position] + bytes([raw[position] ^ 1]) + raw[position + 1:])
            with self.assertRaisesRegex(ValueError, 'instruction bytes differ'):
                memory.audit_binaries(memory.ROOT / memory.ELF, path)

    def sample(self, case, factor, role):
        passes = case['passes'] * factor
        instructions = 1000 + passes * case['instructions_per_pass']
        cycles = 2000 + instructions
        return dict(termination='exit', exit_status=0, stderr_base64='', binary_sha256='f' * 64, max_image_bytes=case.get('max_image_bytes', 256 * 1024 * 1024),
                    args=['program', *memory.arguments(case, passes)], env=[], instructions=instructions,
                    stdout_base64=base64.b64encode(struct.pack('<Q', memory.expected(case, passes))).decode(),
                    build={'vcs.revision': ('a' if role == 'head' else 'b') * 40, 'vcs.modified': 'false'},
                    timing=dict(model='ooo', user_cycles=cycles, retired_instructions=instructions,
                                assumed_hz=3490000000, predicted_user_seconds=cycles / 3490000000, **self.profile))

    def plan_and_shards(self, count=3):
        plan = dict(schema=1, run_id='synthetic-memory-test', prepared_attempt=1,
                    head=dict(commit='a' * 40), base_commit='b' * 40,
                    model_profiles={role: dict(profile=self.profile) for role in ('head', 'base')},
                    memory=dict(elfs={memory.ELF: 'f' * 64, memory.LARGE_ELF: 'f' * 64}, base_large_image_support=False, cases=self.cases, shards=parallel.partition(self.cases, count)))
        cases = {c['id']: c for c in self.cases}
        shards = [dict(schema=1, run_id=plan['run_id'], plan_id=parallel.identity(plan), attempt=1, shard=i,
                       status='pass', errors=[], completed_at=2,
                       memory_records=[dict(case_id=ident, executions={role: [self.sample(cases[ident], factor, role) for factor in (1, 2)]
                                                                      for role in parallel.memory_roles(plan, cases[ident])}) for ident in ids])
                  for i, ids in enumerate(plan['memory']['shards'])]
        return plan, shards

    def test_interleaved_memory_shards_restore_canonical_order(self):
        plan, shards = self.plan_and_shards()
        result = parallel.recorded_memory(plan, list(reversed(shards)), 1)
        for role in ('head', 'base'):
            self.assertEqual([r['id'] for r in result[role]], [c['id'] for c in self.cases])
        memory.validate_comparison(result, self.cases, False)

    def test_transport_rejects_missing_duplicate_and_tampered_evidence(self):
        plan, shards = self.plan_and_shards()
        mutations = [lambda r: r.pop(), lambda r: r.append(r[0]),
                     lambda r: r[0].update(plan_id='0' * 64), lambda r: r[0].update(attempt=2),
                     lambda r: r[0].update(status='fail'), lambda r: r[0]['memory_records'].pop(),
                     lambda r: r[0]['memory_records'][0]['executions'].pop('base'),
                     lambda r: r[0]['memory_records'][0]['executions']['head'].pop(),
                     lambda r: r[0]['memory_records'][0]['executions']['head'][0]['build'].update({'vcs.modified': 'true'}),
                     lambda r: r[0]['memory_records'][0]['executions']['head'][0].update(binary_sha256='0' * 64),
                     lambda r: r[0]['memory_records'][0]['executions']['head'][0].update(stdout_base64='AAAAAAAAAAA='),
                     lambda r: r[0]['memory_records'][0]['executions']['head'][1]['timing'].update(user_cycles=1)]
        for i, mutate in enumerate(mutations):
            bad = copy.deepcopy(shards); mutate(bad)
            with self.subTest(mutation=i), self.assertRaises((ValueError, KeyError)):
                parallel.recorded_memory(plan, bad, 1)
        bad = copy.deepcopy(shards[0]); bad.update(attempt=2, status='fail')
        with self.assertRaises(ValueError): parallel.recorded_memory(plan, shards + [bad], 2)

    def test_renderer_uses_working_sets_units_tooltips_and_collapsed_table(self):
        plan, shards = self.plan_and_shards()
        result = parallel.recorded_memory(plan, shards, 1)
        graphs, table = site.memory_panels(self.reference, result, {role: self.profile for role in ('head', 'base')}, False)
        self.assertEqual(graphs.count('<figure>'), 8)
        self.assertEqual(graphs.count('class="variability-range"'), 140)
        self.assertIn('Unsupported image capacity', table)
        for term in ('8 KiB', '64 MiB', 'Dependent ns/load', 'Aggregate ns/load', 'Effective GB/s', 'data-tip=', 'P10–P90', 'Native quality'):
            self.assertIn(term, graphs)
        self.assertIn('<details id="memory-diagnostics-table">', table)
        self.assertNotIn('<details open', table)
        self.assertNotIn('<table', graphs)
        before = comparison.benchmark_summary(dict(artifact_status='pass', schema=4, cases=[]))
        after = comparison.benchmark_summary(dict(artifact_status='pass', schema=4, cases=[], memory_simulation=result))
        self.assertEqual(before, after)
        with self.assertRaises(ValueError): site.memory_panels(self.reference, {'head': result['head']}, {'head': self.profile, 'base': self.profile}, False)


if __name__ == '__main__':
    unittest.main()
