package linuxuser

import (
	"debug/elf"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../loader/testdata/elf/startup.elf")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func process(t *testing.T, o Options) *Process {
	t.Helper()
	p, e := Parse(fixture(t), o)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func word(t *testing.T, m *AddressSpace, addr uint64) uint64 {
	t.Helper()
	var b [8]byte
	if e := m.Read(addr, b[:]); e != nil {
		t.Fatal(e)
	}
	return binary.LittleEndian.Uint64(b[:])
}
func str(t *testing.T, m *AddressSpace, addr uint64) string {
	t.Helper()
	var out []byte
	for i := 0; i < MaxArgumentBytes; i++ {
		var b [1]byte
		if e := m.Read(addr+uint64(i), b[:]); e != nil {
			t.Fatal(e)
		}
		if b[0] == 0 {
			return string(out)
		}
		out = append(out, b[0])
	}
	t.Fatal("unterminated string")
	return ""
}
func TestLinuxStartup(t *testing.T) {
	for _, args := range [][]string{nil, {"program"}, {"program", "", "two words", "λ"}} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			o := Options{Args: args, Env: []string{"A=b", "EMPTY="}, ExecName: "/guest/startup", Random: [16]byte{1, 2, 3, 4}}
			p := process(t, o)
			m := p.Memory
			sp := p.Registers.SP
			if sp%16 != 0 || p.Registers.PC != 0x400120 || p.Registers.GPR != [31]uint64{} {
				t.Fatalf("registers %+v", p.Registers)
			}
			wantArgs := args
			if len(wantArgs) == 0 {
				wantArgs = []string{""}
			}
			if word(t, m, sp) != uint64(len(wantArgs)) {
				t.Fatal("argc")
			}
			sp += 8
			for _, s := range wantArgs {
				if str(t, m, word(t, m, sp)) != s {
					t.Fatal("argv")
				}
				sp += 8
			}
			if word(t, m, sp) != 0 {
				t.Fatal("argv terminator")
			}
			sp += 8
			for _, s := range o.Env {
				if str(t, m, word(t, m, sp)) != s {
					t.Fatal("envp")
				}
				sp += 8
			}
			if word(t, m, sp) != 0 {
				t.Fatal("env terminator")
			}
			sp += 8
			aux := map[uint64]uint64{}
			for i := 0; i < 32; i++ {
				tag, value := word(t, m, sp), word(t, m, sp+8)
				sp += 16
				if tag == 0 {
					if value != 0 {
						t.Fatal("AT_NULL")
					}
					break
				}
				if _, ok := aux[tag]; ok {
					t.Fatal("duplicate aux tag")
				}
				aux[tag] = value
			}
			for tag, want := range map[uint64]uint64{3: 0x400040, 4: 56, 5: 4, 6: 4096, 7: 0, 8: 0, 9: 0x400120, 11: 1000, 12: 1000, 13: 1000, 14: 1000, 16: 0, 17: 100, 23: 0, 26: 0} {
				if got, ok := aux[tag]; !ok || got != want {
					t.Fatalf("aux%d got%d want%d", tag, got, want)
				}
			}
			if str(t, m, aux[15]) != "aarch64" || str(t, m, aux[31]) != o.ExecName {
				t.Fatal("aux strings")
			}
			var random [16]byte
			if e := m.Read(aux[25], random[:]); e != nil || random != o.Random {
				t.Fatal("AT_RANDOM", e)
			}
			var ph [56]byte
			if e := m.Read(aux[3], ph[:]); e != nil || binary.LittleEndian.Uint32(ph[:]) != uint32(elf.PT_LOAD) {
				t.Fatal("AT_PHDR unmapped")
			}
			t.Logf("%s argc=%d envc=%d aux_entries=%d", p, len(wantArgs), len(o.Env), len(aux))
		})
	}
	p := process(t, Options{})
	if word(t, p.Memory, p.Registers.SP+16) != 0 || word(t, p.Memory, p.Registers.SP+24) != 0 {
		t.Fatal("empty argv/env termination")
	}
}
func TestStartupRejectsInvalidOptions(t *testing.T) {
	tests := []Options{{Args: []string{"bad\x00arg"}}, {Env: []string{"bad\x00env"}}, {ExecName: "bad\x00name"}, {Args: make([]string, MaxArgumentCount+1)}, {Env: make([]string, MaxArgumentCount+1)}, {Args: []string{strings.Repeat("x", MaxArgumentBytes)}}, {StackSize: 1}, {StackSize: MaxStackSize + 4096}, {StackSize: 4096, Args: []string{strings.Repeat("x", 5000)}}}
	for i, o := range tests {
		p, e := Parse(fixture(t), o)
		var typed *StartupError
		if p != nil || !errors.As(e, &typed) {
			t.Fatalf("case%d p=%v e=%v", i, p, e)
		}
	}
	b := fixture(t)
	binary.LittleEndian.PutUint64(b[176+16:], StackTop-4096)
	binary.LittleEndian.PutUint64(b[176+48:], 4096)
	if p, e := Parse(b, Options{}); p != nil || e == nil {
		t.Fatal("stack overlap accepted")
	}
	if p, e := Parse(nil, Options{}); p != nil || e == nil {
		t.Fatal("malformed accepted")
	}
}
func TestMemoryPermissionsAndAtomicFaults(t *testing.T) {
	p := process(t, Options{})
	m := p.Memory
	for _, tc := range []struct {
		addr uint64
		perm elf.ProgFlag
	}{{0x400120, elf.PF_R | elf.PF_X}, {0x401000, elf.PF_R}, {0x402000, elf.PF_R | elf.PF_W}, {p.Registers.SP, elf.PF_R | elf.PF_W}} {
		for _, op := range []struct {
			flag elf.ProgFlag
			f    func(uint64, []byte) error
		}{{elf.PF_R, m.Read}, {elf.PF_W, m.Write}, {elf.PF_X, m.Fetch}} {
			e := op.f(tc.addr, []byte{0})
			if (e == nil) != (tc.perm&op.flag != 0) {
				t.Fatalf("addr%x perm%d error%v", tc.addr, op.flag, e)
			}
		}
	}
	for _, addr := range []uint64{0, 0x800000, StackTop, ^uint64(0)} {
		if e := m.Read(addr, make([]byte, 8)); e == nil {
			t.Fatalf("read invalid%x", addr)
		}
	}
	before := []byte{1, 2, 3, 4}
	if e := m.Write(0x402ffc, before); e != nil {
		t.Fatal(e)
	}
	if e := m.Write(0x402ffc, make([]byte, 8)); e == nil {
		t.Fatal("unmapped cross-boundary write")
	}
	got := make([]byte, 4)
	if e := m.Read(0x402ffc, got); e != nil || string(got) != string(before) {
		t.Fatal("partial failed write")
	}
	dst := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	if e := m.Fetch(0x400ffc, dst); e == nil || string(dst) != string([]byte{9, 9, 9, 9, 9, 9, 9, 9}) {
		t.Fatal("partial failed fetch")
	}
	if e := m.Read(0x400ffc, make([]byte, 8)); e != nil {
		t.Fatal("valid read across adjacent mappings", e)
	}
	maps := m.Mappings()
	maps[0].Flags = elf.PF_W
	if e := m.Write(0x400120, []byte{0}); e == nil {
		t.Fatal("metadata bypasses permission")
	}
	if e := m.Read(^uint64(0), nil); e != nil {
		t.Fatal("zero byte access", e)
	}
}
func TestProcessOwnershipAndBSS(t *testing.T) {
	a := process(t, Options{})
	b := process(t, Options{})
	for addr := uint64(0x402008); addr < 0x402048; addr += 8 {
		if word(t, a.Memory, addr) != 0 {
			t.Fatal("BSS")
		}
	}
	if e := a.Memory.Write(0x402008, []byte{1}); e != nil {
		t.Fatal(e)
	}
	if word(t, b.Memory, 0x402008) != 0 {
		t.Fatal("process memory aliased")
	}
	p, e := Load("../loader/testdata/elf/startup.elf", Options{})
	if e != nil || p.Registers.PC != a.Registers.PC {
		t.Fatal(e)
	}
}

func TestStartupLimitsBoundary(t *testing.T) {
	process(t, Options{Args: make([]string, MaxArgumentCount), Env: make([]string, MaxArgumentCount)})
	process(t, Options{Args: []string{strings.Repeat("x", MaxArgumentBytes-9)}})
	if p, e := Parse(fixture(t), Options{Args: []string{strings.Repeat("x", MaxArgumentBytes-8)}}); p != nil || e == nil {
		t.Fatal("payload one byte beyond limit accepted")
	}
	process(t, Options{StackSize: 4096})
	process(t, Options{StackSize: MaxStackSize})
}
