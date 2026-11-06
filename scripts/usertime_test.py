import base64
import copy
import importlib.util
import math
import statistics
from pathlib import Path
import unittest
import json
import tempfile
from types import SimpleNamespace
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "gate", Path(__file__).with_name("usertime.py")
)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


class UserTimeTests(unittest.TestCase):

    def test_symmetric_error(self):
        self.assertEqual(gate.error_percent(2, 1), 100)
        self.assertEqual(gate.error_percent(0.5, 1), 100)
        self.assertEqual(gate.error_percent(1, 1), 0)
        for value in (0, -1, math.nan, math.inf):
            with self.assertRaises(ValueError):
                gate.error_percent(value, 1)

    def test_invalid_samples(self):
        self.assertEqual(gate.quality([dict(user_seconds=0)]), "invalid_nonpositive")
        self.assertEqual(
            gate.quality([dict(user_seconds=1e-06)] * 5), "below_precision_floor"
        )
        self.assertEqual(gate.quality([dict(user_seconds=0.001)] * 5), "usable")
        self.assertEqual(
            gate.quality(
                [dict(user_seconds=x) for x in (0.001, 0.001, 0.002, 0.004, 0.005)]
            ),
            "noisy",
        )

    def test_inventory(self):
        self.assertEqual(len(gate.inventory()["cases"]), 24)

    def test_native_child_accounting(self):
        result = gate.child_usage("/bin/sh", ["sh", "-c", "printf hi"], {})
        self.assertEqual(result["exit_status"], 0)
        self.assertEqual(base64.b64decode(result["stdout_base64"]), b"hi")
        self.assertGreaterEqual(result["user_seconds"], 0)

    def test_report_fail_closed(self):
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / "report.json"
            args = SimpleNamespace(
                head="/missing",
                base=None,
                base_unavailable=True,
                base_commit="wrong",
                reference="/missing",
                output=str(output),
            )
            with patch.object(
                gate,
                "source_state",
                return_value=dict(commit="head", dirty=False, files={}),
            ), patch.object(gate, "digest", return_value="hash"):
                self.assertEqual(gate.compare24(args), 1)
            report = json.loads(output.read_text())
            self.assertEqual(report["artifact_status"], "fail")
            self.assertEqual(report["accuracy_status"], "unverified")

    def test_inventory_deletion_rejected(self):
        manifest = gate.inventory()
        manifest["cases"].pop()
        original = Path.read_text

        def altered(path, *a, **kw):
            return (
                json.dumps(manifest)
                if path.name == "performance.json"
                else original(path, *a, **kw)
            )

        with patch.object(Path, "read_text", altered):
            with self.assertRaisesRegex(ValueError, "required user-time cases"):
                gate.inventory()

    def test_report_inventory_provenance_and_base_failures(self):
        manifest = gate.inventory()
        golden = b"golden"
        reference = dict(
            schema=2, measurement_id="unit", collection_commit="a"*40, collection_dirty=False,
            container_image="sha256:"+"f"*64, retained_samples_per_case=21, precision_floor_seconds=.0001,
            quality_policy={"nonpositive":"reject-any","precision_floor_seconds":.0001,"relative_p10_p90_limit":.2},
            metric="child_user_cpu_seconds",timer="wait4.ru_utime",machine="aarch64",
            started_at_utc="2026-01-01T00:00:00Z",completed_at_utc="2026-01-01T00:05:00Z",
            environment="virtualized",
            hardware="unit",
            platform="unit",
            core_identity="unknown",
            instantaneous_frequency="unknown",
            harness_sha256=gate.digest(gate.native.__file__),
            support_sha256={
                "workload_contract.py": gate.digest(
                    gate.ROOT / "scripts/workload_contract.py"
                )
            },
            manifest_sha256=gate.digest(gate.ROOT / "tests/usertime/performance.json"),
            cases=[],
        )
        for c in manifest["cases"]:
            reference["cases"].append(
                dict(
                    c,
                    samples=[
                        dict(
                            user_seconds=0.001, system_seconds=0,
                            exit_status=0,
                            stdout_base64=base64.b64encode(golden).decode(),
                            stderr_base64="",
                        )
                        for _ in range(21)
                    ],
                    warmup_sample=dict(user_seconds=.001, system_seconds=0, exit_status=0, stdout_base64=base64.b64encode(golden).decode(), stderr_base64=""),
                    quality="usable",
                    median_user_seconds=0.001,
                )
            )
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            head = root / "head"
            head.write_text("binary")
            ref = root / "ref"
            output = root / "report.json"
            args = SimpleNamespace(
                head=str(head),
                base=str(head),
                base_unavailable=False,
                base_commit="ec5df8efdd80fd02a0db7bd80e9ae413c509852b",
                reference=str(ref),
                output=str(output),
            )

            def execution(binary, c, mode=None):
                return dict(
                    instructions=10,
                    stdout_base64=base64.b64encode(golden).decode(),
                    build={"vcs.revision": "head", "vcs.modified": "false"},
                    timing=dict(
                        predicted_user_seconds=10 / c["frequency_hz"],
                        user_cycles=10,
                        retired_instructions=10,
                        cpi=1,
                        cache={"L1Hits": 0, "L1Misses": 0, "L2Hits": 0, "L2Misses": 0},
                        branch_predictions=0,
                        branch_direction_hits=0,
                        configuration=manifest["profile"]["configuration"],
                        cache_config=manifest["profile"]["cache_config"],
                    ),
                )

            def capability(revision):
                if revision == "arbitrary-missing-feature":
                    raise ValueError("unrecognized unavailable base")
                return {"image_limit_bytes": 256 << 20 if revision == "base" else 0, "output_limit_bytes":64 << 20}

            def run(data, runner=execution):
                ref.write_text(json.dumps(data))
                with patch("compute_usertime.base_capability", side_effect=capability), patch.object(gate, "check_output"), patch.object(
                    gate, "expected", return_value=golden
                ), patch.object(
                    gate,
                    "source_state",
                    return_value=dict(commit="head", dirty=False, files={}),
                ), patch.object(
                    gate, "execute", side_effect=runner
                ):
                    return gate.compare24(args)

            self.assertEqual(run(reference), 0)
            self.assertEqual(
                json.loads(output.read_text())["accuracy_status"], "unverified"
            )
            for mutate in (
                lambda r: r["cases"].pop(),
                lambda r: r["cases"][0]["args"].append("changed"),
                lambda r: r.update(harness_sha256="tampered"),
                lambda r: r["cases"][0].update(quality="invalid_nonpositive"),
            ):
                changed = copy.deepcopy(reference)
                mutate(changed)
                self.assertEqual(run(changed), 1)

            def missing(binary, c, mode=None):
                raise ValueError("head missing case")

            self.assertEqual(run(reference, missing), 1)

            def wrong_revision(binary, c, mode=None):
                r = execution(binary, c)
                r["build"]["vcs.revision"] = "wrong"
                return r

            self.assertEqual(run(reference, wrong_revision), 1)
            # Noisy references must still reject malformed simulator metrics.
            noisy = copy.deepcopy(reference)
            for row in noisy["cases"]:
                for sample in row["samples"][:8]:
                    sample["user_seconds"] = .004
                row["quality"] = gate.quality(row["samples"])
                self.assertEqual(row["quality"], "noisy")
                row["median_user_seconds"] = statistics.median(
                    (sample["user_seconds"] for sample in row["samples"])
                )
            for key, bad in (
                ("predicted_user_seconds", 0),
                ("predicted_user_seconds", math.nan),
                ("predicted_user_seconds", 0.001),
                ("user_cycles", -1),
                ("user_cycles", True),
                ("retired_instructions", 9),
                ("cpi", math.inf),
                ("cpi", 99),
                ("branch_direction_hits", 1),
            ):

                def malformed(binary, c, mode=None):
                    r = execution(binary, c)
                    r["timing"][key] = bad
                    return r

                self.assertEqual(run(noisy, malformed), 1, key)
            args.base_commit = "arbitrary-missing-feature"
            self.assertEqual(run(reference), 1)
            args.base_commit = "base"
            base = root / "base"
            base.write_text("base binary")
            args.base = str(base)
            args.base_unavailable = False

            def available(binary, case, mode=None):
                result = execution(binary, case)
                is_base = binary == str(base)
                result["build"]["vcs.revision"] = "base" if is_base else "head"
                cycles = int(case["frequency_hz"] / 1000) * (2 if is_base else 1)
                result["timing"].update(
                    user_cycles=cycles,
                    predicted_user_seconds=cycles / case["frequency_hz"],
                    cpi=cycles / result["instructions"],
                )
                return result

            self.assertEqual(run(reference, available), 0)
            report = json.loads(output.read_text())
            self.assertEqual(report["base_status"], "available")
            for row in report["cases"]:
                self.assertEqual(row["base"]["error_percent"], 100)
                self.assertEqual(row["head"]["error_percent"], 0)
                self.assertEqual(row["error_change_percentage_points"], -100)

            def failed_base(binary, case, mode=None):
                if binary == str(base):
                    raise ValueError("base-only execution failure")
                return available(binary, case)

            self.assertEqual(run(reference, failed_base), 1)
            report = json.loads(output.read_text())
            self.assertEqual(report["base_status"], "available")
            self.assertIn("base-only execution failure", report["error"])

    def test_execute_rejects_changed_profile_and_input(self):
        c = gate.inventory()["cases"][0]
        profile = gate.inventory()["profile"]
        result = dict(
            binary_sha256=c["sha256"],
            args=c["args"],
            env=c["env"],
            termination="exit",
            exit_status=0,
            stdout_base64=base64.b64encode(b"golden").decode(),
            stderr_base64="",
            instructions=10,
            timing=dict(
                model=c["model"],
                assumed_hz=c["frequency_hz"],
                retired_instructions=10,
                **profile
            ),
        )

        def run(r):
            with patch.object(gate, "check_output"), patch.object(
                gate, "expected", return_value=b"golden"
            ), patch.object(
                gate.subprocess,
                "run",
                return_value=SimpleNamespace(
                    returncode=0, stdout=json.dumps(r), stderr=b""
                ),
            ):
                return gate.execute("fake", c)

        run(result)
        for mutate in (
            lambda r: r.update(binary_sha256="bad"),
            lambda r: r["timing"]["configuration"].update(num_int=99),
            lambda r: r["timing"]["cache_config"].update(L1Latency=100),
        ):
            changed = copy.deepcopy(result)
            mutate(changed)
            with self.assertRaises(ValueError):
                run(changed)


if __name__ == "__main__":
    unittest.main()
