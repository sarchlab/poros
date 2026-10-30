#!/usr/bin/env python3
"""Portable provenance checks; native execution is not needed for mutation tests."""
import importlib.util
import json
import os
import platform
import signal
import subprocess
import struct
import sys
import time
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("macho_native", Path(__file__).with_name("macho-native.py"))
macho = importlib.util.module_from_spec(spec)
spec.loader.exec_module(macho)


class FailureArtifactTests(unittest.TestCase):
    def test_failure_keeps_case_samples_and_diagnostic(self):
        with tempfile.TemporaryDirectory() as directory:
            target=Path(directory)/"nested"/"report.json"
            result={"cases":[{"id":"integer-1000","samples":[{"user_seconds":0.01}],"simulator":{"termination":"memory_fault"}}]}
            macho.preserve_failure(result,target,{"error":"diagnostic timed out"})
            saved=json.loads(target.read_text())
            self.assertEqual(saved["cases"],result["cases"])
            self.assertEqual(saved["bringup_failure_diagnostic"],{"error":"diagnostic timed out"})

    def test_diagnostic_uses_owned_bounded_configuration(self):
        from types import SimpleNamespace
        with tempfile.TemporaryDirectory() as directory:
            binary=Path(directory)/"program";binary.write_bytes(b"fixture")
            options=SimpleNamespace(dyld="/explicit/dyld",shared_cache=None,max_instructions=2000000,execution_timeout_seconds=10)
            def execute(command, timeout, **kwargs):
                self.assertEqual(command[:2],["go","run"])
                self.assertEqual(timeout,120)
                config=json.loads(Path(command[3]).read_text())
                self.assertEqual(config["Args"],["program","1000"])
                self.assertEqual(config["Runtime"],{"captured":1})
                self.assertEqual(config["Startup"],{"identity":"owned"})
                self.assertEqual(config["MaxInstructions"],2000000)
                self.assertIn("RecentSyscalls",Path(command[2]).read_text())
                kwargs["stdout"].write(b'{"Result":{"termination":"memory_fault"}}')
                return 0
            with patch.object(macho,"run_owned_process",side_effect=execute):
                result=macho.diagnose_startup(binary,options,{"args":["program","1000"]},{"captured":1},{"identity":"owned"})
            self.assertEqual(result["observation"]["Result"]["termination"],"memory_fault")
            self.assertEqual(result["source_sha256"],macho.digest(macho.ROOT/"scripts/macho-diagnostic.go"))


class StatObservationTests(unittest.TestCase):
    def test_recheck_requires_exact_enoent(self):
        profile = {"stat_enoent_paths": ["/usr/lib/libSystem.B.dylib"]}
        with patch.object(macho.os, "stat", side_effect=FileNotFoundError(2, "absent")) as query:
            macho.verify_captured_stat_absences(profile)
            query.assert_called_once_with(profile["stat_enoent_paths"][0])
        for outcome in (None, PermissionError(13, "denied")):
            with self.subTest(outcome=outcome), patch.object(macho.os, "stat", side_effect=outcome):
                with self.assertRaises(ValueError):
                    macho.verify_captured_stat_absences(profile)
        with patch.object(macho.os, "stat") as query:
            macho.verify_captured_stat_absences({})
            query.assert_not_called()

    def test_access_recheck_does_not_use_stat_or_permission_boolean(self):
        profile={"access_enoent_paths":["/AppleInternal/XBS/.isChrooted"]}
        for errno in (0,2,13):
            with self.subTest(errno=errno),patch.object(macho,"native_access_errno",return_value=errno) as query:
                if errno==2:macho.verify_captured_access_absences(profile)
                else:
                    with self.assertRaises(ValueError):macho.verify_captured_access_absences(profile)
                query.assert_called_once_with(profile["access_enoent_paths"][0])



class SharedMemorySnapshotTests(unittest.TestCase):
    def test_snapshot_recheck_requires_exact_bytes_and_metadata(self):
        contents=b"captured configuration"
        stat=dict(size=str(len(contents)),device=0,inode="0",mode=420,uid=0,gid=0,links=0,
                  mtime=dict(seconds="0",nanoseconds="0"),generation=0)
        entry=dict(name="runtime.config",sha256=macho.hashlib.sha256(contents).hexdigest(),
                   contents_base64=macho.base64.b64encode(contents).decode(),stat=stat)
        with patch.object(macho,"read_shared_memory_snapshot",return_value=(contents,stat)) as read:
            macho.verify_captured_shared_memory({"read_only_shared_memory":[entry]})
            read.assert_called_once_with("runtime.config")
        for raw,metadata in ((b"changed configuration",stat),(contents,{**stat,"mode":384}),
                             (contents,{**stat,"mtime":dict(seconds="1",nanoseconds="0")}),
                             (contents,{**stat,"generation":1})):
            with patch.object(macho,"read_shared_memory_snapshot",return_value=(raw,metadata)):
                with self.assertRaises(ValueError):macho.verify_captured_shared_memory({"read_only_shared_memory":[entry]})
        with patch.object(macho,"read_shared_memory_snapshot") as read:
            with self.assertRaises(ValueError):macho.verify_captured_shared_memory({"read_only_shared_memory":[{**entry,"sha256":"0"*64}]})
            read.assert_not_called()

    def test_snapshot_reader_closes_descriptor_on_failure(self):
        from unittest.mock import MagicMock
        library=MagicMock();library.shm_open.return_value=7
        with patch.object(macho.ctypes,"CDLL",return_value=library),patch.object(macho,"native_stat64",return_value=dict(size="16")),patch.object(macho.os,"close") as close,patch.object(macho.mmap,"mmap",side_effect=OSError("mapping failed")):
            with self.assertRaises(OSError):macho.read_shared_memory_snapshot("runtime.config")
            close.assert_called_once_with(7)
            library.shm_open.assert_called_once_with(b"runtime.config",os.O_RDONLY,0)

    def test_full_stat64_payload_preserves_precise_metadata(self):
        from unittest.mock import MagicMock
        import ctypes
        library=MagicMock()
        raw=bytearray(144)
        struct.pack_into("<IHHQIII",raw,0,11,420,2,15,16,17,18)
        for i in range(4):struct.pack_into("<qq",raw,32+i*16,-10-i,123+i)
        struct.pack_into("<qqiII",raw,96,32768,4,4096,7,9)
        def query(fd,output):
            self.assertEqual(fd,8);ctypes.memmove(output,bytes(raw),144);return 0
        library.fstat64.side_effect=query
        with patch.object(macho.ctypes,"CDLL",return_value=library):
            result=macho.native_stat64(8)
        self.assertEqual(result["birthtime"],dict(seconds="-13",nanoseconds="126"))
        self.assertEqual((result["rdev"],result["blocks"],result["block_size"],result["flags"],result["generation"]),(18,"4",4096,7,9))
        raw[128]=1
        with patch.object(macho.ctypes,"CDLL",return_value=library):
            with self.assertRaises(ValueError):macho.native_stat64(8)


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
        st=self.program.stat()
        self.output = json.dumps(dict(executable_stat=dict(device=st.st_dev,inode=str(st.st_ino),size=str(st.st_size)), filtered_apple=fields, standard_io=[{"fd":i} for i in range(3)], cs_status=1, sandbox_syscall_permissions=[dict(number=550,decision="0")])).encode()

    def identity(self, value):
        return dict(fsid="1", object_id=str(self.program.stat().st_ino) if value == "exe" else "3",
                    path=str(self.program if value == "exe" else self.dyld))

    def capture(self, identity=None):
        return macho.capture_startup_profile("helper", self.program, self.dyld, identity or self.identity)

    def test_success_binds_before_capture_bytes(self):
        with patch.object(macho, "run_startup_helper", return_value=self.output):
            result, status = self.capture()
        self.assertEqual(status, 1)
        self.assertEqual(result["binary_sha256"], macho.digest(self.program))
        self.assertEqual(result["dyld_sha256"], macho.digest(self.dyld))

    def test_open_descriptor_binding_and_metadata_rejection(self):
        def capture(command, **kwargs):
            fds=kwargs["pass_fds"]
            self.assertEqual(len(fds),4)
            self.assertEqual(command[-4:], [str(fd) for fd in fds])
            self.assertEqual(os.fstat(fds[-1]).st_ino,self.program.stat().st_ino)
            return self.output
        with patch.object(macho,"run_startup_helper",side_effect=capture):
            self.capture()
        for key in ("device","inode","size"):
            data=json.loads(self.output)
            data["executable_stat"][key]=str(int(data["executable_stat"][key])+1)
            with patch.object(macho,"run_startup_helper",return_value=json.dumps(data).encode()):
                with self.assertRaisesRegex(ValueError,"does not match opened input"):
                    self.capture()

    def test_same_bytes_path_replacement_rejected(self):
        def replace(value):
            identity=self.identity(value)
            if value=="dyld":
                replacement=self.program.with_name("replacement")
                replacement.write_bytes(self.program.read_bytes())
                replacement.replace(self.program)
            return identity
        with patch.object(macho,"run_startup_helper",return_value=self.output):
            with self.assertRaisesRegex(ValueError,"changed during startup"):
                self.capture(replace)

    def test_mutation_during_suspended_capture_rejected(self):
        for target in (self.program, self.dyld):
            with self.subTest(target=target.name):
                self.program.write_bytes(b"owned executable")
                self.dyld.write_bytes(b"owned runtime")
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


class NativeSymbolTests(unittest.TestCase):
    UUID="0102030405060708090a0b0c0d0e0f10"

    def image(self):
        data=bytearray(0x300)
        commands=bytearray()
        segment=bytearray(152)
        struct.pack_into("<II",segment,0,0x19,152)
        segment[8:14]=b"__TEXT"
        struct.pack_into("<4Q4I",segment,24,0x1000,0x1000,0,len(data),5,5,1,0)
        segment[72:78]=b"__text"
        struct.pack_into("<QQI",segment,104,0x1200,16,0x200)
        commands+=segment
        commands+=struct.pack("<II",0x1b,24)+bytes.fromhex(self.UUID)
        names=b"\0___chkstk_darwin\0"
        commands+=struct.pack("<6I",2,24,0x220,1,0x240,len(names))
        struct.pack_into("<8I",data,0,0xfeedfacf,0x100000c,0x80000002,7,3,len(commands),0,0)
        data[32:32+len(commands)]=commands
        struct.pack_into("<II",data,0x200,0xd503245f,0xd65f03c0)
        struct.pack_into("<IBBHQ",data,0x220,1,0xf,1,0,0x1200)
        data[0x240:0x240+len(names)]=names
        return data

    def fat(self, blob, duplicate=False):
        count=2 if duplicate else 1
        data=bytearray(0x1000*(count+1))
        struct.pack_into(">II",data,0,0xcafebabe,count)
        for i in range(count):
            struct.pack_into(">5I",data,8+i*20,0x100000c,0x80000002,0x1000*(i+1),len(blob),12)
            data[0x1000*(i+1):0x1000*(i+1)+len(blob)]=blob
        return data

    def test_native_symbol_uses_image_relative_offset(self):
        for data in (self.image(),self.fat(self.image())):
            self.assertEqual(macho.stack_probe_offset(data,self.UUID),0x200)
        with self.assertRaisesRegex(ValueError,"ambiguous matching"):
            macho.stack_probe_offset(self.fat(self.image(),True),self.UUID)

    def test_duplicate_uuid_and_unterminated_symbol_rejected(self):
        data=self.image()
        struct.pack_into("<II",data,16,4,224)
        data[232:256]=struct.pack("<II",0x1b,24)+bytes.fromhex(self.UUID)
        with self.assertRaises(ValueError):macho.stack_probe_offset(data,self.UUID)
        data=self.image();struct.pack_into("<I",data,228,17)
        with self.assertRaises(ValueError):macho.stack_probe_offset(data,self.UUID)

    def test_malformed_symbol_metadata_rejected(self):
        changes=[
            lambda b:b[:31],
            lambda b:(struct.pack_into("<I",b,20,0xffffffff),b)[1],
            lambda b:(struct.pack_into("<I",b,36,148),b)[1],
            lambda b:(struct.pack_into("<I",b,32+176+8,0xfffffffe),b)[1],
            lambda b:(struct.pack_into("<I",b,0x220,0xffffffff),b)[1],
            lambda b:(struct.pack_into("<B",b,0x224,1),b)[1],
            lambda b:(struct.pack_into("<B",b,0x225,2),b)[1],
            lambda b:(struct.pack_into("<Q",b,0x228,0x1800),b)[1],
            lambda b:(struct.pack_into("<I",b,0x200,0),b)[1],
            lambda b:(struct.pack_into("<I",b,32+60,1),b)[1],
            lambda b:(struct.pack_into("<I",b,32+176+12,2),b.__setitem__(slice(0x230,0x240),b[0x220:0x230]),b)[2],
        ]
        for change in changes:
            with self.subTest(change=change),self.assertRaises(ValueError):
                macho.stack_probe_offset(change(self.image()),self.UUID)
        with self.assertRaises(ValueError):
            macho.stack_probe_offset(self.image(),"00"*16)
        data=self.fat(self.image(),True)
        struct.pack_into(">I",data,8+20+8,0x1000)
        with self.assertRaisesRegex(ValueError,"overlapping"):
            macho.stack_probe_offset(data,self.UUID)


class AMFICaptureTests(unittest.TestCase):
    def test_policy_requires_matching_identity_state_and_output(self):
        failures=(None,"status","query","output","entitlements","dtrace-mode","dtrace-cleanup","dtrace-status",
                  "register-stage","register-value","tpro-missing","tpro-changed","tpro-capability","registration-calls",
                  "registration-feature","registration-layout","registration-normalization","stack-hint")
        for failure in failures:
            with self.subTest(failure=failure):
                data=dict(cs_status=7,cs_result=0,cs_errno=0,input_flags=0,output_flags=479,x0=0,x1=0,carry=False)
                data["dtrace"]=dict(opened=True,lazy_invalid_header_accepted=True,own_registration_removed=True,cs_status_after=7,cs_result_after=0)
                data["thread_protection_observation"]=dict(stage="native-tpro-call-entry-and-return",capability=3,register="S3_6_C15_C1_5",interposed=True,entry_capability=3,calls=1,values=[["2310346747086372864","2310346747086372864"]])
                before=bytearray(60);struct.pack_into("<Q",before,0,60)
                after=bytearray(before);struct.pack_into("<Q",after,16,0x20ff);struct.pack_into("<Q",after,36,0x8000)
                data["pthread_registration"]=dict(stage="native-first-registration",calls=1,result=0x400011df,pthread_size=16384,data_size=60,dispatch_queue_offset=160,input_hex=before.hex(),output_hex=after.hex(),main_stack_top=str(0x8000))
                protection=data["thread_protection_observation"];registration=data["pthread_registration"]
                if failure=="register-stage":protection["stage"]="post-loader-main"
                if failure=="register-value":protection["values"][0][0]=str(2**64)
                if failure=="tpro-missing":protection["calls"]=0;protection["values"]=[]
                if failure=="tpro-changed":protection["values"][0][1]="0"
                if failure=="tpro-capability":protection["entry_capability"]=0
                if failure=="registration-calls":registration["calls"]=2
                if failure=="registration-feature":registration["result"]=-1
                if failure=="registration-layout":after[0]=64;registration["output_hex"]=after.hex()
                if failure=="registration-normalization":after[56]=1;registration["output_hex"]=after.hex()
                if failure=="stack-hint":registration["main_stack_top"]=str(0x10000)
                if failure=="dtrace-mode":data["dtrace"]["lazy_invalid_header_accepted"]=False
                if failure=="dtrace-cleanup":data["dtrace"]["own_registration_removed"]=False
                if failure=="dtrace-status":data["dtrace"]["cs_status_after"]=8
                if failure=="status":data["cs_status"]=8
                if failure=="query":data["x0"]=14
                def launch(*args,**kwargs):
                    os.write(kwargs["pass_fds"][0],json.dumps(data).encode())
                    kwargs["stdout"].write(b"wrong" if failure=="output" else b"correct")
                    class Process:
                        pid=999999
                        def wait(self,**kwargs):return 0
                    return Process()
                def check(case,output):
                    if output!=b"correct":raise ValueError("incorrect original output")
                signed=subprocess.CompletedProcess([],0,b"<plist/>" if failure=="entitlements" else b"",b"")
                with patch.object(macho.subprocess,"run",return_value=signed),patch.object(macho.subprocess,"Popen",side_effect=launch),patch.object(macho.os,"killpg"),patch.object(macho,"check_output",side_effect=check):
                    if failure:
                        with self.assertRaises(ValueError):macho.capture_main_policies("program","probe",{"args":["program"]},7,{"thread_protection_capability_byte":3,"kernel_page_shift":14})
                    else:
                        result=macho.capture_main_policies("program","probe",{"args":["program"]},7,{"thread_protection_capability_byte":3,"kernel_page_shift":14})
                        self.assertEqual(result,(dict(input_flags="0",output_flags="479",code_signing_status=7),dict(mode="lazy-untraced",code_signing_status=7),dict(version=60,features=0x400011df,main_qos=str(0x20ff),mutex_policy=0,code_signing_status=7),data))
                        protection.update(capability=0,interposed=False,calls=0,values=[])
                        result=macho.capture_main_policies("program","probe",{"args":["program"]},7,{"thread_protection_capability_byte":0,"kernel_page_shift":14})
                        self.assertEqual(result[-1],data)


class StartupCleanupTests(unittest.TestCase):
    def test_failed_helper_group_does_not_leave_suspended_child(self):
        for caller, mode in ((c,m) for c in ("startup","amfi") for m in ("timeout","error","malformed")):
            with self.subTest(caller=caller,mode=mode), tempfile.TemporaryDirectory() as directory:
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
                if caller == "startup":
                    with self.assertRaises(exception):
                        json.loads(macho.run_startup_helper([sys.executable, str(script), str(record), mode], timeout=1))
                else:
                    owned = macho.run_owned_process
                    def run(command, timeout, **kwargs):
                        kwargs.pop("executable", None)
                        kwargs["env"] = {}
                        return owned([sys.executable,str(script),str(record),mode],1,**kwargs)
                    signed = subprocess.CompletedProcess([],0,b"",b"")
                    exception = subprocess.TimeoutExpired if mode == "timeout" else ValueError
                    with patch.object(macho,"run_owned_process",side_effect=run), patch.object(macho.subprocess,"run",return_value=signed), patch.object(macho,"check_output"):
                        with self.assertRaises(exception):
                            macho.capture_main_policies("program","library",{"args":["program"]},7,{"thread_protection_capability_byte":3,"kernel_page_shift":14})
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
