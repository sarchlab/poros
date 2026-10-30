package darwinuser

import (
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestMprotectNativeContract(t *testing.T) {
	var cases []struct {
		Case    string
		Length  uint64
		Prot    uint64 `json:"requested_protection"`
		X0, X1  uint64
		Carry   bool
		Regions []struct {
			Mapped     bool
			Protection uint32
			Maximum    uint32 `json:"max_protection"`
		}
	}
	kernelNativeResult(t, "mprotect-probe.c", &cases)
	if len(cases) != 20 {
		t.Fatal(len(cases))
	}
	for _, c := range cases {
		t.Run(c.Case, func(t *testing.T) {
			k := kernelState{}
			m := &usermem.AddressSpace{}
			m.MapProtected(0x10000, vmPage*3, usermem.Read|usermem.Write, usermem.Read|usermem.Write, nil)
			r := emu.Registers{}
			r.GPR[0] = 0x10000
			r.GPR[1] = c.Length
			r.GPR[2] = c.Prot
			r.GPR[16] = 74
			switch c.Case {
			case "unaligned_start", "zero_length_unaligned":
				r.GPR[0]++
			case "zero_length_null", "null_address":
				r.GPR[0] = 0
			case "address_overflow":
				r.GPR[0] = ^uint64(0) - 4095
			case "zero_length_high":
				r.GPR[0] = ^uint64(0) - 16383
			case "unmapped_middle", "range_spans_hole":
				m.Unmap(0x14000, vmPage)
				if c.Case == "unmapped_middle" {
					r.GPR[0] = 0x14000
				}
			case "mixed_maximum":
				m.Protect(0x10000, vmPage*3, usermem.Read, false)
				m.Protect(0x18000, vmPage, usermem.Read, true)
			}
			before := r
			handled := k.trap(&r, m)
			if c.Case == "execute_above_max" || c.Case == "unknown_protection_bit" {
				if handled || r != before {
					t.Fatal("unmodeled policy changed state")
				}
				return
			}
			if !handled || r.GPR[0] != c.X0 || r.GPR[1] != c.X1 || r.C != c.Carry || r.GPR[2] != c.Prot {
				t.Fatal(c, r)
			}
			for i, want := range c.Regions {
				address := uint64(0x10000) + uint64(i)*vmPage
				found := false
				for _, v := range m.Mappings() {
					if address >= v.Start && address < v.End {
						found = true
						if v.Flags != machPermissions(want.Protection) {
							t.Fatal("current protection", i, v)
						}
					}
				}
				if found != want.Mapped {
					t.Fatal("mapping presence", i)
				}
				if !found {
					continue
				}
				for _, prot := range []uint32{1, 2, 4} {
					e := m.Protect(address, vmPage, machPermissions(prot), false)
					if (e == nil) != (prot&^want.Maximum == 0) {
						t.Fatal("maximum changed", i, prot, e)
					}
				}
			}
		})
	}
}
