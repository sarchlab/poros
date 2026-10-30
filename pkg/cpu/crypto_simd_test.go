package cpu

import (
	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"testing"
)

func TestOoOCryptoSIMDWaitsForEveryRealSource(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		words                []uint32
		load, dest, consumer int
		value                uint64
	}{
		{"TBL_last_wrapped_table", []uint32{0x3dc00062, 0x4e0363e0, 0x4e083c04, 0xd503201f}, 2, 0, 4, 0x0707070707070707},
		{"EXT_second_source", []uint32{0x3dc00061, 0x6e016002, 0x4e083c44, 0xd503201f}, 1, 2, 4, 7 << 32},
		{"vector_ADD_source", []uint32{0x3dc00061, 0x4ee18422, 0x4e083c44, 0xd503201f}, 1, 2, 4, 14},
		{"scalar_DUP_source", []uint32{0x3dc00063, 0x5e010461, 0x4e083c24, 0xd503201f}, 3, 1, 4, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newExtensionExecution(t, tc.words)
			x.regs.WriteQ(3, 0x3030303030303030, 0x3030303030303030)
			s := sim.MakeBuilder().Build()
			defer s.Terminate()
			spec := ooo.DefaultSpec()
			spec.LoadLatency = 40
			c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
			if e := s.Initialize(); e != nil {
				t.Fatal(e)
			}
			for n := 0; n < 1000 && !c.State.Done; n++ {
				c.Middlewares.Pipeline.Handle(nil)
			}
			if !c.State.Done || c.State.Err != "" || c.State.FpInFlight != 0 || x.regs.GPR[tc.consumer] != tc.value || c.State.FpReady[tc.dest] <= c.State.FpReady[tc.load] || c.State.GprReady[tc.consumer] <= c.State.FpReady[tc.dest] {
				t.Fatal(c.State, x.regs.GPR)
			}
		})
	}
}
func TestOoOINSAndAESEWaitForOldVectorAndGPR(t *testing.T) {
	for _, source := range []uint32{0xb9400083, 0x528000e3} { // delayed GPR load or early immediate
		words := []uint32{0x3dc00067, source, 0x4e041c67, 0x4e284827, 0x4e083ce8, 0xd503201f}
		x := newExtensionExecution(t, words)
		x.regs.GPR[4] = 0x2000
		s := sim.MakeBuilder().Build()
		t.Cleanup(func() { _ = s.Terminate() })
		spec := ooo.DefaultSpec()
		spec.LoadLatency = 40
		c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
		if e := s.Initialize(); e != nil {
			t.Fatal(e)
		}
		for n := 0; n < 1000 && !c.State.Done; n++ {
			c.Middlewares.Pipeline.Handle(nil)
		}
		if !c.State.Done || c.State.Err != "" || c.State.FpInFlight != 0 || c.State.FpReady[7] < uint64(spec.LoadLatency+2) || c.State.FpReady[7] <= c.State.GprReady[3] || c.State.GprReady[8] <= c.State.FpReady[7] || x.regs.GPR[8] != 0x63636363636363c5 {
			t.Fatal(source, c.State, x.regs.GPR)
		}
	}
}
