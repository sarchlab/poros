package darwinuser

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/sarchlab/poros/internal/machotest"
)

func process(t *testing.T, o Options) *Process {
	t.Helper()
	p, e := Parse(machotest.Image(2), machotest.Image(7), o)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func word(t *testing.T, p *Process, a uint64) uint64 {
	t.Helper()
	var b [8]byte
	if e := p.Memory.Read(a, b[:]); e != nil {
		t.Fatal(e)
	}
	return binary.LittleEndian.Uint64(b[:])
}
func textAt(t *testing.T, p *Process, a uint64) string {
	t.Helper()
	var out []byte
	for len(out) < 4096 {
		var b [1]byte
		if e := p.Memory.Read(a, b[:]); e != nil {
			t.Fatal(e)
		}
		if b[0] == 0 {
			return string(out)
		}
		out = append(out, b[0])
		a++
	}
	t.Fatal("unterminated string")
	return ""
}
func TestKernelToDyldStackAndPermissions(t *testing.T) {
	p := process(t, Options{Args: []string{"bench", "63"}, Env: []string{"A=B"}})
	sp := p.Registers.SP
	if p.Registers.PC != DyldBase+0x400 || p.MainEntry != 0x100000400 || sp%16 != 0 {
		t.Fatal(p.Registers)
	}
	if word(t, p, sp) != 0x100000000 || word(t, p, sp+8) != 2 || word(t, p, sp+32) != 0 || word(t, p, sp+48) != 0 || word(t, p, sp+72) != 0 {
		t.Fatal("stack vector layout")
	}
	if textAt(t, p, word(t, p, sp+16)) != "bench" || textAt(t, p, word(t, p, sp+24)) != "63" || textAt(t, p, word(t, p, sp+40)) != "A=B" || textAt(t, p, word(t, p, sp+56)) != "executable_path=bench" || textAt(t, p, word(t, p, sp+64)) != "ptrauth_disabled=1" {
		t.Fatal("startup strings")
	}
	if word(t, p, 0x100004000) != 0 {
		t.Fatal("BSS not zero")
	}
	if e := p.Memory.Write(0x100000400, []byte{0}); e == nil {
		t.Fatal("text writable")
	}
	if e := p.Memory.Fetch(sp, make([]byte, 4)); e == nil {
		t.Fatal("stack executable")
	}
	if e := p.Memory.Read(0, make([]byte, 1)); e == nil {
		t.Fatal("null mapped")
	}
}
func TestExecutionMissingISAAndBudgets(t *testing.T) {
	p := process(t, Options{})
	r, e := p.Run(context.Background(), RunOptions{})
	if e != nil || r.Termination != "unsupported_instruction" || r.Instructions != 6 || r.PC != DyldBase+0x418 || !strings.Contains(r.Diagnostic, "BRK") {
		t.Fatalf("%+v %v", r, e)
	}
	if _, e = p.Run(context.Background(), RunOptions{}); e == nil {
		t.Fatal("rerun allowed")
	}
	p = process(t, Options{})
	r, e = p.Run(context.Background(), RunOptions{MaxInstructions: 2})
	if e != nil || r.Termination != "instruction_limit" || r.Instructions != 2 {
		t.Fatalf("%+v %v", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p = process(t, Options{})
	r, e = p.Run(ctx, RunOptions{})
	if e != nil || r.Termination != "time_limit" || r.Instructions != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	p = process(t, Options{})
	if _, e = p.Run(context.Background(), RunOptions{Timeout: -time.Second}); e == nil {
		t.Fatal("negative timeout")
	}
}
func TestStartupRejectsInvalidInputs(t *testing.T) {
	for _, o := range []Options{{Args: []string{"bad\x00arg"}}, {Env: []string{strings.Repeat("x", MaxArgumentBytes)}}, {StackSize: 1}, {StackSize: MaxStackSize + 16384}, {Args: make([]string, MaxArgumentCount+1)}} {
		if _, e := Parse(machotest.Image(2), machotest.Image(7), o); e == nil {
			t.Fatal("invalid startup accepted")
		}
	}
	// An otherwise valid executable image at the fixed dyld mapping collides.
	b := machotest.Image(2)
	binary.LittleEndian.PutUint64(b[56:], DyldBase)
	binary.LittleEndian.PutUint64(b[128:], DyldBase+16384)
	if _, e := Parse(b, machotest.Image(7), Options{}); e == nil {
		t.Fatal("image collision accepted")
	}
}

func TestARM64eMainRequiresEnabledAuthentication(t *testing.T) {
	b := machotest.Image(2)
	binary.LittleEndian.PutUint32(b[8:], 0x80000002)
	p, e := Parse(b, machotest.Image(7), Options{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := p.Run(context.Background(), RunOptions{})
	if e != nil || r.Instructions != 5 || r.Termination != "unsupported_instruction" || !strings.Contains(r.Diagnostic, "pointer authentication") {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestCommpageExplicitFieldsAndVirtualCounter(t *testing.T) {
	profile := RuntimeProfile{CPUCapabilities: 0x87ffeff39f087fc8, Version: 3, UserPageShift: 14, CacheLineSize: 128, CPUFamily: 0xda33d83d, UserTimebase: 3, ContinuousHWClock: 1, TimebaseNumer: 125, TimebaseDenom: 3}
	p := process(t, Options{RuntimeProfile: &profile})
	if word(t, p, 0xfffffc010) != profile.CPUCapabilities || word(t, p, 0xfffffc0a8) != 0 {
		t.Fatal("profile/virtual epoch")
	}
	var mode [1]byte
	if e := p.Memory.Read(0xfffffc090, mode[:]); e != nil || mode[0] != 3 {
		t.Fatal("native clock mode lost")
	}
	if e := p.Memory.Read(0xfffffc050, make([]byte, 8)); e == nil {
		t.Fatal("unmodeled commpage field guessed")
	}
	if e := p.Memory.Write(0xfffffc090, []byte{0}); e == nil {
		t.Fatal("commpage writable")
	}
	if _, e := p.Run(context.Background(), RunOptions{MaxInstructions: 2}); e != nil {
		t.Fatal(e)
	}
	if p.Registers.Counter.Value != 2 || p.Registers.Counter.Frequency != 24000000 {
		t.Fatal(p.Registers.Counter)
	}
	profile.TimebaseDenom = 0
	if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{RuntimeProfile: &profile}); e == nil {
		t.Fatal("invalid timebase accepted")
	}
}

func TestSelectedReadOnlyCommpageFields(t *testing.T) {
	flags := uint64(1)
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, KernelPageShift: 14, ReadOnlyPageShifts: &PageShifts{User: 14, Kernel: 14}, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, DyldFlags: &flags}
	p := process(t, Options{RuntimeProfile: &profile})
	for _, addr := range []uint64{0xfffffc037, 0xfffff4025, 0xfffff4037} {
		var got [1]byte
		if err := p.Memory.Read(addr, got[:]); err != nil || got[0] != 14 {
			t.Fatal(addr, err)
		}
		if err := p.Memory.Write(addr, []byte{0}); err == nil {
			t.Fatal("read-only field writable")
		}
	}
	if word(t, p, 0xfffffc160) != 1 {
		t.Fatal("dyld flags")
	}
	if err := p.Memory.Read(0xfffffc320, make([]byte, 8)); err == nil {
		t.Fatal("unmodeled commpage data present")
	}
	profile.ReadOnlyPageShifts.Kernel = 12
	if _, err := Parse(machotest.Image(2), machotest.Image(7), Options{RuntimeProfile: &profile}); err == nil {
		t.Fatal("unsupported page size accepted")
	}
}
