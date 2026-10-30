package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/internal/machotest"
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
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, SysctlNames: []SysctlName{{Name: "security.mac.lockdown_mode_state", MIB: []int32{102, 100, 101}}}}
	p, e := Parse(machotest.Image(2), machotest.Image(7), Options{RuntimeProfile: &profile})
	if e != nil {
		t.Fatal(e)
	}
	profile.SysctlNames[0].MIB[0] = 1
	profile.SysctlNames[0].Name = "changed"
	if p.kernel.sysctlNames[0].MIB[0] != 102 || p.kernel.sysctlNames[0].Name != "security.mac.lockdown_mode_state" {
		t.Fatal("mutable namespace borrowed")
	}
	for _, names := range [][]SysctlName{{{Name: "", MIB: []int32{1}}}, {{Name: "a\x00b", MIB: []int32{1}}}, {{Name: "a.", MIB: []int32{1}}}, {{Name: "a..b", MIB: []int32{1}}}, {{Name: "name", MIB: nil}}, {{Name: "name", MIB: []int32{0}}}, {{Name: "name", MIB: []int32{-1}}}, {{Name: "name", MIB: make([]int32, 25)}}, {{Name: "name", MIB: []int32{1}}, {Name: "name", MIB: []int32{2}}}, make([]SysctlName, 33)} {
		if validateSysctlNames(names) == nil {
			t.Fatal("invalid namespace accepted", names)
		}
	}
}
