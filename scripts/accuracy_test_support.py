"""Small, explicitly synthetic evidence for transport/archive unit tests.

These fixtures never become calibration inputs or repository measurements.
"""
import base64
import copy
import json
from pathlib import Path
import shutil
import compute_usertime as compute
import workload_contract as contract


def synthetic_micro_reference(root, manifest):
    rows = []
    for case in manifest['cases']:
        raw = contract.expected(case)
        sample = dict(user_seconds=.001, system_seconds=.0001, exit_status=0,
                      stdout_base64=base64.b64encode(raw).decode(), stderr_base64='')
        rows.append(dict(case, samples=[copy.deepcopy(sample) for _ in range(21)],
                         warmup_sample=sample, median_user_seconds=.001, quality='usable'))
    return dict(schema=2, measurement_id='synthetic-unit-only', environment='virtualized',
                collection_commit='a'*40, collection_dirty=False, container_image='sha256:'+'f'*64,
                retained_samples_per_case=21, precision_floor_seconds=.0001, quality_policy=compute.QUALITY_POLICY,
                metric='child_user_cpu_seconds',timer='wait4.ru_utime',machine='aarch64',
                hardware='unit', platform='unit', core_identity='unknown', instantaneous_frequency='unknown',
                harness_sha256=contract.digest(root/'scripts/native_usertime.py'),
                support_sha256={'workload_contract.py': contract.digest(root/'scripts/workload_contract.py')},
                manifest_sha256=contract.digest(root/contract.manifest_path(manifest)),
                started_at_utc='2026-01-01T00:00:00Z', completed_at_utc='2026-01-01T00:05:00Z', cases=rows)


def small_source(root, destination):
    """Keep production source/ELFs, but use small correctness coordinates as unit inputs."""
    for name in compute.source_paths(root) | {'tests/usertime/manifest.json'}:
        path = destination/name
        path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root/name,path)
    performance = compute.read(destination/'tests/benchmarks/performance.json')
    small = compute.read(destination/'tests/benchmarks/manifest.json')
    for workload, small_workload in zip(performance['workloads'],small['workloads']):
        workload['cases'] = small_workload['cases']
        workload['dimensions'] = small_workload['dimensions']
        workload['binary'] = small_workload['binary']
    path = destination/'tests/benchmarks/performance.json'
    compute.write(path,performance)
    build = compute.read(destination/'tests/benchmarks/provenance.json')
    build['files']['tests/benchmarks/performance.json'] = contract.digest(path)
    compute.write(destination/'tests/benchmarks/provenance.json',build)
    micro = compute.read(destination/'tests/usertime/manifest.json')
    micro.update(suite=contract.PERFORMANCE_SUITE,reference_file=Path(contract.PERFORMANCE_REFERENCE).name)
    for group in micro['workloads']:
        if group['id']=='memory':group['iterations']=32000000
    for case in micro['cases']:
        if case['workload']=='memory':case['args'][1]='32000000'
        case['mapped_image_bytes']=contract.mapped_image_bytes(destination/'tests/usertime'/case['file'])
    compute.write(destination/contract.PERFORMANCE_MANIFEST,micro)
    compute.write(destination/contract.PERFORMANCE_REFERENCE,synthetic_micro_reference(destination,micro))
    return micro


import hashlib
import gzip
import usertime as gate
SHA = "a" * 40


class Evidence:
    """Complete small synthetic schema3 evidence, shared by protocol tests."""
    def __init__(self, root, directory, *, native=True):
        self.source = directory / "source"
        small_source(root, self.source)
        self.folder = self.source / compute.REFERENCE
        self.declared = compute.contract(self.source)
        self.reference = self.make_reference()
        import performance_reference as performance
        performance.export(self.source, self.source / contract.PERFORMANCE_REFERENCE, self.folder, self.source / performance.PATH)
        if native:
            measured = synthetic_native(compute.read(self.source / performance.PATH), performance.inventory(self.source))
            compute.write(self.source / performance.PATH, measured)
        self.report = self.make_report()

    def make_reference(self):
        rows, oracle = [], []
        for case in self.declared["cases"]:
            length = sum(x["count"] * (8 if x["type"] == "double" else 4) for x in case["outputs"])
            data = bytes(length)  # Deliberately synthetic finite test output.
            sha = hashlib.sha256(data).hexdigest()
            target = self.folder / compute.output_name(dict(output_sha256=sha), compute.PROTOCOL)
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(gzip.compress(data, mtime=0))
            sample = dict(user_seconds=.001, system_seconds=.0001, exit_status=0,
                          stdout_sha256=sha, stdout_bytes=length, stderr_bytes=0,
                          stderr_sha256=hashlib.sha256(b"").hexdigest())
            rows.append(dict(id=case["id"], samples=[copy.deepcopy(sample) for _ in range(21)],
                             warmup_sample=sample, median_user_seconds=.001, quality="usable",
                             output_sha256=sha, output_bytes=length))
            oracle.append(dict(id=case["oracle_id"], args=case["args"][1:], binary_sha256=case["sha256"],
                               status="pass", numerical_status="pass", numerical_check="equations" if case["workload"] in {"gramschmidt", "blackscholes", "bodytrack", "canneal", "raytrace", "facesim", "x264"} else "shape_finite", output_sha256=sha, output_bytes=length, native_sha256=sha, qemu_sha256=sha))
        harness = (self.source / "scripts/benchmark-differential.py").read_bytes()
        validation = dict(schema=1, acceptance_status="pass", status="pass", expected_cases=240, qemu="synthetic unit oracle", platform="synthetic", contract="complete native/QEMU output; independent equations for declared sensitive workloads; no simulator execution", numerical_oracle_sha256="1"*64,
                          provenance=dict(git_dirty=False, git_sha=SHA, sources={**self.declared["build"]["files"], **{name: contract.digest(self.source/name) for name in compute.NUMERICAL_SUPPORT}, "scripts/benchmark-differential.py": hashlib.sha256(harness).hexdigest()}),
                          build=self.declared["build"], manifest_sha256=self.declared["program_manifest_sha256"], cases=oracle)
        compute.write(self.folder / "validation-report.json", validation)
        (self.folder / "validation-harness.py").write_bytes(harness)
        compute.write(self.folder / "oracle.json", compute.compact_oracle(validation, self.declared))
        reference = dict(schema=1, protocol=compute.PROTOCOL, measurement_id="synthetic-unit-only", contract=self.declared,
                         collection_commit=SHA, collection_dirty=False, quality_policy=compute.QUALITY_POLICY, container_image="sha256:" + "f" * 64,
                         started_at_utc="2026-01-01T00:05:00Z", completed_at_utc="2026-01-01T00:10:00Z",
                         precision_floor_seconds=.0001, retained_samples_per_case=21, warmup="one complete process per case",
                         metric="child_user_cpu_seconds", timer="wait4.ru_utime", environment="virtualized", machine="aarch64",
                         hardware="unit", platform="unit", core_identity="unknown", instantaneous_frequency="unknown",
                         oracle_sha256=contract.digest(self.folder / "oracle.json"),
                         support_sha256={name: contract.digest(self.source / name) for name in compute.support(compute.PROTOCOL)}, cases=rows)
        compute.write(self.folder / "reference.json", reference)
        return reference

    def make_report(self):
        old_manifest = contract.load_manifest(self.source, performance=True)
        old_reference = compute.read(self.source / contract.PERFORMANCE_REFERENCE)
        capability = compute.base_capability("ec5df8efdd80fd02a0db7bd80e9ae413c509852b")
        old_rows = []
        for case, reference in zip(old_manifest["cases"], old_reference["cases"]):
            seconds = max(reference["median_user_seconds"], .001)
            head = dict(predicted_user_seconds=seconds, instructions=100, cycles=100, **old_manifest["profile"])
            if reference["quality"] == "usable":
                head.update(error_percent=gate.error_percent(seconds, reference["median_user_seconds"]), direction="equal" if seconds == reference["median_user_seconds"] else "slower")
            old_rows.append(dict(id=case["id"], split=case["split"], functional_status="pass", reference_quality=reference["quality"],
                                 reference_user_seconds=reference["median_user_seconds"], head=head, base=copy.deepcopy(head), error_change_percentage_points=0))
        for row, case in zip(old_rows, old_manifest["cases"]):
            row.update(mapped_image_bytes=case["mapped_image_bytes"],base_status=compute.case_base_status(case,capability))
            if row["base_status"]!="available":
                row.pop("base",None); row.pop("error_change_percentage_points",None)
        old = dict(schema=2, artifact_status="pass", accuracy_status="unverified", formula=gate.FORMULA,
                   head=dict(commit=SHA, dirty=False, files={str(p.relative_to(self.source)): contract.digest(p) for p in self.source.rglob("*") if p.is_file()}),
                   base_commit="ec5df8efdd80fd02a0db7bd80e9ae413c509852b", base_capability=capability, base_status="available", head_binary_sha256="d" * 64, base_binary_sha256="e" * 64, workload_manifest=old_manifest, reference_sha256=contract.digest(self.source / contract.PERFORMANCE_REFERENCE),
                   reference={k:v for k,v in old_reference.items() if k != "cases"}, cases=old_rows, summary={k: v for k, v in compute.summaries(old_rows,True).items() if k != "validation-legacy"})
        rows = copy.deepcopy(old_rows)
        for case in self.declared["cases"]:
            rows.append(dict(id=case["id"], split=case["split"], functional_status="pass", reference_quality="usable", reference_user_seconds=.001,
                             head=dict(predicted_user_seconds=.002, instructions=100, cycles=6980000, error_percent=100, direction="slower", **old_manifest["profile"])))
        for row, case in zip(rows[24:],self.declared["cases"]):
            row.update(mapped_image_bytes=case["mapped_image_bytes"],base_status=compute.case_base_status(case,capability))
            if row["base_status"]=="available":
                row["base"]=copy.deepcopy(row["head"]);row["error_change_percentage_points"]=0
        refs = dict(original24=old["reference_sha256"], compute240=contract.digest(self.folder / "reference.json"))
        return dict(schema=3, artifact_status="pass", accuracy_status="unverified", formula=gate.FORMULA,
                    head=old["head"], base_commit=old["base_commit"], base_capability=capability, original_report=old, base_status=old["base_status"], head_binary_sha256=old["head_binary_sha256"], base_binary_sha256=old["base_binary_sha256"], reference=old["reference"],
                    workload_manifest=compute.expanded_manifest(old_manifest, self.declared), cases=rows,
                    reference_files=refs, reference_sha256=compute.reference_identity(refs["original24"], refs["compute240"]),
                    compute_base_status="per_case_resource_limit", compute_reference={key: self.reference[key] for key in ("measurement_id", "environment", "hardware", "platform", "core_identity", "instantaneous_frequency", "completed_at_utc")},
                    summary=compute.summaries(rows,True))



def compact_report(evidence):
    import performance_reference as performance
    report = copy.deepcopy(evidence.report)
    reference = compute.read(evidence.source / performance.PATH)
    for key in ("original_report", "reference_files", "compute_reference", "compute_base_status"):
        report.pop(key, None)
    report.update(schema=4, base_status="per_case_resource_limit",
                  reference_sha256=contract.digest(evidence.source / performance.PATH),
                  reference={key: value for key, value in reference.items() if key not in ("cases", "input_sources", "collector_sources")})
    for row, measured in zip(report["cases"], reference["cases"]):
        row.update(reference_p10_seconds=measured["p10_user_seconds"], reference_p90_seconds=measured["p90_user_seconds"])
        for label in ("head", "base"):
            if label in row:
                row[label].update(predicted_user_seconds=.002, cycles=6980000, error_percent=100, direction="slower")
    report["summary"] = compute.summaries(report["cases"], True)
    return report


def device_selection(ident, commit, files, base_commit=None):
    """A device bound to the repository's declaration, as devices.resolve binds it.

    The declaration's hash joins `files`, the head's source inventory. With a
    base commit the base revision declares the same devices; without one it
    predates device selection."""
    import devices
    declaration = devices.load()
    sha = hashlib.sha256(devices.encode(declaration)).hexdigest()
    files[devices.PATH] = sha
    def bound(revision):
        return dict(commit=revision, path=devices.PATH, sha256=sha, declaration=copy.deepcopy(declaration))
    devices.select(declaration, ident)
    return dict(id=ident, head=bound(commit), base=None if base_commit is None else bound(base_commit))


def as_device(report, ident, *, base_declared=False):
    """Rebind a synthetic compact report to a device.

    A device without a native reference loses every reference time, quality and
    error, and a base that cannot model it loses its predictions, exactly as
    the comparison would have produced the report."""
    import devices
    import performance_compare as comparison
    import performance_reference as performance
    report = copy.deepcopy(report)
    report["device"] = device_selection(ident, report["head"]["commit"], report["head"]["files"],
                                        report["base_commit"] if base_declared else None)
    if devices.measured(report, "application"):
        return report
    for key in ("reference", "reference_sha256"):
        report.pop(key)
    report["output_oracle"] = dict(path=performance.PATH, sha256=report["head"]["files"][performance.PATH],
                                   scope=comparison.OUTPUT_ORACLE_SCOPE)
    for row in report["cases"]:
        for key in [key for key in row if key.startswith("reference_")] + ["error_change_percentage_points"]:
            row.pop(key, None)
        row["reference_quality"] = devices.UNMEASURED
        if not devices.available(report, "base"):
            row.pop("base", None)
            row["base_status"] = devices.BASE_UNAVAILABLE
        for label in ("head", "base"):
            for key in ("error_percent", "direction"):
                row.get(label, {}).pop(key, None)
    report["summary"] = compute.summaries(report["cases"], True)
    return report


def synthetic_native(reference, declared):
    """Explicitly synthetic transport fixture; never a hardware reference."""
    import macho_reference as macho
    import performance_reference as performance
    result = copy.deepcopy(reference)
    result.pop("container_image", None)
    result.update(protocol=performance.NATIVE_PROTOCOL, environment="native", executable_format="Mach-O", machine="arm64",
                  page_size=16384, hardware="Apple M2", platform="macOS-synthetic-unit-only", reference_scope=macho.SCOPE,
                  startup_included=True, collector_origin="local-uncommitted-experiment",
                  collector_sources={name:"1"*64 for name in macho.COLLECTORS},
                  capture_evidence={**{name:"2"*64 for name in ("capture_sha256","build_sha256","audit_sha256","smoke_sha256","oracle_reference_sha256")},
                                    "samples":5544,"warmups":264,"audited_functions":175})
    names = sorted({c["workload"] for c in declared["cases"]})
    result["native_programs"] = {name:dict(source=macho.source_name(name),source_sha256=result["input_sources"][macho.source_name(name)],
        binary_sha256="3"*64,matched_assembly_sha256="4"*64,linux_assembly_sha256="5"*64,audit_sha256="6"*64,
        reserved_register_rename=None,audited_functions=3 if i==0 else 4) for i,name in enumerate(names)}
    for row in result["cases"]:
        row.update(batch_median_user_seconds=[row["median_user_seconds"]]*3,batch_median_spread=0)
    return performance.validate(result, declared)
