package darwinuser

import (
	"bytes"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestMachProtectCopyNativeContract(t *testing.T) {
	var native struct {
		Cases []struct {
			Name                  string
			Initial               uint32 `json:"initial_maximum"`
			Requested, Length, X0 uint64
			SetMax                uint64 `json:"set_maximum"`
			Regions               []struct {
				Mapped              bool
				Protection, Maximum uint32
			}
			Data []int
		}
	}
	kernelNativeResult(t, "vm-protect-copy-probe.c", &native)
	if len(native.Cases) != 16 {
		t.Fatal(len(native.Cases))
	}
	extra := native
	extra.Cases = nil
	kernelNativeResult(t, "vm-copy-max-probe.c", &extra)
	if len(extra.Cases) != 3 {
		t.Fatal(len(extra.Cases))
	}
	native.Cases = append(native.Cases, extra.Cases...)
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			k := kernelState{taskSendRefs: 1}
			m := &usermem.AddressSpace{}
			m.MapProtected(0x10000, 3*vmPage, usermem.Read, machPermissions(c.Initial), bytes.Repeat([]byte{90}, int(3*vmPage)))
			r := emu.Registers{}
			r.GPR[0] = 0x103
			r.GPR[1] = 0x10000
			r.GPR[2] = c.Length
			r.GPR[3] = c.SetMax
			r.GPR[4] = c.Requested
			r.GPR[16] = uint64(0xfffffff2)
			r.N = true
			r.C = true
			switch c.Name {
			case "unaligned_copy_one":
				r.GPR[1]++
			case "copy_spans_hole", "copy_unmapped":
				m.Unmap(0x14000, vmPage)
				if c.Name == "copy_unmapped" {
					r.GPR[1] = 0x14000
				}
			case "copy_zero_null":
				r.GPR[1] = 0
			case "invalid_task":
				r.GPR[0] = 0
			}
			before := r
			handled := k.trap(&r, m)
			if c.Name == "raise_ro_max_copy" {
				if handled || r != before {
					t.Fatal("unmodeled maximum expansion")
				}
				return
			}
			if !handled || r.GPR[0] != c.X0 || r.GPR[1] != before.GPR[1] || r.N != before.N || r.C != before.C {
				t.Fatal(c, r, handled)
			}
			for i, want := range c.Regions {
				addr := uint64(0x10000) + uint64(i)*vmPage
				found := false
				for _, region := range m.Mappings() {
					if addr >= region.Start && addr < region.End {
						found = true
						if region.Flags != machPermissions(want.Protection) {
							t.Fatal("current protection", i, region)
						}
					}
				}
				if found != want.Mapped {
					t.Fatal("mapping", i)
				}
				if found {
					for _, permission := range []uint32{1, 2, 4} {
						err := m.CheckProtection(addr, vmPage, machPermissions(permission))
						if (err == nil) != (permission&^want.Maximum == 0) {
							t.Fatal("maximum", i, permission, err)
						}
					}
				}
				if c.Data[i] >= 0 {
					var b [1]byte
					if m.Read(addr, b[:]) != nil || int(b[0]) != c.Data[i] {
						t.Fatal("private bytes", i)
					}
				}
			}
		})
	}
}
func TestMachCopyPreservesPrivateFilePages(t *testing.T) {
	var native struct {
		Requested uint64
		Return    uint64
		Before    byte `json:"private_before"`
		After     byte `json:"private_after_copy"`
		Written   byte `json:"private_after_write"`
		Other     byte `json:"other_mapping"`
		File      byte `json:"backing_file"`
	}
	kernelNativeResult(t, "vm-copy-file-probe.c", &native)
	source := bytes.Repeat([]byte{native.File}, int(vmPage))
	reader := bytes.NewReader(source)
	m := &usermem.AddressSpace{}
	m.MapReader(0x10000, vmPage, usermem.Read|usermem.Write, usermem.Read|usermem.Write, reader, vmPage)
	m.MapReader(0x20000, vmPage, usermem.Read, usermem.Read|usermem.Write, reader, vmPage)
	m.Write(0x10000, []byte{native.Before})
	m.Protect(0x10000, vmPage, usermem.Read, false)
	k := kernelState{taskSendRefs: 1}
	r := emu.Registers{}
	r.GPR[0] = 0x103
	r.GPR[1] = 0x10000
	r.GPR[2] = vmPage
	r.GPR[4] = native.Requested
	r.GPR[16] = 0xfffffff2
	if !k.trap(&r, m) || r.GPR[0] != native.Return {
		t.Fatal(r)
	}
	var b [1]byte
	m.Read(0x10000, b[:])
	if b[0] != native.After {
		t.Fatal("COW reset dirty page")
	}
	if m.Write(0x10000, []byte{native.Written}) != nil {
		t.Fatal("private write")
	}
	m.Read(0x20000, b[:])
	if b[0] != native.Other || source[0] != native.File {
		t.Fatal("backing modified")
	}
	// Shared-region or executable COPY requests need additional OS semantics.
	for _, prot := range []uint64{19, 21} {
		k.protected = []addressSpan{{0x10000, 0x14000}}
		r.GPR[0] = 0x103
		r.GPR[4] = prot
		before := r
		if k.trap(&r, m) || r != before {
			t.Fatal("protected COPY accepted")
		}
	}
}

func TestMachCopyValidatedCacheBacking(t *testing.T) {
	for _, kind := range []string{"data", "protected-readonly", "executable", "offset", "split", "hole", "replacement"} {
		t.Run(kind, func(t *testing.T) {
			cache := loader.CacheMapping{Address: 0x10000, Size: 2 * vmPage, Protection: 1, MaxProtection: 3}
			if kind == "protected-readonly" {
				cache.MaxProtection = 1
			}
			if kind == "executable" {
				cache.Protection = 5
				cache.MaxProtection = 7
			}
			if kind == "offset" {
				cache.Address += vmPage
			}
			m := &usermem.AddressSpace{}
			m.MapReader(0x10000, 2*vmPage, machPermissions(cache.Protection), machPermissions(cache.MaxProtection), cache, vmPage)
			if kind == "split" {
				m.Protect(0x10000, vmPage, usermem.Read, false)
			}
			if kind == "hole" {
				m.Unmap(0x14000, vmPage)
			}
			if kind == "replacement" {
				m.Unmap(0x14000, vmPage)
				m.MapReader(0x14000, vmPage, usermem.Read, usermem.Read|usermem.Write, cache, vmPage)
			}
			k := kernelState{taskSendRefs: 1, protected: []addressSpan{{0x10000, 0x20000}}}
			r := emu.Registers{}
			r.GPR[0] = 0x103
			r.GPR[1] = 0x10000
			r.GPR[2] = 2 * vmPage
			r.GPR[4] = 19
			r.GPR[16] = 0xfffffff2
			before := r
			ok := k.trap(&r, m)
			want := kind == "data" || kind == "split"
			if ok != want {
				t.Fatal(kind, ok, r)
			}
			if want {
				if r.GPR[0] != 0 || m.Check(0x10000, int(2*vmPage), usermem.Write, "test") != nil {
					t.Fatal("cache data not writable")
				}
			} else if r != before {
				t.Fatal("unsupported cache state mutated")
			}
		})
	}
}
