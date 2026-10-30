package darwinuser

import (
	"encoding/json"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func startupFixture(t *testing.T) *StartupProfile {
	t.Helper()
	image, e := loader.ParseMachO(machotest.Image(2), 2)
	if e != nil {
		t.Fatal(e)
	}
	dyld, e := loader.ParseMachO(machotest.Image(7), 7)
	if e != nil {
		t.Fatal(e)
	}
	return &StartupProfile{BinarySHA256: image.SHA256, DyldSHA256: dyld.SHA256, ExecutableFile: FileIdentity{0x1a0100000d, 0x1234, "/guest/program"}, DyldFile: FileIdentity{0x1a0100000d, 0xfffffff000b33ce, "/usr/lib/dyld"}, CDHash: strings.Repeat("ab", 20), BootHash: strings.Repeat("cd", 20), PFZ: 0xfffe78000, StackSize: 0x7fc000, StackAllocationSize: 64 << 20}
}

func TestCapturedStartupStackAndNamespace(t *testing.T) {
	profile := startupFixture(t)
	p := process(t, Options{Args: []string{"program", "1"}, ExecName: "/guest/program", StartupProfile: profile, EntropySeed: 17})
	var apple []string
	for i := uint64(6); i < 40; i++ {
		a := word(t, p, p.Registers.SP+8*i)
		if a == 0 {
			break
		}
		apple = append(apple, textAt(t, p, a))
	}
	keys := []string{"executable_path", "pfz", "stack_guard", "malloc_entropy", "ptr_munge", "main_stack", "executable_file", "dyld_file", "executable_cdhash", "executable_boothash", "th_port", "security_config"}
	if len(apple) != len(keys) {
		t.Fatal(apple)
	}
	for i, key := range keys {
		if !strings.HasPrefix(apple[i], key+"=") {
			t.Fatal("kernel apple order", apple)
		}
	}
	for _, s := range apple {
		if strings.HasPrefix(s, "ptrauth_disabled=") || strings.HasPrefix(s, "arm64e_abi=") {
			t.Fatal("invented generic-arm64 flag", s)
		}
	}
	guard, e := strconv.ParseUint(strings.TrimPrefix(apple[2], "stack_guard="), 0, 64)
	if e != nil || guard&0xff00 != 0 || guard == 0 {
		t.Fatal(guard, e)
	}
	if apple[5] != "main_stack=0x800000000,0x7fc000,0x7fc000000,0x4000000" || apple[10] != "th_port=0x10b" || p.kernel.threadSendRefs != 1 || p.kernel.taskSendRefs != 0 {
		t.Fatal(apple, p.kernel)
	}
	if p.Memory.Check(StackTop-profile.StackSize, 1, usermem.Read, "stack") != nil || p.Memory.Check(StackTop-profile.StackSize-1, 1, usermem.Read, "guard") == nil {
		t.Fatal("stack usable/guard geometry")
	}
	address, kr, ok := p.kernel.allocateAnonymous(p.Memory, StackTop-(60<<20), vmPage, 0, 1, 3, 7)
	if !ok || kr != 0 || address != StackTop {
		t.Fatal("allocator entered reserved stack growth region", address, kr, ok)
	}
	// Profiles are copied into kernel state, not retained as mutable namespace input.
	profile.ExecutableFile.Path = "/changed"
	if p.kernel.files[0].Path != "/guest/program" {
		t.Fatal("borrowed mutable identity")
	}
	same, seed := startupFixture(t).apple("/guest/program", 17)
	if strings.Join(same, "\n") != strings.Join(apple, "\n") || seed != p.kernel.entropyState {
		t.Fatal("deterministic launch stream")
	}
	other, _ := startupFixture(t).apple("/guest/program", 18)
	if same[2] == other[2] || same[3] == other[3] || same[4] == other[4] {
		t.Fatal("seed did not select launch entropy")
	}
}

func TestStartupProfileBoundaries(t *testing.T) {
	for _, change := range []func(*StartupProfile){func(p *StartupProfile) { p.BinarySHA256 = "wrong" }, func(p *StartupProfile) { p.DyldSHA256 = "wrong" }, func(p *StartupProfile) { p.ExecutableFile.Path = "/guest\x00/program" }, func(p *StartupProfile) { p.DyldFile.Path = "relative" }, func(p *StartupProfile) { p.ExecutableFile.Path = "/guest/../program" }, func(p *StartupProfile) { p.CDHash = "abc" }, func(p *StartupProfile) { p.BootHash = strings.Repeat("AB", 20) }, func(p *StartupProfile) { p.StackSize++ }, func(p *StartupProfile) { p.StackAllocationSize = p.StackSize }, func(p *StartupProfile) { p.StackAllocationSize = maxStackReservation + vmPage }, func(p *StartupProfile) { p.PFZ = 0 }, func(p *StartupProfile) { p.ARM64eABI = "unknown" }, func(p *StartupProfile) { p.DyldFile = p.ExecutableFile }} {
		profile := startupFixture(t)
		change(profile)
		if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: profile}); e == nil {
			t.Fatal("invalid startup profile accepted", profile)
		}
	}
	if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: startupFixture(t), StackSize: DefaultStackSize}); e == nil {
		t.Fatal("native geometry silently overridden")
	}
	// XNU's arm64e_abi key describes library ABI eligibility, not main subtype.
	for _, abi := range []string{"all", "os"} {
		p := startupFixture(t)
		p.ARM64eABI = abi
		if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: p}); e != nil {
			t.Fatal(e)
		}
	}
	p := startupFixture(t)
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	var q StartupProfile
	if e = json.Unmarshal(b, &q); e != nil || !reflect.DeepEqual(q, *p) {
		t.Fatal("64-bit identity JSON did not roundtrip", e, string(b))
	}
}

func TestCommpageKernelReservationNativeContract(t *testing.T) {
	var ob struct {
		Start      string
		Size       uint64
		Protection uint32
		Maximum    uint32 `json:"max_protection"`
	}
	kernelNativeResult(t, "pfz-region-probe.c", &ob)
	lo, err := strconv.ParseUint(ob.Start, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	if lo != commpageRegionStart || lo+ob.Size != commpageRegionEnd || ob.Protection != 0 || ob.Maximum != 0 {
		t.Fatal(ob)
	}
	p := process(t, Options{StartupProfile: startupFixture(t)})
	addr, kr, ok := p.kernel.allocateAnonymous(p.Memory, lo, vmPage, 0, 1, 3, 7)
	if !ok || kr != 0 || addr != commpageRegionEnd {
		t.Fatal("allocator entered commpage reservation", addr, kr, ok)
	}
	if p.Memory.Check(0xfffe78000, 4, usermem.Execute, "PFZ") == nil {
		t.Fatal("unobserved PFZ bytes invented")
	}
	if p.kernel.unmapRange(p.Memory, lo, lo+vmPage) {
		t.Fatal("kernel submap unmap silently allowed")
	}
}

func TestStartupSandboxPolicyIsBoundAndCopied(t *testing.T) {
	profile := startupFixture(t)
	profile.SandboxSyscalls = []SandboxSyscallPermission{{550, 0}}
	p := process(t, Options{StartupProfile: profile})
	profile.SandboxSyscalls[0].Number = 551
	if p.kernel.sandboxSyscalls[0].Number != 550 {
		t.Fatal("mutable captured policy borrowed")
	}
	for _, rules := range [][]SandboxSyscallPermission{{{550, 1}}, {{550, 0}, {550, 0}}, make([]SandboxSyscallPermission, 33)} {
		profile := startupFixture(t)
		profile.SandboxSyscalls = rules
		if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: profile}); e == nil {
			t.Fatal("unsupported policy profile accepted", rules)
		}
	}
}

func TestStartupAMFIPolicyIsBoundAndCopied(t *testing.T) {
	profile := startupFixture(t)
	profile.AMFI = &AMFIPolicy{InputFlags: 0, OutputFlags: 0x1df}
	p := process(t, Options{StartupProfile: profile})
	profile.AMFI.OutputFlags = 0
	if p.kernel.amfi == nil || p.kernel.amfi.OutputFlags != 0x1df {
		t.Fatal("mutable policy borrowed")
	}
}

func TestExecutableStatBoundToOwnedImage(t *testing.T) {
	profile := startupFixture(t)
	profile.ExecutableStat = &FileStat{Mode: 0x81ed, Inode: profile.ExecutableFile.ObjectID, Size: int64(len(machotest.Image(2)))}
	p := process(t, Options{StartupProfile: profile, ExecName: "/alias/program"})
	profile.ExecutableStat.Size = 99
	if p.kernel.executableStat.Size == 99 || !reflect.DeepEqual(p.kernel.executablePaths, []string{"/alias/program", "/guest/program"}) {
		t.Fatal("executable metadata not privately bound")
	}
	for _, change := range []func(*FileStat){func(s *FileStat) { s.Size++ }, func(s *FileStat) { s.Size = -1 }, func(s *FileStat) { s.Inode++ }, func(s *FileStat) { s.Mode = 0x4000 }, func(s *FileStat) { s.Change.Nanoseconds = -1 }} {
		profile := startupFixture(t)
		profile.ExecutableStat = &FileStat{Mode: 0x81ed, Inode: profile.ExecutableFile.ObjectID, Size: int64(len(machotest.Image(2)))}
		change(profile.ExecutableStat)
		if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: profile}); e == nil {
			t.Fatal("unbound executable stat accepted")
		}
	}
	for _, name := range []string{"/alias/program", "/guest/program", "/unknown/program", "/guest/program/"} {
		m := &usermem.AddressSpace{}
		m.Map(0x1000, uint64(len(name)+1), usermem.Read, append([]byte(name), 0))
		m.Map(0x2000, 144, usermem.Read|usermem.Write, nil)
		r := emu.Registers{}
		r.GPR[0] = 0x1000
		r.GPR[1] = 0x2000
		before := r
		ok := p.kernel.stat(&r, m)
		known := name == "/alias/program" || name == "/guest/program"
		if ok != known || (!known && r != before) {
			t.Fatal("unknown path fabricated", name, ok)
		}
	}
}
