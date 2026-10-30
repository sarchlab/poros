package cpu

import (
	"encoding/binary"
	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"golang.org/x/arch/arm64/arm64asm"
	"testing"
)

type extensionExecution struct {
	regs   emu.Registers
	memory emu.TrackedMemory
	end    uint64
	done   bool
}

func (x *extensionExecution) CPURegisters() *emu.Registers    { return &x.regs }
func (x *extensionExecution) MemoryAccesses() []emu.MemAccess { return x.memory.Accesses }
func (x *extensionExecution) Stopped() bool                   { return x.done }
func (x *extensionExecution) Peek(pc uint64) (arm64asm.Inst, error) {
	var b [4]byte
	if err := x.memory.Fetch(pc, b[:]); err != nil {
		return arm64asm.Inst{}, err
	}
	return emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
}
func (x *extensionExecution) StepDetailed() (arm64asm.Inst, bool, error) {
	x.memory.Accesses = nil
	step, err := emu.StepChecked(&x.regs, &x.memory)
	x.done = err != nil || x.regs.PC == x.end
	return step.Instruction, step.Branched, err
}
func newExtensionExecution(t *testing.T, words []uint32) *extensionExecution {
	t.Helper()
	m := &usermem.AddressSpace{}
	code := make([]byte, len(words)*4)
	for i, w := range words {
		binary.LittleEndian.PutUint32(code[4*i:], w)
	}
	if err := m.Map(0x1000, uint64(len(code)), usermem.Execute, code); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 64)
	binary.LittleEndian.PutUint64(data, 7)
	if err := m.Map(0x2000, 64, usermem.Read|usermem.Write, data); err != nil {
		t.Fatal(err)
	}
	x := &extensionExecution{regs: emu.Registers{PC: 0x1000}, memory: emu.TrackedMemory{Space: m}, end: 0x1000 + uint64(len(code))}
	x.regs.GPR[3] = 0x2000
	x.regs.GPR[2] = 9
	return x
}
func TestCheckedAtomicRMWAcrossTimingModels(t *testing.T) {
	// load compare; CASA; load replacement; SWPL zero; load zero; DMB ISH.
	words := []uint32{0xb9400061, 0x88e17c62, 0xb9400064, 0xb87f8065, 0xb9400066, 0xd5033bbf}
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		t.Run(mode, func(t *testing.T) {
			x := newExtensionExecution(t, words)
			switch mode {
			case "functional":
				for !x.Stopped() {
					if _, _, err := x.StepDetailed(); err != nil {
						t.Fatal(err)
					}
				}
			default:
				s := sim.MakeBuilder().Build()
				defer s.Terminate()
				if mode == "inorder" {
					c := modeling.MakeBuilder[*inorder.Comp](inorder.DefaultSpec()).WithSimulation(s).WithResources(inorder.Resources{Execution: x}).Build("CPU")
					if err := s.Initialize(); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 500 && !c.State.Done; i++ {
						c.Middlewares.Pipeline.Handle(nil)
					}
					if !c.State.Done || c.State.Err != "" || c.State.InstDone != uint64(len(words)) {
						t.Fatalf("%+v", c.State)
					}
				} else {
					c := modeling.MakeBuilder[*ooo.Comp](ooo.DefaultSpec()).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
					if err := s.Initialize(); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 500 && !c.State.Done; i++ {
						c.Middlewares.Pipeline.Handle(nil)
					}
					if !c.State.Done || c.State.Err != "" || c.State.InstDone != uint64(len(words)) {
						t.Fatalf("%+v", c.State)
					}
					if c.State.GprReady[4] <= c.State.GprReady[1] || c.State.GprReady[6] <= c.State.GprReady[5] {
						t.Fatal("RMW/load ordering lost")
					}
				}
			}
			if x.regs.GPR[1] != 7 || x.regs.GPR[4] != 9 || x.regs.GPR[5] != 9 || x.regs.GPR[6] != 0 {
				t.Fatal(x.regs.GPR[:7])
			}
		})
	}
}
func TestOoOByteHalfwordDependenciesWaitForLoad(t *testing.T) {
	// LDR Q0,[X3]; UMINV B1,V0.16B; ORR V2.16B,V1.16B,V1.16B;
	// UADDLV H3,V2.8B. The B/H destinations must carry producer readiness.
	x := newExtensionExecution(t, []uint32{0x3dc00060, 0x6e31a801, 0x4ea11c22, 0x2e303843})
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := ooo.DefaultSpec()
	spec.LoadLatency = 40
	c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 500 && !c.State.Done; i++ {
		c.Middlewares.Pipeline.Handle(nil)
	}
	if c.State.Err != "" || c.State.InstDone != 4 || !c.State.Done {
		t.Fatal(c.State.Err, c.State.InstDone)
	}
	for i := 1; i <= 3; i++ {
		if c.State.FpReady[i] <= c.State.FpReady[i-1] {
			t.Fatal("lost subregister dependency", c.State.FpReady[:4])
		}
	}
}
