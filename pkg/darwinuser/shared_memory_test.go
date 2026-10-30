package darwinuser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"os"
	"strings"
	"testing"
)

func sharedMemoryFixture() ReadOnlySharedMemory {
	data := []byte("explicit captured configuration")
	hash := sha256.Sum256(data)
	return ReadOnlySharedMemory{Name: "runtime.config", Stat: FileStat{Mode: 0644, Size: int64(len(data))}, SHA256: hex.EncodeToString(hash[:]), Contents: data}
}
func TestSharedMemoryProfileBoundsAndOwnership(t *testing.T) {
	runtime, identity := nativeDyldFixture()
	entry := sharedMemoryFixture()
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, NativeDyld: &identity, ReadOnlySharedMemory: []ReadOnlySharedMemory{entry}}
	p, e := Parse(machotest.Image(2), runtime, Options{RuntimeProfile: &profile})
	if e != nil {
		t.Fatal(e)
	}
	entry.Contents[0] = 'X'
	profile.ReadOnlySharedMemory[0].Name = "changed"
	if p.kernel.sharedMemory[0].Name != "runtime.config" || p.kernel.sharedMemory[0].Contents[0] != 'e' {
		t.Fatal("borrowed mutable snapshot")
	}
	for _, kind := range []string{"identity", "duplicate", "hash", "size", "empty", "large", "aggregate", "nul", "long-name", "many", "time"} {
		t.Run(kind, func(t *testing.T) {
			q := profile
			q.ReadOnlySharedMemory = []ReadOnlySharedMemory{sharedMemoryFixture()}
			switch kind {
			case "identity":
				q.NativeDyld = nil
			case "duplicate":
				q.ReadOnlySharedMemory = append(q.ReadOnlySharedMemory, q.ReadOnlySharedMemory[0])
			case "hash":
				q.ReadOnlySharedMemory[0].SHA256 = strings.Repeat("0", 64)
			case "size":
				q.ReadOnlySharedMemory[0].Stat.Size++
			case "empty":
				q.ReadOnlySharedMemory[0].Stat.Size = 0
				q.ReadOnlySharedMemory[0].Contents = nil
			case "large":
				q.ReadOnlySharedMemory[0].Stat.Size = maxSharedMemorySnapshotBytes + 1
			case "aggregate":
				for i := 0; i < 3; i++ {
					x := sharedMemoryFixture()
					x.Name = string(rune('a' + i))
					x.Contents = make([]byte, 32768)
					x.Stat.Size = 32768
					h := sha256.Sum256(x.Contents)
					x.SHA256 = hex.EncodeToString(h[:])
					q.ReadOnlySharedMemory = append(q.ReadOnlySharedMemory, x)
				}
			case "nul":
				q.ReadOnlySharedMemory[0].Name = "bad\x00name"
			case "long-name":
				q.ReadOnlySharedMemory[0].Name = strings.Repeat("a", 32)
			case "many":
				q.ReadOnlySharedMemory = make([]ReadOnlySharedMemory, 9)
			case "time":
				q.ReadOnlySharedMemory[0].Stat.Access.Nanoseconds = -1
			}
			if q.validate() == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}
func TestSharedMemoryOpenIsExplicitAndReadOnly(t *testing.T) {
	for _, kind := range []string{"known", "unknown", "write", "null", "bad", "long", "fd-bound"} {
		t.Run(kind, func(t *testing.T) {
			entry := sharedMemoryFixture()
			k := &kernelState{sharedMemory: []ReadOnlySharedMemory{entry}}
			m := &usermem.AddressSpace{}
			name := entry.Name
			if kind == "unknown" {
				name = "not.captured"
			}
			if kind == "long" {
				name = strings.Repeat("a", 32)
			}
			m.Map(0x1000, 128, usermem.Read, append([]byte(name), 0))
			r := emu.Registers{Z: true}
			r.GPR[0] = 0x1000
			r.GPR[2] = ^uint64(0)
			r.GPR[16] = 266
			switch kind {
			case "write":
				r.GPR[1] = 2
			case "null":
				r.GPR[0] = 0
			case "bad":
				r.GPR[0] = 1
			case "fd-bound":
				k.descriptors = make(map[uint32]guestDescriptor)
				for i := uint32(3); i < maxGuestDescriptors; i++ {
					k.descriptors[i] = guestDescriptor{}
				}
			}
			old := r
			ok := k.trap(&r, m)
			switch kind {
			case "known":
				if !ok || r.GPR[0] != 3 || r.C || k.descriptors[3].sharedMemory == nil {
					t.Fatal(r)
				}
			case "null", "bad":
				old.GPR[0] = 14
				old.GPR[1] = 0
				old.C = true
				if !ok || r != old {
					t.Fatal(r)
				}
			default:
				if ok || r != old {
					t.Fatal("unsupported mutated state")
				}
			}
		})
	}
}

func TestSharedMemoryNativeOpenAndStat(t *testing.T) {
	var native struct {
		Dependencies map[string]string `json:"dependencies_sha256"`
		Cases        []struct {
			Name                string
			Flags, Mode, X0, X1 uint64
			Input               uint64 `json:"input_nzcv"`
			Output              uint64 `json:"output_nzcv"`
			Stat                FileStat
			StatResult          int `json:"stat_result"`
			Close               int
		}
	}
	kernelNativeResult(t, "shm-open-probe.c", &native)
	header, e := os.ReadFile("../../scripts/macho-stat.h")
	if e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(header)
	if native.Dependencies["macho-stat.h"] != hex.EncodeToString(hash[:]) || len(native.Cases) != 6 {
		t.Fatal("native ABI source identity mismatch")
	}
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			entry := sharedMemoryFixture()
			entry.Name = "com.apple.featureflags.shm"
			entry.Stat = c.Stat
			entry.Contents = make([]byte, 32768)
			k := &kernelState{sharedMemory: []ReadOnlySharedMemory{entry}}
			m := &usermem.AddressSpace{}
			m.Map(0x1000, 256, usermem.Read|usermem.Write, append([]byte(entry.Name), 0))
			m.Map(0x2000, 160, usermem.Read|usermem.Write, bytes.Repeat([]byte{0xa5}, 160))
			r := emu.Registers{Z: true, V: true}
			r.GPR[0] = 0x1000
			r.GPR[1] = c.Flags
			r.GPR[2] = c.Mode
			r.GPR[16] = 266
			if c.Name == "null-name" {
				r.GPR[0] = 0
			}
			if c.Name == "invalid-name" {
				r.GPR[0] = 1
			}
			old := r
			if !k.trap(&r, m) || r.GPR[0] != c.X0 || r.GPR[1] != c.X1 {
				t.Fatal(c.Name, r)
			}
			old.GPR[0] = c.X0
			old.GPR[1] = c.X1
			old.C = c.Output&0x20000000 != 0
			if r != old {
				t.Fatal("unexpected ABI change")
			}
			if r.C {
				return
			}
			fd := r.GPR[0]
			r.GPR[1] = 0x2000
			r.GPR[16] = 339
			if !k.trap(&r, m) || r.GPR[0] != 0 {
				t.Fatal(r)
			}
			got := make([]byte, 160)
			m.Read(0x2000, got)
			want := c.Stat.bytes()
			if !bytes.Equal(got[:144], want[:]) || !bytes.Equal(got[144:], bytes.Repeat([]byte{0xa5}, 16)) {
				t.Fatal("SHM stat payload mismatch")
			}
			r.GPR[0] = fd
			r.GPR[16] = 6
			if !k.trap(&r, m) || r.GPR[0] != uint64(c.Close) || len(k.descriptors) != 0 {
				t.Fatal("descriptor not closed")
			}
		})
	}
}

func TestSharedMemoryNativeMappings(t *testing.T) {
	var native struct {
		Cases []struct {
			Name                        string
			Length, Prot, Flags, Offset uint64
			InvalidFD                   int `json:"invalid_fd"`
			X0, X1                      uint64
			Input                       uint64 `json:"input_nzcv"`
			Output                      uint64 `json:"output_nzcv"`
			Before                      string `json:"before_close_hex"`
			After                       string `json:"after_close_hex"`
			Protection, Inheritance     int
			MaxProtection               int `json:"max_protection"`
		}
		Read    struct{ X0 uint64 }
		Upgrade struct{ X0 uint64 } `json:"write_upgrade"`
	}
	kernelNativeResult(t, "shm-mmap-probe.c", &native)
	if len(native.Cases) != 17 {
		t.Fatal(len(native.Cases))
	}
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			// Unobserved bytes are synthetic fixture data; only the recorded native
			// window is replayed. The live host's entire config is not a second golden.
			entry := sharedMemoryFixture()
			entry.Stat.Size = 32768
			entry.Contents = make([]byte, 32768)
			for i := range entry.Contents {
				entry.Contents[i] = byte(i*17 + 3)
			}
			before, _ := hex.DecodeString(c.Before)
			after, _ := hex.DecodeString(c.After)
			if len(before) != 0 {
				copy(entry.Contents[c.Offset:], before)
			}
			k := &kernelState{descriptors: map[uint32]guestDescriptor{3: {sharedMemory: &entry}}}
			m := &usermem.AddressSpace{}
			r := emu.Registers{Z: true, V: true}
			r.GPR[1] = c.Length
			r.GPR[2] = c.Prot
			r.GPR[3] = c.Flags
			r.GPR[4] = 3
			r.GPR[5] = c.Offset
			r.GPR[16] = 197
			if c.InvalidFD != 0 {
				r.GPR[4] = 500
			}
			old := r
			ok := k.trap(&r, m)
			if c.Name == "zero-length" || c.Name == "unaligned-offset" {
				if ok || r != old || len(m.Mappings()) != 0 || k.ownedBytes != 0 {
					t.Fatal("unmodeled legacy mapping mutated state")
				}
				return
			}
			if !ok {
				t.Fatal("native mapping unsupported", c.Name)
			}
			failed := c.Output&0x20000000 != 0
			if r.C != failed || r.GPR[1] != c.X1 {
				t.Fatal(r)
			}
			if failed {
				old.GPR[0] = c.X0
				old.GPR[1] = c.X1
				old.C = true
				if r != old || len(m.Mappings()) != 0 {
					t.Fatal("native mmap error mismatch", r)
				}
				return
			}
			address := r.GPR[0]
			old.GPR[0] = address
			old.GPR[1] = 0
			if r != old || address%vmPage != 0 || c.Protection != 1 || c.MaxProtection != 1 || c.Inheritance != 0 {
				t.Fatal(r)
			}
			got := make([]byte, len(before))
			if e := m.Read(address, got); e != nil || !bytes.Equal(got, before) {
				t.Fatal("mapped snapshot mismatch", e)
			}
			if m.Write(address, []byte{1}) == nil {
				t.Fatal("readonly snapshot became writable")
			}
			// Native full-map readonly maxima make BSD mprotect's write upgrade fail.
			protect := emu.Registers{}
			protect.GPR[0] = address
			protect.GPR[1] = c.Length
			protect.GPR[2] = 3
			protect.GPR[16] = 74
			if !k.trap(&protect, m) || protect.GPR[0] != native.Upgrade.X0 || !protect.C {
				t.Fatal("shared write upgrade", protect)
			}
			close := emu.Registers{}
			close.GPR[0] = 3
			close.GPR[16] = 6
			if !k.trap(&close, m) || close.GPR[0] != 0 {
				t.Fatal(close)
			}
			if e := m.Read(address, got); e != nil || !bytes.Equal(got, after) {
				t.Fatal("mapping lost snapshot after close", e)
			}
		})
	}
}

func TestSharedMemoryDescriptorIsNotAFileOrDirectory(t *testing.T) {
	for _, operation := range []string{"openat", "fstatat", "getpath", "read", "write"} {
		t.Run(operation, func(t *testing.T) {
			entry := sharedMemoryFixture()
			k := &kernelState{descriptors: map[uint32]guestDescriptor{3: {sharedMemory: &entry}}}
			m := &usermem.AddressSpace{}
			before := bytes.Repeat([]byte{0xa5}, 256)
			copy(before, []byte("runtime\x00"))
			m.Map(0x1000, 256, usermem.Read|usermem.Write, before)
			r := emu.Registers{N: true, C: true}
			r.GPR[0] = 3
			r.GPR[1] = 0x1000
			r.GPR[2] = 16
			switch operation {
			case "openat":
				r.GPR[16] = 463
				r.GPR[2] = 0x100000
			case "fstatat":
				r.GPR[16] = 470
				r.GPR[2] = 0x1080
			case "getpath":
				r.GPR[16] = 92
				r.GPR[1] = 50
				r.GPR[2] = 0x1080
			case "read":
				r.GPR[16] = 3
			case "write":
				r.GPR[16] = 4
				k.outputLimit = 1024
			}
			old := r
			if k.trap(&r, m) || r != old {
				t.Fatal("shared memory misclassified", operation, r)
			}
			after := make([]byte, 256)
			m.Read(0x1000, after)
			if !bytes.Equal(before, after) || len(k.descriptors) != 1 {
				t.Fatal("unsupported operation mutated state")
			}
		})
	}
}
