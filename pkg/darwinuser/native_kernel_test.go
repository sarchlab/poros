package darwinuser

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"os"
	"testing"
)

func TestMachMapNativeContract(t *testing.T) {
	var result struct {
		Cases []struct {
			Name      string
			Return    uint64
			Readable  bool `json:"gap_readable"`
			Zero      bool `json:"zero_fill"`
			Address   bool `json:"address_equals_gap"`
			MaskMatch bool `json:"returned_mask_match"`
		}
	}
	kernelNativeResult(t, "vm-map-probe.c", &result)
	if len(result.Cases) != 11 {
		t.Fatal("native map case count")
	}
	const gap uint64 = 0x200000000
	for _, tc := range result.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			m.Map(0x1000, 16, usermem.Read|usermem.Write, nil)
			r := vmArgs(t, m, gap)
			r.GPR[2] = vmPage
			r.GPR[4] = 0
			switch tc.Name {
			case "invalid-task":
				r.GPR[0] = 0
			case "null-address-pointer":
				r.GPR[1] = 0
			case "bad-address-pointer":
				r.GPR[1] = 1
			case "zero-size":
				r.GPR[2] = 0
			case "one-byte-size":
				r.GPR[2] = 1
			case "unaligned-fixed-hint":
				var v [8]byte
				binary.LittleEndian.PutUint64(v[:], gap+1)
				m.Write(0x1000, v[:])
			case "invalid-protection":
				r.GPR[5] = 8
			case "noncontiguous-mask":
				r.GPR[3] = 5
			case "maximum-mask":
				r.GPR[3] = ^uint64(0)
			case "readonly-address-pointer":
				m.Protect(0x1000, 16, usermem.Read, false)
			case "noncontiguous-large-anywhere-mask":
				r.GPR[3] = 0x14000
				r.GPR[4] = 1
				var v [8]byte
				binary.LittleEndian.PutUint64(v[:], gap|0x4000)
				m.Write(0x1000, v[:])
			default:
				t.Fatal("unknown native case")
			}
			k := kernelState{taskSendRefs: 1}
			if !k.trap(&r, m) || r.GPR[0] != tc.Return {
				t.Fatal("native return mismatch", r.GPR[0], tc.Return)
			}
			var data [1]byte
			readable := m.Read(gap, data[:]) == nil
			var addr [8]byte
			m.Read(0x1000, addr[:])
			if readable != tc.Readable || (readable && data[0] == 0) != tc.Zero || (binary.LittleEndian.Uint64(addr[:]) == gap) != tc.Address || (binary.LittleEndian.Uint64(addr[:])&r.GPR[3] == 0) != tc.MaskMatch {
				t.Fatal("native side effect mismatch", readable, data, addr)
			}
		})
	}
}

func TestHandledSVCClearsExclusiveLikeNative(t *testing.T) {
	b, err := os.ReadFile("../../tests/macho/kernel-native-m2.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Observations []struct {
			Source string
			Hash   string `json:"source_sha256"`
			Result struct {
				Control int `json:"control_success"`
				SVC     int `json:"svc_failed"`
				Bad     int
			}
		}
	}
	if err = json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ob := range data.Observations {
		if ob.Source == "exclusive-svc-probe.c" {
			src, e := os.ReadFile("../../tests/macho/" + ob.Source)
			if e != nil {
				t.Fatal(e)
			}
			hash := sha256.Sum256(src)
			if hex.EncodeToString(hash[:]) != ob.Hash || ob.Result.Bad != 0 || ob.Result.Control != 100 || ob.Result.SVC != 100 {
				t.Fatal("native SVC evidence invalid")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("native SVC evidence missing")
	}
	for _, svc := range []bool{false, true} {
		runtime := machotest.Image(7)
		word := uint32(0xd503201f)
		if svc {
			word = 0xd4001001
		}
		// LDXR W10,[X9]; MOV X16,#20; SVC/NOP; MOV W11,#9; STXR W12,W11,[X9]; BRK.
		for i, w := range []uint32{0x885f7d2a, 0xd2800290, word, 0x5280012b, 0x880c7d2b, 0xd4200000} {
			binary.LittleEndian.PutUint32(runtime[machotest.EntryOffset+i*4:], w)
		}
		p, e := Parse(machotest.Image(2), runtime, Options{})
		if e != nil {
			t.Fatal(e)
		}
		p.Registers.Exclusive.Policy = emu.ExclusiveM2Local64
		p.Registers.GPR[9] = 0x100004000
		p.Memory.Write(0x100004000, []byte{7, 0, 0, 0})
		result, e := p.Run(context.Background(), RunOptions{})
		if e != nil || result.Instructions != 5 || result.Termination != "unsupported_instruction" {
			t.Fatal(result, e)
		}
		status, value := uint64(0), uint32(9)
		if svc {
			status = 1
			value = 7
		}
		var got [4]byte
		p.Memory.Read(0x100004000, got[:])
		if p.Registers.GPR[12] != status || binary.LittleEndian.Uint32(got[:]) != value || p.Registers.Exclusive.Valid {
			t.Fatal("native SVC reservation mismatch", svc, p.Registers.GPR[12], got)
		}
	}
}

func TestMachDeallocateNativeContract(t *testing.T) {
	var dataset struct {
		Observations []struct {
			Source string
			Hash   string `json:"source_sha256"`
			Result struct {
				Cases []struct {
					Case         int
					Return       uint64
					BeforeMask   uint32 `json:"before_mask"`
					AfterMask    uint32 `json:"after_mask"`
					MappedBefore uint32 `json:"mapped_before"`
					MappedAfter  uint32 `json:"mapped_after"`
					PageSize     uint64 `json:"page_size"`
				}
			}
		}
	}
	data, e := os.ReadFile("../../tests/macho/kernel-native-m2.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &dataset); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, ob := range dataset.Observations {
		if ob.Source != "vm-deallocate-probe.c" {
			continue
		}
		found = true
		src, e := os.ReadFile("../../tests/macho/" + ob.Source)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(src)
		if hex.EncodeToString(sum[:]) != ob.Hash || len(ob.Result.Cases) != 14 {
			t.Fatal("native deallocate identity/cases")
		}
		for _, tc := range ob.Result.Cases {
			if tc.PageSize != vmPage {
				t.Fatal("native page mismatch")
			}
			const base uint64 = 0x200000000
			m := &usermem.AddressSpace{}
			if e := m.Map(base, 7*vmPage, usermem.Read|usermem.Write, nil); e != nil {
				t.Fatal(e)
			}
			r := emu.Registers{C: true}
			r.GPR[16] = 0xfffffff4
			r.GPR[0] = 0x103
			r.GPR[1] = base
			r.GPR[2] = 5 * vmPage
			switch tc.Case {
			case 0:
			case 1:
				r.GPR[1] += vmPage
				r.GPR[2] = 0
			case 2:
				r.GPR[0] = 0
			case 3:
				r.GPR[1] += vmPage + 1
				r.GPR[2] = 1
			case 4:
				r.GPR[1] += vmPage + 1
				r.GPR[2] = vmPage
			case 5:
				m.Unmap(base+2*vmPage, vmPage)
				r.GPR[1] += 2 * vmPage
				r.GPR[2] = vmPage
			case 6:
				m.Unmap(base+2*vmPage, vmPage)
			case 7:
				r.GPR[1] = 0
				r.GPR[2] = 0
			case 8:
				r.GPR[1] = 0
				r.GPR[2] = 1
			case 9:
				r.GPR[1] = ^uint64(0)
				r.GPR[2] = 1
			case 10:
				r.GPR[1] = 1 << 47
				r.GPR[2] = vmPage
			case 11:
				r.GPR[1] = ^uint64(0) - vmPage + 1
				r.GPR[2] = 2 * vmPage
			case 12:
				r.GPR[1] += vmPage
				r.GPR[2] = vmPage
				m.Protect(base+vmPage, vmPage, 0, false)
			case 13:
				r.GPR[1] += vmPage
				r.GPR[2] = 3 * vmPage
				m.Protect(base+2*vmPage, vmPage, usermem.Read, false)
			default:
				t.Fatal(tc.Case)
			}
			masks := func() (uint32, uint32) {
				var read, mapped uint32
				for i := uint32(0); i < 5; i++ {
					a := base + uint64(i)*vmPage
					if m.Check(a, 1, usermem.Read, "probe") == nil {
						read |= 1 << i
					}
					for _, v := range m.Mappings() {
						if v.Start <= a && a < v.End {
							mapped |= 1 << i
						}
					}
				}
				return read, mapped
			}
			read, mapped := masks()
			if read != tc.BeforeMask || mapped != tc.MappedBefore {
				t.Fatal("native setup mismatch", tc.Case, read, mapped)
			}
			k := kernelState{taskSendRefs: 1}
			if !k.trap(&r, m) || r.GPR[0] != tc.Return || !r.C {
				t.Fatal(tc, r)
			}
			read, mapped = masks()
			if read != tc.AfterMask || mapped != tc.MappedAfter {
				t.Fatal("native unmap mismatch", tc.Case, read, mapped)
			}
		}
	}
	if !found {
		t.Fatal("missing native deallocate observations")
	}
}

func TestNativeFinalizedDyldRegistration(t *testing.T) {
	var dataset struct {
		Observations []struct {
			Source string
			Hash   string `json:"source_sha256"`
			Result struct {
				Completed bool `json:"startup_already_completed"`
				Cases     []struct {
					Case          int
					X0, X1, Carry uint64
					Unchanged     bool `json:"metadata_unchanged"`
				}
			}
		}
	}
	b, e := os.ReadFile("../../tests/macho/kernel-native-m2.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &dataset); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, ob := range dataset.Observations {
		if ob.Source != "proc-dyld-probe.c" {
			continue
		}
		found = true
		src, e := os.ReadFile("../../tests/macho/" + ob.Source)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(src)
		if hex.EncodeToString(sum[:]) != ob.Hash || !ob.Result.Completed || len(ob.Result.Cases) != 8 {
			t.Fatal("native finalized metadata provenance")
		}
		for _, tc := range ob.Result.Cases {
			k := kernelState{taskSendRefs: 1, dyldInfoAddress: 0x2000, dyldInfoSize: 0x170, dyldInfoFinal: true}
			r := emu.Registers{}
			r.GPR[16] = 336
			r.GPR[0] = 15
			r.GPR[1] = 100
			r.GPR[4] = 0x2000
			r.GPR[5] = 0x170
			switch tc.Case {
			case 0:
			case 1:
				r.GPR[4] = 0
			case 2:
				r.GPR[1]++
			case 3:
				r.GPR[4] = 1
				r.GPR[5] = 0
			case 4:
				r.GPR[4] = ^uint64(0)
				r.GPR[5] = 1
			case 5:
				r.GPR[5] = 0
			case 6:
				r.GPR[5] = 0xffffffff
			case 7:
				r.GPR[2] = 99
			default:
				t.Fatal(tc.Case)
			}
			if !k.trap(&r, &usermem.AddressSpace{}) || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != (tc.Carry != 0) || !tc.Unchanged || k.dyldInfoAddress != 0x2000 || k.dyldInfoSize != 0x170 {
				t.Fatal(tc, r, k)
			}
		}
	}
	if !found {
		t.Fatal("missing native finalized metadata")
	}
}

func TestMunmapNativeContract(t *testing.T) {
	var dataset struct {
		Observations []struct {
			Source string
			Hash   string `json:"source_sha256"`
			Result struct {
				Cases []struct {
					Case          int
					X0, X1, Carry uint64
					BeforeMask    uint32 `json:"before_mask"`
					AfterMask     uint32 `json:"after_mask"`
					MappedBefore  uint32 `json:"mapped_before"`
					MappedAfter   uint32 `json:"mapped_after"`
					PageSize      uint64 `json:"page_size"`
				}
			}
		}
	}
	data, e := os.ReadFile("../../tests/macho/kernel-native-m2.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &dataset); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, ob := range dataset.Observations {
		if ob.Source != "munmap-probe.c" {
			continue
		}
		found = true
		src, e := os.ReadFile("../../tests/macho/" + ob.Source)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(src)
		if hex.EncodeToString(sum[:]) != ob.Hash || len(ob.Result.Cases) != 14 {
			t.Fatal("native deallocate identity/cases")
		}
		for _, tc := range ob.Result.Cases {
			if tc.PageSize != vmPage {
				t.Fatal("native page mismatch")
			}
			const base uint64 = 0x200000000
			m := &usermem.AddressSpace{}
			if e := m.Map(base, 7*vmPage, usermem.Read|usermem.Write, nil); e != nil {
				t.Fatal(e)
			}
			r := emu.Registers{C: true}
			r.GPR[16] = 73
			r.GPR[0] = base
			r.GPR[1] = 5 * vmPage
			switch tc.Case {
			case 0:
			case 1:
				r.GPR[0] += vmPage
				r.GPR[1] = 0
			case 2:
				r.GPR[0] += vmPage
				r.GPR[1] = 1
			case 3:
				r.GPR[0] += vmPage + 1
				r.GPR[1] = 1
			case 4:
				r.GPR[0] += vmPage + 1
				r.GPR[1] = vmPage
			case 5:
				m.Unmap(base+2*vmPage, vmPage)
				r.GPR[0] += 2 * vmPage
				r.GPR[1] = vmPage
			case 6:
				m.Unmap(base+2*vmPage, vmPage)
			case 7:
				r.GPR[0] = 0
				r.GPR[1] = 0
			case 8:
				r.GPR[0] = 0
				r.GPR[1] = 1
			case 9:
				r.GPR[0] = ^uint64(0)
				r.GPR[1] = 1
			case 10:
				r.GPR[0] = 1 << 47
				r.GPR[1] = vmPage
			case 11:
				r.GPR[0] = ^uint64(0) - vmPage + 1
				r.GPR[1] = 2 * vmPage
			case 12:
				r.GPR[0] += vmPage
				r.GPR[1] = vmPage
				m.Protect(base+vmPage, vmPage, 0, false)
			case 13:
				r.GPR[0] += vmPage
				r.GPR[1] = 3 * vmPage
				m.Protect(base+2*vmPage, vmPage, usermem.Read, false)
			default:
				t.Fatal(tc.Case)
			}
			masks := func() (uint32, uint32) {
				var read, mapped uint32
				for i := uint32(0); i < 5; i++ {
					a := base + uint64(i)*vmPage
					if m.Check(a, 1, usermem.Read, "probe") == nil {
						read |= 1 << i
					}
					for _, v := range m.Mappings() {
						if v.Start <= a && a < v.End {
							mapped |= 1 << i
						}
					}
				}
				return read, mapped
			}
			read, mapped := masks()
			if read != tc.BeforeMask || mapped != tc.MappedBefore {
				t.Fatal("native setup mismatch", tc.Case, read, mapped)
			}
			k := kernelState{taskSendRefs: 1}
			if !k.trap(&r, m) || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != (tc.Carry != 0) {
				t.Fatal(tc, r)
			}
			read, mapped = masks()
			if read != tc.AfterMask || mapped != tc.MappedAfter {
				t.Fatal("native unmap mismatch", tc.Case, read, mapped)
			}
		}
	}
	if !found {
		t.Fatal("missing native deallocate observations")
	}
}
