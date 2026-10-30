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
	if word(t, p, sp) != 0x100000000 || word(t, p, sp+8) != 2 || word(t, p, sp+32) != 0 || word(t, p, sp+48) != 0 || word(t, p, sp+64) != 0 {
		t.Fatal("stack vector layout")
	}
	if textAt(t, p, word(t, p, sp+16)) != "bench" || textAt(t, p, word(t, p, sp+24)) != "63" || textAt(t, p, word(t, p, sp+40)) != "A=B" || textAt(t, p, word(t, p, sp+56)) != "executable_path=bench" {
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
	if e != nil || r.Termination != "unsupported_instruction" || r.Instructions != 5 || r.PC != DyldBase+0x414 || !strings.Contains(r.Diagnostic, "HINT") {
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
