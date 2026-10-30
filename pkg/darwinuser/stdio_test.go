package darwinuser

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

func stdioFixture() []InitialDescriptor {
	return []InitialDescriptor{{FD: 0, Flags: 0, Stat: FileStat{Mode: 0x2000, Device: 1, Inode: 1}}, {FD: 1, Flags: 2, Stat: FileStat{Mode: 0x8180, Device: 2, Inode: 2}}, {FD: 2, Flags: 2, Stat: FileStat{Mode: 0x8180, Device: 2, Inode: 3}}}
}
func TestInitialDescriptorsValidateAndArePrivate(t *testing.T) {
	original := stdioFixture()
	if err := validateInitialDescriptors(original); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func([]InitialDescriptor) []InitialDescriptor{
		func(v []InitialDescriptor) []InitialDescriptor { return v[:2] }, func(v []InitialDescriptor) []InitialDescriptor { v[1].FD = 0; return v }, func(v []InitialDescriptor) []InitialDescriptor { v[2].FD = 3; return v },
		func(v []InitialDescriptor) []InitialDescriptor { v[0].Flags = 2; return v }, func(v []InitialDescriptor) []InitialDescriptor { v[0].Stat.Mode = 0x8000; return v }, func(v []InitialDescriptor) []InitialDescriptor { v[1].Stat.Mode = 0x4000; return v },
		func(v []InitialDescriptor) []InitialDescriptor { v[1].Stat.Size = 1; return v }, func(v []InitialDescriptor) []InitialDescriptor { v[2].Stat.Blocks = 1; return v }, func(v []InitialDescriptor) []InitialDescriptor { v[2].Stat = v[1].Stat; return v },
	} {
		bad := mutate(append([]InitialDescriptor(nil), original...))
		if validateInitialDescriptors(bad) == nil {
			t.Fatal("invalid standard descriptors accepted", bad)
		}
	}
	profile := startupFixture(t)
	profile.StandardIO = original
	p := process(t, Options{StartupProfile: profile})
	q := process(t, Options{StartupProfile: profile})
	profile.StandardIO[1].Stat.Inode = 999
	p.kernel.descriptors[1].file.offset = 17
	if p.kernel.descriptors[1].file.stat.Inode != 2 || q.kernel.descriptors[1].file.offset != 0 {
		t.Fatal("shared caller/process file state")
	}
	// A dup shares one open-file description; closing the original retains it.
	r := emu.Registers{}
	r.GPR[0] = 1
	if !p.kernel.duplicateDescriptor(&r) || r.GPR[0] != 3 {
		t.Fatal(r)
	}
	r.GPR[0] = 1
	p.kernel.closeDescriptor(&r)
	if p.kernel.descriptors[3].file.offset != 17 {
		t.Fatal("dup description lost")
	}
	r.GPR[0] = 3
	p.kernel.duplicateDescriptor(&r)
	if r.GPR[0] != 1 {
		t.Fatal("lowest closed standard FD not reused")
	}
}

func TestFstatOwnedRegularOutputNativeContract(t *testing.T) {
	var ob struct {
		Fields FileStat `json:"decoded_fields"`
		Cases  []struct {
			Case   string
			X0, X1 uint64
			Carry  bool
			Bytes  string
		}
	}
	kernelNativeResult(t, "fstat-stderr-probe.c", &ob)
	if len(ob.Cases) != 10 {
		t.Fatal(len(ob.Cases))
	}
	for _, tc := range ob.Cases {
		t.Run(tc.Case, func(t *testing.T) {
			k := kernelState{standardIO: true, descriptors: map[uint32]guestDescriptor{2: {file: &guestOpenFile{stat: ob.Fields, flags: 2}}}}
			m := &usermem.AddressSpace{}
			r := emu.Registers{C: true}
			r.GPR[0] = 2
			r.GPR[1] = 0x10000
			r.GPR[16] = 339
			size, perm := 160, usermem.Read|usermem.Write
			switch tc.Case {
			case "low32_fd":
				r.GPR[0] |= 1 << 32
			case "invalid_fd":
				r.GPR[0] = 500
			case "null_output":
				r.GPR[1] = 0
			case "unmapped_output":
				r.GPR[1] = 1
			case "readonly_output":
				perm = usermem.Read
			case "partial_eight":
				size = 8
			case "partial_128":
				size = 128
			case "missing_last_byte":
				size = 143
			case "exact_144":
				size = 144
			}
			initial := bytes.Repeat([]byte{0xa5}, size)
			m.Map(0x10000, uint64(size), perm, initial)
			before := r
			handled := k.trap(&r, m)
			got := make([]byte, size)
			m.Read(0x10000, got)
			if strings.HasPrefix(tc.Case, "partial_") || tc.Case == "missing_last_byte" {
				if handled || r != before || !bytes.Equal(got, initial) {
					t.Fatal("partial native copyout did not fail unsupported before mutation")
				}
				return
			}
			if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:2], r.C)
			}
			want := initial
			if tc.Bytes != "" {
				var err error
				want, err = hex.DecodeString(tc.Bytes)
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(got, want) {
				t.Fatal("native stat/tail differs", tc.Case)
			}
		})
	}
}

func TestWriteMatchesNativePrivateFileContract(t *testing.T) {
	var cases []struct {
		Case       string
		X0, X1, X2 uint64
		Carry      bool
		Offset     uint64
		DupOffset  uint64 `json:"dup_offset"`
		Contents   string
	}
	kernelNativeResult(t, "write-probe.c", &cases)
	if len(cases) != 15 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			file := &guestOpenFile{flags: 2}
			k := kernelState{standardIO: true, outputFiles: [2]*guestOpenFile{file, nil}, descriptors: map[uint32]guestDescriptor{1: {file: file}, 3: {file: file}, 0: {file: &guestOpenFile{flags: 0}}}}
			m := &usermem.AddressSpace{}
			input := []byte("hello!\n")
			r := emu.Registers{C: true}
			r.GPR[0] = 1
			r.GPR[1] = 0x10000
			r.GPR[2] = tc.X2
			r.GPR[16] = 4
			switch tc.Case {
			case "low32_fd":
				r.GPR[0] |= 1 << 32
			case "invalid_fd", "invalid_fd_zero":
				r.GPR[0] = 500
			case "readonly_fd", "readonly_fd_zero":
				r.GPR[0] = 0
			case "zero_null", "null_nonzero":
				r.GPR[1] = 0
			case "zero_unmapped", "unmapped_nonzero":
				r.GPR[1] = 1
			case "readonly_input", "exact_boundary":
				input = bytes.Repeat([]byte{'Z'}, 7)
			case "partial_one":
				input = input[:1]
			case "partial_six":
				input = input[:6]
			case "dup_shared_offset":
				file.data = []byte("prefix:")
				file.offset = 7
				r.GPR[0] = 3
			}
			if err := m.Map(0x10000, uint64(len(input)), usermem.Read, input); err != nil {
				t.Fatal(err)
			}
			before := r
			handled := k.trap(&r, m)
			if strings.HasPrefix(tc.Case, "partial_") {
				if handled || r != before || len(file.data) != 0 {
					t.Fatal("partial input mutated state")
				}
				return
			}
			want, err := hex.DecodeString(tc.Contents)
			if err != nil {
				t.Fatal(err)
			}
			if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.GPR[2] != tc.X2 || r.C != tc.Carry || file.offset != tc.Offset || k.descriptors[3].file.offset != tc.DupOffset || !bytes.Equal(file.data, want) {
				t.Fatal(tc, r.GPR[:3], file)
			}
		})
	}
}

func TestOutputBudgetAndUnsupportedPostWriteStat(t *testing.T) {
	file := &guestOpenFile{flags: 2, data: []byte("abc"), offset: 3}
	k := kernelState{standardIO: true, outputFiles: [2]*guestOpenFile{file, nil}, descriptors: map[uint32]guestDescriptor{1: {file: file}}}
	m := &usermem.AddressSpace{}
	m.Map(0x10000, 1, usermem.Read, []byte{'x'})
	for _, limit := range []int{-1, 2, 3} {
		k.outputLimit = limit
		r := emu.Registers{}
		r.GPR[0] = 1
		r.GPR[1] = 0x10000
		r.GPR[2] = 1
		before := r
		if k.write(&r, m) || r != before || string(file.data) != "abc" {
			t.Fatal("budget overflow mutated output", limit)
		}
	}
	r := emu.Registers{}
	r.GPR[0] = 1
	r.GPR[1] = 0x10000
	before := r
	if k.fstat(&r, m) || r != before {
		t.Fatal("invented postwrite metadata")
	}
}

func TestExecutionOutputSnapshotSurvivesClose(t *testing.T) {
	profile := startupFixture(t)
	profile.StandardIO = stdioFixture()
	p := process(t, Options{StartupProfile: profile})
	file := p.kernel.descriptors[1].file
	file.data = []byte("captured\n")
	file.offset = uint64(len(file.data))
	r := emu.Registers{}
	r.GPR[0] = 1
	p.kernel.closeDescriptor(&r)
	x := Execution{process: p}
	result := x.Result()
	if string(result.Stdout) != "captured\n" || len(result.Stderr) != 0 {
		t.Fatal(result)
	}
	result.Stdout[0] = 'X'
	if string(x.Result().Stdout) != "captured\n" {
		t.Fatal("result borrowed mutable output")
	}
}
