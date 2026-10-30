package darwinuser

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

func kernelNativeResult(t *testing.T, source string, result any) {
	t.Helper()
	b, e := os.ReadFile("../../tests/macho/kernel-native-m2.json")
	if e != nil {
		t.Fatal(e)
	}
	var data struct {
		Observations []struct {
			Source string
			Hash   string `json:"source_sha256"`
			Result json.RawMessage
		}
	}
	if e = json.Unmarshal(b, &data); e != nil {
		t.Fatal(e)
	}
	for _, o := range data.Observations {
		if o.Source != source {
			continue
		}
		s, e := os.ReadFile("../../tests/macho/" + source)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(s)
		if hex.EncodeToString(sum[:]) != o.Hash {
			t.Fatal("native source identity mismatch")
		}
		if e = json.Unmarshal(o.Result, result); e != nil {
			t.Fatal(e)
		}
		return
	}
	t.Fatal("missing native observation", source)
}

func TestSelfPortReferenceNativeContract(t *testing.T) {
	var ob struct {
		Before  uint32
		First   uint32 `json:"after_first_self"`
		Second  uint32 `json:"after_second_self"`
		Zero    uint64 `json:"zero_delta_return"`
		Drop    uint64 `json:"drop_return"`
		After   uint32 `json:"after_drop"`
		Restore uint64 `json:"restore_return"`
		Final   uint32
		Same    bool `json:"same_name"`
	}
	kernelNativeResult(t, "port-refs-probe.c", &ob)
	if !ob.Same || ob.Before == 0 {
		t.Fatal(ob)
	}
	// Native startup already owned references. Use its measured baseline only in
	// this differential test; a newly constructed process starts with none.
	k := kernelState{taskSendRefs: ob.Before}
	m := &usermem.AddressSpace{}
	r := emu.Registers{C: true}
	r.GPR[16] = uint64(uint32(0xffffffe4))
	if !k.trap(&r, m) || r.GPR[0] != 0x103 || k.taskSendRefs != ob.First {
		t.Fatal(r, k)
	}
	if !k.trap(&r, m) || r.GPR[0] != 0x103 || k.taskSendRefs != ob.Second {
		t.Fatal(r, k)
	}
	for _, tc := range []struct {
		delta int32
		want  uint64
		refs  uint32
	}{{0, ob.Zero, ob.Second}, {-1, ob.Drop, ob.After}, {-1, ob.Restore, ob.Final}} {
		r.GPR[16] = 0xffffffed
		r.GPR[0] = 0x103
		r.GPR[1] = 0x103
		r.GPR[2] = 0
		r.GPR[3] = uint64(uint32(tc.delta))
		if !k.trap(&r, m) || r.GPR[0] != tc.want || k.taskSendRefs != tc.refs || !r.C || r.GPR[1] != 0x103 {
			t.Fatal(tc, r, k)
		}
	}
}

func TestNullPortReferencesNativeContract(t *testing.T) {
	var ob struct {
		Cases []struct {
			Name, Right uint32
			Delta       int32
			Return      uint64
		}
	}
	kernelNativeResult(t, "port-null-probe.c", &ob)
	if len(ob.Cases) != 24 {
		t.Fatal(len(ob.Cases))
	}
	for _, tc := range ob.Cases {
		k := kernelState{taskSendRefs: 1}
		r := emu.Registers{C: true}
		r.GPR[16] = 0xffffffed
		r.GPR[0] = 0x103
		r.GPR[1] = uint64(tc.Name)
		r.GPR[2] = uint64(tc.Right)
		r.GPR[3] = uint64(uint32(tc.Delta))
		if !k.trap(&r, &usermem.AddressSpace{}) || r.GPR[0] != tc.Return || !r.C || k.taskSendRefs != 1 {
			t.Fatal(tc, r, k)
		}
	}
}

func TestSelfRightsAcquisitionAndUnsupportedGuards(t *testing.T) {
	k := kernelState{}
	m := &usermem.AddressSpace{}
	for _, tc := range []struct {
		trap uint64
		name uint64
		refs *uint32
	}{{0xffffffe4, 0x103, &k.taskSendRefs}, {0xffffffe3, 0x107, &k.hostSendRefs}, {0xffffffe5, 0x10b, &k.threadSendRefs}} {
		r := emu.Registers{}
		r.GPR[16] = tc.trap
		if !k.trap(&r, m) || r.GPR[0] != tc.name || *tc.refs != 1 {
			t.Fatal(tc, r)
		}
	}
	for _, tc := range []struct {
		name, right uint32
		delta       int32
	}{{0x103, 0, -1}, {0x107, 0, -2}, {0x10b, 0, 65535}, {0x999, 0, 0}, {0x103, 1, 0}} {
		before := k
		r := emu.Registers{}
		r.GPR[16] = 0xffffffed
		r.GPR[0] = 0x103
		r.GPR[1] = uint64(tc.name)
		r.GPR[2] = uint64(tc.right)
		r.GPR[3] = uint64(uint32(tc.delta))
		old := r
		if k.trap(&r, m) || r != old || k.taskSendRefs != before.taskSendRefs || k.hostSendRefs != before.hostSendRefs || k.threadSendRefs != before.threadSendRefs {
			t.Fatal("unsupported guard mutated state", tc)
		}
	}
	k.taskSendRefs = 0
	r := emu.Registers{}
	r.GPR[16] = 0xffffffed
	r.GPR[0] = 0x103
	if !k.trap(&r, m) || r.GPR[0] != 0x10000003 {
		t.Fatal("unowned task name accepted", r)
	}
}

func TestConstructPortNativeContract(t *testing.T) {
	var ob struct {
		Cases []struct {
			Case        int
			OptionsSize int `json:"options_size"`
			Return      uint64
			Changed     bool   `json:"output_changed"`
			Matches     bool   `json:"output_matches_new"`
			Added       int    `json:"ports_added"`
			Type        uint32 `json:"new_type"`
			Receive     uint32 `json:"receive_refs"`
			Send        uint32 `json:"send_refs"`
			Context     uint64
		}
	}
	kernelNativeResult(t, "port-construct-probe.c", &ob)
	if len(ob.Cases) != 12 {
		t.Fatal(len(ob.Cases))
	}
	for _, tc := range ob.Cases {
		if tc.OptionsSize != 24 {
			t.Fatal(tc)
		}
		k := kernelState{taskSendRefs: 1}
		m := &usermem.AddressSpace{}
		m.Map(0x1000, vmPage, usermem.Read|usermem.Write, nil)
		var options [24]byte
		binary.LittleEndian.PutUint32(options[:], 0x1000)
		r := emu.Registers{C: true}
		r.GPR[16] = 0xffffffe8
		r.GPR[0] = 0x103
		r.GPR[1] = 0x1000
		r.GPR[3] = 0x1100
		switch tc.Case {
		case 0:
		case 1:
			clear(options[:4])
		case 2:
			r.GPR[2] = 0x12345678
		case 3:
			binary.LittleEndian.PutUint32(options[4:8], 99)
		case 4:
			for i := 8; i < 24; i++ {
				options[i] = 255
			}
		case 5:
			r.GPR[1] = 0
		case 6:
			r.GPR[1] = 1
		case 7:
			r.GPR[3] = 0
		case 8:
			r.GPR[3] = 0x6000
			m.Map(0x6000, vmPage, usermem.Read, nil)
		case 9:
			r.GPR[0] = 0
		case 10:
			binary.LittleEndian.PutUint32(options[:4], 0x80000000)
		case 11:
			binary.LittleEndian.PutUint32(options[:4], 0x1002)
			binary.LittleEndian.PutUint32(options[4:8], ^uint32(0))
		default:
			t.Fatal(tc.Case)
		}
		m.Write(0x1000, options[:])
		var output [4]byte
		binary.LittleEndian.PutUint32(output[:], 0xdeadbeef)
		m.Write(0x1100, output[:])
		if !k.trap(&r, m) || r.GPR[0] != tc.Return || !r.C || len(k.receivePorts) != tc.Added {
			t.Fatal(tc, r, k)
		}
		m.Read(0x1100, output[:])
		name := binary.LittleEndian.Uint32(output[:])
		_, exists := k.receivePorts[name]
		if (name != 0xdeadbeef) != tc.Changed || exists != tc.Matches {
			t.Fatal("copyout mismatch", tc, name)
		}
		for _, port := range k.receivePorts {
			if tc.Receive != 1 || tc.Send != 0 || tc.Type != 0x20000 || port.context != tc.Context || port.typedReply != (tc.Case != 1) || port.queueLimit != 5 {
				t.Fatal("receive port metadata", tc, port)
			}
		}
	}
}

func TestConstructPortBoundaries(t *testing.T) {
	for _, flags := range []uint32{1, 4, 0x10, 0x400, 0x800, 0x2000} {
		m := &usermem.AddressSpace{}
		var options [24]byte
		binary.LittleEndian.PutUint32(options[:], flags)
		m.Map(0x1000, 24, usermem.Read, options[:])
		k := kernelState{taskSendRefs: 1}
		r := emu.Registers{}
		r.GPR[16] = 0xffffffe8
		r.GPR[0] = 0x103
		r.GPR[1] = 0x1000
		r.GPR[3] = 0x2000
		before := r
		if k.trap(&r, m) || r != before || len(k.receivePorts) != 0 {
			t.Fatal("unsupported port options mutated state", flags)
		}
	}
	// Copyin covers the complete struct even when the first eight bytes suffice
	// for this port type; no allocation occurs on a truncated options mapping.
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 23, usermem.Read, nil)
	k := kernelState{taskSendRefs: 1}
	r := emu.Registers{}
	r.GPR[16] = 0xffffffe8
	r.GPR[0] = 0x103
	r.GPR[1] = 0x1000
	if !k.trap(&r, m) || r.GPR[0] != 10 || len(k.receivePorts) != 0 {
		t.Fatal(r, k)
	}
}
