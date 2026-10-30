package darwinuser

import (
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestCapturedAccessNativeContract(t *testing.T) {
	var native struct {
		Path  string
		Cases []struct {
			Mode          uint32
			Kind          int `json:"path_kind"`
			X0, X1, Carry uint64
		}
	}
	kernelNativeResult(t, "access-runtime-probe.c", &native)
	if len(native.Cases) != 12 {
		t.Fatal(len(native.Cases))
	}
	for _, c := range native.Cases {
		k := kernelState{accessENOENTPaths: []string{native.Path}}
		m := &usermem.AddressSpace{}
		m.Map(0x1000, 1024, usermem.Read, append([]byte(native.Path), 0))
		r := emu.Registers{}
		r.GPR[0] = 0x1000
		r.GPR[1] = uint64(c.Mode)
		r.GPR[16] = 33
		if c.Kind == 1 || c.Kind == 3 {
			r.GPR[0] = 0
		}
		if c.Kind == 2 {
			r.GPR[0] = 0x5000
		}
		before := r
		ok := k.trap(&r, m)
		if c.Mode != 0 {
			if ok || r != before {
				t.Fatal("uncaptured permission mode")
			}
			continue
		}
		if !ok || r.GPR[0] != c.X0 || r.GPR[1] != c.X1 || r.C != (c.Carry != 0) {
			t.Fatal(c, r)
		}
	}
	// Neither another method's absence nor an unknown path authorizes ENOENT.
	for _, path := range []string{native.Path, "/uncaptured"} {
		m := &usermem.AddressSpace{}
		m.Map(0x1000, 1024, usermem.Read, append([]byte(path), 0))
		k := kernelState{statENOENTPaths: []string{native.Path}}
		r := emu.Registers{}
		r.GPR[0] = 0x1000
		before := r
		if k.access(&r, m) || r != before {
			t.Fatal("inferred access observation")
		}
	}
}
func TestCapturedAccessProfileValidation(t *testing.T) {
	runtime, identity := nativeDyldFixture()
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, NativeDyld: &identity, AccessENOENTPaths: []string{"/observed/absent"}}
	p, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile})
	if err != nil {
		t.Fatal(err)
	}
	profile.AccessENOENTPaths[0] = "/changed"
	if p.kernel.accessENOENTPaths[0] != "/observed/absent" {
		t.Fatal("borrowed profile")
	}
	for _, paths := range [][]string{{"relative"}, {"/x", "/x"}, {"/x/../y"}, {"/bad\x00name"}, {"/"}} {
		profile.AccessENOENTPaths = paths
		if _, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile}); err == nil {
			t.Fatal("invalid namespace", paths)
		}
	}
	profile.AccessENOENTPaths = []string{"/known"}
	profile.Directories = []RuntimeDirectory{{Path: "/known", CanonicalPath: "/known"}}
	if profile.validate() == nil {
		t.Fatal("directory conflict")
	}
	profile.Directories = nil
	profile.NativeDyld = nil
	if profile.validate() == nil {
		t.Fatal("unbound runtime")
	}
}
