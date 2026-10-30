package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"strconv"
	"strings"
	"testing"
)

func TestSandboxSyscallNativeContract(t *testing.T) {
	var cases []struct {
		Case      string
		X0, X1    uint64
		Carry     bool
		Unchanged bool   `json:"request_unchanged"`
		Output    string `json:"output_hex"`
	}
	kernelNativeResult(t, "sandbox-raw-probe.c", &cases)
	if len(cases) != 21 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			request := make([]byte, 160)
			output := bytes.Repeat([]byte{0xa5}, 32)
			if tc.Case == "zero_output" {
				clear(output)
			}
			for off, value := range map[int]uint64{0: 0x2000, 8: 100, 16: 0x4000, 24: 0x41, 32: 550, 40: 1} {
				binary.LittleEndian.PutUint64(request[off:], value)
			}
			r := emu.Registers{C: true}
			r.GPR[0] = 0x5000
			r.GPR[1] = 2
			r.GPR[2] = 0x1000
			r.GPR[16] = 381
			requestPerm, outputPerm := usermem.Read|usermem.Write, usermem.Read|usermem.Write
			policy := "Sandbox"
			length := 160
			switch tc.Case {
			case "readonly_request":
				requestPerm = usermem.Read
			case "null_request":
				r.GPR[2] = 0
			case "null_output":
				binary.LittleEndian.PutUint64(request, 0)
			case "readonly_output":
				outputPerm = usermem.Read
			case "null_policy":
				r.GPR[0] = 0
			case "unknown_policy":
				policy = "PorosMissingPolicy"
			case "bad_operation":
				binary.LittleEndian.PutUint64(request[16:], 1)
			case "truncated_request":
				length = 8
			default:
				if strings.HasPrefix(tc.Case, "request") {
					for _, n := range []int{48, 56, 64, 80, 96, 112, 128, 144, 152, 159, 160} {
						if tc.Case == "request"+strconv.Itoa(n) {
							length = n
						}
					}
				}
			}
			for _, v := range []struct {
				addr uint64
				data []byte
				perm usermem.Permission
			}{{0x1000, request[:length], requestPerm}, {0x2000, output, outputPerm}, {0x4000, []byte("syscall-unix\x00"), usermem.Read}, {0x5000, append([]byte(policy), 0), usermem.Read}} {
				if e := m.Map(v.addr, uint64(len(v.data)), v.perm, v.data); e != nil {
					t.Fatal(e)
				}
			}
			k := kernelState{sandboxSyscalls: []SandboxSyscallPermission{{550, 0}}}
			before := r
			handled := k.trap(&r, m)
			if tc.Case == "unknown_policy" {
				if handled || r != before {
					t.Fatal("unmodeled policy assumed missing")
				}
			} else if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:3], r.C)
			}
			got := make([]byte, 32)
			m.Read(0x2000, got)
			want, e := hex.DecodeString(tc.Output)
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("native output extent differs", got, want)
			}
			original := make([]byte, length)
			m.Read(0x1000, original)
			if !tc.Unchanged || !bytes.Equal(original, request[:length]) {
				t.Fatal("request was modified")
			}
		})
	}
}

func TestSandboxUnknownRulesAndPartialOutputFailClosed(t *testing.T) {
	for _, kind := range []string{"missing", "nonzero", "other-syscall", "other-pid", "other-operation", "other-flags", "reserved", "partial-output"} {
		t.Run(kind, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			request := make([]byte, 160)
			for off, value := range map[int]uint64{0: 0x2000, 8: 100, 16: 0x4000, 24: 0x41, 32: 550, 40: 1} {
				binary.LittleEndian.PutUint64(request[off:], value)
			}
			k := kernelState{sandboxSyscalls: []SandboxSyscallPermission{{550, 0}}}
			operation := "syscall-unix"
			size := uint64(32)
			switch kind {
			case "missing":
				k.sandboxSyscalls = nil
			case "nonzero":
				k.sandboxSyscalls[0].Decision = 1
			case "other-syscall":
				request[32]++
			case "other-pid":
				request[8]++
			case "other-operation":
				operation = "unmodeled"
			case "other-flags":
				request[40]++
			case "reserved":
				request[159] = 1
			case "partial-output":
				size = 1
			}
			m.Map(0x1000, 160, usermem.Read, request)
			m.Map(0x2000, size, usermem.Read|usermem.Write, bytes.Repeat([]byte{0xa5}, int(size)))
			m.Map(0x4000, uint64(len(operation)+1), usermem.Read, append([]byte(operation), 0))
			m.Map(0x5000, 8, usermem.Read, []byte("Sandbox\x00"))
			r := emu.Registers{C: true}
			r.GPR[0] = 0x5000
			r.GPR[1] = 2
			r.GPR[2] = 0x1000
			r.GPR[16] = 381
			before := r
			if k.trap(&r, m) || r != before {
				t.Fatal("unsupported query mutated state")
			}
			b := make([]byte, size)
			m.Read(0x2000, b)
			if !bytes.Equal(b, bytes.Repeat([]byte{0xa5}, int(size))) {
				t.Fatal("unsupported query changed output")
			}
		})
	}
}
