#!/usr/bin/env python3
"""Exact-target backfill routing and failed/retried artifact boundaries."""
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import accuracy_backfill as backfill
import accuracy_site as site
import publish_accuracy as publisher


class BackfillTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);self.source=self.root/"source";self.source.mkdir()
        self.commit="a"*40;self.target=dict(kind="main",commit=self.commit)

    def test_explicit_modern_legacy_and_reused_routes(self):
        with patch.object(site,"git",return_value=self.commit):
            record=backfill.route(self.source,self.root/"pages",self.root/"legacy",self.target)
            self.assertEqual(record["route"],"legacy")
            site.write(self.source/"tests/usertime/performance.json",dict(schema=2,suite="whole-program-performance-six-point-v1"))
            (self.source/"scripts").mkdir();(self.source/"scripts/accuracy_parallel.py").write_text("unit")
            record=backfill.route(self.source,self.root/"pages",self.root/"parallel",self.target)
            self.assertEqual(record["route"],"parallel")
            archived=self.root/"pages/calibration/runs"/self.commit;archived.mkdir(parents=True)
            (archived/"proof").write_text("validated bundle")
            with patch.object(site,"reusable_bundle",return_value=True):
                record=backfill.route(self.source,self.root/"pages",self.root/"reuse",dict(self.target,kind="release"))
            self.assertEqual(record["route"],"reuse")
            self.assertEqual((self.root/"reuse/evidence/proof").read_text(),"validated bundle")
            site.write(self.source/"tests/usertime/performance.json",dict(schema=99,suite="unknown"))
            with self.assertRaisesRegex(ValueError,"unknown performance"):
                backfill.route(self.source,self.root/"pages",self.root/"unknown",self.target)
        with patch.object(site,"git",return_value="b"*40):
            with self.assertRaisesRegex(ValueError,"resolved target"):
                backfill.route(self.source,self.root/"pages",self.root/"wrong",self.target)

    def test_missing_or_failed_parallel_produces_failure_without_serial_fallback(self):
        resolved=self.root/"resolved";site.write(resolved/"backfill.json",dict(schema=1,target=self.target,route="parallel",base="b"*40))
        for status in ("success","failure","cancelled"):
            output=self.root/status
            with patch.object(site,"git",return_value=self.commit):
                self.assertEqual(backfill.package(self.source,resolved,self.root/"missing",output,self.commit,status),1)
            report,_=site.validate_bundle(output,self.commit)
            self.assertEqual(report["artifact_status"],"fail")
            self.assertEqual(site.read(output/"publication-target.json"),self.target)

    def test_stale_success_cannot_mask_failed_new_attempt(self):
        resolved=self.root/"resolved";site.write(resolved/"backfill.json",dict(schema=1,target=self.target,route="parallel",base="b"*40))
        selected=self.root/"selected";site.write(selected/"bundle.json",{"old":"success"})
        with patch.object(site,"git",return_value=self.commit),patch.object(publisher,"verify_source",side_effect=AssertionError("must not consume stale success")):
            self.assertEqual(backfill.package(self.source,resolved,selected,self.root/"out",self.commit,"failure"),1)

    def test_resolved_artifact_retry_has_explicit_sentinel(self):
        artifacts=dict(artifacts=[dict(name="accuracy-backfill-resolved-1",expired=False),dict(name="accuracy-backfill-resolved-3",expired=False)])
        destination=self.root/"resolved";site.write(destination/"backfill.json",dict(target=self.target))
        with patch.object(publisher,"api",return_value=artifacts),patch.object(publisher,"run",return_value=subprocess.CompletedProcess([],0)):
            self.assertEqual(publisher.download_attempt_bundle(12,2,"accuracy-backfill-resolved",destination,required="backfill.json"),1)
            self.assertIsNone(publisher.download_attempt_bundle(12,2,"accuracy-backfill-resolved",destination))

    def test_shared_workflow_is_read_only_and_has_one_parallel_matrix(self):
        root=Path(__file__).resolve().parents[1]
        evaluation=(root/".github/workflows/evaluation.yml").read_text()
        parallel=(root/".github/workflows/accuracy-evaluate.yml").read_text()
        pages=(root/".github/workflows/accuracy-pages.yml").read_text()
        self.assertIn("uses: ./.github/workflows/accuracy-evaluate.yml",evaluation)
        self.assertIn("uses: ./.github/workflows/accuracy-evaluate.yml",pages)
        self.assertNotIn("contents: write",parallel)
        self.assertIn("fail-fast: false",parallel)
        self.assertIn("max-parallel: 16",parallel)
        self.assertIn("if: always()",parallel)
        self.assertIn("name: User-time accuracy report",parallel)
        self.assertIn("actions: read",pages)
        self.assertIn('if: always() && needs.resolve.result == \'success\'',pages)
        self.assertIn('--route-status "$ROUTE_STATUS"',pages)


if __name__=="__main__":unittest.main()
