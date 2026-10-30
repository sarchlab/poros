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
	b, err := os.ReadFile("../../tests/macho/kernel-native-m2.json")
	if err != nil {
		t.Fatal(err)
	}
	var dataset struct {
		Observations []struct {
			Source string
			Hash   string `json:"source_sha256"`
			Result struct {
				Cases []struct {
					Name      string
					Return    uint64
					Readable  bool `json:"gap_readable"`
					Zero      bool `json:"zero_fill"`
					Address   bool `json:"address_equals_gap"`
					MaskMatch bool `json:"returned_mask_match"`
				}
			}
		}
	}
	if err = json.Unmarshal(b, &dataset); err != nil {
		t.Fatal(err)
	}
	if len(dataset.Observations) < 1 {
		t.Fatal("native observation missing")
	}
	ob := dataset.Observations[0]
	source, err := os.ReadFile("../../tests/macho/" + ob.Source)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	if hex.EncodeToString(sum[:]) != ob.Hash || len(ob.Result.Cases) != 11 {
		t.Fatal("native source/cases mismatch")
	}
	const gap uint64 = 0x200000000
	for _, tc := range ob.Result.Cases {
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
			k := kernelState{}
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
