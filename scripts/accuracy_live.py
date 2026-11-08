"""Local snapshot transport; all HTML and statistics belong to accuracy_site."""
import copy
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import hashlib
import math
from pathlib import Path
from urllib.parse import urlsplit

import accuracy_site as site
import model_profiles
import usertime

KIND = "poros-accuracy-snapshot-v1"


def validate_snapshot(value):
    if not isinstance(value, dict) or value.get("kind") != KIND:
        raise ValueError("unsupported live snapshot")
    report, progress = value["report"], value["progress"]
    if not isinstance(report, dict) or not isinstance(progress, dict) or not isinstance(report.get("head"), dict):
        raise ValueError("invalid snapshot report/progress")
    if (report.get("schema") != 4 or report.get("artifact_status") != "partial"
            or report.get("accuracy_status") != "unverified" or report.get("formula") != site.FORMULA
            or report["head"].get("dirty") is not False):
        raise ValueError("snapshot must remain partial and unpublishable")
    site.sha(report["head"]["commit"]); site.sha(report["base_commit"])
    profiles = model_profiles.validate(report)
    declared = site.report_manifest(report)
    cases = {c["id"]: c for c in declared["cases"]}
    ids = [r["id"] for r in report["cases"]]
    if ids != [ident for ident in cases if ident in ids]:
        raise ValueError("snapshot cases duplicated, unknown or reordered")
    for row in report["cases"]:
        case = cases[row["id"]]
        real = row["reference_user_seconds"]
        if not isinstance(real, (float, int)) or not math.isfinite(real) or real <= 0:
            raise ValueError("invalid snapshot reference time")
        if row["reference_quality"] not in ("usable", "noisy") or row.get("functional_status") != "pass" or "head" not in row:
            raise ValueError("snapshot requires validated completed cases")
        for role in ("head", "base"):
            if role not in row:
                continue
            prediction = row[role]
            seconds = prediction["predicted_user_seconds"]
            if (type(prediction["cycles"]) is not int or prediction["cycles"] <= 0
                    or type(prediction["instructions"]) is not int or prediction["instructions"] <= 0
                    or any(prediction[k] != profiles[role][k] for k in ("configuration", "cache_config"))
                    or not math.isfinite(seconds) or not math.isclose(seconds, prediction["cycles"] / case["frequency_hz"], rel_tol=1e-12)):
                raise ValueError("invalid snapshot timing/profile")
            if row["reference_quality"] == "usable" and not math.isclose(prediction["error_percent"], usertime.error_percent(seconds, real), rel_tol=1e-12, abs_tol=1e-12):
                raise ValueError("snapshot error differs from timing")
    if (progress["state"] not in ("running", "waiting", "failed", "complete")
            or progress["completed_cases"] != len(ids) or progress["required_cases"] != len(cases)
            or type(progress["completed_shards"]) is not int or type(progress["required_shards"]) is not int
            or not 0 <= progress["completed_shards"] <= progress["required_shards"]
            or progress["required_shards"] <= 0
            or not isinstance(progress["errors"], list) or not all(isinstance(e, str) for e in progress["errors"])
            or not isinstance(progress["active_cases"], list)
            or any(i not in cases or i in ids for i in progress["active_cases"])):
        raise ValueError("invalid snapshot progress")
    if datetime.fromisoformat(progress["updated_at"]).utcoffset() is None:
        raise ValueError("snapshot timestamp needs timezone")
    if progress["state"] == "complete" and (len(ids) != len(cases) or progress["errors"] or progress["active_cases"] or progress["completed_shards"] != progress["required_shards"]):
        raise ValueError("incomplete snapshot marked complete")
    reference = value.get("throughput_reference")
    if reference is not None:
        site.throughput.validate_reference(reference, report["head"]["files"])
        site.throughput.validate_simulation(report["throughput_simulation"], reference["cases"])
        provenance = value["throughput_provenance"]
        if (provenance["commit"] != report["head"]["commit"]
                or provenance["binary_sha256"] != report["head_binary_sha256"]
                or provenance["model_profile"] != report["model_profiles"]["head"]):
            raise ValueError("snapshot throughput revision mismatch")
    for name, suite in (("forwarding", site.forwarding), ("scheduling", site.scheduling), ("memory", site.memory)):
        if name + "_reference" in value:
            raise ValueError(name + " reference must use the exact byte-bound text transport")
        reference_text = value.get(name + "_reference_text")
        if suite.declared(report["head"]["files"]):
            if (not isinstance(reference_text, str)
                    or hashlib.sha256(reference_text.encode()).hexdigest() != report["head"]["files"].get(suite.REFERENCE)):
                raise ValueError("snapshot " + name + " reference bytes differ from tested source")
            reference = json.loads(reference_text)
            suite.validate_reference(reference, report["head"]["files"])
            suite.validate_comparison(report.get(name + "_simulation"), reference["cases"])
            expected = {key: report[key] for key in ("base_commit", "head_binary_sha256", "base_binary_sha256", "model_profiles")}
            expected["head_commit"] = report["head"]["commit"]
            if value.get(name + "_provenance") != expected:
                raise ValueError("snapshot " + name + " revision mismatch")
        elif reference_text is not None or name + "_simulation" in report or name + "_provenance" in value:
            raise ValueError("snapshot " + name + " suite absent from source")
    return value


class SnapshotView:
    def __init__(self, snapshot, bundle=None):
        self.snapshot = Path(snapshot)
        self.bundle = Path(bundle) if bundle else None
        self.previous = None

    def load(self):
        try:
            value = validate_snapshot(site.read(self.snapshot))
            self.previous = copy.deepcopy(value)
        except (OSError, ValueError, KeyError, TypeError, AttributeError) as error:
            if self.previous is None:
                raise ValueError("no valid live snapshot available") from error
            value = copy.deepcopy(self.previous)
            value["progress"].update(state="waiting", errors=["Snapshot temporarily unavailable; showing last validated snapshot."])
        report = value["report"]
        live = dict(value["progress"], validated_bundle=False)
        age = (datetime.now(timezone.utc) - datetime.fromisoformat(live["updated_at"])).total_seconds()
        if age > 45 and live["state"] in ("running", "waiting"):
            live.update(state="waiting", errors=[*live["errors"], "Snapshot producer has not updated for more than 45 seconds; displayed progress may be stale."])
        entry = dict(commit=report["head"]["commit"], measurement_completed_at=report.get("reference", {}).get("completed_at_utc"))
        assets = {"snapshot.json": json.dumps(value, indent=2).encode()}
        throughput = value.get("throughput_reference")
        frontend = None
        forwarding = json.loads(value["forwarding_reference_text"]) if "forwarding_reference_text" in value else None
        scheduling = json.loads(value["scheduling_reference_text"]) if "scheduling_reference_text" in value else None
        memory = json.loads(value["memory_reference_text"]) if "memory_reference_text" in value else None
        if memory is not None:
            assets["memory.json"] = value["memory_reference_text"].encode()
        if scheduling is not None:
            assets["scheduling.json"] = value["scheduling_reference_text"].encode()
        if forwarding is not None:
            assets["forwarding.json"] = value["forwarding_reference_text"].encode()
        if throughput is not None:
            assets["throughput.json"] = json.dumps(throughput, indent=2).encode()
        if live["state"] == "complete" and self.bundle is not None:
            try:
                completed, manifest = site.validate_bundle(self.bundle, entry["commit"])
                identity_fields = ("head", "base_commit", "head_binary_sha256", "base_binary_sha256",
                                   "model_profiles", "workload_manifest", "reference_sha256", "reference")
                if completed["artifact_status"] != "pass" or any(completed.get(key) != report.get(key) for key in identity_fields):
                    raise ValueError("final bundle is not a matching complete result")
                completed_assets = {name: site.regular(self.bundle / name).read_bytes() for name in manifest["files"]}
                if any(hashlib.sha256(completed_assets[name]).hexdigest() != expected for name, expected in manifest["files"].items()):
                    raise ValueError("bundle changed during snapshot transition")
                completed_assets["bundle.json"] = json.dumps(manifest, indent=2).encode()
                measurement_completed = site.measurement_completed(completed, self.bundle)
                completed_throughput = json.loads(completed_assets["throughput.json"]) if "throughput.json" in completed_assets else None
                completed_forwarding = json.loads(completed_assets["forwarding.json"]) if "forwarding.json" in completed_assets else None
                completed_scheduling = json.loads(completed_assets["scheduling.json"]) if "scheduling.json" in completed_assets else None
                completed_memory = json.loads(completed_assets["memory.json"]) if "memory.json" in completed_assets else None
                completed_frontend = json.loads(completed_assets["frontend.json"]) if "frontend.json" in completed_assets else None
                # Commit the transition only after all verified assets are ready.
                report = completed
                live["validated_bundle"] = True
                entry.update(evidence_files=sorted(manifest["files"]), measurement_completed_at=measurement_completed)
                assets = {"snapshot.json": assets["snapshot.json"], **completed_assets}
                throughput, frontend, forwarding, scheduling, memory = completed_throughput, completed_frontend, completed_forwarding, completed_scheduling, completed_memory
            except (OSError, ValueError, KeyError, TypeError, AttributeError) as error:
                live.update(state="failed", errors=["Final bundle validation failed: " + str(error)])
        return site.detail(report, entry, preview=True, frontend=frontend, throughput_reference=throughput, live=live, forwarding_reference=forwarding, scheduling_reference=scheduling, memory_reference=memory).encode(), assets


def handler(view):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            try:
                page, assets = view.load()
                path = urlsplit(self.path).path
                if path == "/":
                    data, content_type = page, "text/html; charset=utf-8"
                elif path[1:] in assets:
                    data = assets[path[1:]]
                    content_type = "application/json" if path.endswith(".json") else "application/octet-stream"
                else:
                    self.send_error(404)
                    return
                self.send_response(200)
                self.send_header("Content-Type", content_type)
                self.send_header("Cache-Control", "no-store")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)
            except (OSError, ValueError, KeyError, TypeError, AttributeError):
                self.send_error(503, "Live snapshot temporarily unavailable")
    return Handler


def serve(snapshot, bundle, port):
    # Only loopback; request paths are looked up in a validated asset map.
    ThreadingHTTPServer(("127.0.0.1", port), handler(SnapshotView(snapshot, bundle))).serve_forever()
