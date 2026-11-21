#!/usr/bin/env python3
"""Model revisions may differ without changing or weakening native evidence."""
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import accuracy_site as site
import accuracy_test_support as fixtures
import compute_usertime as compute
import devices
import model_profiles as models
import performance_reference as reference
import publish_accuracy as publisher

ROOT = Path(__file__).resolve().parents[1]


class ModelProfileTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.temporary.cleanup)
        cls.root = Path(cls.temporary.name)
        cls.fixture = fixtures.Evidence(ROOT, cls.root / "fixture")
        cls.source = cls.fixture.source
        for name, value in (("pkg/loader/elf.go", "loader_source"), ("pkg/linuxuser/run.go", "runtime_source")):
            path = cls.source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(cls.fixture.report["base_capability"][value])
        cls.old = copy.deepcopy(cls.fixture.report["workload_manifest"]["profile"])
        cls.new = copy.deepcopy(cls.old)
        cls.new["configuration"]["fetch_width"] = 8
        cls.new["configuration"].update(num_load=3, num_store=2)
        cls.new["cache_config"]["DisableAdjacentL1Prefetch"] = True
        cls.git("init"); cls.git("config", "user.name", "Unit test"); cls.git("config", "user.email", "unit@example.invalid")
        cls.git("add", "."); cls.git("commit", "-m", "Legacy collection")
        cls.legacy = cls.git("rev-parse", "HEAD")
        declaration = cls.source / models.PATH
        declaration.parent.mkdir(parents=True, exist_ok=True)
        declaration.write_bytes(models.encode(cls.old))
        cls.git("add", "."); cls.git("commit", "-m", "Base model declaration")
        cls.base = cls.git("rev-parse", "HEAD")
        measured = compute.read(cls.source / reference.PATH)
        measured.update(collection_commit=cls.legacy, oracle_commit=cls.legacy)
        compute.write(cls.source / reference.PATH, measured)
        declaration.write_bytes(models.encode(cls.new))
        cls.git("add", "."); cls.git("commit", "-m", "Head model and compact evidence")
        cls.head = cls.git("rev-parse", "HEAD")
        cls.report = fixtures.compact_report(cls.fixture)
        cls.report.update(base_commit=cls.base, reference_sha256=site.digest(cls.source / reference.PATH),
                          reference={k:v for k,v in measured.items() if k not in ("cases", "input_sources", "collector_sources")})
        cls.report["head"] = dict(commit=cls.head, dirty=False, files={p:site.digest(cls.source / p) for p in cls.git("ls-files").splitlines()})
        cls.report["model_profiles"] = models.resolve(cls.source, cls.report["head"], cls.base)
        for row in cls.report["cases"]:
            row["head"].update(copy.deepcopy(cls.new))
        raw = cls.root / "report.json"
        compute.write(raw, cls.report)
        raw.with_suffix(".md").write_text("Synthetic profile transport test, not measured performance")
        cls.bundle = cls.root / "bundle"
        site.bundle(raw, cls.source, cls.bundle, cls.head)

    @classmethod
    def git(cls, *args):
        return subprocess.check_output(["git", "-C", str(cls.source), *args], text=True, stderr=subprocess.DEVNULL).strip()

    def test_distinct_profiles_roundtrip_and_publisher_bind_both_revisions(self):
        self.assertEqual(models.validate(self.report), {"head":self.new, "base":self.old})
        validated, _ = site.validate_bundle(self.bundle, self.head)
        self.assertEqual(validated, self.report)
        publisher.verify_source(self.source, self.bundle, self.head)
        # Even coherently rehashed base metadata must match the actual base Git commit.
        forged = copy.deepcopy(self.report)
        entry = forged["model_profiles"]["base"]
        entry["profile"] = copy.deepcopy(self.new)
        entry["sha256"] = hashlib.sha256(models.encode(self.new)).hexdigest()
        with patch.object(site, "validate_bundle", return_value=(forged, {})):
            with self.assertRaisesRegex(ValueError, "base model declaration.*Git"):
                publisher.verify_source(self.source, self.bundle, self.head)

    def test_modern_diagnostic_profile_does_not_require_application_manifest(self):
        report = copy.deepcopy(self.report)
        report.pop('workload_manifest')
        self.assertEqual(models.validate(report), {'head': self.new, 'base': self.old})
        report['head']['files'].pop(models.PATH)
        report.pop('model_profiles')
        with self.assertRaisesRegex(ValueError, 'missing legacy'):
            models.validate(report)

    def test_new_fields_are_revision_bound_without_normalizing_archives(self):
        profiles = models.validate(self.report)
        self.assertNotIn("num_load", profiles["base"]["configuration"])
        self.assertNotIn("num_store", profiles["base"]["configuration"])
        self.assertNotIn("DisableAdjacentL1Prefetch", profiles["base"]["cache_config"])
        self.assertEqual(profiles["head"]["configuration"]["num_load"], 3)
        self.assertEqual(profiles["head"]["configuration"]["num_store"], 2)
        self.assertIs(profiles["head"]["cache_config"]["DisableAdjacentL1Prefetch"], True)
        # An old-shaped row cannot silently inherit the new declaration.
        for section, key in (("configuration", "num_load"),
                             ("configuration", "num_store"),
                             ("cache_config", "DisableAdjacentL1Prefetch")):
            bad = copy.deepcopy(self.report)
            bad["cases"][0]["head"][section].pop(key)
            raw = self.root / ("missing-" + key + ".json")
            compute.write(raw, bad)
            raw.with_suffix(".md").write_text("Synthetic missing configuration field")
            with self.assertRaisesRegex(ValueError, "profile/count mismatch"):
                site.bundle(raw, self.source, self.root / ("missing-" + key), self.head)

    def test_missing_declaration_changed_profiles_and_both_hashes_fail(self):
        for mutate in (lambda r:r.pop("model_profiles"),
                       lambda r:r["model_profiles"]["base"]["profile"]["configuration"].update(fetch_width=99),
                       lambda r:r["model_profiles"]["head"]["profile"]["configuration"].update(fetch_width=99),
                       lambda r:r["model_profiles"]["base"].update(sha256="0"*64),
                       lambda r:r["model_profiles"]["head"].update(sha256="0"*64),
                       lambda r:r["model_profiles"]["base"].update(commit=self.head)):
            bad = copy.deepcopy(self.report); mutate(bad)
            with self.assertRaises(ValueError):
                models.validate(bad)

    def test_non_object_role_declarations_fail_validation(self):
        for role in ("head", "base"):
            for value in (None, [], "invalid", 8, False):
                with self.subTest(role=role, value=value):
                    bad = copy.deepcopy(self.report)
                    bad["model_profiles"][role] = value
                    with self.assertRaisesRegex(ValueError, role + " model declaration must be an object"):
                        models.validate(bad)

    def test_legacy_base_uses_its_revision_manifest(self):
        report = copy.deepcopy(self.report)
        report["base_commit"] = self.legacy
        report["model_profiles"] = models.resolve(self.source, report["head"], self.legacy)
        self.assertEqual(report["model_profiles"]["base"]["path"], models.LEGACY_PATH)
        self.assertEqual(models.verify_source(self.source, report)["base"], self.old)
        report["model_profiles"]["base"]["sha256"] = "0"*64
        with self.assertRaisesRegex(ValueError, "exact Git revision"):
            models.verify_source(self.source, report)

    def test_fallback_precedence_and_fail_closed(self):
        with tempfile.TemporaryDirectory() as folder:
            source = Path(folder)
            def git(*args):
                return subprocess.check_output(["git", "-C", folder, *args], text=True, stderr=subprocess.DEVNULL).strip()
            def commit():
                git("add", "."); git("commit", "-m", "Profile selection fixture")
                return git("rev-parse", "HEAD")
            git("init"); git("config", "user.name", "Unit test"); git("config", "user.email", "unit@example.invalid")
            (source / "README").write_text("No profile yet")
            with self.assertRaisesRegex(ValueError, "no declared model profile"):
                models.revision(source, commit())
            oldest = source / models.LEGACY_PATHS[1]
            oldest.parent.mkdir(parents=True)
            compute.write(oldest, {"profile": self.old})
            selected = models.revision(source, commit())
            self.assertEqual(selected["path"], models.LEGACY_PATHS[1])
            self.assertEqual(selected["sha256"], site.digest(oldest))
            recent = source / models.LEGACY_PATH
            compute.write(recent, {"profile": self.new})
            self.assertEqual(models.revision(source, commit())["profile"], self.new)
            declaration = source / models.PATH
            declaration.parent.mkdir(parents=True)
            declaration.write_bytes(models.encode(self.old))
            self.assertEqual(models.revision(source, commit())["path"], models.PATH)
            declaration.write_text("{}")
            with self.assertRaisesRegex(ValueError, "invalid model profile"):
                models.revision(source, commit())

    def test_rehashed_wrong_row_configuration_is_rejected(self):
        for label in ("head", "base"):
            bad = copy.deepcopy(self.report)
            row = next(r for r in bad["cases"] if label in r)
            row[label]["configuration"]["fetch_width"] = 16
            folder = self.root / ("bad-" + label)
            raw = self.root / ("bad-" + label + ".json")
            compute.write(raw,bad);raw.with_suffix(".md").write_text("Synthetic tampered data")
            with self.assertRaisesRegex(ValueError,"profile/count mismatch"):
                site.bundle(raw,self.source,folder,self.head)


class DeviceProfileTests(unittest.TestCase):
    """A device with parameters of its own is bound to its own profile, at each revision separately."""

    def test_each_role_binds_the_profile_its_revision_declares_for_the_device(self):
        grace_path = "evaluation/model-profile-grace.json"
        m2, grace = compute.read(ROOT / models.PATH), compute.read(ROOT / grace_path)
        declaration = devices.load()
        # Both revisions of this synthetic history predate M5 Max calibration.
        devices.select(declaration, "m5-max")["parameter_source"] = "m2"
        placeholder = copy.deepcopy(declaration)
        devices.select(placeholder, "grace")["parameter_source"] = "m2"
        with tempfile.TemporaryDirectory() as folder:
            source = Path(folder)
            def git(*args):
                return subprocess.check_output(["git", "-C", folder, *args], text=True, stderr=subprocess.DEVNULL).strip()
            def commit(message):
                git("add", "-A"); git("commit", "-m", message)
                revision = git("rev-parse", "HEAD")
                return dict(commit=revision, files={p: site.digest(source / p) for p in git("ls-files").splitlines()})
            git("init"); git("config", "user.name", "Unit test"); git("config", "user.email", "unit@example.invalid")
            (source / "evaluation").mkdir()
            (source / models.PATH).write_bytes(models.encode(m2))
            (source / devices.PATH).write_bytes(devices.encode(placeholder))
            base = commit("Grace carries the M2 parameters")["commit"]
            (source / devices.PATH).write_bytes(devices.encode(declaration))
            (source / grace_path).write_bytes(models.encode(grace))
            head = commit("Grace has parameters of its own")

            chosen = devices.resolve(source, head, base, "grace")
            bound = models.resolve(source, head, base, chosen)
            self.assertEqual((bound["head"]["path"], bound["head"]["profile"]), (grace_path, grace))
            self.assertEqual((bound["base"]["path"], bound["base"]["profile"]), (models.PATH, m2))
            self.assertEqual(bound["head"]["sha256"], head["files"][grace_path])
            report = dict(head=head, base_commit=base, device=chosen, model_profiles=bound)
            self.assertEqual(models.verify_source(source, report), {"head": grace, "base": m2})
            # Once the base declares Grace's parameters too, both roles are the Grace profile.
            both = models.resolve(source, head, head["commit"], devices.resolve(source, head, head["commit"], "grace"))
            self.assertEqual((both["head"], both["base"]["path"]), (both["base"], grace_path))
            # The default device, a device that carries its parameters, and no device at all keep the M2 profile.
            for ident in ("m2", "m5-max", None):
                other = models.resolve(source, head, base, None if ident is None else devices.resolve(source, head, base, ident))
                self.assertEqual((other["head"]["path"], other["head"]["profile"], other["base"]["path"]), (models.PATH, m2, models.PATH))

            # Another device's profile in either role is rejected offline, coherent hashes included.
            def claimed(role, entry):
                bad = copy.deepcopy(report)
                bad["model_profiles"][role] = dict(copy.deepcopy(entry), commit=report["model_profiles"][role]["commit"])
                return bad
            for bad in (claimed("head", bound["base"]), claimed("base", bound["head"])):
                with self.assertRaisesRegex(ValueError, "model profile source identity mismatch"):
                    models.validate(bad)
            edited = copy.deepcopy(report)
            edited["model_profiles"]["head"]["profile"]["configuration"]["freq"] = 3490000000
            with self.assertRaisesRegex(ValueError, "declaration hash mismatch"):
                models.validate(edited)
            edited["model_profiles"]["head"]["sha256"] = hashlib.sha256(models.encode(edited["model_profiles"]["head"]["profile"])).hexdigest()
            with self.assertRaisesRegex(ValueError, "head model declaration differs from tested source"):
                models.validate(edited)
            # Rehashed into the source inventory as well, only Git shows the difference.
            edited["head"] = dict(head, files=dict(head["files"], **{grace_path: edited["model_profiles"]["head"]["sha256"]}))
            models.validate(edited)
            with self.assertRaisesRegex(ValueError, "head model declaration differs from exact Git revision"):
                models.verify_source(source, edited)

            # A revision that declares the device's own parameters must also declare its profile, canonically.
            (source / grace_path).write_text(json.dumps(grace))
            loose = commit("Non-canonical Grace profile")
            with self.assertRaisesRegex(ValueError, "canonical JSON"):
                models.resolve(source, loose, base, devices.resolve(source, loose, base, "grace"))
            (source / grace_path).unlink()
            missing = commit("Grace profile removed")
            with self.assertRaisesRegex(ValueError, "no declared model profile"):
                models.resolve(source, missing, base, devices.resolve(source, missing, base, "grace"))


if __name__ == "__main__":
    unittest.main()
