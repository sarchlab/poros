package darwinuser

import (
	"bytes"
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func TestFSGetPathNativeContract(t *testing.T) {
	var cases []struct {
		Case          int
		X0, X1, Carry uint64
		Length        int  `json:"path_length_with_nul"`
		Matches       bool `json:"path_matches"`
		Changed       int  `json:"changed_extent"`
	}
	kernelNativeResult(t, "fsgetpath-probe.c", &cases)
	if len(cases) != 16 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		m := &usermem.AddressSpace{}
		initial := bytes.Repeat([]byte{0xa5}, 4096)
		if err := m.Map(0x10000, 4096, usermem.Read|usermem.Write, initial); err != nil {
			t.Fatal(err)
		}
		fsid := uint64(0x123456780000000d)
		inode := uint64(123)
		path := "/" + strings.Repeat("x", tc.Length-2)
		k := kernelState{files: []FileIdentity{{FSID: fsid, ObjectID: inode, Path: path}}}
		r := emu.Registers{C: true}
		r.GPR[0] = 0x10000
		r.GPR[1] = 4096
		r.GPR[2] = 0x20000
		r.GPR[3] = inode
		r.GPR[16] = 427
		switch tc.Case {
		case 1:
			r.GPR[1] = 0
		case 2:
			r.GPR[1] = uint64(tc.Length - 1)
		case 3:
			r.GPR[1] = uint64(tc.Length)
		case 4:
			r.GPR[0] = 0
		case 5:
			r.GPR[2] = 0
		case 6:
			r.GPR[2] = 1
			r.GPR[1] = 0
		case 7:
			fsid &= 0xffffffff00000000
		case 8:
			fsid ^= 0x1234567800000000
		case 9:
			r.GPR[3] = ^uint64(0)
		case 10:
			r.GPR[1] = ^uint64(0)
		case 11:
			r.GPR[1] = 1
		case 12:
			r.GPR[1] = 8192
		case 13:
			r.GPR[1] = 8193
		case 14:
			r.GPR[0] = 0
			r.GPR[1] = 0
		case 15:
			r.GPR[2] = 0
			r.GPR[1] = ^uint64(0)
		}
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], fsid)
		if err := m.Map(0x20000, 8, usermem.Read, b[:]); err != nil {
			t.Fatal(err)
		}
		before := r
		handled := k.trap(&r, m)
		if tc.Case == 7 { // Native filesystem-specific missing-mount result is not a generic rule.
			if handled || r != before {
				t.Fatal("unknown mount did not fail closed")
			}
			continue
		}
		if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != (tc.Carry != 0) {
			t.Fatal(tc, r.GPR[:4], r.C)
		}
		got := make([]byte, 4096)
		if err := m.Read(0x10000, got); err != nil {
			t.Fatal(err)
		}
		want := append([]byte(nil), initial...)
		if tc.Matches {
			copy(want, append([]byte(path), 0))
			if tc.Changed != tc.Length {
				t.Fatal(tc)
			}
		} else if tc.Changed != 0 {
			t.Fatal(tc)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("path or untouched tail differs", tc)
		}
	}
}
func TestFSGetPathMixedPermissionOutputIsExplicitlyUnsupported(t *testing.T) {
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 1, usermem.Write, nil)
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], 1)
	m.Map(0x2000, 8, usermem.Read, b[:])
	k := kernelState{files: []FileIdentity{{FSID: 1, ObjectID: 2, Path: "/file"}}}
	r := emu.Registers{}
	r.GPR[0] = 0x1000
	r.GPR[1] = 8
	r.GPR[2] = 0x2000
	r.GPR[3] = 2
	before := r
	if k.fsgetpath(&r, m) || r != before {
		t.Fatal("partial-copyout scope silently expanded")
	}
}
