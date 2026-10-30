#!/usr/bin/env python3
"""Portable provenance checks; native execution is not needed for mutation tests."""
import importlib.util
import json
import os
import platform
import signal
import subprocess
import sys
import time
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("macho_native", Path(__file__).with_name("macho-native.py"))
macho = importlib.util.module_from_spec(spec)
spec.loader.exec_module(macho)


class StartupIdentityTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.program = Path(self.directory.name) / "program"
        self.dyld = Path(self.directory.name) / "dyld"
        self.program.write_bytes(b"owned executable")
        self.dyld.write_bytes(b"owned runtime")
        values = dict(executable_file="exe", dyld_file="dyld", executable_cdhash="ab"*20,
                      executable_boothash="cd"*20, pfz="0xfffe78000", security_config="0x0")
        fields = [dict(key=k, value=v) for k, v in values.items()]
        fields.append(dict(key="main_stack", stack_size=0x7fc000,
                           allocation_size=64<<20, top_minus_allocation_base=64<<20))
        self.output = json.dumps(dict(filtered_apple=fields, sandbox_syscall_permissions=[dict(number=550,decision="0")])).encode()

    def identity(self, value):
        return dict(fsid="1", object_id="2" if value == "exe" else "3",
                    path=str(self.program if value == "exe" else self.dyld))

    def capture(self, identity=None):
        return macho.capture_startup_profile("helper", self.program, self.dyld, identity or self.identity)

    def test_success_binds_before_capture_bytes(self):
        with patch.object(macho, "run_startup_helper", return_value=self.output):
            result = self.capture()
        self.assertEqual(result["binary_sha256"], macho.digest(self.program))
        self.assertEqual(result["dyld_sha256"], macho.digest(self.dyld))

    def test_mutation_during_suspended_capture_rejected(self):
        for target in (self.program, self.dyld):
            with self.subTest(target=target.name):
                def capture(*args, **kwargs):
                    target.write_bytes(target.read_bytes()+b"changed")
                    return self.output
                with patch.object(macho, "run_startup_helper", side_effect=capture):
                    with self.assertRaisesRegex(ValueError, "changed during startup"):
                        self.capture()

    def test_mutation_during_identity_resolution_rejected(self):
        def resolve(value):
            if value == "dyld":
                self.program.write_bytes(b"changed while resolving")
            return self.identity(value)
        with patch.object(macho, "run_startup_helper", return_value=self.output):
            with self.assertRaisesRegex(ValueError, "changed during startup"):
                self.capture(resolve)

    def test_canonical_path_must_name_same_bytes(self):
        other = Path(self.directory.name) / "other"
        other.write_bytes(b"different executable")
        def resolve(value):
            identity = self.identity(value)
            if value == "exe":
                identity["path"] = str(other)
            return identity
        with patch.object(macho, "run_startup_helper", return_value=self.output):
            with self.assertRaisesRegex(ValueError, "changed during startup"):
                self.capture(resolve)


class StartupCleanupTests(unittest.TestCase):
    def test_failed_helper_group_does_not_leave_suspended_child(self):
        for mode in ("timeout", "error", "malformed"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                script, record = Path(directory) / "helper.py", Path(directory) / "child.pid"
                script.write_text("""import os,signal,sys,time
child=os.fork()
if child==0:
    os.kill(os.getpid(),signal.SIGSTOP)
    os._exit(0)
os.waitpid(child,os.WUNTRACED)
with open(sys.argv[1],"w") as f:f.write(str(child))
if sys.argv[2]=="error":os._exit(9)
if sys.argv[2]=="malformed":
    os.write(1,b"not JSON")
    os._exit(0)
time.sleep(30)
""")
                exception = {"timeout": subprocess.TimeoutExpired, "error": subprocess.CalledProcessError, "malformed": json.JSONDecodeError}[mode]
                with self.assertRaises(exception):
                    json.loads(macho.run_startup_helper([sys.executable, str(script), str(record), mode], timeout=1))
                child = int(record.read_text())
                deadline = time.monotonic()+3
                while True:
                    try:
                        os.kill(child, 0)
                    except ProcessLookupError:
                        break
                    # Linux container init may retain an already-dead zombie;
                    # the requirement is no live or stopped owned child.
                    if platform.system() == "Linux":
                        try:
                            state = Path(f"/proc/{child}/stat").read_text().rsplit(")", 1)[1].split()[0]
                        except FileNotFoundError:
                            break
                        if state == "Z":
                            break
                    if time.monotonic() >= deadline:
                        os.kill(child, signal.SIGKILL)
                        self.fail("owned suspended child survived helper cleanup")
                    time.sleep(0.02)


if __name__ == "__main__":
    unittest.main()
