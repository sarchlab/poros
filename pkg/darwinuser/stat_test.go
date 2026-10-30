package darwinuser

import (
	"bytes"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestFileStatNativeContract(t *testing.T) {
	var ob struct {
		Size   int
		Fields FileStat `json:"decoded_fields"`
		Cases  []struct {
			Case   string
			X0, X1 uint64
			Carry  bool
			Bytes  string
		}
	}
	kernelNativeResult(t, "fstatat-directory-probe.c", &ob)
	if ob.Size != 144 || len(ob.Cases) != 10 {
		t.Fatal(ob.Size, len(ob.Cases))
	}
	const alias = "/System/Cryptexes/OS/System/Library/dyld"
	const canonical = "/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld"
	for _, tc := range ob.Cases {
		t.Run(tc.Case, func(t *testing.T) {
			k := kernelState{directories: []RuntimeDirectory{{Path: alias, CanonicalPath: canonical, Symlink: true, Stat: &ob.Fields}}, descriptors: map[uint32]guestDescriptor{4: {path: "/System/Volumes/Preboot/Cryptexes/OS"}}}
			r := emu.Registers{C: true}
			r.GPR[16] = 470
			r.GPR[0] = 4
			r.GPR[1] = 0x1000
			r.GPR[2] = 0x2000
			name := "System/Library/dyld/"
			size := 160
			perm := usermem.Read | usermem.Write
			switch tc.Case {
			case "selected_absolute_directory":
				name = alias + "/"
			case "absolute_ignores_bad_fd":
				name = alias + "/"
				r.GPR[0] = ^uint64(0)
			case "relative_bad_fd":
				r.GPR[0] = ^uint64(0)
			case "empty_path":
				name = ""
			case "null_path":
				r.GPR[1] = 0
			case "bad_path":
				r.GPR[1] = 1
			case "null_output":
				r.GPR[2] = 0
			case "readonly_output":
				perm = usermem.Read
			case "short_output_prefix":
				size = 8
			}
			m := &usermem.AddressSpace{}
			m.Map(0x1000, uint64(len(name)+1), usermem.Read, append([]byte(name), 0))
			m.Map(0x2000, uint64(size), perm, bytes.Repeat([]byte{0xa5}, size))
			before := r
			handled := k.trap(&r, m)
			if tc.Case == "short_output_prefix" {
				if handled || r != before {
					t.Fatal("partial stat copyout is not generally modeled")
				}
			} else if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:4], r.C)
			}
			out := make([]byte, size)
			m.Read(0x2000, out)
			want, e := hex.DecodeString(tc.Bytes)
			if e != nil {
				t.Fatal(e)
			}
			if len(want) == 0 {
				want = bytes.Repeat([]byte{0xa5}, size)
			}
			if !bytes.Equal(out, want) {
				t.Fatal(tc, out)
			}
		})
	}
}

func TestFileStatValidation(t *testing.T) {
	regular := FileStat{Mode: 0x8000}
	if regular.validate() != nil || validateDirectories([]RuntimeDirectory{{Path: "/x", CanonicalPath: "/x", Stat: &regular}}) == nil {
		t.Fatal("file kind not validated at directory boundary")
	}
	for _, s := range []FileStat{{Mode: 0x4000, Access: StatTime{Nanoseconds: -1}}, {Mode: 0x4000, Birth: StatTime{Nanoseconds: 1_000_000_000}}} {
		if s.validate() == nil {
			t.Fatal("invalid stat accepted")
		}
	}
}

func TestStatExecutableNativeContract(t *testing.T) {
	var ob struct {
		Fields FileStat `json:"decoded_fields"`
		Cases  []struct {
			Case   string
			X0, X1 uint64
			Carry  bool
			Bytes  string
		}
	}
	kernelNativeResult(t, "stat-executable-probe.c", &ob)
	if len(ob.Cases) != 11 {
		t.Fatal(len(ob.Cases))
	}
	for _, tc := range ob.Cases {
		t.Run(tc.Case, func(t *testing.T) {
			k := kernelState{executableStat: &ob.Fields, executablePaths: []string{"/guest/program", "/canonical/program"}}
			m := &usermem.AddressSpace{}
			name := "/guest/program"
			r := emu.Registers{C: true}
			r.GPR[0] = 0x1000
			r.GPR[1] = 0x2000
			r.GPR[16] = 338
			size, perm := 160, usermem.Read|usermem.Write
			switch tc.Case {
			case "empty_path":
				name = ""
			case "null_path":
				r.GPR[0] = 0
			case "unmapped_path":
				r.GPR[0] = 1
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
			m.Map(0x1000, uint64(len(name)+1), usermem.Read, append([]byte(name), 0))
			initial := bytes.Repeat([]byte{0xa5}, size)
			m.Map(0x2000, uint64(size), perm, initial)
			before := r
			handled := k.trap(&r, m)
			got := make([]byte, size)
			m.Read(0x2000, got)
			if size < 144 {
				if handled || r != before || !bytes.Equal(got, initial) {
					t.Fatal("partial native copyout must remain unsupported without mutation")
				}
				return
			}
			if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:2], r.C)
			}
			want := initial
			if tc.Bytes != "" {
				var e error
				want, e = hex.DecodeString(tc.Bytes)
				if e != nil {
					t.Fatal(e)
				}
			}
			if !bytes.Equal(got, want) {
				t.Fatal("native output differs")
			}
		})
	}
}
