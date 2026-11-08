#!/usr/bin/env python3
"""Native reference, archive boundaries, and local evidence import regressions."""
import copy
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import accuracy_test_support as fixtures
import accuracy_site as site
import compute_usertime as compute
import macho_reference as macho
import streamcluster_reference as repeated
import performance_reference as performance
import publish_accuracy as publisher

ROOT = Path(__file__).resolve().parents[1]


class NativeReferenceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.temp.cleanup)
        cls.fixture = fixtures.Evidence(ROOT, Path(cls.temp.name))
        cls.declared, cls.reference = performance.load(cls.fixture.source, require_native=True)

    def test_committed_capture_coverage_and_quality(self):
        declared, measured = performance.load(ROOT, require_native=True)
        self.assertEqual(len(measured['cases']), 264)
        self.assertEqual(len(measured['native_programs']), 44)
        self.assertEqual(measured['capture_evidence']['samples'], 5544)
        self.assertEqual({r['id'] for r in measured['cases'] if r['quality'] != 'usable'},
                         set())
        self.assertEqual(sum(r['quality']=='usable' for r in measured['cases']),264)
        self.assertEqual(sum(p['reserved_register_rename'] is not None for p in measured['native_programs'].values()),14)
        self.assertLess((ROOT/performance.PATH).stat().st_size,300000)

    def test_streamcluster_replacement_binding_and_unchanged_rows(self):
        declared, measured = performance.load(ROOT, require_native=True)
        replacement = measured['streamcluster_replacement']
        previous = json.loads(subprocess.check_output(['git','-C',str(ROOT),'show',
            replacement['base_commit']+':'+performance.PATH]))
        ids = set(replacement['case_ids'])
        self.assertEqual([r for r in measured['cases'] if r['id'] not in ids],
                         [r for r in previous['cases'] if r['id'] not in ids])
        mutations = [lambda r:r.pop('streamcluster_replacement'), lambda r:r.pop('historical_input_sources'),
            lambda r:r['streamcluster_replacement'].update(collection_dirty=False),
            lambda r:r['streamcluster_replacement'].update(samples=125),
            lambda r:r['streamcluster_replacement'].update(elf_sha256='0'*64),
            lambda r:r['streamcluster_replacement'].update(rows_sha256='0'*64),
            lambda r:r['streamcluster_replacement']['input_sources'].update({macho.REPEAT_PREFIX+'repeat.S':'0'*64}),
            lambda r:r['streamcluster_replacement'].update(case_ids=['streamcluster-1'])]
        for i,mutate in enumerate(mutations):
            bad=copy.deepcopy(measured);mutate(bad)
            with self.subTest(mutation=i),self.assertRaises(ValueError):performance.validate(bad,declared)

    def test_replacement_history_cannot_be_rehashed_to_hide_other_changes(self):
        _, measured = performance.load(ROOT, require_native=True)
        head = subprocess.check_output(['git','-C',str(ROOT),'rev-parse','HEAD'],text=True).strip()
        publisher.verify_replacement_source(ROOT, measured, head)
        bad = copy.deepcopy(measured)
        bad['historical_input_sources']['tests/benchmarks/build.sh'] = '0'*64
        with self.assertRaisesRegex(ValueError,'historical acquisition'):
            publisher.verify_replacement_source(ROOT,bad,head)
        bad = copy.deepcopy(measured)
        bad['cases'][0]['median_user_seconds'] *= 2
        ids = set(bad['streamcluster_replacement']['case_ids'])
        bad['streamcluster_replacement']['retained_rows_sha256'] = compute.object_digest([r for r in bad['cases'] if r['id'] not in ids])
        with self.assertRaisesRegex(ValueError,'historical acquisition'):
            publisher.verify_replacement_source(ROOT,bad,head)
        declared = performance.inventory(ROOT)
        for c in declared['cases']:
            origin = performance.reference_origin(measured,c)
            wanted = measured['streamcluster_replacement']['native_sha256'] if c['workload']=='streamcluster' else measured['native_programs'][c['workload']]['binary_sha256']
            self.assertEqual(origin['native_sha256'],wanted)

    def test_repeat_capture_rejects_missing_duplicate_and_mislabeled_trials(self):
        configs=[dict(id=str(i),n=i+1,k=16) for i in range(18)]
        inputs=dict(configs=configs,expected={str(i+1):dict(bytes=(i+1)*8,sha256='1'*64) for i in range(18)})
        def sample(c,t):
            return dict(id=c['id'],n=c['n'],k=16,trial=t,batch=t//7 if t>=0 else -1,exit=0,
                all_repetitions_match=True,output_block_sha256='1'*64,output_bytes=c['n']*8*16,
                unix_time=1767225610,user_seconds=.1,system_seconds=.01,wall_seconds=.12)
        raw=dict(configs=configs,started_at='2026-01-01T00:00:00+00:00',completed_at='2026-01-01T00:01:00+00:00',
                 samples=[sample(c,t) for c in configs for t in range(21)],warmups=[sample(c,-1) for c in configs])
        repeated.validate_samples(raw,inputs)
        mutations=[lambda r:r['samples'].pop(),lambda r:r['samples'].__setitem__(1,r['samples'][0]),
                   lambda r:r['warmups'][0].update(trial=0,batch=0),lambda r:r['configs'][0].update(k=32),
                   lambda r:r['samples'][0].update(output_block_sha256='0'*64),
                   lambda r:r['samples'][0].update(user_seconds=float('nan')),
                   lambda r:r['samples'][0].update(unix_time=0)]
        for i,mutate in enumerate(mutations):
            bad=copy.deepcopy(raw);mutate(bad)
            with self.subTest(mutation=i),self.assertRaises(ValueError):repeated.validate_samples(bad,inputs)

    def test_native_metadata_and_statistics_fail_closed(self):
        mutations = [lambda r:r.update(environment='virtualized'), lambda r:r.update(protocol=performance.PROTOCOL),
                     lambda r:r.update(page_size=4096), lambda r:r.update(executable_format='ELF'),
                     lambda r:r.update(startup_included=False), lambda r:r.update(collector_origin='git'),
                     lambda r:r['collector_sources'].pop('audit.py'), lambda r:r['capture_evidence'].update(samples=5543),
                     lambda r:r['native_programs'].pop('mvt'), lambda r:r['native_programs']['mvt'].update(source_sha256='0'*64),
                     lambda r:r['native_programs']['mvt'].update(reserved_register_rename=29),
                     lambda r:r['cases'].pop(), lambda r:r['cases'][0].update(quality='noisy'),
                     lambda r:r['cases'][0].update(batch_median_spread=1),
                     lambda r:r['cases'][0].update(batch_median_user_seconds=[float('nan')]*3)]
        for mutation in mutations:
            bad=copy.deepcopy(self.reference);mutation(bad)
            with self.subTest(mutation=mutation),self.assertRaises(ValueError):
                performance.validate(bad,self.declared)

    def test_vm_is_archivable_but_not_an_active_reference(self):
        with tempfile.TemporaryDirectory() as directory:
            old=fixtures.Evidence(ROOT,Path(directory),native=False)
            performance.load(old.source)
            with self.assertRaisesRegex(ValueError,'active accuracy requires'):
                performance.load(old.source,require_native=True)
            report=fixtures.compact_report(old)
            raw=Path(directory)/'report.json';compute.write(raw,report);raw.with_suffix('.md').write_text('Legacy archive')
            site.bundle(raw,old.source,Path(directory)/'bundle',fixtures.SHA)
            self.assertIn('Linux ARM64 VM',(Path(directory)/'bundle/index.html').read_text())

    def test_failed_report_without_reference_does_not_claim_vm_or_native(self):
        report=dict(schema=1,artifact_status="fail",accuracy_status="unverified",head_commit=fixtures.SHA,cases=[],error="Prepare failed")
        page=site.detail(report,dict(commit=fixtures.SHA),preview=True)
        self.assertIn("Reference environment is unavailable",page)
        self.assertNotIn("Linux ARM64 VM",page)
        self.assertNotIn("native macOS on Apple M2",page)

    def test_published_native_sources_do_not_invent_a_committed_collector(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);source=root/'source';shutil.copytree(self.fixture.source,source)
            capability=self.fixture.report['base_capability']
            for name,content in [('pkg/loader/elf.go',capability['loader_source']),('pkg/linuxuser/run.go',capability['runtime_source'])]:
                path=source/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_text(content)
            def git(*args):return subprocess.check_output(['git','-C',str(source),*args],text=True,stderr=subprocess.DEVNULL).strip()
            git('init');git('config','user.name','Unit');git('config','user.email','unit@example.invalid');git('add','.');git('commit','-m','Frozen inputs')
            collection=git('rev-parse','HEAD')
            measured=copy.deepcopy(self.reference);measured.update(collection_commit=collection,oracle_commit=collection)
            compute.write(source/performance.PATH,measured);git('add','.');git('commit','-m','Imported native evidence');head=git('rev-parse','HEAD')
            report=fixtures.compact_report(self.fixture)
            report.update(base_commit=collection,reference_sha256=site.digest(source/performance.PATH),reference={k:v for k,v in measured.items() if k not in ('cases','input_sources','collector_sources')})
            report['head']=dict(commit=head,dirty=False,files={n:site.digest(source/n) for n in git('ls-files').splitlines()})
            raw=root/'report.json';compute.write(raw,report);raw.with_suffix('.md').write_text('Native fixture')
            site.bundle(raw,source,root/'bundle',head)
            publisher.verify_source(source,root/'bundle',head)
            page=(root/'bundle/index.html').read_text()
            self.assertIn('native macOS on Apple M2',page)
            self.assertIn('Frozen ELF/computation source',page)
            self.assertNotIn('Linux ARM64 VM',page)
            self.assertNotIn('Exact native collection source',page)

    def test_local_import_checks_capture_census_and_output_before_export(self):
        # A small synthetic capture has the full transport census but no real
        # timing or executables. It is never installed as a repository reference.
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);evidence=root/'evidence';(evidence/'evidence').mkdir(parents=True)
            declared=self.declared
            old=copy.deepcopy(self.reference)
            # Use the legacy fixture as the source's existing ELF output oracle.
            legacy=fixtures.Evidence(ROOT,root/'legacy',native=False)
            old=compute.read(legacy.source/performance.PATH)
            original=json.dumps(old).encode();collection='a'*40
            raw=dict(schema=1,protocol='matched-macho-native-reference-v1',status='pass',completed_passes=21,
                     source_dirty=False,source_head=collection,old_reference_sha256=macho.hashlib.sha256(original).hexdigest(),
                     case_inventory_sha256=macho.hashlib.sha256(json.dumps(declared['cases'],sort_keys=True).encode()).hexdigest(),
                     started_at_utc='2026-01-01T00:00:00+00:00',completed_at_utc='2026-01-01T00:01:00+00:00',
                     environment='native',executable_format='Mach-O',page_size=16384,hardware='Apple M2',platform='macOS-unit',
                     instantaneous_frequency='unknown',metric='child_user_cpu_seconds',timer='wait4.ru_utime')
            audits=[];build={'workloads':{}};smoke=[];samples=[];warmups=[]
            for n in macho.COLLECTORS:(evidence/n).write_text('synthetic collector '+n)
            (evidence/'bin').mkdir();(evidence/'assembly').mkdir()
            for i,name in enumerate(sorted({c['workload'] for c in declared['cases']})):
                count=3 if i==0 else 4
                (evidence/f'bin/{name}-matched').write_bytes(b'synthetic')
                (evidence/f'assembly/{name}-matched.S').write_text('synthetic')
                (evidence/f'assembly/{name}-linux.s').write_text('\n'.join(f'.type f{x}, %function' for x in range(count)))
                build['workloads'][name]=dict(source='/frozen/'+macho.source_name(name),source_sha256=old['input_sources'][macho.source_name(name)],
                    binary_sha256=macho.digest(evidence/f'bin/{name}-matched'),matched_assembly_sha256=macho.digest(evidence/f'assembly/{name}-matched.S'),reserved_register_rename=None)
                audits.extend(dict(workload=name,function=f'f{x}',elf_count=1,mac_count=1,diffs=[],register_renamed_instructions=0,pass_=True) for x in range(count))
            for a in audits:a['pass']=a.pop('pass_')
            for row in old['cases']:
                smoke.append(dict(id=row['id'],pass_=True,exit=0,stderr='',sha256=row['output_sha256'],bytes=row['output_bytes']))
                def sample(t):return dict(id=row['id'],trial=t,batch=t//7 if t>=0 else -1,exit=0,output_sha256=row['output_sha256'],output_bytes=row['output_bytes'],user_seconds=.001,system_seconds=.0001,wall_seconds=.002,unix_time=1767225610)
                samples.extend(sample(t) for t in range(21));warmups.append(sample(-1))
            for name,data in [('build',build),('instruction-audit',audits),('smoke',smoke)]:compute.write(evidence/f'evidence/{name}.json',data)
            raw.update(samples=samples,warmups=warmups,build_manifest_sha256=macho.digest(evidence/'evidence/build.json'),audit_sha256=macho.digest(evidence/'evidence/instruction-audit.json'),collector_sha256=macho.digest(evidence/'capture.py'))
            def git_read(args,**kwargs):
                if 'rev-parse' in args:return collection+'\n'
                name=args[-1].partition(':')[2]
                return original if name==performance.PATH else (legacy.source/name).read_bytes()
            def run_capture(data,target):
                compute.write(evidence/'capture.json',data)
                with patch.object(macho.subprocess,'check_output',side_effect=git_read),patch.object(performance,'require_collection_ancestor'):
                    return macho.export(legacy.source,evidence,target)
            result=run_capture(raw,root/'good.json');self.assertEqual(len(result['cases']),264)
            mutations=[lambda d:d['samples'].pop(),lambda d:d['samples'].__setitem__(1,d['samples'][0]),
                       lambda d:d['warmups'].pop(),lambda d:d['samples'][0].update(output_sha256='0'*64),
                       lambda d:d['samples'][0].update(unix_time=0),lambda d:d.update(audit_sha256='0'*64)]
            for i,mutate in enumerate(mutations):
                bad=copy.deepcopy(raw);mutate(bad)
                with self.subTest(mutation=i),self.assertRaises(ValueError):run_capture(bad,root/f'bad{i}.json')


if __name__=='__main__':unittest.main()
