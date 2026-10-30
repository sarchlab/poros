package darwinuser

import (
	"context"
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestKernelNamespaceAndTLS(t *testing.T) {
	var k kernelState
	r := emu.Registers{C: true, N: true}
	r.GPR[16] = uint64(uint32(0xffffffe4))
	if !k.trap(&r, nil) || r.GPR[0] != 0x103 || !r.C {
		t.Fatal(r)
	}
	var ports = map[uint64]bool{0x103: true, 0x107: true, 0x10b: true}
	for i := 0; i < 10; i++ {
		r.GPR[16] = uint64(uint32(0xffffffe6))
		if !k.trap(&r, nil) || ports[r.GPR[0]] {
			t.Fatal("reply port reused")
		}
		ports[r.GPR[0]] = true
	}
	r.GPR[16] = 0x80000000
	r.GPR[3] = 2
	r.GPR[0] = 0x12340
	before := r
	if !k.trap(&r, nil) || r.ThreadPointer.Value != 0x12340 || !r.ThreadPointer.Enabled {
		t.Fatal(r)
	}
	before.ThreadPointer = r.ThreadPointer
	if before != r {
		t.Fatal("platform set clobbered state")
	}
	r.GPR[3] = 3
	r.GPR[0] = 0
	if !k.trap(&r, nil) || r.GPR[0] != 0x12340 {
		t.Fatal("get TLS")
	}
	r.GPR[16] = 372
	if !k.trap(&r, nil) || r.GPR[0] != 1 || r.C {
		t.Fatal("BSD result")
	}
	r.GPR[16] = 9999
	before = r
	if k.trap(&r, nil) || r != before {
		t.Fatal("unknown trap committed")
	}
}
func TestDarwinRejectsLinuxSVCWithoutCommit(t *testing.T) {
	p := process(t, Options{})
	code := make([]byte, 4)
	binary.LittleEndian.PutUint32(code, 0xd4000001)
	if err := p.Memory.Map(0x200000000, 4, usermem.Execute, code); err != nil {
		t.Fatal(err)
	}
	p.Registers.PC = 0x200000000
	p.Registers.GPR[16] = 372
	before := p.Registers
	result, err := p.Run(context.Background(), RunOptions{})
	if err != nil || result.Termination != "unsupported_instruction" || result.Instructions != 0 || p.Registers != before {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestMachProtectRoundsAndChecksMaximum(t *testing.T) {
	p := process(t, Options{})
	r := &p.Registers
	for _, tc := range []struct{ addr, size, prot, max, task, want uint64 }{
		{0x100004001, 1, 1, 0, 0x103, 0},      // page rounding protects all of data page
		{0x100004000, 0x4000, 7, 0, 0x103, 2}, // beyond Mach-O maximum
		{0x300000000, 1, 1, 0, 0x103, 1},
		{0x100004000, 1, 1, 0, 0x107, 4},
		{^uint64(0), 2, 1, 0, 0x103, 4},
	} {
		r.GPR[16] = 0xfffffff2
		r.GPR[0] = tc.task
		r.GPR[1] = tc.addr
		r.GPR[2] = tc.size
		r.GPR[3] = tc.max
		r.GPR[4] = tc.prot
		r.C = true
		if !p.kernel.trap(r, p.Memory) || r.GPR[0] != tc.want || !r.C {
			t.Fatalf("%+v: %v", tc, r.GPR[:5])
		}
	}
	if err := p.Memory.Write(0x100004000, []byte{0}); err == nil {
		t.Fatal("protect did not change page permissions")
	}
}
func TestCrossarchNamespaceErrors(t *testing.T) {
	for _, tc := range []struct{ namespace, want uint64 }{{0, 45}, {1, 22}, {0x100000000, 45}} {
		var k kernelState
		r := emu.Registers{}
		r.GPR[0] = tc.namespace
		r.GPR[16] = 38
		if !k.trap(&r, nil) || r.GPR[0] != tc.want || !r.C {
			t.Fatal(r)
		}
	}
}

func TestCSRQueriesAndErrors(t *testing.T) {
	config := uint32(0)
	k := kernelState{csrConfig: &config}
	m := &usermem.AddressSpace{}
	if err := m.Map(0x1000, 4, usermem.Read|usermem.Write, []byte{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ op, addr, size, want uint64 }{{0, 0x1000, 4, 1}, {1, 0x1000, 4, 0}, {0, 0x1000, 4, 0}, {0, 0, 4, 22}, {0, 0x1000, 3, 22}, {0, 0x2000, 4, 14}, {1, 0x2000, 4, 14}, {2, 0, 0, 78}} {
		r := emu.Registers{}
		r.GPR[16] = 483
		r.GPR[0] = tc.op
		r.GPR[1] = tc.addr
		r.GPR[2] = tc.size
		if !k.trap(&r, m) || r.GPR[0] != tc.want || r.C != (tc.want != 0) {
			t.Fatalf("%+v got=%v", tc, r)
		}
	}
	config = 1
	if err := m.Write(0x1000, []byte{8, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	r := emu.Registers{}
	r.GPR[16] = 483
	r.GPR[1] = 0x1000
	r.GPR[2] = 4
	if !k.trap(&r, m) || r.GPR[0] != 0 || r.C {
		t.Fatal("implied debugger allowance missing")
	}
	k.csrConfig = nil
	before := r
	if k.trap(&r, m) || r != before {
		t.Fatal("uncaptured policy guessed")
	}
}
func TestDarwinSuccessfulTrapRetiresOnce(t *testing.T) {
	p := process(t, Options{})
	code := make([]byte, 4)
	binary.LittleEndian.PutUint32(code, 0xd4001001)
	if err := p.Memory.Map(0x200000000, 4, usermem.Execute, code); err != nil {
		t.Fatal(err)
	}
	p.Registers.PC = 0x200000000
	p.Registers.GPR[16] = 372
	result, err := p.Run(context.Background(), RunOptions{MaxInstructions: 1})
	if err != nil || result.Instructions != 1 || result.Termination != "instruction_limit" || p.Registers.PC != 0x200000004 || p.Registers.GPR[0] != 1 {
		t.Fatalf("%+v %v", result, err)
	}
}
