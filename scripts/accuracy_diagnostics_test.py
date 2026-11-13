#!/usr/bin/env python3
"""Parallel diagnostic transport tests; synthetic counters are not timing evidence."""
import base64
import contextlib
import copy
import io
import hashlib
import fnmatch
from pathlib import Path
import struct
import tempfile
import unittest
from unittest.mock import patch

import accuracy_parallel as parallel
import accuracy_progress as progress
import accuracy_site as site
import accuracy_test_support as fixtures
import devices
import forwarding_microbenchmark as forwarding
import memory_microbenchmark as memory
import model_profiles as models
import probe_checkpoint as checkpoint
import recurrence_microbenchmark as recurrence
import throughput_microbenchmark as throughput
import usertime as user


def synthetic_execution(binary, root, case, index, n, mode, revision, profile, suite=throughput.THROUGHPUT, device=None):
    """A simulator given a target reports it; one that predates targets reports none."""
    if suite in (forwarding, recurrence):
        elf, checksum = suite.ELF, suite.expected(case, n)
    else:
        elf, checksum = suite.elf, suite.checksum(case, n)
    instructions = 20 + n * case['instructions_per_iteration']
    return dict(termination='exit', exit_status=0, stderr_base64='', binary_sha256=user.digest(root/elf),
        args=['program', str(index), str(n)], env=[], instructions=instructions,
        stdout_base64=base64.b64encode(struct.pack('<Q', checksum)).decode(),
        build={'vcs.revision': revision, 'vcs.modified': 'false'},
        timing=dict(retired_instructions=instructions, user_cycles=30+3*n, **copy.deepcopy(profile),
                    **({} if device is None else {'target': copy.deepcopy(device)})))


class DiagnosticTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = parallel.ROOT
        cls.state = user.source_state()
        cls.state.update(commit='a'*40, dirty=False)
        cls.profile = parallel.read(cls.source/models.PATH)
        # This source declares devices, so its plans name one. The base revision
        # here predates them: it is the default device and is run without a target.
        cls.device = fixtures.device_selection('m2', cls.state['commit'], cls.state['files'])

    def setUp(self):
        temp = tempfile.TemporaryDirectory(); self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.prepared = self.root/'prepared'; self.prepared.mkdir()
        for role in ('head', 'base'):
            (self.prepared/role).write_text('synthetic nonexecutable '+role)
        base_profile = copy.deepcopy(self.profile)
        base_profile['configuration']['fetch_width'] = 1
        self.plan = dict(schema=1, run_id='test', prepared_attempt=1, head=copy.deepcopy(self.state),
            base_commit='b'*40, model_profiles={role: {'profile': p} for role,p in [('head',self.profile),('base',base_profile)]},
            device=copy.deepcopy(self.device))
        parallel.write(self.prepared/'plan.json', self.plan)
        self.tasks = parallel.diagnostic_tasks(self.plan,self.source)
        self.saved = self.root/'diagnostics'
        self.calls = []
        def execute(*args, **kwargs):
            self.calls.append((str(args[0]), args[3], args[4]))
            return synthetic_execution(*args, **kwargs)
        self.patches = contextlib.ExitStack(); self.addCleanup(self.patches.close)
        self.patches.enter_context(patch.object(throughput,'execute',side_effect=execute))
        self.patches.enter_context(patch.object(forwarding,'execute',side_effect=lambda *a,**kw: execute(*a,suite=forwarding,**kw)))
        self.patches.enter_context(patch.object(recurrence,'execute',side_effect=lambda *a,**kw: execute(*a,suite=recurrence,**kw)))
        self.patches.enter_context(patch.object(parallel,'load_plan',return_value=(self.plan,{})))

    def run_point(self, index, attempt=1):
        folder = self.saved/str(index)
        with contextlib.redirect_stdout(io.StringIO()):
            return parallel.run_diagnostic_point(self.prepared,'test',attempt,index,folder,folder/'diagnostic.json',source=self.source)

    def test_inventory_preserves_old_plan_and_source_bound_148_points_558_executions(self):
        before = (self.prepared/'plan.json').read_bytes()
        inventory = parallel.diagnostic_inventory(self.prepared,'test',1,self.source,self.root/'inventory.json')
        self.assertEqual((self.prepared/'plan.json').read_bytes(),before)
        self.assertEqual(inventory['plan_id'],parallel.identity(self.plan))
        self.assertEqual(inventory['head'],self.plan['head']['commit'])
        self.assertEqual([t['diagnostic_index'] for t in inventory['tasks']],list(range(148)))
        self.assertEqual(sum(len(t['roles'])*2 for t in self.tasks),558)
        self.assertEqual({name:sum(t['suite']==name for t in self.tasks) for name in ('throughput','forwarding','scheduling','capacity','recurrence')},
                         dict(throughput=17,forwarding=13,scheduling=4,capacity=73,recurrence=41))
        with patch.object(throughput,'cases',wraps=throughput.cases) as cases:
            self.assertEqual(len(parallel.diagnostic_tasks(self.plan,self.source)),148)
            self.assertTrue(all(call.args[0]==self.source for call in cases.call_args_list))
        archived=copy.deepcopy(self.plan);archived['head']['files']={}
        self.assertEqual(parallel.diagnostic_tasks(archived,self.source),[])

    def test_all_points_match_serial_measurement_and_resume_never_executes(self):
        # Deliberately finish in reverse order. Replay must restore reference order.
        for index in reversed(range(148)):
            self.assertEqual(self.run_point(index),0)
        self.assertEqual(len(self.calls),558)
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
             patch.object(forwarding,'execute',side_effect=AssertionError('completed simulation rerun')), \
             patch.object(recurrence,'execute',side_effect=AssertionError('completed simulation rerun')):
            for index in (0,17,30,34,106,107,147):self.assertEqual(self.run_point(index),0)
            self.assertEqual(parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved)[0],rows)
        receipt=parallel.read(self.saved/'0/diagnostic.json')
        self.assertEqual(receipt['reporting']['commit'],user.source_state()['commit'])
        self.assertEqual(receipt['task'],self.tasks[0])

    def test_partial_role_single_sample_and_out_of_order_point(self):
        self.assertEqual(self.run_point(18),0)
        task=self.tasks[17];suite=forwarding
        saved=parallel.diagnostic_checkpoint(self.plan,self.prepared,self.saved/'17','forwarding','head')
        suite.measure_case(self.prepared/'head',self.source,task['case'],task['index'],
            saved.identity['revision'],saved.identity['profile'],checkpoint=saved,**devices.options(self.plan,'head'))
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

    def test_unmeasured_device_runs_every_point_on_head_and_keeps_its_evidence_apart(self):
        self.assertEqual(self.run_point(1),0)  # Evidence of the default device, for later.
        default_device=devices.identity(self.plan,'head')
        # load_plan returns this same object, so the campaign is now M1's.
        self.plan['device']=fixtures.device_selection('m1',self.plan['head']['commit'],self.plan['head']['files'])
        self.calls.clear()
        with contextlib.ExitStack() as stack:
            for suite in site.DIAGNOSTIC_SUITES.values():
                stack.enter_context(patch.object(suite,'load_reference',
                    side_effect=AssertionError('read a native reference this device does not have')))
            tasks=parallel.diagnostic_tasks(self.plan,self.source)
            self.assertEqual(len(tasks),148)  # Every suite still runs.
            self.assertTrue(all(task['roles']==['head'] for task in tasks))  # The base cannot model M1.
            self.saved=self.root/'m1'
            for index in (0,17,147):self.assertEqual(self.run_point(index),0)
            rows,states=parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved,partial=True)
        self.assertEqual(len(self.calls),6)
        self.assertEqual({binary for binary,_,_ in self.calls},{str(self.prepared/'head')})
        self.assertEqual([r['id'] for r in rows['throughput']],[tasks[0]['case']['id']])
        self.assertEqual([r['id'] for r in rows['forwarding']['head']],[tasks[17]['case']['id']])
        self.assertEqual(rows['forwarding']['base'],[])
        saved=parallel.read(self.saved/'0/throughput-head-1-1024.json')
        self.assertEqual(saved['key']['device']['name'],'m1')
        self.assertEqual(saved['result']['timing']['target'],devices.identity(self.plan,'head'))
        # A simulator that answers as another device does not satisfy the point.
        with patch.object(throughput,'execute',side_effect=lambda *a,**kw:synthetic_execution(*a,**dict(kw,device=default_device))):
            self.assertEqual(self.run_point(2),1)
        self.assertIn('simulated device mismatch',parallel.read(self.saved/'2/diagnostic.json')['errors'][0])
        # The default device's evidence for the same point is neither reused nor replayed.
        import shutil
        shutil.copytree(self.root/'diagnostics/1',self.saved/'1')
        self.assertEqual(self.run_point(1),1)
        self.assertEqual(len(self.calls),6)
        with self.assertRaisesRegex(ValueError,'checkpoint identity'):
            parallel.recorded_diagnostics(self.plan,self.prepared,self.source,self.saved,partial=True)

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
        receipt=folder/'diagnostic.json'
        original=receipt.read_bytes()
        with contextlib.redirect_stdout(io.StringIO()):
            status=parallel.run_diagnostic_point(self.prepared,'test',1,1,folder,folder/'diagnostic.json',source=self.source)
        self.assertEqual(status,1)
        self.assertEqual(len(self.calls),2)
        self.assertEqual(receipt.read_bytes(),original)
        self.assertEqual(parallel.read(receipt)['status'],'pass')
        self.assertFalse(lock.exists())
        # A failed owner's receipt still owns the directory even without raw files.
        for path in folder.glob('throughput-*.json'):path.unlink()
        with contextlib.redirect_stdout(io.StringIO()):
            status=parallel.run_diagnostic_point(self.prepared,'test',1,1,folder,receipt,source=self.source)
        self.assertEqual(status,1)
        self.assertEqual(receipt.read_bytes(),original)
        self.assertEqual(len(self.calls),2)
        self.assertFalse(lock.exists())

    def test_missing_checkpoint_argument_is_explicit_and_undeclared_source_remains_compatible(self):
        with self.assertRaisesRegex(ValueError,'requires --checkpoints with recorded diagnostic evidence'):
            parallel.recorded_diagnostics(self.plan,self.prepared,self.source,None)
        archived=copy.deepcopy(self.plan);archived['head']['files']={}
        self.assertEqual(parallel.recorded_diagnostics(archived,self.prepared,self.source,None),({},{}))

    def test_failed_job_retry_excludes_partial_artifact_and_reuses_prior_successes(self):
        # Exercise the hosted artifact naming a retry depended on, without GitHub.
        plan_id=parallel.identity(self.plan)
        tasks=self.tasks[:2]
        with patch.object(parallel,'diagnostic_tasks',return_value=tasks), \
             patch.object(throughput,'cases',return_value=[t['case'] for t in tasks]):
            self.assertEqual(self.run_point(0),0)
            task=tasks[1]
            saved=parallel.diagnostic_checkpoint(self.plan,self.prepared,self.saved/'1','throughput','head')
            n=1024
            row=synthetic_execution(self.prepared/'head',self.source,task['case'],task['index'],n,'ooo',
                saved.identity['revision'],saved.identity['profile'])
            saved(task['case'],task['index'],n,lambda:row,
                lambda r:throughput.validate_execution(r,self.source,task['case'],task['index'],n,'ooo',
                    saved.identity['revision'],saved.identity['profile']))
            partial=list((self.saved/'1').glob('*.json'))
            self.assertEqual(len(partial),1)
            # The failed attempt uploads only status under a non-matching name.
            failed_status=self.root/'failed-status'
            parallel.write(failed_status/'diagnostic.json',dict(task=task,attempt=1,status='fail'))
            artifacts={f'accuracy-diagnostic-{plan_id}-0-1':self.saved/'0',
                       f'accuracy-diagnostic-status-{plan_id}-1-1':failed_status}
            for path in partial:path.unlink() # A fresh CI runner for the retry.
            self.assertEqual(self.run_point(1,attempt=2),0)
            artifacts[f'accuracy-diagnostic-{plan_id}-1-2']=self.saved/'1'
            self.assertEqual(parallel.read(self.saved/'0/diagnostic.json')['attempt'],1)
            self.assertEqual(parallel.read(self.saved/'1/diagnostic.json')['attempt'],2)
            downloaded=self.root/'downloaded'
            import shutil
            for name,folder in artifacts.items():
                if fnmatch.fnmatchcase(name,f'accuracy-diagnostic-{plan_id}-*'):
                    shutil.copytree(folder,downloaded/name)
            with patch.object(throughput,'execute',side_effect=AssertionError('aggregate reran diagnostics')):
                rows,_=parallel.recorded_diagnostics(self.plan,self.prepared,self.source,downloaded)
            self.assertEqual([r['id'] for r in rows['throughput']],[t['case']['id'] for t in tasks])
            self.assertEqual(len(self.calls),4) # Point 0 was not rerun at attempt 2.

    def test_live_export_uses_raw_partial_roles_and_activity_with_552_point_denominator(self):
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
            saved.identity['revision'],saved.identity['profile'],checkpoint=saved,**devices.options(self.plan,'head'))
        status=self.root/'diagnostic-status.json'
        self.assertIn('device',self.plan)
        state=dict(head=self.plan['head']['commit'],base=self.plan['base_commit'],run_id='test',
            plan_id=parallel.identity(self.plan),total=148,completed=[18],active={'17':'worker'},failed=[])
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
            dict(required=552,completed=1,active=1,remaining=550))
        for mutate in (lambda s:s.update(head='c'*40),lambda s:s.update(completed=[18,18]),
                       lambda s:s.update(active={'148':'worker'}),lambda s:s.update(active={'17':'a','017':'b'})):
            bad=copy.deepcopy(state);mutate(bad);parallel.write(status,bad)
            with self.assertRaises(ValueError):exporter.export()
        # The plan names a device, so head, base and run id are not enough to own a status file.
        for mutate,reason in ((lambda s:s.pop('plan_id'),'diagnostic status has no plan_id'),
                              (lambda s:s.update(plan_id='0'*64),'diagnostic status belongs to another prepared plan')):
            bad=copy.deepcopy(state);mutate(bad);parallel.write(status,bad)
            with self.assertRaisesRegex(ValueError,reason):exporter.export()


if __name__=='__main__':unittest.main()
