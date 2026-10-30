package darwinuser

import (
	"bytes"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func TestDescriptorIoctlNativeContracts(t *testing.T) {
	for _, source := range []string{"fiodtype-probe.c", "tiocgeta-probe.c"} {
		t.Run(source, func(t *testing.T) { testDescriptorIoctlNative(t, source) })
	}
}
func testDescriptorIoctlNative(t *testing.T, source string) {
	var native struct {
		Command uint64
		Cleanup []int
		Cases   []struct {
			Name                string
			FD, Command, X0, X1 uint64
			Input               uint64 `json:"input_nzcv"`
			Output              uint64 `json:"output_nzcv"`
			Buffer              string `json:"buffer_hex"`
			Window              string `json:"mapping_window_hex"`
		}
	}
	kernelNativeResult(t, source, &native)
	if (native.Command != fioDType && native.Command != tiocGetA) || len(native.Cases) != 19 {
		t.Fatal(native)
	}
	for _, result := range native.Cleanup {
		if result != 0 {
			t.Fatal("native cleanup failed")
		}
	}
	fds := map[string]uint32{}
	for _, c := range native.Cases {
		fds[c.Name] = uint32(c.FD)
	}
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			snapshot := sharedMemoryFixture()
			k := &kernelState{descriptors: map[uint32]guestDescriptor{
				fds["regular"]:   {file: &guestOpenFile{stat: FileStat{Mode: 0x8180}}},
				fds["null"]:      {file: &guestOpenFile{nullDevice: true, stat: FileStat{Mode: 0x21b6}}},
				fds["directory"]: {path: "/"}, fds["shared-memory"]: {sharedMemory: &snapshot}}}
			m := &usermem.AddressSpace{}
			before := bytes.Repeat([]byte{0xa5}, len(c.Buffer)/2)
			window := bytes.Repeat([]byte{0xa5}, 8)
			m.Map(0x1000, uint64(len(before)), usermem.Read|usermem.Write, before)
			m.Map(0x2000, 8, usermem.Read|usermem.Write, window)
			m.Protect(0x2004, 4, usermem.Read, false)
			address := uint64(0x1004)
			switch {
			case strings.HasSuffix(c.Name, "null-output"):
				address = 0
			case strings.HasSuffix(c.Name, "invalid-output") || c.Name == "invalid-both":
				address = 1
			case strings.HasSuffix(c.Name, "readonly-output"):
				address = 0x2004
			case strings.HasSuffix(c.Name, "partial-output"):
				address = 0x2002
			case strings.HasSuffix(c.Name, "unaligned-output"):
				address = 0x1005
			}
			r := emu.Registers{Z: true, V: true}
			r.GPR[0] = c.FD
			r.GPR[1] = c.Command
			r.GPR[2] = address
			r.GPR[16] = 54
			old := r
			ok := k.trap(&r, m)
			got := make([]byte, len(before))
			mapped := make([]byte, 8)
			m.Read(0x1000, got)
			m.Read(0x2000, mapped)
			if c.Name == "cmd-high32" || (native.Command == fioDType && c.Name == "null-device-partial-output") {
				if ok || r != old || !bytes.Equal(got, before) || !bytes.Equal(mapped, window) {
					t.Fatal("unsupported boundary mutated state")
				}
				return
			}
			expected, _ := hex.DecodeString(c.Buffer)
			expectedWindow, _ := hex.DecodeString(c.Window)
			old.GPR[0] = c.X0
			old.GPR[1] = c.X1
			old.C = c.Output&0x20000000 != 0
			if !ok || r != old || c.Input != 0x50000000 || !bytes.Equal(got, expected) || !bytes.Equal(mapped, expectedWindow) {
				t.Fatalf("native ioctl mismatch supported=%v got=%v want=%v buffer=%x/%x window=%x/%x", ok, r, old, got, expected, mapped, expectedWindow)
			}
		})
	}
}

func TestFIODTypeNeedsExplicitNullDeviceAndSurvivesDup(t *testing.T) {
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 8, usermem.Read|usermem.Write, nil)
	unknown := &guestOpenFile{stat: FileStat{Mode: 0x2000}}
	k := &kernelState{descriptors: map[uint32]guestDescriptor{3: {file: unknown}}}
	r := emu.Registers{}
	r.GPR[0] = 3
	r.GPR[1] = fioDType
	r.GPR[2] = 0x1000
	r.GPR[16] = 54
	old := r
	if k.trap(&r, m) || r != old {
		t.Fatal("generic character device inferred as null")
	}
	unknown.nullDevice = true
	dup := emu.Registers{}
	dup.GPR[0] = 3
	if !k.duplicateDescriptor(&dup) || dup.GPR[0] != 4 {
		t.Fatal(dup)
	}
	close := emu.Registers{}
	close.GPR[0] = 3
	if !k.closeDescriptor(&close) {
		t.Fatal(close)
	}
	r.GPR[0] = 4
	if !k.trap(&r, m) || r.GPR[0] != 0 {
		t.Fatal("dup lost typed device", r)
	}
	var result [4]byte
	m.Read(0x1000, result[:])
	if result != [4]byte{2, 0, 0, 0} {
		t.Fatal(result)
	}
}
