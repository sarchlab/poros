package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

func TestCodeSigningStatusNativeContract(t *testing.T) {
	var cases []struct {
		Case         string
		Size, X0, X1 uint64
		Carry        bool
		Bytes        string
	}
	kernelNativeResult(t, "csops-status-probe.c", &cases)
	if len(cases) != 14 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			k := kernelState{amfi: &AMFIPolicy{CodeSigningStatus: 570556929}}
			r := emu.Registers{C: true}
			r.GPR[0] = 0
			r.GPR[1] = 0
			r.GPR[2] = 0x10000
			r.GPR[3] = tc.Size
			r.GPR[16] = 169
			length, perm := 16, usermem.Read|usermem.Write
			switch tc.Case {
			case "explicit_own_pid":
				r.GPR[0] = 100
			case "pid_low32_zero":
				r.GPR[0] = 1 << 32
			case "null_output_four_size", "null_output_zero_size":
				r.GPR[2] = 0
			case "readonly_output":
				perm = usermem.Read
			case "unmapped_output":
				r.GPR[2] = 1
			case "partial_one":
				length = 1
			case "partial_three":
				length = 3
			case "exact_four_guarded":
				length = 4
			}
			initial := bytes.Repeat([]byte{0xa5}, length)
			if err := m.Map(0x10000, uint64(length), perm, initial); err != nil {
				t.Fatal(err)
			}
			before := r
			handled := k.trap(&r, m)
			got := make([]byte, length)
			m.Read(0x10000, got)
			if strings.HasPrefix(tc.Case, "partial_") {
				if handled || r != before || !bytes.Equal(got, initial) {
					t.Fatal("partial-copyout case did not fail closed")
				}
				return
			}
			if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:4], r.C)
			}
			if tc.Bytes != "" {
				want, e := hex.DecodeString(tc.Bytes)
				if e != nil || !bytes.Equal(got, want) {
					t.Fatal("native output mismatch", got, want, e)
				}
			} else if !bytes.Equal(got, initial) {
				t.Fatal("unreferenced memory changed")
			}
		})
	}
}

func TestCodeSigningStatusCapturedValuesAndUnsupportedRequests(t *testing.T) {
	for _, status := range []uint32{0, 1, 0xaabbccdd} {
		m := &usermem.AddressSpace{}
		m.Map(0x10000, 4, usermem.Read|usermem.Write, nil)
		k := kernelState{amfi: &AMFIPolicy{CodeSigningStatus: status}}
		r := emu.Registers{}
		r.GPR[2] = 0x10000
		if !k.codeSigningStatus(&r, m) || r.C {
			t.Fatal("query failed")
		}
		var b [4]byte
		m.Read(0x10000, b[:])
		if binary.LittleEndian.Uint32(b[:]) != status {
			t.Fatal("hardcoded code-signing status")
		}
		for _, which := range []string{"foreign", "operation", "uncaptured"} {
			test := r
			test.GPR[2] = 0x10000
			state := k
			switch which {
			case "foreign":
				test.GPR[0] = 101
			case "operation":
				test.GPR[1] = 1
			case "uncaptured":
				state.amfi = nil
			}
			before := test
			if state.codeSigningStatus(&test, m) || test != before {
				t.Fatal(which)
			}
		}
	}
}
