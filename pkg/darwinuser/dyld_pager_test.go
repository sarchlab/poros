package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"io"
	"strconv"
	"testing"
)

func linkingBlob(address uint64, starts []uint16, binds []uint64) []byte {
	u := binary.LittleEndian
	segmentSize := 22 + 2*len(starts)
	chainSize := 8 + segmentSize
	bindOffset := 40 + chainSize
	b := make([]byte, bindOffset+8*len(binds))
	u.PutUint32(b, 7)
	u.PutUint16(b[4:], uint16(vmPage))
	u.PutUint16(b[6:], 6)
	u.PutUint32(b[8:], uint32(bindOffset))
	u.PutUint32(b[12:], uint32(len(binds)))
	u.PutUint32(b[16:], 40)
	u.PutUint32(b[20:], uint32(chainSize))
	u.PutUint64(b[32:], address)
	u.PutUint32(b[40:], 1)
	u.PutUint32(b[44:], 8)
	u.PutUint32(b[48:], uint32(segmentSize))
	u.PutUint16(b[52:], uint16(vmPage))
	u.PutUint16(b[54:], 6)
	u.PutUint16(b[68:], uint16(len(starts)))
	for i, v := range starts {
		u.PutUint16(b[70+i*2:], v)
	}
	for i, v := range binds {
		u.PutUint64(b[bindOffset+i*8:], v)
	}
	return b
}
func TestDyldPagerNativeStartupPointers(t *testing.T) {
	var observed struct {
		RuntimeBase       string `json:"runtime_base"`
		PointerFormat     int    `json:"pointer_format"`
		SectionFileOffset uint64 `json:"section_file_offset"`
		Cases             []struct {
			Index                      int
			Encoding, Actual, Expected string
		}
	}
	kernelNativeResult(t, "linking-startup-probe.c", &observed)
	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	if observed.PointerFormat != 6 || len(observed.Cases) != 7 {
		t.Fatal(observed)
	}
	base := parse(observed.RuntimeBase)
	start := observed.SectionFileOffset % vmPage
	data := make([]byte, vmPage)
	binds := make([]uint64, 14)
	for _, c := range observed.Cases {
		binary.LittleEndian.PutUint64(data[start+uint64(c.Index)*8:], parse(c.Encoding))
	}
	binds[10] = parse(observed.Cases[2].Expected)
	binds[9] = parse(observed.Cases[5].Expected)
	binds[13] = parse(observed.Cases[6].Expected)
	blob := linkingBlob(base, []uint16{uint16(start)}, binds)
	binary.LittleEndian.PutUint64(blob[56:], vmPage)
	info, e := parseDyldLinkInfo(blob)
	if e != nil {
		t.Fatal(e)
	}
	pager := linkedFilePages{file: filePages{source: bytes.NewReader(data), size: vmPage, fileSize: vmPage}, address: base + vmPage, info: info}
	out := make([]byte, vmPage)
	if n, e := pager.ReadAt(out, 0); n != len(out) || e != nil {
		t.Fatal(n, e)
	}
	for _, c := range observed.Cases {
		got := binary.LittleEndian.Uint64(out[start+uint64(c.Index)*8:])
		if got != parse(c.Actual) || c.Actual != c.Expected {
			t.Fatal(c, got)
		}
	}
	// Original file and link-info inputs remain immutable; resident writes are private.
	m := &usermem.AddressSpace{}
	m.MapReader(base+vmPage, vmPage, usermem.Read|usermem.Write, usermem.Read|usermem.Write, pager, vmPage)
	m.Write(base+vmPage+start, []byte{99})
	fresh := make([]byte, 8)
	pager.ReadAt(fresh, int64(start))
	if binary.LittleEndian.Uint64(fresh) != parse(observed.Cases[0].Expected) {
		t.Fatal("private write leaked")
	}
}
func TestDyldPagerBoundsFaultsAndOwnership(t *testing.T) {
	valid := linkingBlob(0x10000, []uint16{16, 0xffff}, []uint64{0x12340000})
	for _, change := range []func([]byte){func(b []byte) { b[0] = 6 }, func(b []byte) { b[6] = 2 }, func(b []byte) { binary.LittleEndian.PutUint32(b[8:], ^uint32(0)) }, func(b []byte) { binary.LittleEndian.PutUint32(b[12:], ^uint32(0)) }, func(b []byte) { binary.LittleEndian.PutUint32(b[16:], 39) }, func(b []byte) { binary.LittleEndian.PutUint32(b[20:], ^uint32(0)) }, func(b []byte) { binary.LittleEndian.PutUint32(b[40:], 129) }, func(b []byte) { binary.LittleEndian.PutUint32(b[44:], 4) }, func(b []byte) { binary.LittleEndian.PutUint16(b[68:], 100) }, func(b []byte) { binary.LittleEndian.PutUint16(b[70:], 0x8000) }} {
		b := append([]byte(nil), valid...)
		change(b)
		if _, e := parseDyldLinkInfo(b); e == nil {
			t.Fatal("bad metadata accepted", hex.EncodeToString(b))
		}
	}
	info, e := parseDyldLinkInfo(valid)
	if e != nil {
		t.Fatal(e)
	}
	clear(valid)
	data := bytes.Repeat([]byte{0x55}, int(vmPage*2))
	binary.LittleEndian.PutUint64(data[16:], 1<<63|255<<24)
	p := linkedFilePages{file: filePages{source: bytes.NewReader(data), size: vmPage * 2, fileSize: vmPage * 2}, address: 0x10000, info: info}
	out := make([]byte, 12)
	if n, e := p.ReadAt(out, 12); n != 12 || e != nil || binary.LittleEndian.Uint64(out[4:]) != 0x123400ff {
		t.Fatal(n, e, out)
	}
	if n, e := p.ReadAt(out, int64(vmPage)); n != 12 || e != nil || !bytes.Equal(out, data[vmPage:vmPage+12]) {
		t.Fatal("START_NONE changed raw page", n, e)
	}
	if n, e := p.ReadAt(out, int64(vmPage*2)); n != 0 || e != io.EOF {
		t.Fatal(n, e)
	}
	p.address = 0x20000
	if _, e := p.ReadAt(out, 0); e == nil {
		t.Fatal("uncovered page passed through raw")
	}
	p.address = 0x10000
	for _, raw := range []uint64{1<<63 | 1, 0xfff << 51, 1 << 51} {
		binary.LittleEndian.PutUint64(data[16:], raw)
		before := bytes.Repeat([]byte{0xa5}, 32)
		dst := append([]byte(nil), before...)
		if n, e := p.ReadAt(dst, 0); n != 0 || e == nil || !bytes.Equal(dst, before) {
			t.Fatal("bad chain leaked bytes", n, e)
		}
	}
}
func FuzzDyldLinkInfo(f *testing.F) {
	f.Add(linkingBlob(0x10000, []uint16{0}, []uint64{7}))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		_, _ = parseDyldLinkInfo(b)
	})
}

func TestMapWithLinkingIdentityReplacementAndDisable(t *testing.T) {
	for _, fat := range []bool{false, true} {
		t.Run(fmt.Sprint(fat), func(t *testing.T) {
			data := make([]byte, vmPage)
			binary.LittleEndian.PutUint64(data[8:], 1<<63|7<<24)
			raw, hash := machotest.SignedDataImage(data)
			if fat {
				raw = machotest.Universal(raw)
			}
			image, e := loader.ParseMachO(raw, 2)
			if e != nil {
				t.Fatal(e)
			}
			segment := image.Segments[1]
			address := segment.Address
			offset := image.SliceOffset() + segment.FileOffset
			m := &usermem.AddressSpace{}
			m.MapReader(address, vmPage, usermem.Read|usermem.Write, usermem.Read|usermem.Write, imageSegment{image, segment}, vmPage)
			k := kernelState{sandboxSyscalls: []SandboxSyscallPermission{{Number: 550}}, executableImage: image, executableCDHash: hash, amfi: &AMFIPolicy{CodeSigningStatus: 1}, descriptors: map[uint32]guestDescriptor{3: {file: &guestOpenFile{reader: image}}}}
			region := make([]byte, 32)
			u := binary.LittleEndian
			u.PutUint32(region, 3)
			u.PutUint32(region[4:], 3)
			u.PutUint64(region[8:], offset)
			u.PutUint64(region[16:], address)
			u.PutUint64(region[24:], vmPage)
			blob := linkingBlob(address, []uint16{8}, []uint64{0x12340000})
			m.Map(0x10000, 32, usermem.Read|usermem.Write, region)
			m.Map(0x11000, uint64(len(blob)), usermem.Read|usermem.Write, blob)
			r := emu.Registers{C: true}
			r.GPR[0] = 0x10000
			r.GPR[1] = 1
			r.GPR[2] = 0x11000
			r.GPR[3] = uint64(len(blob))
			r.GPR[16] = 550
			before := r
			// Both CDHash and original file offset are checked before replacement.
			k.executableCDHash = "bad"
			if k.trap(&r, m) || r != before {
				t.Fatal("unbound signature accepted")
			}
			k.executableCDHash = hash
			wrong := make([]byte, 8)
			u.PutUint64(wrong, offset+vmPage)
			m.Write(0x10008, wrong)
			if k.trap(&r, m) || r != before {
				t.Fatal("wrong original offset accepted")
			}
			u.PutUint64(wrong, offset)
			m.Write(0x10008, wrong)
			m.Write(address+8, []byte{99})
			m.Protect(address, vmPage/2, usermem.Read, false)
			if !k.trap(&r, m) || r.GPR[0] != 0 || r.C {
				t.Fatal(r)
			}
			out := make([]byte, 8)
			m.Read(address+8, out)
			if u.Uint64(out) != 0x12340007 {
				t.Fatal("did not restore/fix original bytes", out)
			}
			if m.Protect(address, vmPage, usermem.Execute, false) == nil {
				t.Fatal("pager maximum permits execution")
			}
			delete(k.descriptors, 3)
			m.Write(address+8, []byte{1})
			original := make([]byte, 8)
			image.ReadAt(original, int64(offset+8))
			if u.Uint64(original) != 1<<63|7<<24 {
				t.Fatal("COW changed file")
			}
			r = before
			r.GPR[16] = 294
			r.GPR[0] = ^uint64(0)
			if !k.trap(&r, m) || r.GPR[0] != 0 || !k.mapLinkingDisabled {
				t.Fatal("disable trap")
			}
			r = before
			if !k.trap(&r, m) || r.GPR[0] != 22 || !r.C {
				t.Fatal("disabled pager accepted")
			}
		})
	}
}
