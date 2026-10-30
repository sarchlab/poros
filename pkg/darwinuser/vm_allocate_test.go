package darwinuser

import (
	"encoding/binary"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

func TestMachVMAllocateNativeContract(t *testing.T) {
	var cases []struct {
		Case                      string
		Return                    uint64
		X1Preserved               bool `json:"x1_preserved"`
		NZCV                      string
		AddressUnchanged          bool   `json:"address_unchanged"`
		AddressZero               bool   `json:"address_zero"`
		GapReadable               bool   `json:"gap_readable"`
		GapZero                   bool   `json:"gap_zero"`
		AllocationZero            bool   `json:"allocation_zero"`
		RegionSize                uint64 `json:"region_size"`
		Protection, MaxProtection int
	}
	kernelNativeResult(t, "vm-allocate-probe.c", &cases)
	if len(cases) != 11 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			const arena uint64 = 0x100008000
			const gap = arena + 65536
			m := &usermem.AddressSpace{}
			m.Map(arena, vmPage, 0, nil)
			hint := gap
			perm := usermem.Read | usermem.Write
			r := emu.Registers{N: true, C: true}
			r.GPR[0] = 0x103
			r.GPR[1] = 0x1000
			r.GPR[2] = 36
			r.GPR[3] = 0
			r.GPR[16] = 0xfffffff6
			switch tc.Case {
			case "anywhere_tag60_36":
				r.GPR[3] = 0x3c000001
				hint = 0
			case "invalid_task":
				r.GPR[0] = 0
			case "dead_task":
				r.GPR[0] = 0xffffffff
			case "null_address_pointer":
				r.GPR[1] = 0
			case "unmapped_address_pointer":
				r.GPR[1] = 1
			case "zero_size":
				r.GPR[2] = 0
			case "fixed_unaligned":
				hint++
			case "readonly_address_pointer":
				perm = usermem.Read
			case "fixed_collision":
				hint = arena
			case "overflow_size":
				r.GPR[2] = ^uint64(0)
			}
			var slot [8]byte
			binary.LittleEndian.PutUint64(slot[:], hint)
			m.Map(0x1000, 8, perm, slot[:])
			k := kernelState{taskSendRefs: 1}
			before := r
			if !k.trap(&r, m) || r.GPR[0] != tc.Return || r.GPR[1] != before.GPR[1] || !tc.X1Preserved || tc.NZCV != "a0000000" || !r.N || !r.C || r.Z || r.V {
				t.Fatal("return/register mismatch", tc, r.GPR[:4])
			}
			m.Read(0x1000, slot[:])
			address := binary.LittleEndian.Uint64(slot[:])
			if (address == hint) != tc.AddressUnchanged || (address == 0) != tc.AddressZero {
				t.Fatal("address copyout differs", address, hint, tc)
			}
			var byteAtGap [1]byte
			readable := m.Read(gap, byteAtGap[:]) == nil
			if readable != tc.GapReadable || (readable && byteAtGap[0] == 0) != tc.GapZero {
				t.Fatal("allocation side effect differs", tc, readable, byteAtGap)
			}
			if tc.AllocationZero {
				data := make([]byte, 36)
				if m.Read(address, data) != nil {
					t.Fatal("missing allocation")
				}
				for _, b := range data {
					if b != 0 {
						t.Fatal("allocation not zero")
					}
				}
				found := false
				for _, region := range m.Mappings() {
					if region.Start == address {
						found = true
						if region.End-region.Start != tc.RegionSize || region.Flags != usermem.Read|usermem.Write {
							t.Fatal("size/protection differs", region, tc)
						}
					}
				}
				if !found {
					t.Fatal("mapping absent")
				}
				if m.Protect(address, vmPage, usermem.Read|usermem.Write|usermem.Execute, false) != nil {
					t.Fatal("maximum permissions do not include all")
				}
			}
		})
	}
}

func TestMachVMAddressPartialCopyoutUnsupportedBeforeAllocation(t *testing.T) {
	for _, number := range []uint64{0xfffffff6, 0xfffffff1} {
		m := &usermem.AddressSpace{}
		m.Map(0x1000, 4, usermem.Read|usermem.Write, nil)
		m.Map(0x1004, 4, usermem.Read, nil)
		k := kernelState{taskSendRefs: 1}
		r := emu.Registers{}
		r.GPR[0] = 0x103
		r.GPR[1] = 0x1000
		r.GPR[2] = 36
		r.GPR[3] = 1
		r.GPR[4] = 1
		r.GPR[5] = 3
		r.GPR[16] = number
		before := r
		if k.trap(&r, m) || r != before || k.anonymousBytes != 0 || len(m.Mappings()) != 2 {
			t.Fatal("partial copyout mutated process", number)
		}
	}
}
