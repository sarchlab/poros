package darwinuser

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"os"
	"strconv"
	"testing"
)

func pthreadKernel() *kernelState {
	return &kernelState{pthreadPolicy: &PthreadPolicy{Version: 60, Features: 0x400011df, MainQoS: 0x20ff}}
}
func pthreadRegisters(size, length uint64) emu.Registers {
	r := emu.Registers{Z: true, V: true}
	r.GPR[0] = 0x2000
	r.GPR[1] = 0x3000
	r.GPR[2] = size
	r.GPR[3] = 0x1000
	r.GPR[4] = length
	r.GPR[5] = 160
	r.GPR[16] = 366
	return r
}
func decodeNativeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, e := hex.DecodeString(s)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func assertPthreadReturn(t *testing.T, got, old emu.Registers, x0, x1, nzcv uint64) {
	t.Helper()
	old.GPR[0] = x0
	old.GPR[1] = x1
	old.N = nzcv>>31&1 != 0
	old.Z = nzcv>>30&1 != 0
	old.C = nzcv>>29&1 != 0
	old.V = nzcv>>28&1 != 0
	if got != old {
		t.Fatal("BSD registration return", got, old)
	}
}
func TestNativePthreadRegistrationFieldNormalization(t *testing.T) {
	var native struct {
		Rows []struct {
			Name          string
			X0, X1        uint64
			NZCV          uint64 `json:"output_nzcv"`
			Size, Pthsize uint64
			Before        string `json:"before_hex"`
			After         string `json:"after_hex"`
		}
	}
	kernelNativeResult(t, "registration-fields-probe.c", &native)
	if len(native.Rows) != 22 {
		t.Fatal(len(native.Rows))
	}
	for _, c := range native.Rows {
		t.Run(c.Name, func(t *testing.T) {
			before, after := decodeNativeHex(t, c.Before), decodeNativeHex(t, c.After)
			m := &usermem.AddressSpace{}
			if e := m.Map(0x1000, uint64(len(before)), usermem.Read|usermem.Write, before); e != nil {
				t.Fatal(e)
			}
			k := pthreadKernel()
			r := pthreadRegisters(c.Pthsize, c.Size)
			old := r
			if !k.trap(&r, m) {
				t.Fatal("unsupported native case")
			}
			assertPthreadReturn(t, r, old, c.X0, c.X1, c.NZCV)
			if c.X0 == uint64(k.pthreadPolicy.Features) {
				binary.LittleEndian.PutUint64(after[36:44], StackTop&^(vmPage-1))
				if k.pthreadRegistration == nil || k.pthreadRegistration.ThreadStart != old.GPR[0] || k.pthreadRegistration.WorkqueueStart != old.GPR[1] || k.pthreadRegistration.PthreadSize != uint32(c.Pthsize) {
					t.Fatal("registration not retained")
				}
			} else if k.pthreadRegistration != nil {
				t.Fatal("error registered process")
			}
			got := make([]byte, len(before))
			m.Read(0x1000, got)
			if !bytes.Equal(got, after) {
				t.Fatalf("native buffer mismatch\ngot %x\nwant%x", got, after)
			}
		})
	}
}
func TestNativePthreadRegistrationErrorsAndDuplicate(t *testing.T) {
	var native struct {
		Actual int `json:"actual_result"`
		Cases  []struct {
			Name   string
			X0, X1 uint64
			NZCV   uint64 `json:"output_nzcv"`
			After  string `json:"after_hex"`
		}
	}
	kernelNativeResult(t, "registration-error-probe.c", &native)
	if len(native.Cases) != 8 || native.Actual != 0x400011df {
		t.Fatal(native)
	}
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			b := decodeNativeHex(t, c.After)
			m := &usermem.AddressSpace{}
			m.Map(0x1000, uint64(len(b)), usermem.Read|usermem.Write, b)
			k := pthreadKernel()
			r := pthreadRegisters(16384, 60)
			switch c.Name {
			case "negative-pthsize":
				r.GPR[2] = 0xffffffff
			case "oversize-pthsize":
				r.GPR[2] = 0x7fffffff
			case "zero-data-size":
				r.GPR[4] = 0
			case "seven-data-size":
				r.GPR[4] = 7
			case "invalid-data-pointer":
				r.GPR[3] = 1
			case "version59-size60":
			case "version60-size61":
				r.GPR[4] = 61
			case "duplicate-registration":
				k.pthreadRegistration = &pthreadRegistration{}
			default:
				t.Fatal(c.Name)
			}
			old := r
			before := k.pthreadRegistration
			if !k.trap(&r, m) {
				t.Fatal("unsupported")
			}
			assertPthreadReturn(t, r, old, c.X0, c.X1, c.NZCV)
			got := make([]byte, len(b))
			m.Read(0x1000, got)
			if !bytes.Equal(got, b) || k.pthreadRegistration != before {
				t.Fatal("error mutation")
			}
		})
	}
}
func TestNativePthreadReadonlyCopyoutCommitsRegistration(t *testing.T) {
	type result struct{ X0, X1, NZCV uint64 }
	var native struct {
		First  result `json:"first_registration"`
		Second result `json:"second_registration"`
		Before string `json:"before_hex"`
		After  string `json:"after_hex"`
	}
	kernelNativeResult(t, "registration-copyout-probe.c", &native)
	b := decodeNativeHex(t, native.Before)
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 60, usermem.Read, b)
	k := pthreadKernel()
	for _, want := range []result{native.First, native.Second} {
		r := pthreadRegisters(16384, 60)
		old := r
		if !k.trap(&r, m) {
			t.Fatal("unsupported")
		}
		assertPthreadReturn(t, r, old, want.X0, want.X1, want.NZCV)
		if k.pthreadRegistration == nil {
			t.Fatal("copyout error rolled back registration")
		}
	}
	got := make([]byte, 60)
	m.Read(0x1000, got)
	if !bytes.Equal(got, decodeNativeHex(t, native.After)) {
		t.Fatal("readonly buffer changed")
	}
}
func TestNativePthreadNullLegacyData(t *testing.T) {
	type result struct{ X0, X1, NZCV uint64 }
	var native struct {
		Rows []struct {
			Name                  string
			First, Duplicate      result
			Size, Offset, Pthsize uint64
		}
	}
	kernelNativeResult(t, "registration-null-probe.c", &native)
	if len(native.Rows) != 4 {
		t.Fatal(len(native.Rows))
	}
	for _, c := range native.Rows {
		t.Run(c.Name, func(t *testing.T) {
			k := pthreadKernel()
			m := &usermem.AddressSpace{}
			r := pthreadRegisters(c.Pthsize, c.Size)
			r.GPR[3] = 0
			r.GPR[5] = c.Offset
			old := r
			if !k.trap(&r, m) {
				t.Fatal("unsupported")
			}
			assertPthreadReturn(t, r, old, c.First.X0, c.First.X1, c.First.NZCV)
			if k.pthreadRegistration == nil {
				t.Fatal("missing legacy registration")
			}
			b := make([]byte, 60)
			binary.LittleEndian.PutUint64(b, 60)
			m.Map(0x1000, 60, usermem.Read|usermem.Write, b)
			r = pthreadRegisters(16384, 60)
			old = r
			if !k.trap(&r, m) {
				t.Fatal("duplicate unsupported")
			}
			assertPthreadReturn(t, r, old, c.Duplicate.X0, c.Duplicate.X1, c.Duplicate.NZCV)
		})
	}
}
func TestPthreadUnsupportedAndBoundPolicy(t *testing.T) {
	for _, kind := range []string{"missing-policy", "partial-copyout"} {
		k := pthreadKernel()
		r := pthreadRegisters(16384, 60)
		m := &usermem.AddressSpace{}
		b := make([]byte, 60)
		binary.LittleEndian.PutUint64(b, 60)
		m.Map(0x1000, 60, usermem.Read|usermem.Write, b)
		if kind == "missing-policy" {
			k.pthreadPolicy = nil
		} else {
			m.Protect(0x1010, 44, usermem.Read, false)
		}
		old := r
		if k.trap(&r, m) || r != old || k.pthreadRegistration != nil || k.pthreadStackHint != 0 {
			t.Fatal("unsupported mutated state", kind)
		}
	}
	startup := startupFixture(t)
	startup.AMFI = &AMFIPolicy{CodeSigningStatus: 7}
	startup.Pthread = &PthreadPolicy{Version: 60, Features: 3, MainQoS: 99, CodeSigningStatus: 7}
	p, e := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: startup})
	if e != nil {
		t.Fatal(e)
	}
	startup.Pthread.MainQoS = 100
	if p.kernel.pthreadPolicy.MainQoS != 99 {
		t.Fatal("borrowed policy")
	}
	for _, kind := range []string{"signing", "version", "features"} {
		bad := *startup.Pthread
		switch kind {
		case "signing":
			bad.CodeSigningStatus = 8
		case "version":
			bad.Version = 64
		case "features":
			bad.Features = 0
		}
		startup.Pthread = &bad
		if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{StartupProfile: startup}); e == nil {
			t.Fatal("invalid policy", kind)
		}
		startup.Pthread = &PthreadPolicy{Version: 60, Features: 3, MainQoS: 99, CodeSigningStatus: 7}
	}
}

func TestDirectNativeStartupRegistrationAndThreadState(t *testing.T) {
	var native struct {
		Rows []struct {
			Workload    string
			Hash        string `json:"executable_sha256"`
			Unchanged   bool   `json:"native_unchanged"`
			Observation struct {
				CS         uint32 `json:"cs_status"`
				CSResult   int    `json:"cs_result"`
				StackTop   uint64 `json:"main_stack_top"`
				Calls      int    `json:"registration_calls"`
				Result     int
				Args       [6]uint64
				Before     string      `json:"before_hex"`
				After      string      `json:"after_hex"`
				Capability uint8       `json:"tpro_capability"`
				TproCalls  int         `json:"tpro_calls"`
				Values     [][2]string `json:"tpro_values"`
			}
		}
	}
	kernelNativeResult(t, "startup-interpose-probe.c", &native)
	if len(native.Rows) != 4 {
		t.Fatal(len(native.Rows))
	}
	for _, row := range native.Rows {
		data, e := os.ReadFile("../../tests/macho/" + row.Workload + ".macho")
		if e != nil {
			t.Fatal(e)
		}
		hash := sha256.Sum256(data)
		c := row.Observation
		if hex.EncodeToString(hash[:]) != row.Hash || !row.Unchanged || c.CSResult != 0 || c.CS == 0 || c.Calls != 1 || c.Result <= 0 || c.Capability != 3 || c.TproCalls != 1 || len(c.Values) != 1 || c.Values[0][0] != c.Values[0][1] {
			t.Fatal(row)
		}
		before, after := decodeNativeHex(t, c.Before), decodeNativeHex(t, c.After)
		if len(before) != 60 || len(after) != 60 || binary.LittleEndian.Uint64(after[36:44]) != c.StackTop&^(vmPage-1) {
			t.Fatal("native registration stack hint")
		}
		value, e := strconv.ParseUint(c.Values[0][0], 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		r := emu.Registers{PC: 0x1000, AppleThreadProtection: emu.AppleThreadProtectionState{Enabled: true, Value: value}}
		m := &usermem.AddressSpace{}
		m.Map(0x1000, 4, usermem.Execute, []byte{0xa8, 0xf1, 0x3e, 0xd5})
		if _, e := emu.StepChecked(&r, m); e != nil || r.GPR[8] != value {
			t.Fatal(e, r)
		}
	}
}
