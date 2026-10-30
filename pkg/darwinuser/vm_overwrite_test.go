package darwinuser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/sarchlab/poros/pkg/usermem"
)

func TestMachOverwriteNativeContract(t *testing.T) {
	var native struct {
		Cases []struct {
			Case               int
			Return             uint64
			SlotDelta          int64 `json:"slot_delta"`
			InMappingSlotDelta int64 `json:"in_mapping_slot_delta"`
			Pages              []struct {
				Readable, Mapped bool
				Byte             byte
				Protection       uint32
				Maximum          uint32 `json:"max_protection"`
			}
		}
	}
	kernelNativeResult(t, "vm-overwrite-probe.c", &native)
	if len(native.Cases) != 20 {
		t.Fatal(len(native.Cases))
	}
	const base = uint64(0x100000000)
	for _, tc := range native.Cases {
		t.Run(fmt.Sprint(tc.Case), func(t *testing.T) {
			m := &usermem.AddressSpace{}
			k := kernelState{taskSendRefs: 1}
			if _, kr, ok := k.allocateAnonymous(m, base, 8*vmPage, 0, 0, 3, 7); !ok || kr != 0 {
				t.Fatal(kr, ok)
			}
			for i := 0; i < 8; i++ {
				if e := m.Write(base+uint64(i)*vmPage, bytes.Repeat([]byte{byte(0x40 + i)}, int(vmPage))); e != nil {
					t.Fatal(e)
				}
			}
			if e := m.Map(0x1000, 16, usermem.Read|usermem.Write, nil); e != nil {
				t.Fatal(e)
			}
			r := vmArgs(t, m, base+2*vmPage)
			r.GPR[2], r.GPR[4] = 2*vmPage, 0x07004000
			switch tc.Case {
			case 1:
				k.unmapRange(m, base+2*vmPage, base+4*vmPage)
			case 2:
				k.unmapRange(m, base+3*vmPage, base+4*vmPage)
			case 3:
				r = vmArgs(t, m, base+2*vmPage+1)
				r.GPR[2], r.GPR[4] = 1, 0x07004000
			case 4:
				r.GPR[5] = 0
			case 5:
				r.GPR[5] = 1
			case 6:
				r.GPR[5] = 8
			case 7:
				r.GPR[2] = 0
			case 8:
				r.GPR[0] = 0
			case 9:
				r.GPR[1] = 0
			case 10:
				m.Protect(0x1000, 16, usermem.Read, false)
			case 11:
				r.GPR[4] |= 1
			case 12:
				r.GPR[3] = ^uint64(0)
			case 13:
				r.GPR[1] = base + 2*vmPage
				var slot [8]byte
				binary.LittleEndian.PutUint64(slot[:], r.GPR[1])
				m.Write(r.GPR[1], slot[:])
			case 14:
				m.Protect(base+2*vmPage, 2*vmPage, 0, false)
			case 15:
				m.Protect(base+2*vmPage, vmPage, usermem.Read, false)
			case 16:
				r.GPR[4] = 0x07000000
			case 17:
				r = vmArgs(t, m, base+3*vmPage-1)
				r.GPR[2], r.GPR[4] = vmPage+1, 0x07004000
			case 18:
				r.GPR[3] = 5
			case 19:
				k.unmapRange(m, base+2*vmPage, base+3*vmPage)
			}
			before := r
			if !k.trap(&r, m) || r.GPR[0] != tc.Return {
				t.Fatal(tc.Case, r.GPR, tc.Return)
			}
			want := before
			want.GPR[0] = tc.Return
			if r != want {
				t.Fatal("Mach trap changed unrelated register state")
			}
			var slot [8]byte
			if e := m.Read(0x1000, slot[:]); e != nil {
				t.Fatal(e)
			}
			address := binary.LittleEndian.Uint64(slot[:])
			if tc.Case == 11 {
				// Native placement is intentionally not deterministic; assert the
				// new range is separate and zero, while old pages below stay intact.
				if address < base+8*vmPage && address+2*vmPage > base {
					t.Fatal("ANYWHERE overwrote hint")
				}
				data := make([]byte, 2*vmPage)
				if m.Read(address, data) != nil || !bytes.Equal(data, make([]byte, len(data))) {
					t.Fatal("new anywhere mapping")
				}
			} else if int64(address-base) != tc.SlotDelta {
				t.Fatal("address copyout", address, tc.SlotDelta)
			}
			for i, p := range tc.Pages {
				address := base + uint64(i)*vmPage
				var b [1]byte
				readable := m.Read(address, b[:]) == nil
				if readable != p.Readable || (readable && !(tc.Case == 13 && i == 2) && b[0] != p.Byte) {
					t.Fatal("page data", i, b, p)
				}
				mapped := false
				for _, v := range m.Mappings() {
					if v.Start <= address && address < v.End {
						mapped = true
						if v.Flags != machPermissions(p.Protection) {
							t.Fatal("protection", i, v, p)
						}
					}
				}
				if mapped != p.Mapped {
					t.Fatal("mapped", i)
				}
				if mapped {
					for _, flag := range []uint32{1, 2, 4} {
						if (m.CheckProtection(address, vmPage, machPermissions(flag)) == nil) != (flag&^p.Maximum == 0) {
							t.Fatal("maximum", i, flag)
						}
					}
				}
			}
			if tc.Case == 13 {
				m.Read(base+2*vmPage, slot[:])
				if int64(binary.LittleEndian.Uint64(slot[:])-base) != tc.InMappingSlotDelta {
					t.Fatal("in-mapping copyout")
				}
			}
		})
	}
}

func TestMachOverwriteAtOwnedByteLimitUsesNetReplacement(t *testing.T) {
	m := &usermem.AddressSpace{}
	k := kernelState{taskSendRefs: 1}
	const base = uint64(0x100000000)
	if _, code, ok := k.allocateAnonymous(m, base, maxOwnedMappingBytes, 0, 0, 3, 7); !ok || code != 0 {
		t.Fatal(code, ok)
	}
	if e := m.Map(0x1000, 16, usermem.Read|usermem.Write, nil); e != nil {
		t.Fatal(e)
	}
	r := vmArgs(t, m, base+vmPage)
	r.GPR[4] = 0x4000
	if !k.trap(&r, m) || r.GPR[0] != 0 || k.ownedBytes != maxOwnedMappingBytes || k.ownedMappings != 3 {
		t.Fatal(r, k.ownedBytes, k.ownedMappings)
	}
}

func TestMachOverwriteOwnershipLimitsAndFileIsolation(t *testing.T) {
	const base = uint64(0x100000000)
	for _, kind := range []string{"protected", "reserved", "unowned", "quota", "split-limit", "private-file"} {
		t.Run(kind, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			k := kernelState{taskSendRefs: 1}
			file := bytes.Repeat([]byte{0x55}, int(3*vmPage))
			if _, kr, ok := k.allocateMemory(m, base, 3*vmPage, 0, 0, 3, 7, bytes.NewReader(file)); !ok || kr != 0 {
				t.Fatal(kr)
			}
			m.Write(base, []byte{0x66})
			m.Write(base+vmPage, []byte{0x77})
			m.Write(base+2*vmPage, []byte{0x88})
			m.Map(0x1000, 16, usermem.Read|usermem.Write, nil)
			r := vmArgs(t, m, base+vmPage)
			r.GPR[4] = 0x4000
			switch kind {
			case "protected":
				k.protected = []addressSpan{{base + vmPage, base + 2*vmPage}}
			case "reserved":
				k.reserved = []addressSpan{{base + vmPage, base + 2*vmPage}}
			case "unowned":
				k.ownedRanges = nil
			case "quota":
				k.ownedRanges = append(k.ownedRanges, addressSpan{0x200000000, 0x200000000 + maxOwnedMappingBytes})
			case "split-limit":
				for i := 0; i < 4094; i++ {
					lo := uint64(0x200000000) + uint64(i)*vmPage
					k.ownedRanges = append(k.ownedRanges, addressSpan{lo, lo + vmPage})
				}
			}
			before := r
			mappings := m.Mappings()
			owned := append([]addressSpan(nil), k.ownedRanges...)
			ok := k.trap(&r, m)
			if kind == "private-file" {
				if !ok || r.GPR[0] != 0 || k.ownedBytes != 3*vmPage || k.ownedMappings != 3 {
					t.Fatal(k, r)
				}
				for i, want := range []byte{0x66, 0, 0x88} {
					var b [1]byte
					m.Read(base+uint64(i)*vmPage, b[:])
					if b[0] != want {
						t.Fatal(i, b)
					}
				}
				if !bytes.Equal(file, bytes.Repeat([]byte{0x55}, len(file))) {
					t.Fatal("file changed")
				}
				return
			}
			if kind == "quota" || kind == "split-limit" {
				before.GPR[0] = 3
				if !ok {
					t.Fatal("missing resource error")
				}
			} else if ok {
				t.Fatal("unmodeled replacement accepted")
			}
			var b [1]byte
			m.Read(base+vmPage, b[:])
			if r != before || b[0] != 0x77 || !reflect.DeepEqual(mappings, m.Mappings()) || !reflect.DeepEqual(owned, k.ownedRanges) {
				t.Fatal("failed overwrite mutated state")
			}
		})
	}
}
