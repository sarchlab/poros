package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func TestNativeDyldIdentityMustMatchSelectedSlice(t *testing.T) {
	runtime := machotest.Image(7)
	offset := int(binary.LittleEndian.Uint32(runtime[20:])) + 32
	binary.LittleEndian.PutUint32(runtime[16:], 5)
	binary.LittleEndian.PutUint32(runtime[20:], uint32(offset-32+24))
	binary.LittleEndian.PutUint32(runtime[offset:], 0x1b)
	binary.LittleEndian.PutUint32(runtime[offset+4:], 24)
	for i := 0; i < 16; i++ {
		runtime[offset+8+i] = byte(i + 1)
	}
	identity := RuntimeDyldIdentity{UUID: "0102030405060708090a0b0c0d0e0f10", Subtype: 0x80000002}
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, NativeDyld: &identity}
	if _, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile}); err != nil {
		t.Fatal(err)
	}
	identity.Subtype = 0x80000001
	if _, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile}); err == nil {
		t.Fatal("wrong native dyld subtype accepted")
	}
	identity.Subtype = 0x80000002
	identity.UUID = "other"
	if _, err := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile}); err == nil {
		t.Fatal("wrong native dyld UUID accepted")
	}
}

func TestCapturedSysctlNamesAreValidatedAndCopied(t *testing.T) {
	value := int32(1)
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, SysctlNames: []SysctlName{{Name: "security.mac.lockdown_mode_state", MIB: []int32{102, 100, 101}, Int32: &value}}}
	p, e := Parse(machotest.Image(2), machotest.Image(7), Options{RuntimeProfile: &profile})
	if e != nil {
		t.Fatal(e)
	}
	value = 0
	profile.SysctlNames[0].MIB[0] = 1
	profile.SysctlNames[0].Name = "changed"
	if *p.kernel.sysctlNames[0].Int32 != 1 || p.kernel.sysctlNames[0].MIB[0] != 102 || p.kernel.sysctlNames[0].Name != "security.mac.lockdown_mode_state" {
		t.Fatal("mutable namespace borrowed")
	}
	for _, names := range [][]SysctlName{{{Name: "", MIB: []int32{1}}}, {{Name: "a\x00b", MIB: []int32{1}}}, {{Name: "a.", MIB: []int32{1}}}, {{Name: "a..b", MIB: []int32{1}}}, {{Name: "name", MIB: nil}}, {{Name: "name", MIB: []int32{0}}}, {{Name: "name", MIB: []int32{-1}}}, {{Name: "name", MIB: make([]int32, 25)}}, {{Name: "name", MIB: []int32{1}}, {Name: "name", MIB: []int32{2}}}, make([]SysctlName, 33), {{Name: "another", MIB: []int32{1}, Int32: &value}}, {{Name: "one", MIB: []int32{1}}, {Name: "two", MIB: []int32{1}}}} {
		if validateSysctlNames(names) == nil {
			t.Fatal("invalid namespace accepted", names)
		}
	}
}

func TestCapturedStringSysctlBoundsAndOwnership(t *testing.T) {
	value := ""
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, SysctlNames: []SysctlName{{Name: "kern.bootargs", MIB: []int32{1, 143}, String: &value}}}
	p, e := Parse(machotest.Image(2), machotest.Image(7), Options{RuntimeProfile: &profile})
	if e != nil {
		t.Fatal(e)
	}
	value = "changed"
	if *p.kernel.sysctlNames[0].String != "" {
		t.Fatal("borrowed string profile")
	}
	integer := int32(1)
	for _, entry := range []SysctlName{{Name: "other", MIB: []int32{1}, String: &value}, {Name: "kern.bootargs", MIB: []int32{1}, String: &value, Int32: &integer}} {
		if validateSysctlNames([]SysctlName{entry}) == nil {
			t.Fatal("invalid captured value accepted")
		}
	}
	for _, value := range []string{"bad\x00string", strings.Repeat("a", 1024)} {
		if validateSysctlNames([]SysctlName{{Name: "kern.bootargs", MIB: []int32{1}, String: &value}}) == nil {
			t.Fatal("invalid string accepted")
		}
	}
}

func TestCapturedDevelopmentFirmwareCommpage(t *testing.T) {
	var native struct {
		Legacy   uint32 `json:"legacy_dev_firm"`
		ReadOnly uint32 `json:"read_only_dev_firm"`
	}
	kernelNativeResult(t, "commpage-dev-firm.c", &native)
	for _, values := range [][2]uint32{{native.Legacy, native.ReadOnly}, {1, 0x12345678}} {
		profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, DevFirmware: &values[0], ReadOnlyDevFirmware: &values[1]}
		m := &usermem.AddressSpace{}
		if e := profile.mapCommpage(m); e != nil {
			t.Fatal(e)
		}
		for i, address := range []uint64{0xfffffc084, 0xfffff4084} {
			var b [4]byte
			if e := m.Read(address, b[:]); e != nil || binary.LittleEndian.Uint32(b[:]) != values[i] {
				t.Fatal(e, b)
			}
			if m.Write(address, []byte{0}) == nil {
				t.Fatal("writable commpage")
			}
		}
		profile.DevFirmware = nil
		profile.ReadOnlyDevFirmware = nil
		m = &usermem.AddressSpace{}
		if e := profile.mapCommpage(m); e != nil {
			t.Fatal(e)
		}
		for _, address := range []uint64{0xfffffc084, 0xfffff4084} {
			var b [4]byte
			if m.Read(address, b[:]) == nil {
				t.Fatal("invented uncaptured field")
			}
		}
	}
}
