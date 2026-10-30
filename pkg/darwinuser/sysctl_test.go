package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestSysctlNameToOIDNativeContract(t *testing.T) {
	var ob struct {
		Name  string
		MIB   []int32
		Cases []struct {
			Case    string
			X0, X1  uint64
			Carry   bool
			Initial uint64 `json:"initial_capacity"`
			Final   uint64 `json:"final_length"`
			Output  string `json:"output_hex"`
		}
	}
	kernelNativeResult(t, "sysctl-name-probe.c", &ob)
	if len(ob.Cases) != 17 || len(ob.MIB) != 3 {
		t.Fatal(ob)
	}
	for _, tc := range ob.Cases {
		t.Run(tc.Case, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			input := []byte(ob.Name)
			output := bytes.Repeat([]byte{0xa5}, 64)
			perm := usermem.Read | usermem.Write
			r := emu.Registers{C: true}
			r.GPR[0] = 0x1000
			r.GPR[1] = 2
			r.GPR[2] = 0x2000
			r.GPR[3] = 0x3000
			r.GPR[4] = 0x4000
			r.GPR[16] = 202
			switch tc.Case {
			case "include_nul":
				input = append(input, 0)
			case "trailing_dot":
				input = append(input, '.')
			case "embedded_nul_tail":
				input = append(append(input, 0), []byte("garbage")...)
			case "size_only":
				r.GPR[2] = 0
			case "readonly_output":
				perm = usermem.Read
			case "null_length":
				r.GPR[3] = 0
			case "bad_mib":
				r.GPR[0] = 1
			case "bad_new_pointer":
				r.GPR[4] = 1
			case "unknown_name":
				input = []byte("poros.nonexistent")
			}
			r.GPR[5] = uint64(len(input))
			if tc.Case == "empty_input" {
				r.GPR[5] = 0
			}
			if tc.Case == "too_long_input" {
				r.GPR[5] = 1024
			}
			var length [8]byte
			binary.LittleEndian.PutUint64(length[:], tc.Initial)
			m.Map(0x1000, 8, usermem.Read, []byte{0, 0, 0, 0, 3, 0, 0, 0})
			m.Map(0x2000, 64, perm, output)
			m.Map(0x3000, 8, usermem.Read|usermem.Write, length[:])
			m.Map(0x4000, uint64(len(input)), usermem.Read, input)
			k := kernelState{sysctlNames: []SysctlName{{Name: ob.Name, MIB: ob.MIB}}}
			before := r
			handled := k.trap(&r, m)
			if tc.Case == "unknown_name" {
				if handled || r != before {
					t.Fatal("unobserved namespace absence invented")
				}
			} else if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:6], r.C)
			}
			actual := make([]byte, 64)
			m.Read(0x2000, actual)
			want, e := hex.DecodeString(tc.Output)
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(actual[:32], want) || !bytes.Equal(actual[32:], output[32:]) {
				t.Fatal("OID copyout or unchanged tail differs", tc, actual)
			}
			m.Read(0x3000, length[:])
			if binary.LittleEndian.Uint64(length[:]) != tc.Final {
				t.Fatal("oldlen sideeffect mismatch", tc, length)
			}
		})
	}
}

func TestSysctlUnsupportedStateDoesNotMutate(t *testing.T) {
	for _, kind := range []string{"missing-profile", "numeric-query", "bad-length", "readonly-length", "partial-output"} {
		t.Run(kind, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			r := emu.Registers{C: true}
			r.GPR[0] = 0x1000
			r.GPR[1] = 2
			r.GPR[2] = 0x2000
			r.GPR[3] = 0x3000
			r.GPR[4] = 0x4000
			r.GPR[5] = 4
			r.GPR[16] = 202
			mib := []byte{0, 0, 0, 0, 3, 0, 0, 0}
			outSize := uint64(32)
			perm := usermem.Read | usermem.Write
			k := kernelState{sysctlNames: []SysctlName{{Name: "name", MIB: []int32{1, 2, 3}}}}
			switch kind {
			case "missing-profile":
				k.sysctlNames = nil
			case "numeric-query":
				mib[0] = 1
			case "bad-length":
				r.GPR[3] = 1
			case "readonly-length":
				perm = usermem.Read
			case "partial-output":
				outSize = 1
			}
			var length [8]byte
			binary.LittleEndian.PutUint64(length[:], 32)
			m.Map(0x1000, 8, usermem.Read, mib)
			m.Map(0x2000, outSize, usermem.Read|usermem.Write, bytes.Repeat([]byte{0xa5}, int(outSize)))
			m.Map(0x3000, 8, perm, length[:])
			m.Map(0x4000, 4, usermem.Read, []byte("name"))
			before := r
			if k.trap(&r, m) || r != before {
				t.Fatal("unsupported query changed architectural state")
			}
			out := make([]byte, outSize)
			m.Read(0x2000, out)
			if !bytes.Equal(out, bytes.Repeat([]byte{0xa5}, int(outSize))) {
				t.Fatal("unsupported query changed output")
			}
			var after [8]byte
			m.Read(0x3000, after[:])
			if after != length {
				t.Fatal("unsupported query changed length")
			}
		})
	}
}
