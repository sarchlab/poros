package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func TestReadFileNativeContract(t *testing.T) {
	var cases []struct {
		Case              string
		X0, X1, X2        uint64
		Carry             bool
		Offset            uint64
		DupOffset         uint64 `json:"dup_offset"`
		IndependentOffset uint64 `json:"independent_offset"`
		Bytes             string
	}
	kernelNativeResult(t, "read-file-large-probe.c", &cases)
	if len(cases) != 24 {
		t.Fatal(len(cases))
	}
	payload := []byte("0123456789abcdefghijklmnopqrstuv")
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			file := &guestOpenFile{reader: bytes.NewReader(payload), stat: FileStat{Size: int64(len(payload))}}
			independent := &guestOpenFile{reader: file.reader, stat: file.stat, offset: 20}
			k := kernelState{descriptors: map[uint32]guestDescriptor{3: {file: file}, 4: {file: file}, 5: {file: independent}}}
			r := emu.Registers{C: true}
			r.GPR[0] = 3
			r.GPR[1] = 0x10000
			r.GPR[2] = tc.X2
			r.GPR[16] = 3
			size, perm := 32, usermem.Read|usermem.Write
			switch tc.Case {
			case "low32_fd":
				r.GPR[0] |= 1 << 32
			case "invalid_fd", "invalid_fd_zero", "oversize_invalid_fd":
				r.GPR[0] = 500
			case "zero_null", "null_nonzero":
				r.GPR[1] = 0
			case "zero_unmapped", "unmapped_nonzero":
				r.GPR[1] = 1
			case "readonly_output":
				perm = usermem.Read
			case "exact_boundary":
				size = 16
			case "partial_eight":
				size = 8
			case "partial_fifteen":
				size = 15
			case "dup_shared_offset":
				file.offset = 8
				r.GPR[0] = 4
			case "short_at_eof":
				file.offset = 24
			case "eof":
				file.offset = 32
			case "eof_null_output", "oversize_eof_null", "int32_max_eof_null":
				file.offset = 32
				r.GPR[1] = 0
			case "short_eof_exact_boundary":
				file.offset = 24
				size = 8
			}
			m := &usermem.AddressSpace{}
			initial := bytes.Repeat([]byte{0xa5}, size)
			m.Map(0x10000, uint64(size), perm, initial)
			before := r
			handled := k.trap(&r, m)
			got := make([]byte, size)
			m.Read(0x10000, got)
			if strings.HasPrefix(tc.Case, "partial_") {
				if handled || r != before || file.offset != 0 || !bytes.Equal(initial, got) {
					t.Fatal("partial output mutated state")
				}
				return
			}
			if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.GPR[2] != tc.X2 || r.C != tc.Carry || file.offset != tc.Offset || k.descriptors[4].file.offset != tc.DupOffset || independent.offset != tc.IndependentOffset {
				t.Fatal(tc, r.GPR[:3], file.offset)
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
				t.Fatal("native output differs", tc.Case)
			}
		})
	}
}
func TestOpenExecutableNativeContract(t *testing.T) {
	var cases []struct {
		Case       string
		X0, X1     uint64
		Carry      bool
		FirstBytes string `json:"first_bytes"`
	}
	kernelNativeResult(t, "open-executable-probe.c", &cases)
	if len(cases) != 7 {
		t.Fatal(len(cases))
	}
	image, err := loader.ParseMachO(machotest.Image(2), 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			k := kernelState{executableImage: image, executableStat: &FileStat{Size: int64(image.FileSize), Mode: 0x81ed}, executablePaths: []string{"/guest/program"}}
			name := "/guest/program"
			r := emu.Registers{C: true}
			r.GPR[0] = 0x1000
			r.GPR[16] = 5
			switch tc.Case {
			case "flags_low32":
				r.GPR[1] = 1 << 32
			case "cloexec":
				r.GPR[1] = 0x1000000
			case "nofollow_regular":
				r.GPR[1] = 0x100
			case "empty_path":
				name = ""
			case "null_path":
				r.GPR[0] = 0
			case "unmapped_path":
				r.GPR[0] = 1
			}
			m := &usermem.AddressSpace{}
			m.Map(0x1000, uint64(len(name)+1), usermem.Read, append([]byte(name), 0))
			before := r
			handled := k.trap(&r, m)
			if tc.Case == "cloexec" || tc.Case == "nofollow_regular" {
				if handled || r != before || len(k.descriptors) != 0 {
					t.Fatal("uncaptured open flags mutated namespace")
				}
				return
			}
			if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:2])
			}
			if !tc.Carry {
				var got [4]byte
				file := k.descriptors[uint32(r.GPR[0])].file
				n, e := file.reader.ReadAt(got[:], 0)
				if e != nil || n != 4 || hex.EncodeToString(got[:]) != tc.FirstBytes {
					t.Fatal("not original Mach-O bytes", got, e)
				}
			}
		})
	}
}

func TestExecutableBackingSurvivesCallerAndGuestMutation(t *testing.T) {
	input := machotest.Image(2)
	// Owned writable text fixture lets the test mutate a file-backed guest page.
	binary.LittleEndian.PutUint32(input[32+56:], 7)
	binary.LittleEndian.PutUint32(input[32+60:], 7)
	image, err := loader.ParseMachO(input, 2)
	if err != nil {
		t.Fatal(err)
	}
	profile := startupFixture(t)
	profile.BinarySHA256 = image.SHA256
	profile.ExecutableStat = &FileStat{Mode: 0x81ed, Inode: profile.ExecutableFile.ObjectID, Size: int64(len(input))}
	original := append([]byte(nil), input...)
	p, err := Parse(input, machotest.Image(7), Options{StartupProfile: profile, ExecName: "/guest/program"})
	if err != nil {
		t.Fatal(err)
	}
	clear(input)
	if err = p.Memory.Write(0x100000400, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(original))
	n, err := p.kernel.executableImage.ReadAt(got, 0)
	if err != nil || n != len(got) || !bytes.Equal(got, original) {
		t.Fatal("guest or caller changed file bytes", n, err)
	}
	p.Memory.Map(0x1000, 15, usermem.Read, []byte("/guest/program\x00"))
	r := emu.Registers{}
	r.GPR[0] = 0x1000
	if !p.kernel.open(&r, p.Memory) || r.C {
		t.Fatal(r)
	}
	first := uint32(r.GPR[0])
	r.GPR[0] = 0x1000
	r.GPR[1] = 0
	p.kernel.open(&r, p.Memory)
	second := uint32(r.GPR[0])
	if first == second || p.kernel.descriptors[first].file == p.kernel.descriptors[second].file {
		t.Fatal("separate opens share offsets")
	}
	p.kernel.descriptors[first].file.offset = 17
	if p.kernel.descriptors[second].file.offset != 0 {
		t.Fatal("independent offset changed")
	}
	r.GPR[0] = uint64(first)
	p.kernel.duplicateDescriptor(&r)
	dup := uint32(r.GPR[0])
	if p.kernel.descriptors[dup].file != p.kernel.descriptors[first].file {
		t.Fatal("dup lost shared description")
	}
	r.GPR[0] = uint64(first)
	p.kernel.closeDescriptor(&r)
	if p.kernel.descriptors[dup].file.offset != 17 {
		t.Fatal("close destroyed alias")
	}
	p.Memory.Map(0x2000, 144, usermem.Read|usermem.Write, nil)
	r.GPR[0] = uint64(dup)
	r.GPR[1] = 0x2000
	if !p.kernel.fstat(&r, p.Memory) || r.C {
		t.Fatal("immutable nonempty file stat rejected")
	}
}

func TestExecutableFGetPathReturnsCapturedCanonicalAlias(t *testing.T) {
	profile := startupFixture(t)
	profile.ExecutableStat = &FileStat{Mode: 0x81ed, Inode: profile.ExecutableFile.ObjectID, Size: int64(len(machotest.Image(2)))}
	p := process(t, Options{StartupProfile: profile, ExecName: "/alias/program"})
	p.Memory.Map(0x1000, 15, usermem.Read, []byte("/alias/program\x00"))
	p.Memory.Map(0x2000, 64, usermem.Read|usermem.Write, bytes.Repeat([]byte{0xa5}, 64))
	r := emu.Registers{}
	r.GPR[0] = 0x1000
	if !p.kernel.open(&r, p.Memory) || r.C {
		t.Fatal(r)
	}
	r.GPR[1] = 50
	r.GPR[2] = 0x2000
	if !p.kernel.fcntl(&r, p.Memory) || r.C {
		t.Fatal(r)
	}
	got := make([]byte, 64)
	p.Memory.Read(0x2000, got)
	want := bytes.Repeat([]byte{0xa5}, 64)
	copy(want, []byte("/guest/program\x00"))
	if !bytes.Equal(got, want) {
		t.Fatal("canonical alias/tail differs", got)
	}
}
