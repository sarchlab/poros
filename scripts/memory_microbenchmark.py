#!/usr/bin/env python3
"""Source-bound single-core memory latency/parallelism/bandwidth diagnostics."""
import argparse
import base64
from datetime import datetime, timezone
from functools import lru_cache
import hashlib
import json
import math
from pathlib import Path
import platform
import re
import shutil
import statistics
import struct
import subprocess

ROOT = Path(__file__).resolve().parents[1]
DIRECTORY = 'evaluation/microbenchmarks/memory/'
ELF, REFERENCE = DIRECTORY + 'memory.elf', DIRECTORY + 'm2.json'
LARGE_ELF = DIRECTORY + 'memory-large.elf'
UNSUPPORTED_IMAGE = 'unsupported_image_capacity'
DEFINITION = DIRECTORY + 'cases.json'
MEASUREMENT_INPUTS = [DIRECTORY + n for n in ('cases.json', 'driver.h', 'loops.S', 'native.c')]
COLLECTOR_SOURCES = [DIRECTORY + n for n in ('linux.c', 'entry.S', 'build.sh', 'memory.elf')]
COLLECTOR_SOURCES += ['scripts/memory_microbenchmark.py', 'tests/usertime/layout.ld']
INPUTS = MEASUREMENT_INPUTS + COLLECTOR_SOURCES + [LARGE_ELF]
PROTOCOL = 'assembly-memory-v1'
NODE_BYTES, WARMUP_PASSES, CHASE_TAIL = 128, 2, 17
MAX_BYTES, MASK = 64 * 1024 * 1024, (1 << 64) - 1
WRITE_VALUE = 0x5a5a5a5a5a5a5a5a
TRIALS, MIN_SECONDS, MAX_SPREAD, PILOT_TARGET = 9, .02, .10, .10
PATTERNS = [dict(id=f'chase{k}', kind=i + 1, chains=k) for i, k in enumerate((1, 2, 4, 8, 16))]
PATTERNS += [dict(id=n, kind=i + 6) for i, n in enumerate(('read', 'write', 'copy'))]
FUNCTIONS = [p['id'] if p['kind'] <= 5 else 'stream_' + p['id'] for p in PATTERNS]


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def require(ok, message):
    if not ok:
        raise ValueError(message)


def positive(value):
    return type(value) is int and value > 0


def finite(value):
    return type(value) in (int, float) and math.isfinite(value) and value > 0


def sha(value):
    return isinstance(value, str) and re.fullmatch(r'[0-9a-f]{64}', value) is not None


def read(path):
    return json.loads(Path(path).read_text())


def write(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True, allow_nan=False) + '\n')


def parse_cases(definition):
    require(isinstance(definition, dict) and type(definition.get('schema')) is int and definition['schema'] == 2, 'invalid memory case definition')
    keys = {'schema', 'patterns', 'chase_working_set_bytes', 'stream_working_set_bytes'}
    require(set(definition) == keys and json.dumps(definition['patterns'], sort_keys=True) == json.dumps(PATTERNS, sort_keys=True), 'invalid memory patterns/definition')
    def sizes(key, maximum):
        values = definition[key]
        require(isinstance(values, list) and len(values) >= 2 and all(positive(n) for n in values)
                and values == sorted(set(values)) and all(8192 <= n <= maximum and n % (NODE_BYTES * 16) == 0 for n in values), 'invalid memory working sets')
        return values
    chase = sizes('chase_working_set_bytes', 256 * 1024 * 1024)
    stream = sizes('stream_working_set_bytes', MAX_BYTES)
    result = []
    for size in sorted(set(chase + stream)):
        for pattern in PATTERNS:
            if size not in (chase if pattern['kind'] <= 5 else stream): continue
            case = dict(pattern, id=f'{pattern["id"]}-{size}', pattern=pattern['id'], working_set_bytes=size)
            minimum = max(1, ((65536 * NODE_BYTES if pattern['kind'] <= 5 else 1048576) + size - 1) // size)
            case['passes'] = 1 << (minimum - 1).bit_length()
            case['elf'] = LARGE_ELF if size > 128 * 1024 * 1024 else ELF
            case['max_image_bytes'] = (512 if size > 128 * 1024 * 1024 else 256) * 1024 * 1024
            case['instructions_per_pass'] = instructions_per_pass(case)
            case['operations_per_pass'] = size // NODE_BYTES if pattern['kind'] <= 5 else size // (16 if pattern['kind'] == 8 else 8)
            case['effective_bytes_per_pass'] = 0 if pattern['kind'] <= 5 else size
            result.append(case)
    return result


def cases(root=ROOT):
    return parse_cases(read(Path(root) / DEFINITION))


def case_elf(case):
    return case['elf']


def supports_large_images(root, revision):
    # Prepared binaries can target Linux while prepare runs on macOS, so a runtime
    # help probe cannot be used here. This deliberately follows the flag registration
    # in cmd/poros/elf.go at the exact source revision; tests lock that coupling.
    result = subprocess.run(['git', '-C', str(root), 'show', revision + ':cmd/poros/elf.go'], capture_output=True, text=True, check=True)
    return 'fs.Uint64("max-image-bytes",' in result.stdout


def unavailable(case):
    return dict(id=case['id'], status=UNSUPPORTED_IMAGE)


def instructions_per_pass(case):
    size, kind = case['working_set_bytes'], case['kind']
    if kind <= 5:
        chains = case['chains']
        return (size // NODE_BYTES // chains) * (chains + 2)
    return {6: 4 + 19 * (size // 64), 7: 4 + 11 * (size // 64), 8: 5 + 20 * (size // 128)}[kind]


@lru_cache(maxsize=16)
def permutation(size):
    """Independent Python oracle for the specified xorshift64/Fisher-Yates recipe."""
    values = list(range(size // NODE_BYTES))
    state = 0x6a09e667f3bcc909
    for i in range(len(values) - 1, 0, -1):
        state ^= state << 13 & MASK
        state ^= state >> 7
        state ^= state << 17 & MASK
        j = state % (i + 1)
        values[i], values[j] = values[j], values[i]
    return values


def expected(case, passes):
    kind, size = case['kind'], case['working_set_bytes']
    if kind <= 5:
        values = permutation(size)
        length = len(values) // case['chains']
        return sum(values[c * length + CHASE_TAIL % length] * NODE_BYTES for c in range(case['chains']))
    count = size // 8
    return {6: passes * (count * (count + 1) // 2) & MASK, 7: WRITE_VALUE, 8: count // 2}[kind]


def arguments(case, passes):
    # Equal-length arguments keep parsing work identical in differential runs.
    return [str(case['kind']), str(case['working_set_bytes']), f'{passes:08d}']


def instruction_budget(case, passes):
    # Covers deterministic construction, full warmup, verification and measured work.
    return 200000 + 80 * (case['working_set_bytes'] // 8) + instructions_per_pass(case) * (passes + WARMUP_PASSES)


def execute(binary, root, case, passes, mode='ooo', revision=None, profile=None):
    cmd = [str(binary), 'elf', '--mode', mode, '--max-instructions', str(instruction_budget(case, passes)),
           '--timeout', '600s']
    if profile is not None:
        cmd += ['--frequency-hz', str(profile['configuration']['freq'])]
    if case['max_image_bytes'] > 256 * 1024 * 1024:
        cmd += ['--max-image-bytes', str(case['max_image_bytes'])]
    cmd += [str(Path(root) / case_elf(case)), *arguments(case, passes)]
    process = subprocess.run(cmd, capture_output=True, timeout=620)
    require(process.returncode == 0 and not process.stderr, 'memory execution failed: ' + case['id'] + ' ' + process.stdout.decode(errors='replace')[-1500:])
    result = json.loads(process.stdout)
    validate_execution(result, case, passes, digest(Path(root) / case_elf(case)), mode, revision, profile)
    return result


def validate_execution(result, case, passes, elf_hash, mode='ooo', revision=None, profile=None):
    require(result.get('termination') == 'exit' and result.get('exit_status') == 0 and not result.get('stderr_base64')
            and result.get('binary_sha256') == elf_hash and result.get('args') == ['program', *arguments(case, passes)]
            and result.get('env') == [] and positive(result.get('instructions'))
            and result['instructions'] <= instruction_budget(case, passes), 'invalid memory execution/identity')
    require(base64.b64decode(result['stdout_base64'], validate=True) == struct.pack('<Q', expected(case, passes)), 'memory checksum mismatch')
    if 'max_image_bytes' in result or case['max_image_bytes'] > 256 * 1024 * 1024:
        require(type(result.get('max_image_bytes')) is int and result['max_image_bytes'] == case['max_image_bytes'], 'memory image capacity provenance mismatch')
    if revision is not None:
        require(result.get('build', {}).get('vcs.revision') == revision and result['build'].get('vcs.modified') == 'false', 'memory simulator revision mismatch')
    if mode == 'functional':
        require('timing' not in result, 'functional memory execution unexpectedly timed')
        return
    timing = result.get('timing', {})
    require(timing.get('model') == 'ooo' and positive(timing.get('user_cycles')) and timing.get('retired_instructions') == result['instructions'], 'invalid memory timing')
    if profile is not None:
        require(all(timing.get(k) == profile[k] for k in ('configuration', 'cache_config')), 'memory model profile mismatch')
    hz = timing.get('assumed_hz')
    if profile is not None:
        require(hz == profile['configuration']['freq'], 'memory frequency differs from model profile')
    require(finite(hz) and finite(timing.get('predicted_user_seconds'))
            and math.isclose(timing['predicted_user_seconds'], timing['user_cycles'] / hz, rel_tol=1e-12), 'invalid memory cycle conversion')


def summarize_samples(case, samples):
    require(len(samples) == 2, 'memory short/long execution pair missing')
    instructions = samples[1]['instructions'] - samples[0]['instructions']
    cycles = samples[1]['timing']['user_cycles'] - samples[0]['timing']['user_cycles']
    row = dict(id=case['id'], instructions=instructions, cycles=cycles)
    validate_rows([row], [case])
    return row


def validate_rows(rows, declared):
    require(isinstance(rows, list) and [r.get('id') for r in rows] == [c['id'] for c in declared], 'memory simulation case coverage mismatch')
    for row, case in zip(rows, declared):
        require(type(row.get('instructions')) is int and row['instructions'] == case['passes'] * case['instructions_per_pass']
                and positive(row.get('cycles')), 'invalid memory differential counters')


def measure_case(binary, root, case, revision, profile):
    samples = [execute(binary, root, case, case['passes'] * factor, revision=revision, profile=profile) for factor in (1, 2)]
    summarize_samples(case, samples)
    return samples


def measure(binary, root, revision, profile):
    large = supports_large_images(root, revision)
    return [unavailable(case) if case['max_image_bytes'] > 256 * 1024 * 1024 and not large
            else summarize_samples(case, measure_case(binary, root, case, revision, profile)) for case in cases(root)]


def declared(files):
    return any(name in files for name in [*MEASUREMENT_INPUTS, ELF, REFERENCE])


def validate_comparison(simulations, declared_cases, base_large_image_support):
    require(type(base_large_image_support) is bool, 'memory base image capability must be a boolean')
    require(isinstance(simulations, dict) and set(simulations) == {'base', 'head'}, 'memory requires both base and head simulation')
    for role, rows in simulations.items():
        require(isinstance(rows, list) and [r.get('id') for r in rows] == [c['id'] for c in declared_cases], 'memory simulation case coverage mismatch')
        for row, case in zip(rows, declared_cases):
            unsupported = role == 'base' and case['max_image_bytes'] > 256 * 1024 * 1024 and not base_large_image_support
            if unsupported:
                require(row == unavailable(case), 'memory unsupported base row must match declared capability')
            else:
                validate_rows([row], [case])


def smoke(binary, root=ROOT, qemu=None):
    # All patterns, two small power-of-two footprints and the new 96 KiB chase shape.
    declared = [c for c in cases(root) if c['working_set_bytes'] in (8192, 32768, 98304)]
    for case in declared:
        samples = []
        for passes in (1, 2):
            actual = execute(binary, root, case, passes, 'functional')
            timed = execute(binary, root, case, passes)
            require(actual['instructions'] == timed['instructions'] and actual['stdout_base64'] == timed['stdout_base64'], 'memory functional/timing mismatch')
            if qemu is not None:
                p = subprocess.run([str(qemu), str(Path(root) / ELF), *arguments(case, passes)], capture_output=True, timeout=30)
                require(p.returncode == 0 and not p.stderr and p.stdout == base64.b64decode(actual['stdout_base64']), 'memory QEMU differential failed')
            samples.append(timed)
        trial_case = dict(case, passes=1)
        summarize_samples(trial_case, samples)
        print(case['id'], 'functional/timing/checksum/instruction delta' + ('/QEMU' if qemu else ''), 'pass', flush=True)


def percentile(values, q):
    values = sorted(values)
    rank = (len(values) - 1) * q
    low = int(rank)
    return values[low] + (values[min(low + 1, len(values) - 1)] - values[low]) * (rank - low)


def distribution(values):
    require(len(values) == TRIALS and all(finite(v) for v in values), 'invalid memory native paired samples')
    p10, median, p90 = [percentile(values, q) for q in (.1, .5, .9)]
    spread = (p90 - p10) / median
    return dict(p10_seconds=p10, median_seconds=median, p90_seconds=p90, relative_spread=spread,
                quality='usable' if median >= MIN_SECONDS and spread <= MAX_SPREAD else 'noisy')


def parse_native(lines, case, passes, trials):
    require(len(lines) == trials, 'incomplete memory native samples')
    samples = []
    for trial, line in enumerate(lines):
        fields = line.split()
        require(len(fields) == 5, 'invalid native memory row')
        number, short, long, first, second = fields
        short, long = float(short), float(long)
        require(int(number) == trial and finite(short) and finite(long)
                and int(first) == expected(case, passes) and int(second) == expected(case, 2 * passes), 'native memory checksum/timer mismatch')
        samples.append(dict(trial=trial, short_seconds=short, long_seconds=long, difference_seconds=long - short,
                            short_checksum=int(first), long_checksum=int(second)))
    return samples


def symbol_bytes(path, names):
    """Read exact fixed function bodies from ELF64 or Mach-O64 symbol tables."""
    data = Path(path).read_bytes()
    symbols, segments = {}, []
    if data[:4] == b'\x7fELF':
        require(data[4:6] == b'\x02\x01', 'expected little-endian ELF64')
        phoff, shoff = struct.unpack_from('<QQ', data, 32)
        phsize, phnum, shsize, shnum = struct.unpack_from('<HHHH', data, 54)
        for i in range(phnum):
            kind, _, offset, address, _, size, _, _ = struct.unpack_from('<IIQQQQQQ', data, phoff + phsize * i)
            if kind == 1: segments.append((address, size, offset))
        sections = [struct.unpack_from('<IIQQQQIIQQ', data, shoff + shsize * i) for i in range(shnum)]
        for section in sections:
            if section[1] != 2: continue
            strings = sections[section[6]]
            strings = data[strings[4]:strings[4] + strings[5]]
            for offset in range(section[4], section[4] + section[5], section[9]):
                name, _, _, _, address, _ = struct.unpack_from('<IBBHQQ', data, offset)
                label = strings[name:strings.find(b'\0', name)].decode()
                symbols[label] = address
    else:
        require(struct.unpack_from('<I', data)[0] == 0xfeedfacf, 'expected Mach-O64')
        commands = struct.unpack_from('<I', data, 16)[0]
        offset, symtab = 32, None
        for _ in range(commands):
            kind, size = struct.unpack_from('<II', data, offset)
            if kind == 0x19:
                address, _, fileoff, filesize = struct.unpack_from('<QQQQ', data, offset + 24)
                segments.append((address, filesize, fileoff))
            if kind == 2: symtab = struct.unpack_from('<IIII', data, offset + 8)
            offset += size
        require(symtab is not None, 'native symbols missing')
        symoff, nsyms, stroff, strsize = symtab
        strings = data[stroff:stroff + strsize]
        for i in range(nsyms):
            name, _, _, _, address = struct.unpack_from('<IBBHQ', data, symoff + i * 16)
            label = strings[name:strings.find(b'\0', name)].decode()
            symbols[label.removeprefix('_')] = address
    result = {}
    for name in names:
        start, end = symbols[name], symbols[name + '_end']
        require(start < end and (end - start) % 4 == 0, 'invalid probe symbol extent')
        matches = [(offset + start - address, end - start) for address, size, offset in segments if address <= start < end <= address + size]
        require(len(matches) == 1, 'probe body not in one file-backed segment')
        offset, size = matches[0]
        result[name] = data[offset:offset + size]
    return result


def audit_binaries(elf, native):
    linux, mac = symbol_bytes(elf, FUNCTIONS), symbol_bytes(native, FUNCTIONS)
    require(linux == mac, 'Mach-O/ELF computation instruction bytes differ')
    return {name: dict(bytes=len(body), sha256=hashlib.sha256(body).hexdigest()) for name, body in linux.items()}


def validate_reference(reference, files=None):
    require(isinstance(reference, dict) and reference.get('protocol') == PROTOCOL and reference.get('hardware') == 'Apple M2'
            and reference.get('executable_format') == 'Mach-O' and reference.get('timer') == 'CLOCK_THREAD_CPUTIME_ID'
            and reference.get('core_identity') == 'unknown' and reference.get('instantaneous_frequency') == 'unknown'
            and reference.get('warmup_passes') == WARMUP_PASSES and reference.get('trials') == TRIALS
            and reference.get('pilot_target_seconds') == PILOT_TARGET
            and reference.get('quality_rule') == dict(minimum_difference_seconds=MIN_SECONDS, maximum_relative_spread=MAX_SPREAD), 'invalid memory native protocol')
    require(sha(reference.get('native_binary_sha256')) and isinstance(reference.get('platform'), str)
            and reference['platform'].startswith('macOS-') and isinstance(reference.get('compiler'), str)
            and reference['compiler'], 'invalid memory native build/environment')
    try:
        stamp = datetime.fromisoformat(reference['collected_at_utc'])
        require(stamp.utcoffset() is not None and stamp.utcoffset().total_seconds() == 0, 'memory collection timestamp not UTC')
    except (KeyError, TypeError, ValueError):
        raise ValueError('invalid memory native collection timestamp') from None
    sources = reference.get('sources', {})
    require(set(sources) == set(INPUTS) and all(sha(v) for v in sources.values()), 'incomplete memory source identities')
    definition = reference.get('definition')
    require(isinstance(definition, str) and hashlib.sha256(definition.encode()).hexdigest() == sources[DEFINITION], 'memory case definition identity mismatch')
    declared_cases = parse_cases(json.loads(definition))
    require(type(reference.get('native_capacity_bytes')) is int and reference['native_capacity_bytes'] == 256 * 1024 * 1024, 'invalid native buffer capacity')
    rows = reference.get('cases')
    require(isinstance(rows, list) and len(rows) == len(declared_cases), 'memory native coverage mismatch')
    for row, case in zip(rows, declared_cases):
        require(all(type(row.get(k)) is type(v) and row[k] == v for k, v in case.items()), 'memory native case declaration mismatch')
        require(positive(row.get('native_passes')) and row['native_passes'] <= 16777216 and not row['native_passes'] & (row['native_passes'] - 1), 'invalid memory native repetition count')
        require(all(finite(row.get(k)) for k in ('p10_seconds', 'median_seconds', 'p90_seconds'))
                and row['p10_seconds'] <= row['median_seconds'] <= row['p90_seconds'], 'invalid memory distribution')
        spread = (row['p90_seconds'] - row['p10_seconds']) / row['median_seconds']
        require(type(row.get('relative_spread')) in (int, float) and math.isfinite(row['relative_spread'])
                and math.isclose(row['relative_spread'], spread, rel_tol=1e-12, abs_tol=1e-12), 'memory spread mismatch')
        wanted = 'usable' if row['median_seconds'] >= MIN_SECONDS and spread <= MAX_SPREAD else 'noisy'
        require(row.get('quality') == wanted and sha(row.get('raw_samples_sha256')), 'memory quality/evidence mismatch')
    audit = reference.get('instruction_audit', {})
    require(set(audit) == set(FUNCTIONS) and all(set(v) == {'bytes', 'sha256'} and positive(v['bytes'])
            and v['bytes'] % 4 == 0 and sha(v['sha256']) for v in audit.values()), 'memory instruction audit incomplete')
    if files is not None:
        bound_inputs = [*MEASUREMENT_INPUTS, ELF, LARGE_ELF]
        require(all(files.get(name) == sources[name] for name in bound_inputs), 'memory reference differs from tested measurement sources or ELF')
    return reference


def load_reference(root=ROOT):
    root = Path(root)
    reference = read(root / REFERENCE)
    elfs = [ELF, LARGE_ELF]
    validate_reference(reference, {name: digest(root / name) for name in [*MEASUREMENT_INPUTS, *elfs]})
    for elf in elfs:
        audit = {name: dict(bytes=len(body), sha256=hashlib.sha256(body).hexdigest())
                 for name, body in symbol_bytes(root / elf, FUNCTIONS).items()}
        require(reference['instruction_audit'] == audit, 'memory ELF differs from audited instruction bodies')
    return reference


def collect(output, evidence_dir, root=ROOT):
    root, evidence = Path(root), Path(evidence_dir)
    require(subprocess.check_output(['sysctl', '-n', 'machdep.cpu.brand_string'], text=True).strip() == 'Apple M2', 'memory collection requires Apple M2')
    evidence.mkdir(parents=True, exist_ok=False)
    binary = evidence / 'native'
    subprocess.run(['clang', '-O2', '-arch', 'arm64', '-DMAX_BYTES=268435456UL', str(root / DIRECTORY / 'native.c'), str(root / DIRECTORY / 'loops.S'), '-o', str(binary)], check=True)
    audit = audit_binaries(root / ELF, binary)
    require(audit_binaries(root / LARGE_ELF, binary) == audit, 'large ELF loop audit mismatch')
    write(evidence / 'instruction-audit.json', audit)
    source_hashes = {name: digest(root / name) for name in INPUTS}
    for name in INPUTS:
        target = evidence / "source" / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root / name, target)
    reference = dict(protocol=PROTOCOL, hardware='Apple M2', executable_format='Mach-O', platform=platform.platform(),
                     compiler=subprocess.check_output(['clang', '--version'], text=True).splitlines()[0],
                     collected_at_utc=datetime.now(timezone.utc).isoformat(), timer='CLOCK_THREAD_CPUTIME_ID',
                     core_identity='unknown', instantaneous_frequency='unknown', warmup_passes=WARMUP_PASSES, trials=TRIALS, pilot_target_seconds=PILOT_TARGET,
                     quality_rule=dict(minimum_difference_seconds=MIN_SECONDS, maximum_relative_spread=MAX_SPREAD),
                     native_capacity_bytes=256 * 1024 * 1024, native_binary_sha256=digest(binary), sources=source_hashes, definition=(root / DEFINITION).read_text(),
                     instruction_audit=audit, cases=[])
    for case in cases(root):
        started = datetime.now(timezone.utc).isoformat()
        passes, pilots = case["passes"], []
        while True:
            lines = subprocess.check_output([str(binary), str(case['kind']), str(case['working_set_bytes']), str(passes), '1'], text=True, timeout=60).splitlines()
            sample = parse_native(lines, case, passes, 1)[0]
            pilots.append(dict(passes=passes, sample=sample))
            if sample['difference_seconds'] >= PILOT_TARGET or passes >= 16777216: break
            passes *= 2
        lines = subprocess.check_output([str(binary), str(case['kind']), str(case['working_set_bytes']), str(passes), str(TRIALS)], text=True, timeout=120).splitlines()
        samples = parse_native(lines, case, passes, TRIALS)
        raw = dict(case=case, native_passes=passes, pilots=pilots, samples=samples, started_at_utc=started, completed_at_utc=datetime.now(timezone.utc).isoformat(), native_binary_sha256=reference['native_binary_sha256'])
        raw_path = evidence / (case['id'] + '.json')
        write(raw_path, raw)
        row = dict(case, native_passes=passes, raw_samples_sha256=digest(raw_path), **distribution([s['difference_seconds'] for s in samples]))
        reference['cases'].append(row)
        print(case['id'], row['quality'], 'median', row['median_seconds'], 'spread', row['relative_spread'], flush=True)
    require(source_hashes == {name: digest(root / name) for name in INPUTS}, 'memory collection source changed')
    validate_reference(reference)
    write(output, reference)
    write(evidence / 'reference.json', reference)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    smoke_parser = commands.add_parser('smoke')
    smoke_parser.add_argument('--poros', type=Path, required=True)
    smoke_parser.add_argument('--qemu', type=Path)
    collect_parser = commands.add_parser('collect')
    collect_parser.add_argument('--output', type=Path, default=ROOT / REFERENCE)
    collect_parser.add_argument('--evidence-dir', type=Path, required=True)
    args = parser.parse_args()
    if args.command == 'smoke': smoke(args.poros, qemu=args.qemu)
    else: collect(args.output, args.evidence_dir)


if __name__ == '__main__':
    main()
