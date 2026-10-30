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
func TestCheckedAtomicAndOrderedMemoryAcrossTimingModels(t *testing.T) {
	// Load compare; CASA; load replacement; SWPL zero; load zero; DMB ISH;
	// STLR replacement; LDAR result; NOP.
	words := []uint32{0xb9400061, 0x88e17c62, 0xb9400064, 0xb87f8065, 0xb9400066, 0xd5033bbf, 0x889ffc62, 0x88dffc67, 0x885f7c61, 0x88057c62, 0x885f7c61, 0xd5033f5f, 0x88047c62, 0xb8270068, 0xb9400069, 0xd503201f}
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		t.Run(mode, func(t *testing.T) {
			x := newExtensionExecution(t, words)
			x.regs.Exclusive.Policy = emu.ExclusiveM2Local64
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
					if c.State.GprReady[4] <= c.State.GprReady[5] || c.State.GprReady[7] <= c.State.GprReady[6] {
						t.Fatal("RMW/load ordering lost")
					}
				}
			}
			if x.regs.GPR[1] != 9 || x.regs.GPR[4] != 1 || x.regs.GPR[5] != 0 || x.regs.GPR[6] != 0 || x.regs.GPR[7] != 9 || x.regs.GPR[8] != 9 || x.regs.GPR[9] != 18 {
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

func TestOoOPointerStripWaitsForLoad(t *testing.T) {
	// LDR X1,[X3]; XPACD X1; ADD X2,X1,#1. XPAC has a destructive
	// source operand; its result must wait for the original loaded pointer.
	x := newExtensionExecution(t, []uint32{0xf9400061, 0xdac147e1, 0x91000422, 0xd503201f})
	x.regs.PointerStrip = emu.PointerStripDarwin47Bit
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
	if c.State.Err != "" || !c.State.Done || c.State.InstDone != 4 || x.regs.GPR[2] != 8 {
		t.Fatal(c.State.Err, c.State.InstDone, x.regs.GPR[2])
	}
	if c.State.GprReady[1] < 40 {
		t.Fatal("lost XPAC read dependency", c.State.GprReady[:4])
	}
}

func TestOoOPartialRegisterWritesWaitForOldDestination(t *testing.T) {
	for _, tc := range []struct {
		word uint32
		want uint64
	}{{0xf2b579a1, 0xabcd0007}, {0xb3703c41, 0x90007}, {0xb3400c41, 9}} {
		x := newExtensionExecution(t, []uint32{0xf9400061, tc.word, 0xd503201f})
		s := sim.MakeBuilder().Build()
		spec := ooo.DefaultSpec()
		spec.LoadLatency = 40
		c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
		if err := s.Initialize(); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 500 && !c.State.Done; i++ {
			c.Middlewares.Pipeline.Handle(nil)
		}
		if c.State.Err != "" || !c.State.Done || x.regs.GPR[1] != tc.want || c.State.GprReady[1] < 40 {
			t.Fatal(tc, x.regs.GPR[1], c.State.GprReady[1], c.State.Err)
		}
		s.Terminate()
	}
}

func TestOoOFullBitfieldOverwriteDoesNotWaitForOldDestination(t *testing.T) {
	for _, word := range []uint32{0xb340fc41, 0x33007c41} {
		x := newExtensionExecution(t, []uint32{0xf9400061, word, 0x91000424})
		s := sim.MakeBuilder().Build()
		spec := ooo.DefaultSpec()
		spec.LoadLatency = 40
		c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
		if err := s.Initialize(); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 500 && !c.State.Done; i++ {
			c.Middlewares.Pipeline.Handle(nil)
		}
		if c.State.Err != "" || !c.State.Done || x.regs.GPR[4] != 10 || c.State.GprReady[4] >= 40 {
			t.Fatal(word, c.State.Err, x.regs.GPR[4], c.State.GprReady[4])
		}
		s.Terminate()
	}
}

func TestOoOVectorInsertWaitsForOldDestination(t *testing.T) {
	// LDR Q0,[X3]; MOV V0.D[1],V1.D[0]; ORR V2.16B,V0.16B,V0.16B.
	x := newExtensionExecution(t, []uint32{0x3dc00060, 0x6e180420, 0x4ea01c02})
	x.regs.WriteQ(1, 0x1234, 0xabcd)
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
	if c.State.Err != "" || !c.State.Done || x.regs.FP[2].Lo != 7 || x.regs.FP[2].Hi != 0x1234 || c.State.FpReady[2] < 40 {
		t.Fatal(c.State.Err, x.regs.FP[2], c.State.FpReady[2])
	}
}

func TestOoOUnsignedLaneMoveWaitsForVectorLoad(t *testing.T) {
	// LDR Q0,[X3]; UMOV W1,V0.B[0]; EOR X2,X1,XZR.
	x := newExtensionExecution(t, []uint32{0x3dc00060, 0x0e013c01, 0xca1f0022, 0xd503201f})
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := ooo.DefaultSpec()
	spec.LoadLatency = 40
	c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
	if e := s.Initialize(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 500 && !c.State.Done; i++ {
		c.Middlewares.Pipeline.Handle(nil)
	}
	if c.State.Err != "" || !c.State.Done || x.regs.GPR[2] != 7 || c.State.GprReady[1] <= c.State.FpReady[0] || c.State.GprReady[2] <= c.State.GprReady[1] {
		t.Fatal(c.State.Err, x.regs.GPR[2], c.State.FpReady[0], c.State.GprReady[:3])
	}
}

func TestOoOImmediateArithmeticTracksOrdinaryRegSPOperands(t *testing.T) {
	// A delayed load feeds ADD then SUB. An independent ADD is the control:
	// it must become ready before the load, not inherit the chain's delay.
	x := newExtensionExecution(t, []uint32{0xf9400061, 0x91000422, 0xd1000c44, 0x910004c5, 0xd503201f})
	x.regs.GPR[6] = 20
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
	if c.State.Err != "" || !c.State.Done || c.State.InstDone != 5 || x.regs.GPR[2] != 8 || x.regs.GPR[4] != 5 || x.regs.GPR[5] != 21 {
		t.Fatal(c.State.Err, c.State.InstDone, x.regs.GPR[:7])
	}
	if c.State.GprReady[2] <= c.State.GprReady[1] || c.State.GprReady[4] <= c.State.GprReady[2] || c.State.GprReady[5] >= c.State.GprReady[1] {
		t.Fatal("lost immediate arithmetic dependency or false control dependency", c.State.GprReady[:7])
	}
}

func TestOoOImmediateOverwriteReplacesStaleProducerReadiness(t *testing.T) {
	// The independent ADD overwrites X1 after the delayed load. With register
	// renaming its consumer follows ADD, not the overwritten load's timestamp.
	x := newExtensionExecution(t, []uint32{0xf9400061, 0x910004c1, 0xca1f0022, 0xd503201f})
	x.regs.GPR[6] = 20
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
	if c.State.Err != "" || !c.State.Done || c.State.InstDone != 4 || x.regs.GPR[2] != 21 {
		t.Fatal(c.State.Err, c.State.InstDone, x.regs.GPR[:7])
	}
	if c.State.GprReady[1] >= 40 || c.State.GprReady[2] >= 40 || c.State.GprReady[2] <= c.State.GprReady[1] {
		t.Fatal("stale producer retained after overwrite", c.State.GprReady[:3])
	}
}

func TestOoODuplicateWaitsForGPRLoad(t *testing.T) {
	// LDR X1,[X3]; DUP V0.4H,W1; UMOV W2,V0.H[1].
	x := newExtensionExecution(t, []uint32{0xf9400061, 0x0e020c20, 0x0e063c02, 0xd503201f})
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := ooo.DefaultSpec()
	spec.LoadLatency = 40
	c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
	if e := s.Initialize(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 500 && !c.State.Done; i++ {
		c.Middlewares.Pipeline.Handle(nil)
	}
	if c.State.Err != "" || !c.State.Done || x.regs.GPR[2] != 7 || c.State.FpReady[0] <= c.State.GprReady[1] || c.State.GprReady[2] <= c.State.FpReady[0] {
		t.Fatal(c.State.Err, x.regs.GPR[2], c.State.FpReady[0], c.State.GprReady[:3])
	}
}

func TestOoOVectorBICImmediateReadsOldDestination(t *testing.T) {
	// LDR Q0,[X3]; BIC V0.4H,#255,LSL#8; UMOV W1,V0.H[0].
	x := newExtensionExecution(t, []uint32{0x3dc00060, 0x2f07b7e0, 0x0e023c01, 0xd503201f})
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := ooo.DefaultSpec()
	spec.LoadLatency = 40
	c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
	if e := s.Initialize(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 500 && !c.State.Done; i++ {
		c.Middlewares.Pipeline.Handle(nil)
	}
	if c.State.Err != "" || !c.State.Done || x.regs.GPR[1] != 7 || c.State.FpReady[0] < 40 || c.State.GprReady[1] <= c.State.FpReady[0] {
		t.Fatal(c.State.Err, x.regs.GPR[1], c.State.FpReady[0], c.State.GprReady[1])
	}
}

func TestOoOVectorShiftAndUnzipWaitForCountLoad(t *testing.T) {
	// LDR Q1,[X3]; USHL V0.4H,V0.4H,V1.4H; UZP1 V0.8B,V0.8B,V0.8B; UMOV W2,V0.B[0].
	x := newExtensionExecution(t, []uint32{0x3dc00061, 0x2e614400, 0x0e001800, 0x0e013c02, 0xd503201f})
	x.regs.WriteQ(0, 7, 0)
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := ooo.DefaultSpec()
	spec.LoadLatency = 40
	c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
	if e := s.Initialize(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 500 && !c.State.Done; i++ {
		c.Middlewares.Pipeline.Handle(nil)
	}
	if c.State.Err != "" || !c.State.Done || x.regs.GPR[2] != 128 || c.State.FpReady[0] <= c.State.FpReady[1] || c.State.GprReady[2] <= c.State.FpReady[0] {
		t.Fatal(c.State.Err, x.regs.GPR[2], c.State.FpReady[:2], c.State.GprReady[2])
	}
}
