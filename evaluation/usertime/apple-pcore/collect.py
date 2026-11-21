import ctypes
import fcntl
import hashlib
import json
import os
import platform
import random
import resource
import statistics
import subprocess
import sys
import tempfile
import time
from collections import defaultdict
from pathlib import Path

root=Path(__file__).resolve().parent
read=lambda p:json.loads(p.read_text())

def sha(path):
    with path.open('rb') as f:
        return hashlib.file_digest(f,'sha256').hexdigest()

def save(name,value):
    final=root/name
    temporary=final.with_suffix('.tmp')
    temporary.write_text(json.dumps(value,indent=2)+'\n')
    temporary.replace(final)

def topology():
    folders=sorted(Path('/sys/devices/system/cpu/cpufreq').glob('policy*'))
    return [dict(cpus=list(map(int,(p/'related_cpus').read_text().split())),
                 maximum_khz=int((p/'cpuinfo_max_freq').read_text()),
                 governor=(p/'scaling_governor').read_text().strip(),
                 minimum_policy_khz=int((p/'scaling_min_freq').read_text()),maximum_policy_khz=int((p/'scaling_max_freq').read_text())) for p in folders]

host=platform.node(); target=sys.argv[1]; expected_host={'m1':'marin-0','m2':'marin-5'}[target]
expected_model={'m1':'Apple Mac mini (M1, 2020)','m2':'Apple Mac mini (M2, 2023)'}[target]
model=Path('/proc/device-tree/model').read_bytes().rstrip(b'\0').decode()
assert host==expected_host and model==expected_model and platform.machine()=='aarch64' and os.sysconf('SC_PAGESIZE')==16384
cpu=4; cpus=topology(); fastest=max(cpus,key=lambda x:x['maximum_khz']); assert cpu in fastest['cpus'] and fastest['governor']=='performance'
cap={i:int(Path(f'/sys/devices/system/cpu/cpu{i}/cpu_capacity').read_text()) for p in cpus for i in p['cpus'] if Path(f'/sys/devices/system/cpu/cpu{i}/cpu_capacity').is_file()}
if cap: assert cap[cpu]==max(cap.values())
getcpu=ctypes.CDLL(None).sched_getcpu; getcpu.restype=ctypes.c_int
assert os.sched_getaffinity(0)=={cpu} and getcpu()==cpu
resource.setrlimit(resource.RLIMIT_CORE,(0,0))
lock=(root/'pinned-capture.lock').open('a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
assert not (root/'trials.jsonl').exists()
cases=read(root/'cases.json'); assert len(cases)==len({c['id'] for c in cases})==264
for c in cases: assert sha(root/c['path'])==c['sha256']
seed=7031; meta=dict(target=target,hardware=model,host=host,platform=platform.platform(),binary_origin_revision='7031aa3b432cc8a1c83b11f6c0d732cb0a943022',
   timer='wait4.ru_utime',core=cpu,affinity=sorted(os.sched_getaffinity(0)),cpu_capacity=cap,cpufreq_policies=cpus,governor=fastest['governor'], measured_blocks=21,warmup_blocks=1,
   seed=seed,page_size=os.sysconf('SC_PAGESIZE'),inputs_sha256=sha(root/'cases.json'),collector_sha256=sha(Path(__file__)),start_load=os.getloadavg(),started_unix=time.time())
save('native-meta.json',meta)
samples=defaultdict(list)
with (root/'trials.jsonl').open('x',buffering=1) as log:
    for block in range(22):
        assert os.sched_getaffinity(0)=={cpu} and getcpu()==cpu and topology()==cpus
        order=list(cases);random.Random(seed+block).shuffle(order)
        for c in order:
            start=time.monotonic()
            with tempfile.TemporaryFile() as out,tempfile.TemporaryFile() as err:
                child=subprocess.Popen(c['args'],executable=str(root/c['path']),env=dict(e.split('=',1) for e in c['env']),stdout=out,stderr=err)
                while True:
                    pid,st,ru=os.wait4(child.pid,os.WNOHANG)
                    if pid: child.returncode=os.waitstatus_to_exitcode(st); break
                    if time.monotonic()-start>25:
                        child.kill();os.waitpid(child.pid,0);child.returncode=-9
                        raise RuntimeError(f"timeout: {c['id']}")
                    time.sleep(.001)
                length=out.tell();out.seek(0);identity=hashlib.file_digest(out,'sha256').hexdigest()
                if child.returncode or err.tell() or (length,identity)!=(c['output_bytes'],c['output_sha256']):
                    raise RuntimeError((c['id'],child.returncode,err.tell(),length,identity))
            row=dict(id=c['id'],block=block,user_seconds=ru.ru_utime,system_seconds=ru.ru_stime,exit_status=child.returncode,
                     output_bytes=length,output_sha256=identity,parent_cpu_at_reap=getcpu())
            assert row['parent_cpu_at_reap']==cpu
            print(json.dumps(row,separators=(',',':')),file=log)
            if block:samples[c['id']].append(ru.ru_utime)
        save('progress.json',dict(completed_blocks=block+1,total_blocks=22,elapsed=time.time()-meta['started_unix'],load=os.getloadavg()))
        print('completed block',block+1,'/22',flush=True)
rows=[]
for c in cases:
    values=sorted(samples[c['id']]);assert len(values)==21
    med=values[10]; lo=values[2]; hi=values[18]
    quality='nonpositive' if values[0]<=0 else 'below_floor' if med<.0001 else 'noisy' if (hi-lo)/med>.20 else 'usable'
    rows.append(dict(id=c['id'],median_seconds=med,p10_seconds=lo,p90_seconds=hi,quality=quality,elf_sha256=c['sha256'],output_bytes=c['output_bytes'],output_sha256=c['output_sha256']))
save('native-reference.json',dict(metadata=meta,cases=rows,trials_sha256=sha(root/'trials.jsonl'),completed_unix=time.time()))
