#!/usr/bin/env python3
"""Parallel diagnostic transport tests; synthetic counters are not timing evidence."""
import base64
import contextlib
import copy
import io
import hashlib
from pathlib import Path
import struct
import tempfile
import unittest
from unittest.mock import patch

import accuracy_parallel as parallel
import accuracy_progress as progress
import accuracy_site as site
import forwarding_microbenchmark as forwarding
import memory_microbenchmark as memory
import model_profiles as models
import probe_checkpoint as checkpoint
import throughput_microbenchmark as throughput
import usertime as user


def synthetic_execution(binary, root, case, index, n, mode, revision, profile, suite=throughput.THROUGHPUT):
    if suite is None:
        elf, checksum = forwarding.ELF, forwarding.expected(case, n)
    else:
        elf, checksum = suite.elf, suite.checksum(case, n)
    instructions = 20 + n * case['instructions_per_iteration']
    return dict(termination='exit', exit_status=0, stderr_base64='', binary_sha256=user.digest(root/elf),
        args=['program', str(index), str(n)], env=[], instructions=instructions,
        stdout_base64=base64.b64encode(struct.pack('<Q', checksum)).decode(),
        build={'vcs.revision': revision, 'vcs.modified': 'false'},
        timing=dict(retired_instructions=instructions, user_cycles=30+3*n, **copy.deepcopy(profile)))


class DiagnosticTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = parallel.ROOT
        cls.state = user.source_state()
        cls.state.update(commit='a'*40, dirty=False)
        cls.profile = parallel.read(cls.source/models.PATH)

    def setUp(self):
        temp = tempfile.TemporaryDirectory(); self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.prepared = self.root/'prepared'; self.prepared.mkdir()
        for role in ('head', 'base'):
            (self.prepared/role).write_text('synthetic nonexecutable '+role)
        base_profile = copy.deepcopy(self.profile)
        base_profile['configuration']['fetch_width'] = 1
        self.plan = dict(schema=1, run_id='test', prepared_attempt=1, head=copy.deepcopy(self.state),
            base_commit='b'*40, model_profiles={role: {'profile': p} for role,p in [('head',self.profile),('base',base_profile)]})
        parallel.write(self.prepared/'plan.json', self.plan)
        self.tasks = parallel.diagnostic_tasks(self.plan,self.source)
        self.saved = self.root/'diagnostics'
        self.calls = []
        def execute(*args, **kwargs):
            self.calls.append((str(args[0]), args[3], args[4]))
            return synthetic_execution(*args, **kwargs)
        self.patches = contextlib.ExitStack(); self.addCleanup(self.patches.close)
        self.patches.enter_context(patch.object(throughput,'execute',side_effect=execute))
        self.patches.enter_context(patch.object(forwarding,'execute',side_effect=lambda *a,**kw: execute(*a,suite=None,**kw)))
        self.patches.enter_context(patch.object(parallel,'load_plan',return_value=(self.plan,{})))

    def run_point(self, index):
        folder = self.saved/str(index)
        with contextlib.redirect_stdout(io.StringIO()):
            return parallel.run_diagnostic_point(self.prepared,'test',1,index,folder,folder/'diagnostic.json',source=self.source)

    def test_inventory_preserves_old_plan_and_source_bound_107_points_394_executions(self):
        before = (self.prepared/'plan.json').read_bytes()
        inventory = parallel.diagnostic_inventory(self.prepared,'test',1,self.source,self.root/'inventory.json')
        self.assertEqual((self.prepared/'plan.json').read_bytes(),before)
        self.assertEqual(inventory['plan_id'],parallel.identity(self.plan))
        self.assertEqual(inventory['head'],self.plan['head']['commit'])
        self.assertEqual([t['diagnostic_index'] for t in inventory['tasks']],list(range(107)))
        self.assertEqual(sum(len(t['roles'])*2 for t in self.tasks),394)
        self.assertEqual({name:sum(t['suite']==name for t in self.tasks) for name in ('throughput','forwarding','scheduling','capacity')},
                         dict(throughput=17,forwarding=13,scheduling=4,capacity=73))
        with patch.object(throughput,'cases',wraps=throughput.cases) as cases:
            self.assertEqual(len(parallel.diagnostic_tasks(self.plan,self.source)),107)
            self.assertTrue(all(call.args[0]==self.source for call in cases.call_args_list))
        archived=copy.deepcopy(self.plan);archived['head']['files']={}
        self.assertEqual(parallel.diagnostic_tasks(archived,self.source),[])

    def test_all_points_match_serial_measurement_and_resume_never_executes(self):
        # Deliberately finish in reverse order. Replay must restore reference order.
        for index in reversed(range(107)):
            self.assertEqual(self.run_point(index),0)
        self.assertEqual(len(self.calls),394)
        rows, states = parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved)
        self.assertTrue(all(s['state']=='complete' for s in states.values()))
        for name,suite in site.DIAGNOSTIC_SUITES.items():
            if name=='memory':continue
            for role in ('head',) if name=='throughput' else ('head','base'):
                expected=suite.measure(self.prepared/role,self.source,
                    self.plan['head']['commit'] if role=='head' else self.plan['base_commit'],
                    self.plan['model_profiles'][role]['profile'])
                self.assertEqual(rows[name] if name=='throughput' else rows[name][role],expected)
        with patch.object(throughput,'execute',side_effect=AssertionError('completed simulation rerun')), \
             patch.object(forwarding,'execute',side_effect=AssertionError('completed simulation rerun')):
            for index in (0,17,30,34,106):self.assertEqual(self.run_point(index),0)
            self.assertEqual(parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved)[0],rows)
        receipt=parallel.read(self.saved/'0/diagnostic.json')
        self.assertEqual(receipt['reporting']['commit'],user.source_state()['commit'])
        self.assertEqual(receipt['task'],self.tasks[0])

    def test_partial_role_single_sample_and_out_of_order_point(self):
        self.assertEqual(self.run_point(18),0)
        task=self.tasks[17];suite=forwarding
        saved=parallel.diagnostic_checkpoint(self.plan,self.prepared,self.saved/'17','forwarding','head')
        suite.measure_case(self.prepared/'head',self.source,task['case'],task['index'],
            saved.identity['revision'],saved.identity['profile'],checkpoint=saved)
        short=self.saved/'17'/saved.filename(task['index'],1024)
        long=self.saved/'17'/saved.filename(task['index'],2048)
        original=long.read_bytes();long.unlink()
        rows,states=parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved,partial=True)
        self.assertEqual([r['id'] for r in rows['forwarding']['head']],[self.tasks[18]['case']['id']])
        self.assertEqual(states['forwarding']['active'],[dict(role='head',id=task['case']['id'])])
        long.write_bytes(original)
        rows,_=parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved,partial=True)
        self.assertEqual([r['id'] for r in rows['forwarding']['head']],[t['case']['id'] for t in self.tasks[17:19]])
        self.assertEqual([r['id'] for r in rows['forwarding']['base']],[self.tasks[18]['case']['id']])
        with self.assertRaisesRegex(ValueError,'missing diagnostic checkpoint'):
            parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved)
        self.assertTrue(short.exists())

    def test_missing_corrupt_duplicate_foreign_keys_and_raw_validation_fail_closed(self):
        self.assertEqual(self.run_point(0),0)
        path=self.saved/'0/throughput-head-1-1024.json'
        original=parallel.read(path)
        def replay():
            return parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved,partial=True)
        for mutate in (lambda r:r.update(sha256='0'*64),
                       lambda r:r['key'].update(role='base'),
                       lambda r:r['key']['source'].update(commit='c'*40),
                       lambda r:r['key']['source'].update(files_sha256='0'*64),
                       lambda r:r['key'].update(binary_sha256='0'*64),
                       lambda r:r['key'].update(index=2),
                       lambda r:r['result'].update(stdout_base64='AA=='),
                       lambda r:r['result']['timing']['configuration'].update(fetch_width=999)):
            bad=copy.deepcopy(original);mutate(bad)
            if bad['sha256']!='0'*64:bad['sha256']=checkpoint.digest({'key':bad['key'],'result':bad['result']})
            parallel.write(path,bad)
            with self.assertRaises((ValueError,KeyError)):replay()
        parallel.write(path,original)
        duplicate=self.saved/'duplicate'/path.name
        parallel.write(duplicate,original)
        with self.assertRaisesRegex(ValueError,'duplicate'):replay()
        duplicate.unlink()
        unknown=self.saved/'unexpected.json';parallel.write(unknown,original)
        with self.assertRaisesRegex(ValueError,'unexpected'):replay()
        unknown.unlink();path.unlink()
        with self.assertRaisesRegex(ValueError,'missing'):
            parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved)
        with self.assertRaises(FileNotFoundError):
            saved=parallel.diagnostic_checkpoint(self.plan,self.prepared,self.saved/'0','throughput','head')
            saved.read(self.tasks[0]['case'],1,1024,lambda _:None)

    def test_directory_has_one_owner_and_conflicting_point_is_rejected(self):
        folder=self.saved/'0';folder.mkdir(parents=True)
        lock=folder/'.diagnostic.lock';lock.write_text('another worker')
        with self.assertRaises(FileExistsError):self.run_point(0)
        self.assertEqual(self.calls,[])
        lock.unlink();self.assertEqual(self.run_point(0),0)
        with contextlib.redirect_stdout(io.StringIO()):
            status=parallel.run_diagnostic_point(self.prepared,'test',1,1,folder,folder/'diagnostic.json',source=self.source)
        self.assertEqual(status,1)
        self.assertEqual(len(self.calls),2)

    def test_live_export_uses_raw_partial_roles_and_activity_with_511_point_denominator(self):
        cases,_=parallel.inputs(self.source)
        self.plan.update(cases=cases,shards=parallel.partition(cases,132),
            references={'performance264':user.digest(self.source/parallel.REFERENCE)},
            binaries={role:user.digest(self.prepared/role) for role in ('head','base')},
            base_capability=parallel.compute.base_capability('HEAD',self.source),
            memory=dict(cases=memory.cases(self.source),shards=[],base_large_image_support=True))
        for role,declaration in self.plan['model_profiles'].items():
            declaration.update(commit=self.plan['head']['commit'] if role=='head' else self.plan['base_commit'],
                path=models.PATH,sha256=hashlib.sha256(models.encode(declaration['profile'])).hexdigest())
        self.assertEqual(self.run_point(18),0)
        # An independently completed head role must be visible before its base.
        task=self.tasks[17]
        saved=parallel.diagnostic_checkpoint(self.plan,self.prepared,self.saved/'17','forwarding','head')
        forwarding.measure_case(self.prepared/'head',self.source,task['case'],task['index'],
            saved.identity['revision'],saved.identity['profile'],checkpoint=saved)
        status=self.root/'diagnostic-status.json'
        state=dict(head=self.plan['head']['commit'],base=self.plan['base_commit'],run_id='test',
            total=107,completed=[18],active={'17':'worker'},failed=[])
        parallel.write(status,state)
        exporter=progress.SnapshotExporter(self.prepared,self.source,self.root/'shards','test',1,
            checkpoints=self.saved,diagnostic_status=status)
        snapshot=exporter.export()
        report=snapshot['report']
        self.assertEqual(report['head'],self.plan['head'])
        self.assertEqual(report['reporting']['commit'],user.source_state()['commit'])
        self.assertEqual([r['id'] for r in report['forwarding_simulation']['head']],
            [t['case']['id'] for t in self.tasks[17:19]])
        self.assertEqual(report['diagnostic_progress']['forwarding']['active'],[dict(role='base',id=task['case']['id'])])
        references={name:suite.load_reference(self.source) for name,suite in site.DIAGNOSTIC_SUITES.items()}
        self.assertEqual(site.overall_progress(report,references,snapshot['progress']),
            dict(required=511,completed=1,active=1,remaining=509))
        for mutate in (lambda s:s.update(head='c'*40),lambda s:s.update(completed=[18,18]),
                       lambda s:s.update(active={'107':'worker'}),lambda s:s.update(active={'17':'a','017':'b'})):
            bad=copy.deepcopy(state);mutate(bad);parallel.write(status,bad)
            with self.assertRaises(ValueError):exporter.export()


if __name__=='__main__':unittest.main()
