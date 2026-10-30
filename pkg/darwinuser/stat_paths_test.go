package darwinuser

import (
	"bytes"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func TestCapturedStatENOENTNativePrecedence(t *testing.T) {
	var observed struct {
		Path   string
		Return int `json:"libc_return"`
		Errno  int `json:"libc_errno"`
		Cases  []struct {
			Case      string
			X0, X1    uint64
			Carry     bool
			Unchanged bool `json:"output_unchanged"`
		}
	}
	kernelNativeResult(t, "stat-cache-library-probe.c", &observed)
	if observed.Return != -1 || observed.Errno != 2 || len(observed.Cases) != 6 {
		t.Fatal(observed)
	}
	for _, tc := range observed.Cases {
		t.Run(tc.Case, func(t *testing.T) {
			k := kernelState{statENOENTPaths: []string{observed.Path}}
			m := &usermem.AddressSpace{}
			m.Map(0x1000, uint64(len(observed.Path)+1), usermem.Read, append([]byte(observed.Path), 0))
			r := emu.Registers{}
			r.GPR[0] = 0x1000
			r.GPR[1] = 0x2000
			r.GPR[16] = 338
			size, perm := 160, usermem.Read|usermem.Write
			switch tc.Case {
			case "null_output":
				r.GPR[1] = 0
			case "unmapped_output":
				r.GPR[1] = 1
			case "readonly_output":
				perm = usermem.Read
			case "partial_eight":
				size = 8
			case "missing_last_byte":
				size = 143
			}
			initial := bytes.Repeat([]byte{0xa5}, size)
			m.Map(0x2000, uint64(size), perm, initial)
			if !k.trap(&r, m) || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry || !tc.Unchanged {
				t.Fatal(tc, r)
			}
			got := make([]byte, size)
			m.Read(0x2000, got)
			if !bytes.Equal(got, initial) {
				t.Fatal("absence result touched output")
			}
			// A stat observation does not establish the semantics of open/lstat.
			r = emu.Registers{}
			r.GPR[0] = 0x1000
			before := r
			if k.open(&r, m) || r != before {
				t.Fatal("stat fact leaked into open")
			}
		})
	}
}
func TestStatENOENTNamespaceValidationAndOwnership(t *testing.T) {
	for _, paths := range [][]string{{"relative"}, {"/"}, {"/bad\x00name"}, {"/a/../b"}, {"/a", "/a"}, {"/" + strings.Repeat("a", 1023)}, make([]string, 257)} {
		if validateStatENOENTPaths(paths, nil) == nil {
			t.Fatal("invalid path accepted", paths)
		}
	}
	dirs := []RuntimeDirectory{{Path: "/alias", CanonicalPath: "/canonical", Symlink: true}}
	for _, name := range []string{"/alias", "/canonical"} {
		if validateStatENOENTPaths([]string{name}, dirs) == nil {
			t.Fatal("directory conflict accepted")
		}
	}
	runtime, identity := nativeDyldFixture()
	parsed, err := loader.ParseMachO(runtime, 7)
	if err != nil {
		t.Fatal(err)
	}
	startup := startupFixture(t)
	startup.DyldSHA256 = parsed.SHA256
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, NativeDyld: &identity, StatENOENTPaths: []string{"/captured/absent"}}
	p, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile, StartupProfile: startup, ExecName: "/alias/program"})
	if err != nil {
		t.Fatal(err)
	}
	profile.StatENOENTPaths[0] = "/changed"
	if p.kernel.statENOENTPaths[0] != "/captured/absent" {
		t.Fatal("borrowed caller namespace")
	}
	for _, name := range []string{"/alias/program", "/guest/program", "/usr/lib/dyld"} {
		profile.StatENOENTPaths = []string{name}
		if _, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile, StartupProfile: startup, ExecName: "/alias/program"}); err == nil {
			t.Fatal("known image conflict accepted", name)
		}
	}
	profile.StatENOENTPaths = []string{"/captured/absent"}
	profile.NativeDyld = nil
	if _, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile}); err == nil {
		t.Fatal("unbound runtime stat observation accepted")
	}
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 9, usermem.Read, []byte("/unknown\x00"))
	r := emu.Registers{}
	r.GPR[0] = 0x1000
	r.GPR[1] = 1
	before := r
	if p.kernel.stat(&r, m) || r != before {
		t.Fatal("unknown path invented ENOENT")
	}
}
