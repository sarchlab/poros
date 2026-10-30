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
