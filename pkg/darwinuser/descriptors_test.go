package darwinuser

import (
	"bytes"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestRootDirectoryNativeContract(t *testing.T) {
	var cases []struct {
		Case   string
		X0, X1 uint64
		Carry  bool
	}
	kernelNativeResult(t, "root-directory-probe.c", &cases)
	if len(cases) != 14 {
		t.Fatal(len(cases))
	}
	k := kernelState{}
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 2, usermem.Read, []byte{'/', 0})
	m.Map(0x2000, 9, usermem.Read, []byte("/unknown\x00"))
	var fd uint64
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			r := emu.Registers{C: true}
			r.GPR[16] = 5
			r.GPR[0] = 0x1000
			r.GPR[1] = 0x20100000
			switch tc.Case {
			case "root_directory":
				r.GPR[1] = 0x100000
			case "root_readonly":
				r.GPR[1] = 0
			case "close_owned_directory", "close_already_closed":
				r.GPR[16] = 6
				r.GPR[0] = fd
				r.GPR[1] = 0xabcdef
			case "close_invalid_minus1":
				r.GPR[16] = 6
				r.GPR[0] = ^uint64(0)
			case "empty_path":
				r.GPR[0]++
			case "null_path":
				r.GPR[0] = 0
			case "unmapped_path":
				r.GPR[0] = 1
			case "regular_file_via_symlink_parent":
				r.GPR[0] = 0x2000
			}
			before := r
			handled := k.trap(&r, m)
			if tc.Case == "regular_file_via_symlink_parent" {
				if handled || r != before {
					t.Fatal("invented uncaptured file metadata")
				}
				return
			}
			if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:3], r.C)
			}
			if before.GPR[16] == 5 && !r.C {
				fd = r.GPR[0]
				if k.descriptors[uint32(fd)].path != "/" {
					t.Fatal(k.descriptors)
				}
			}
		})
	}
	if len(k.descriptors) != 0 {
		t.Fatal("leaked guest descriptor")
	}
}

func TestRootDescriptorAllocationAndUnsupportedModes(t *testing.T) {
	k := kernelState{}
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 2, usermem.Read, []byte{'/', 0})
	for fd := uint32(3); fd < maxGuestDescriptors; fd++ {
		r := emu.Registers{}
		r.GPR[0] = 0x1000
		if !k.open(&r, m) || r.GPR[0] != uint64(fd) {
			t.Fatal(fd, r.GPR[0])
		}
	}
	r := emu.Registers{}
	r.GPR[0] = 0x1000
	before := r
	if k.open(&r, m) || r != before {
		t.Fatal("invented native FD limit")
	}
	r.GPR[0] = 7
	if !k.closeDescriptor(&r) || r.C {
		t.Fatal(r)
	}
	r.GPR[0] = 0x1000
	if !k.open(&r, m) || r.GPR[0] != 7 {
		t.Fatal("lowest free FD not reused")
	}
	for _, flags := range []uint64{1, 2, 0x200, 0x400} {
		r := emu.Registers{}
		r.GPR[0] = 0x1000
		r.GPR[1] = flags
		before := r
		if k.open(&r, m) || r != before {
			t.Fatal("unsupported mutating open changed state")
		}
	}
}

func TestCapturedDirectoryOpenAtNativeContract(t *testing.T) {
	for _, source := range []string{"runtime-directory-probe.c", "rosetta-directory-probe.c"} {
		t.Run(source, func(t *testing.T) { testCapturedDirectoryOpenAtNative(t, source) })
	}
}
func testCapturedDirectoryOpenAtNative(t *testing.T, source string) {
	var ob struct {
		Directory struct {
			Path          string
			CanonicalPath string `json:"canonical_path"`
		}
		Cases []struct {
			Case   string
			X0, X1 uint64
			Carry  bool
		}
	}
	kernelNativeResult(t, source, &ob)
	if len(ob.Cases) != 10 {
		t.Fatal(len(ob.Cases))
	}
	for _, tc := range ob.Cases {
		t.Run(tc.Case, func(t *testing.T) {
			k := kernelState{directories: []RuntimeDirectory{{Path: ob.Directory.Path, CanonicalPath: ob.Directory.CanonicalPath, Symlink: true}}, descriptors: map[uint32]guestDescriptor{3: {path: "/"}}}
			r := emu.Registers{C: true}
			r.GPR[16] = 463
			r.GPR[0] = 3
			r.GPR[1] = 0x1000
			r.GPR[2] = 0x100000
			name := ob.Directory.Path[1:]
			switch tc.Case {
			case "relative_nofollow_any":
				r.GPR[2] |= 0x20000000
			case "absolute_directory":
				name = ob.Directory.Path
			case "absolute_ignores_bad_fd":
				name = ob.Directory.Path
				r.GPR[0] = ^uint64(0)
			case "relative_bad_fd":
				r.GPR[0] = ^uint64(0)
			case "empty_path":
				name = ""
			case "null_path":
				r.GPR[1] = 0
			case "unmapped_path":
				r.GPR[1] = 1
			case "root_dot":
				name = "."
			case "root_dotdot":
				name = ".."
			}
			m := &usermem.AddressSpace{}
			m.Map(0x1000, uint64(len(name)+1), usermem.Read, append([]byte(name), 0))
			if !k.trap(&r, m) || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:3], r.C)
			}
			if !r.C {
				node := k.descriptors[uint32(r.GPR[0])]
				want := ob.Directory.CanonicalPath
				if tc.Case == "root_dot" || tc.Case == "root_dotdot" {
					want = "/"
				}
				if node.path != want {
					t.Fatal(node)
				}
			}
		})
	}
}

func TestDirectoryNamespaceDoesNotCleanUnknownTraversal(t *testing.T) {
	for _, name := range []string{"System/Cryptexes/OS/../other", "System/../System/Cryptexes/OS", "/uncaptured"} {
		k := kernelState{directories: []RuntimeDirectory{{Path: "/System/Cryptexes/OS", CanonicalPath: "/canonical", Symlink: true}}, descriptors: map[uint32]guestDescriptor{3: {path: "/"}}}
		m := &usermem.AddressSpace{}
		m.Map(0x1000, uint64(len(name)+1), usermem.Read, append([]byte(name), 0))
		r := emu.Registers{}
		r.GPR[0] = 3
		r.GPR[1] = 0x1000
		r.GPR[2] = 0x100000
		before := r
		if k.openAt(&r, m) || r != before || len(k.descriptors) != 1 {
			t.Fatal("invented traversal", name, r)
		}
	}
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 2, usermem.Read, []byte{'.', 0})
	r := emu.Registers{}
	r.GPR[0] = 0xfffffffe
	r.GPR[1] = 0x1000
	r.GPR[2] = 0x100000
	before := r
	k := kernelState{}
	if k.openAt(&r, m) || r != before {
		t.Fatal("AT_FDCWD misreported as invalid FD")
	}
}

func TestDirectoryDuplicateNativeLifecycle(t *testing.T) {
	var cases []struct {
		Case   string
		X0, X1 uint64
		Carry  bool
	}
	kernelNativeResult(t, "dup-directory-probe.c", &cases)
	if len(cases) != 12 {
		t.Fatal(len(cases))
	}
	k := kernelState{descriptors: map[uint32]guestDescriptor{3: {path: "/"}, 4: {path: "/captured/directory", flags: 0x100000}}}
	for _, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			r := emu.Registers{C: true}
			r.GPR[16] = 41
			r.GPR[0] = 4
			r.GPR[1] = 0xabcdef
			switch tc.Case {
			case "close_original", "close_last_alias4":
				r.GPR[16] = 6
			case "duplicate_survives_close_and_reuses_lowest":
				r.GPR[0] = 5
			case "close_first_duplicate", "close_last_alias5":
				r.GPR[16] = 6
				r.GPR[0] = 5
			case "duplicate_low_signed32_fd":
				r.GPR[0] = 0x100000004
			case "duplicate_invalid_minus1":
				r.GPR[0] = ^uint64(0)
			case "duplicate_invalid_minus2":
				r.GPR[0] = ^uint64(0) - 1
			case "duplicate_unopened_fd":
				r.GPR[0] = 500
			case "close_root":
				r.GPR[16] = 6
				r.GPR[0] = 3
			}
			number := r.GPR[16]
			if !k.trap(&r, &usermem.AddressSpace{}) || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc, r.GPR[:2], r.C)
			}
			if number == 41 && !r.C && k.descriptors[uint32(r.GPR[0])].path != "/captured/directory" {
				t.Fatal("duplicated node identity lost")
			}
		})
	}
	if len(k.descriptors) != 0 {
		t.Fatal("directory aliases leaked", k.descriptors)
	}
}

func TestDyldDirectoryTrailingSeparatorNativeOpen(t *testing.T) {
	var cases []struct {
		Path   string
		X0, X1 uint64
		Carry  bool
		Device uint32
		Inode  uint64
		Mode   uint16
	}
	kernelNativeResult(t, "dyld-directory-open-probe.c", &cases)
	if len(cases) != 3 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		const canonical = "/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld"
		node := RuntimeDirectory{Path: "/System/Cryptexes/OS/System/Library/dyld", CanonicalPath: canonical, Symlink: true, Stat: &FileStat{Device: tc.Device, Inode: tc.Inode, Mode: tc.Mode}}
		k := kernelState{directories: []RuntimeDirectory{node}, descriptors: map[uint32]guestDescriptor{3: {path: "/"}, 4: {path: "/System/Volumes/Preboot/Cryptexes/OS"}}}
		m := &usermem.AddressSpace{}
		m.Map(0x1000, uint64(len(tc.Path)+1), usermem.Read, append([]byte(tc.Path), 0))
		r := emu.Registers{C: true}
		r.GPR[0] = 4
		r.GPR[1] = 0x1000
		r.GPR[2] = 0x100000
		r.GPR[16] = 463
		if !k.trap(&r, m) || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry || k.descriptors[uint32(r.GPR[0])].path != canonical {
			t.Fatal(tc, r.GPR[:3], k.descriptors)
		}
	}
}

func TestDirectoryGetPathNativeCopyout(t *testing.T) {
	var ob struct {
		Expected string
		Size     int
		Cases    []struct {
			Case   string
			X0, X1 uint64
			Carry  bool
			Bytes  string
		}
	}
	kernelNativeResult(t, "fgetpath-directory-probe.c", &ob)
	if ob.Size != len(ob.Expected)+1 || len(ob.Cases) != 11 {
		t.Fatal(ob.Size, len(ob.Cases))
	}
	for _, tc := range ob.Cases {
		t.Run(tc.Case, func(t *testing.T) {
			k := kernelState{descriptors: map[uint32]guestDescriptor{6: {path: ob.Expected}}}
			r := emu.Registers{C: true}
			r.GPR[0] = 6
			r.GPR[1] = 50
			r.GPR[2] = 0x2000
			r.GPR[16] = 92
			want, e := hex.DecodeString(tc.Bytes)
			if e != nil {
				t.Fatal(e)
			}
			size := len(want)
			if size == 0 {
				size = 1024
			}
			perm := usermem.Read | usermem.Write
			switch tc.Case {
			case "low32_fd":
				r.GPR[0] = 0x100000006
			case "invalid_fd":
				r.GPR[0] = ^uint64(0)
			case "null_output":
				r.GPR[2] = 0
			case "unmapped_output":
				r.GPR[2] = 1
			case "readonly_output":
				perm = usermem.Read
			}
			m := &usermem.AddressSpace{}
			m.Map(0x2000, uint64(size), perm, bytes.Repeat([]byte{0xa5}, size))
			before := r
			handled := k.trap(&r, m)
			partial := size < ob.Size
			if partial {
				if handled || r != before {
					t.Fatal("unmodeled partial output mutated state")
				}
				want = bytes.Repeat([]byte{0xa5}, size)
			} else if !handled || r.GPR[0] != tc.X0 || r.GPR[1] != tc.X1 || r.C != tc.Carry {
				t.Fatal(tc.Case, r.GPR[:3], r.C)
			}
			actual := make([]byte, size)
			m.Read(0x2000, actual)
			if len(want) == 0 {
				want = bytes.Repeat([]byte{0xa5}, size)
			}
			if !bytes.Equal(actual, want) {
				t.Fatal(tc.Case, actual)
			}
		})
	}
}
