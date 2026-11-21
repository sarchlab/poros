package ooo

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

func TestL1ReadReplayReservesActualPipes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		second uint64
		period int
		replay bool
	}{
		{"same address", 0x4000, 256, false},
		{"same line", 0x4008, 256, false},
		{"different bank", 0x4040, 256, false},
		{"different line and same bank", 0x4100, 256, true},
		{"disabled", 0x4100, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mem.DefaultCacheConfig()
			cfg.L1Latency = 3
			cfg.TLBPageBytes = 0
			cache := mem.NewCache(cfg)
			for _, addr := range []uint64{0x4000, tc.second, 0x4080} {
				cache.Prefetch(addr)
			}
			// Three independent LDR X registers, then a consumer of the second.
			p := specPipeline(t, []uint32{0xf9400023, 0xf9400044, 0xf94000c5, 0x91000487}, cache, func(s *Spec) {
				s.NumLDST = 3
				s.L1ReadBankPeriod = tc.period
			})
			reg := p.comp.Resources.Emu.Regs
			reg.WriteX(1, 0x4000)
			reg.WriteX(2, tc.second)
			reg.WriteX(6, 0x4080)
			p.fetchAndProcess()
			s := &p.comp.State
			want := uint64(4)
			if tc.replay {
				want = 7
			}
			if s.RobCount != 4 || s.Rob[0] != 4 || s.Rob[1] != want || s.Rob[2] != 4 || s.Rob[3] != want+1 {
				t.Fatalf("ROB %v, expected second load at %d", s.Rob[:4], want)
			}
			bookings := 0
			for _, pipe := range s.LdstUnits {
				bookings += len(pipe)
			}
			if tc.replay && bookings != 4 || !tc.replay && bookings != 3 {
				t.Fatalf("load-pipe bookings %v", s.LdstUnits)
			}
			if tc.period == 0 && s.L1ReadIssues != nil {
				t.Fatal("disabled profile allocated bank state")
			}
			if tc.replay {
				b, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				var restored State
				if err = json.Unmarshal(b, &restored); err != nil || !reflect.DeepEqual(s.L1ReadIssues, restored.L1ReadIssues) {
					t.Fatalf("restored reservations %v, error %v", restored.L1ReadIssues, err)
				}
				p.Handle(nil)
				if _, old := s.L1ReadIssues[1]; old || len(s.L1ReadIssues[4]) != 1 {
					t.Fatalf("pruned reservations %v", s.L1ReadIssues)
				}
			}
		})
	}
}

func TestL1ReadReplayOnlyOnReadyScalarAccess(t *testing.T) {
	cfg := mem.DefaultCacheConfig()
	cache := mem.NewCache(cfg)
	cache.Prefetch(0x4000)
	cache.Access(0x4100, 8, false, 10)
	p := specPipeline(t, []uint32{nop}, cache, func(s *Spec) { s.L1ReadBankPeriod = 256 })
	ordinary := arm64asm.Inst{Op: arm64asm.LDR}
	indexed := arm64asm.Inst{Op: arm64asm.LDR, Args: arm64asm.Args{arm64asm.X4, arm64asm.MemImmediate{Base: arm64asm.RegSP(arm64asm.X1), Mode: arm64asm.AddrPostIndex}}}
	for _, tc := range []struct {
		inst arm64asm.Inst
		acc  emu.MemAccess
	}{
		{ordinary, emu.MemAccess{Addr: 0x4100, Size: 8}}, // tag present, first fill still pending
		{ordinary, emu.MemAccess{Addr: 0x8000, Size: 8}}, // miss
		{ordinary, emu.MemAccess{Addr: 0x403c, Size: 8}}, // cross-line
		{ordinary, emu.MemAccess{Addr: 0x4004, Size: 4}},
		{arm64asm.Inst{Op: arm64asm.LDP}, emu.MemAccess{Addr: 0x4000, Size: 8}},
		{indexed, emu.MemAccess{Addr: 0x4000, Size: 8}},
	} {
		if issue := p.replayL1Read(tc.inst, []emu.MemAccess{tc.acc}, 10); issue != 10 || p.comp.State.L1ReadIssues != nil {
			t.Fatalf("reserved an excluded access %+v at %d", tc, issue)
		}
	}
	p.comp.State.L1ReadIssues = map[uint64][]uint64{math.MaxUint64 - 1: {0x4100}}
	defer func() {
		if recover() == nil {
			t.Error("expected replay cycle overflow")
		}
	}()
	p.replayL1Read(ordinary, []emu.MemAccess{{Addr: 0x4000, Size: 8}}, math.MaxUint64-1)
}

func TestForwardedLoadBypassesL1ReadPort(t *testing.T) {
	cfg := mem.DefaultCacheConfig()
	cache := mem.NewCache(cfg)
	cache.Prefetch(0x4000)
	p := specPipeline(t, []uint32{0xf9400023}, cache, func(s *Spec) { s.L1ReadBankPeriod = 256 })
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	s := &p.comp.State
	s.StoreBuffer = []storeBufferEntry{{Addr: 0x4000, Size: 8, CompleteCycle: 100, DrainCycle: 101, ReadyCycle: 1, FastForwardEligible: true}}
	p.fetchAndProcess()
	if s.RobCount != 1 || s.L1ReadIssues != nil || cache.Stats().L1Hits != 0 {
		t.Fatalf("forwarding reserved an L1 read: count %d, bank state %v, cache %+v", s.RobCount, s.L1ReadIssues, cache.Stats())
	}
}
