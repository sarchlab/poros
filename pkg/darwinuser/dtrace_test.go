package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func dtraceState() kernelState {
	return kernelState{dtrace: &DTracePolicy{Mode: "lazy-untraced"}, descriptors: map[uint32]guestDescriptor{3: {dtrace: true}}}
}
func dtraceCall(k *kernelState, m *usermem.AddressSpace, fd, command, arg uint64) (emu.Registers, bool) {
	r := emu.Registers{}
	r.GPR[0] = fd
	r.GPR[1] = command
	r.GPR[2] = arg
	r.GPR[16] = 54
	ok := k.trap(&r, m)
	return r, ok
}
func TestDTraceNativeIOCTL(t *testing.T) {
	var observation struct {
		Cases []struct {
			Mode          int
			Before, After string
			X0, X1, Carry uint64
			Removals      []struct{ Generation, X0, X1, Carry uint64 }
		}
	}
	kernelNativeResult(t, "dtrace-ioctl-probe.c", &observation)
	cases := observation.Cases
	if len(cases) != 11 {
		t.Fatal(len(cases))
	}
	for _, c := range cases {
		t.Run(string(rune('A'+c.Mode)), func(t *testing.T) {
			k := dtraceState()
			k.dofs = map[uint64][80]byte{}
			for i := uint64(1); i <= 6; i++ {
				k.dofs[i] = [80]byte{}
			}
			m := &usermem.AddressSpace{}
			before, _ := hex.DecodeString(c.Before)
			after, _ := hex.DecodeString(c.After)
			m.Map(0x10000, 0x4000, usermem.Read|usermem.Write, before)
			var ptr [8]byte
			binary.LittleEndian.PutUint64(ptr[:], 0x10000)
			m.Map(0x20000, 8, usermem.Read, ptr[:])
			arg, fd, command := uint64(0x20000), uint64(3), uint64(dtraceAddDOF)
			switch c.Mode {
			case 4:
				arg = 0
			case 5:
				m.Unmap(0x20000, 8)
				m.Map(0x20000, 8, usermem.Read, nil)
			case 6:
				m.Protect(0x10000, 0x4000, usermem.Read, false)
			case 7:
				fd = 999999
			case 9:
				command = 0x200068ff
			}
			r, ok := dtraceCall(&k, m, fd, command, arg)
			if !ok || r.GPR[0] != c.X0 || r.GPR[1] != c.X1 || r.C != (c.Carry != 0) || r.GPR[2] != arg {
				t.Fatal(c.Mode, r, ok)
			}
			got := make([]byte, len(after))
			m.Read(0x10000, got)
			if !bytes.Equal(got, after) {
				t.Fatal("copyout differs", c.Mode)
			}
			for _, remove := range c.Removals {
				r, ok = dtraceCall(&k, m, 3, dtraceRemoveDOF, remove.Generation)
				if !ok || r.GPR[0] != remove.X0 || r.C != (remove.Carry != 0) {
					t.Fatal("registration side effect", r)
				}
			}
			if len(k.dofs) != 6 {
				t.Fatal("unexpected registrations", len(k.dofs))
			}
		})
	}
}
func TestDTraceNativeLifecycle(t *testing.T) {
	// Explicit JSON generation names are decoded below; the lifecycle starts with
	// no native-runtime registrations, so subtract the observed initial offset.
	var raw struct {
		Cases []struct {
			Operation     string
			X0, X1, Carry uint64
			A             uint64 `json:"generation_a"`
			B             uint64 `json:"generation_b"`
		}
	}
	kernelNativeResult(t, "dtrace-lifecycle-probe.c", &raw)
	if len(raw.Cases) != 10 {
		t.Fatal(len(raw.Cases))
	}
	baseline := raw.Cases[0].A - 1
	k := dtraceState()
	m := &usermem.AddressSpace{}
	m.Map(0x10000, 0x4000, usermem.Read|usermem.Write, nil)
	var ptr [8]byte
	binary.LittleEndian.PutUint64(ptr[:], 0x10000)
	m.Map(0x20000, 8, usermem.Read, ptr[:])
	for _, c := range raw.Cases {
		command, arg := uint64(dtraceRemoveDOF), c.A-baseline
		switch c.Operation {
		case "add_two", "add_after_first_removed", "add_after_last_removed":
			count := 1
			if c.Operation == "add_two" {
				count = 2
			}
			data := make([]byte, 8+80*count)
			binary.LittleEndian.PutUint64(data, uint64(count))
			m.Write(0x10000, data)
			command, arg = dtraceAddDOF, 0x20000
		case "remove_first_after_reopen":
			r := emu.Registers{}
			r.GPR[0] = 3
			k.closeDescriptor(&r)
			k.allocateDescriptor(guestDescriptor{dtrace: true})
		case "remove_zero":
			arg = 0
		case "remove_negative":
			arg = ^uint64(0)
		}
		r, ok := dtraceCall(&k, m, 3, command, arg)
		if !ok || r.GPR[0] != c.X0 || r.GPR[1] != c.X1 || r.C != (c.Carry != 0) {
			t.Fatal(c.Operation, r)
		}
		if command == dtraceAddDOF {
			m.Read(0x10050, ptr[:])
			if binary.LittleEndian.Uint64(ptr[:]) != c.A-baseline {
				t.Fatal("generation sequence", c.Operation)
			}
		}
	}
	if len(k.dofs) != 0 {
		t.Fatal("remaining registrations")
	}
}
func TestDTraceCapturedOpenAndUnsupportedBoundaries(t *testing.T) {
	var native struct {
		Cases []struct {
			Flags         uint32
			X0, X1, Carry uint64
		}
	}
	kernelNativeResult(t, "dtrace-open-probe.c", &native)
	for _, c := range native.Cases {
		k := dtraceState()
		k.descriptors = nil
		m := &usermem.AddressSpace{}
		m.Map(0x1000, 128, usermem.Read, append([]byte(dtraceHelperPath), 0))
		r := emu.Registers{}
		r.GPR[0] = 0x1000
		r.GPR[1] = uint64(c.Flags)
		if !k.open(&r, m) || r.GPR[0] != c.X0 || r.C {
			t.Fatal(c, r)
		}
	}
	k := dtraceState()
	m := &usermem.AddressSpace{}
	data := make([]byte, 88)
	binary.LittleEndian.PutUint64(data, 1)
	m.Map(0x10000, 88, usermem.Read, data)
	m.Protect(0x10000, 8, usermem.Read, true)
	var ptr [8]byte
	binary.LittleEndian.PutUint64(ptr[:], 0x10000)
	m.Map(0x20000, 8, usermem.Read, ptr[:])
	// A fully readonly record commits then reports EFAULT; partial writable state
	// is deliberately unsupported, with no registrations or copied prefix.
	m.Unmap(0x10000, 88)
	m.Map(0x10000, 88, usermem.Read|usermem.Write, data)
	m.Protect(0x10008, 80, usermem.Read, false)
	if _, ok := dtraceCall(&k, m, 3, dtraceAddDOF, 0x20000); ok || len(k.dofs) != 0 {
		t.Fatal("partial output mutated")
	}
	for _, command := range []uint64{0, 0x800868ff, 0x400868ff, 0x180086804} {
		if _, ok := dtraceCall(&k, m, 3, command, 0); ok {
			t.Fatal("unknown ioctl modeled")
		}
	}
	k.dofs = map[uint64][80]byte{1: {}}
	r, ok := dtraceCall(&k, m, 3, dtraceRemoveDOF, 0x100000001)
	if !ok || r.GPR[0] != 0 || len(k.dofs) != 0 {
		t.Fatal("REMOVE int argument")
	}
}

func TestDTraceTotalLimitCopyoutErrorPrecedence(t *testing.T) {
	k := dtraceState()
	k.dofs = map[uint64][80]byte{}
	for i := uint64(1); i <= 1024; i++ {
		k.dofs[i] = [80]byte{}
	}
	m := &usermem.AddressSpace{}
	data := make([]byte, 88)
	binary.LittleEndian.PutUint64(data, 1)
	m.Map(0x10000, 88, usermem.Read, data)
	var ptr [8]byte
	binary.LittleEndian.PutUint64(ptr[:], 0x10000)
	m.Map(0x20000, 8, usermem.Read, ptr[:])
	r, ok := dtraceCall(&k, m, 3, dtraceAddDOF, 0x20000)
	if !ok || r.GPR[0] != 22 || len(k.dofs) != 1024 {
		t.Fatal("prior EINVAL must survive failed copyout", r)
	}
	m.Unmap(0x10000, 88)
	m.Map(0x10000, 8, usermem.Read, data[:8])
	r, ok = dtraceCall(&k, m, 3, dtraceAddDOF, 0x20000)
	if !ok || r.GPR[0] != 14 {
		t.Fatal("full input copy precedes merged limit", r)
	}
}
func TestDTraceProfileBindingAndOwnership(t *testing.T) {
	profile := startupFixture(t)
	profile.AMFI = &AMFIPolicy{CodeSigningStatus: 7}
	profile.DTrace = &DTracePolicy{Mode: "lazy-untraced", CodeSigningStatus: 7}
	p := process(t, Options{StartupProfile: profile})
	profile.DTrace.Mode = "changed"
	if p.kernel.dtrace.Mode != "lazy-untraced" {
		t.Fatal("borrowed mutable policy")
	}
	for _, policy := range []*DTracePolicy{{Mode: "eager", CodeSigningStatus: 7}, {Mode: "lazy-untraced", CodeSigningStatus: 8}} {
		profile.DTrace = policy
		if _, err := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: profile}); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
}

func TestDTraceDeviceNamespaceConflicts(t *testing.T) {
	for _, kind := range []string{"main", "dyld", "execname", "directory", "stat"} {
		t.Run(kind, func(t *testing.T) {
			profile := startupFixture(t)
			profile.AMFI = &AMFIPolicy{CodeSigningStatus: 7}
			profile.DTrace = &DTracePolicy{Mode: "lazy-untraced", CodeSigningStatus: 7}
			runtime, identity := nativeDyldFixture()
			image, err := loader.ParseMachO(runtime, 7)
			if err != nil {
				t.Fatal(err)
			}
			profile.DyldSHA256 = image.SHA256
			runtimeProfile := &RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, NativeDyld: &identity}
			options := Options{StartupProfile: profile, RuntimeProfile: runtimeProfile}
			switch kind {
			case "main":
				profile.ExecutableFile.Path = dtraceHelperPath
			case "dyld":
				profile.DyldFile.Path = dtraceHelperPath
			case "execname":
				options.ExecName = dtraceHelperPath
			case "directory":
				runtimeProfile.Directories = []RuntimeDirectory{{Path: dtraceHelperPath, CanonicalPath: dtraceHelperPath}}
			case "stat":
				runtimeProfile.StatENOENTPaths = []string{dtraceHelperPath}
			}
			if _, err := Parse(machotest.Image(2), runtime, options); err == nil || !strings.Contains(err.Error(), "DTrace helper conflicts") {
				t.Fatal("device namespace conflict accepted")
			}
		})
	}
	// Without a captured mode, knowing the global commpage byte cannot authorize
	// helper opens or IOCTL execution.
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 128, usermem.Read, append([]byte(dtraceHelperPath), 0))
	k := kernelState{}
	r := emu.Registers{}
	r.GPR[0] = 0x1000
	r.GPR[1] = 2
	before := r
	if k.open(&r, m) || r != before || len(k.descriptors) != 0 {
		t.Fatal("uncaptured device")
	}
}

func TestDTraceNativeRegistryBoundary(t *testing.T) {
	var native struct {
		Next     uint64                         `json:"initial_next_generation"`
		Fill     uint64                         `json:"fill_count"`
		Cleaned  uint64                         `json:"cleanup_count"`
		High     struct{ X0, X1, Carry uint64 } `json:"high_command"`
		Overflow struct {
			X0, X1, Carry uint64
			Unchanged     bool `json:"record_unchanged"`
		} `json:"overflow_readonly"`
	}
	kernelNativeResult(t, "dtrace-boundary-probe.c", &native)
	if native.Next+native.Fill-1 != 1024 || native.Cleaned != native.Fill || !native.Overflow.Unchanged {
		t.Fatal(native)
	}
	k := dtraceState()
	k.dofs = map[uint64][80]byte{}
	for i := uint64(1); i < native.Next; i++ {
		k.dofs[i] = [80]byte{}
	}
	m := &usermem.AddressSpace{}
	data := make([]byte, 8+80*int(native.Fill))
	binary.LittleEndian.PutUint64(data, native.Fill)
	m.Map(0x10000, uint64(len(data)), usermem.Read|usermem.Write, data)
	var ptr [8]byte
	binary.LittleEndian.PutUint64(ptr[:], 0x10000)
	m.Map(0x30000, 8, usermem.Read, ptr[:])
	r, ok := dtraceCall(&k, m, 3, dtraceAddDOF, 0x30000)
	if !ok || r.C || len(k.dofs) != 1024 {
		t.Fatal("fill", r)
	}
	binary.LittleEndian.PutUint64(ptr[:], 1)
	m.Write(0x10000, ptr[:])
	m.Protect(0x10000, uint64(len(data)), usermem.Read, false)
	r, ok = dtraceCall(&k, m, 3, dtraceAddDOF, 0x30000)
	if !ok || r.GPR[0] != native.Overflow.X0 || r.GPR[1] != native.Overflow.X1 || r.C != (native.Overflow.Carry != 0) {
		t.Fatal("overflow", r)
	}
	for i := native.Next; i < native.Next+native.Fill; i++ {
		r, ok = dtraceCall(&k, m, 3, dtraceRemoveDOF, i)
		if !ok || r.C {
			t.Fatal("cleanup", i)
		}
	}
	// High command bits are significant. Native reports ENOTTY, but arbitrary
	// command copyin classes remain explicitly outside this implementation.
	r, ok = dtraceCall(&k, m, 3, 0x180086804, 0x30000)
	if ok || native.High.X0 != 25 {
		t.Fatal("high ioctl bits discarded", r)
	}
}
