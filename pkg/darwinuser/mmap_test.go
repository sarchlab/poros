package darwinuser

import (
	"bytes"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestMmapFileNativeContract(t *testing.T) {
	var cases []struct {
		Case          string
		Success       bool
		Errno, X1     uint64
		RegionSize    uint64 `json:"region_size"`
		Protection    int32
		MaxProtection int32 `json:"max_protection"`
		Bytes         string
		TailReadable  bool   `json:"eof_tail_readable"`
		Tail          string `json:"eof_tail"`
	}
	kernelNativeResult(t, "file-mmap-probe.c", &cases)
	if len(cases) != 15 {
		t.Fatal(len(cases))
	}
	payload := make([]byte, 33472)
	for i := range payload {
		payload[i] = byte(i*17 + (i>>14)*29 + 3)
	}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			file := &guestOpenFile{reader: bytes.NewReader(payload), stat: FileStat{Size: int64(len(payload))}, offset: 19}
			k := kernelState{descriptors: map[uint32]guestDescriptor{3: {file: file}}}
			m := &usermem.AddressSpace{}
			r := emu.Registers{C: true}
			r.GPR[1] = 33472
			r.GPR[2] = 1
			r.GPR[3] = 0x40002
			r.GPR[4] = 3
			r.GPR[16] = 197
			switch tc.Case {
			case "private_cow":
				r.GPR[2] = 3
			case "zero_length":
				r.GPR[1] = 0
			case "bad_fd":
				r.GPR[4] = ^uint64(0)
			case "low32_fd":
				r.GPR[4] |= 1 << 32
			case "aligned_offset":
				r.GPR[5] = 16384
				r.GPR[1] = 16
			case "unaligned_offset":
				r.GPR[5] = 1
				r.GPR[1] = 16
			case "negative_offset":
				r.GPR[5] = ^uint64(0)
				r.GPR[1] = 16
			case "beyond_eof_page":
				r.GPR[5] = 49152
				r.GPR[1] = 16
			case "partial_eof_page":
				r.GPR[5] = 32768
				r.GPR[1] = 16
			case "private_without_unix03":
				r.GPR[3] = 2
			case "no_sharing_type":
				r.GPR[3] = 0x40000
			case "fixed_owned_replacement":
				r.GPR[3] |= 0x10
			case "overflow_length":
				r.GPR[1] = ^uint64(0)
			}
			before := r
			handled := k.trap(&r, m)
			if tc.Case == "beyond_eof_page" || tc.Case == "fixed_owned_replacement" {
				if handled || r != before || len(m.Mappings()) != 0 || k.ownedBytes != 0 {
					t.Fatal("unmodeled mapping fabricated")
				}
				return
			}
			if !handled || r.C == tc.Success || r.GPR[1] != tc.X1 {
				t.Fatal(tc, r.GPR[:6], r.C)
			}
			if !tc.Success {
				if r.GPR[0] != tc.Errno || len(m.Mappings()) != 0 {
					t.Fatal(tc, r.GPR[0])
				}
				return
			}
			address := r.GPR[0]
			maps := m.Mappings()
			if address%vmPage != 0 || len(maps) != 1 || maps[0].Start != address || maps[0].End-address != tc.RegionSize || maps[0].Flags != machPermissions(uint32(tc.Protection)) || k.ownedBytes != tc.RegionSize || file.offset != 19 {
				t.Fatal(maps, address, file.offset)
			}
			got := make([]byte, 16)
			if err := m.Read(address, got); err != nil {
				t.Fatal(err)
			}
			want, _ := hex.DecodeString(tc.Bytes)
			if !bytes.Equal(got, want) {
				t.Fatal("file bytes differ", got, want)
			}
			if tc.TailReadable {
				tailAddress := address + uint64(len(payload)) - before.GPR[5]
				if err := m.Read(tailAddress, got); err != nil {
					t.Fatal(err)
				}
				want, _ := hex.DecodeString(tc.Tail)
				if !bytes.Equal(got, want) {
					t.Fatal("final page tail not native zero")
				}
			}
			if tc.Case == "after_close" {
				c := emu.Registers{}
				c.GPR[0] = 3
				k.closeDescriptor(&c)
				if err := m.Read(address, got); err != nil || !bytes.Equal(got, payload[:16]) {
					t.Fatal("closed FD invalidated mapping")
				}
			}
			if tc.Case == "private_cow" {
				other := before
				k.trap(&other, m)
				if other.C {
					t.Fatal(other)
				}
				if err := m.Write(address, []byte{0xff}); err != nil {
					t.Fatal(err)
				}
				var b [1]byte
				if err := m.Read(other.GPR[0], b[:]); err != nil || b[0] != payload[0] {
					t.Fatal("private mapping changed second map")
				}
				file.reader.ReadAt(b[:], 0)
				if b[0] != payload[0] {
					t.Fatal("private map changed file")
				}
			}
			// The captured maximum remains7 even though runtime executable transitions
			// require separate policy. AddressSpace must preserve this native metadata.
			if err := m.Protect(address, tc.RegionSize, machPermissions(uint32(tc.MaxProtection)), false); err != nil {
				t.Fatal("maximum protection differs", err)
			}
			if !k.unmapRange(m, address, address+tc.RegionSize) {
				t.Fatal("owned file unmap failed")
			}
		})
	}
}
func TestMmapUnmodeledNativeBoundariesRemainUnsupported(t *testing.T) {
	var cases []struct {
		Case    string
		Success bool
		Errno   uint64
	}
	kernelNativeResult(t, "file-mmap-boundary-probe.c", &cases)
	if len(cases) != 5 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			if !tc.Success || tc.Errno != 0 {
				t.Fatal("native counterexample missing")
			}
			k := kernelState{}
			m := &usermem.AddressSpace{}
			r := emu.Registers{}
			r.GPR[1] = 33472
			r.GPR[2] = 1
			r.GPR[3] = 0x40002
			r.GPR[4] = 3
			switch tc.Case {
			case "legacy_no_sharing_type":
				r.GPR[3] = 0
			case "legacy_zero_length":
				r.GPR[3] = 2
				r.GPR[1] = 0
			case "write_only_protection":
				r.GPR[2] = 2
			case "unaligned_hint":
				r.GPR[0] = 0x100000001
			case "legacy_unaligned_offset":
				r.GPR[3] = 2
				r.GPR[5] = 1
				r.GPR[1] = 16
			default:
				t.Fatal(tc.Case)
			}
			before := r
			if k.mmapFile(&r, m) || r != before || len(m.Mappings()) != 0 {
				t.Fatal("unmodeled native boundary fabricated")
			}
		})
	}
}

func TestOwnedExecutableProtectionTransitionsRemainUnsupported(t *testing.T) {
	for _, tc := range []struct {
		name                string
		address, size, prot uint64
		allowed             bool
	}{
		{"owned", 0x4000, 0x4000, 5, false}, {"crossing", 0x4000, 0x8000, 5, false}, {"rounded_overlap", 0x4001, 1, 4, false}, {"adjacent_image", 0x8000, 0x4000, 5, true}, {"owned_nonexecute", 0x4000, 0x4000, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := kernelState{taskSendRefs: 1, ownedRanges: []addressSpan{{0x4000, 0x8000}}}
			m := &usermem.AddressSpace{}
			m.MapProtected(0x4000, 0x8000, usermem.Read|usermem.Write, usermem.Read|usermem.Write|usermem.Execute, nil)
			r := emu.Registers{}
			r.GPR[0] = 0x103
			r.GPR[1] = tc.address
			r.GPR[2] = tc.size
			r.GPR[4] = tc.prot
			r.GPR[16] = uint64(uint32(0xfffffff2))
			before := r
			handled := k.trap(&r, m)
			if handled != tc.allowed {
				t.Fatal(handled, tc.allowed)
			}
			if !handled && (r != before || m.Check(0x4000, 1, usermem.Write, "unchanged") != nil || m.Check(0x4000, 1, usermem.Execute, "unsupported") == nil) {
				t.Fatal("unsupported policy mutated mapping")
			}
			if handled && r.GPR[0] != 0 {
				t.Fatal(r.GPR[0])
			}
		})
	}
}
