#!/usr/bin/env python3
"""Resume uses validated raw probe executions, never fabricated compact counters."""
import base64
import copy
from pathlib import Path
import struct
import tempfile
import unittest
from unittest.mock import patch

import probe_checkpoint as checkpoint
import throughput_microbenchmark as suite


class CheckpointTests(unittest.TestCase):
    def test_resume_and_fail_closed_identity_raw_output_and_counts(self):
        root = suite.ROOT
        case = suite.cases(root)[0]
        with tempfile.TemporaryDirectory() as temporary:
            folder = Path(temporary)
            binary = folder / 'binary'; binary.write_bytes(b'synthetic test binary')
            saved = checkpoint.Checkpoint(folder / 'saved', 'throughput', 'head', {'commit': 'a'*40}, binary, 'a'*40, {}, suite.digest(root / suite.ELF))
            calls = []
            def execute(binary, root, case, index, n, mode, revision, profile, spec):
                calls.append(n)
                return dict(termination='exit', exit_status=0, stderr_base64='', binary_sha256=suite.digest(root / suite.ELF),
                    args=['program', str(index), str(n)], env=[], instructions=10+n*case['instructions_per_iteration'],
                    stdout_base64=base64.b64encode(struct.pack('<Q', spec.checksum(case,n))).decode(),
                    build={'vcs.revision': revision, 'vcs.modified': 'false'},
                    timing=dict(retired_instructions=10+n*case['instructions_per_iteration'], user_cycles=20+2*n,
                                configuration={}, cache_config={}))
            progress = []
            profile = {'configuration': {}, 'cache_config': {}}
            with patch.object(suite, 'cases', return_value=[case]), patch.object(suite, 'execute', side_effect=execute):
                first = suite.measure(binary, root, 'a'*40, profile, checkpoint=saved, progress=lambda rows, active: progress.append((copy.deepcopy(rows),active)))
                again = suite.measure(binary, root, 'a'*40, profile, checkpoint=saved)
            self.assertEqual(first, again)
            self.assertEqual(calls, list(suite.ITERATIONS))
            self.assertEqual(progress[0], ([],case['id']))
            self.assertEqual(progress[-1], (first,None))
            path = next((folder/'saved').glob('*.json'))
            original = checkpoint.progress.site.read(path)
            for mutate in (lambda r: r['key'].update(iterations=3),
                           lambda r: r['result'].update(stdout_base64='AA=='),
                           lambda r: r['result'].update(instructions=-1),
                           lambda r: r['result']['timing'].update(configuration={'wrong':1})):
                bad=copy.deepcopy(original);mutate(bad)
                # Rehashing corrupted raw records cannot evade execution validation.
                bad['sha256']=checkpoint.digest({'key':bad['key'],'result':bad['result']})
                checkpoint.progress.atomic_write(path,bad)
                with patch.object(suite,'cases',return_value=[case]), patch.object(suite,'execute',side_effect=AssertionError('rerun')):
                    with self.assertRaises(ValueError):
                        suite.measure(binary,root,'a'*40,profile,checkpoint=saved)
            checkpoint.progress.atomic_write(path,original)
            binary.write_bytes(b'changed binary')
            changed=checkpoint.Checkpoint(folder/'saved','throughput','head',{'commit':'a'*40},binary,'a'*40,{},suite.digest(root/suite.ELF))
            with patch.object(suite,'cases',return_value=[case]), patch.object(suite,'execute',side_effect=AssertionError('rerun')):
                with self.assertRaises(ValueError):suite.measure(binary,root,'a'*40,profile,checkpoint=changed)


if __name__ == '__main__':
    unittest.main()
