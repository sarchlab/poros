package darwinuser

import (
	"encoding/binary"
	"fmt"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"io"
)

// dyldLinkInfo is the owned version-7 kernel pager request. Only the ordinary
// ARM64 offset-based 64-bit chains are admitted; authenticated formats need a
// different effective architectural contract. No guest dyld work is skipped.
type dyldLinkInfo struct {
	imageAddress uint64
	binds        []uint64
	segments     []dyldLinkSegment
}
type dyldLinkSegment struct {
	address uint64
	starts  []uint16
}

func parseDyldLinkInfo(blob []byte) (*dyldLinkInfo, error) {
	fail := func() (*dyldLinkInfo, error) {
		return nil, fmt.Errorf("unsupported or malformed dyld linking metadata")
	}
	u := binary.LittleEndian
	if len(blob) < 40 || len(blob) > 1<<20 || u.Uint32(blob) != 7 || u.Uint16(blob[4:]) != uint16(vmPage) || u.Uint16(blob[6:]) != 6 {
		return fail()
	}
	bo, bc, co, cs := uint64(u.Uint32(blob[8:])), uint64(u.Uint32(blob[12:])), uint64(u.Uint32(blob[16:])), uint64(u.Uint32(blob[20:]))
	n := uint64(len(blob))
	if bo < 40 || bo > n || bc > (n-bo)/8 || co < 40 || co > n || cs < 8 || cs > n-co {
		return fail()
	}
	if bc != 0 && bo < co+cs && co < bo+bc*8 {
		return fail()
	}
	p := &dyldLinkInfo{imageAddress: u.Uint64(blob[32:])}
	for i := uint64(0); i < bc; i++ {
		p.binds = append(p.binds, u.Uint64(blob[bo+i*8:]))
	}
	chains := blob[co : co+cs]
	count := uint64(u.Uint32(chains))
	if count == 0 || count > 128 || count > (cs-4)/4 {
		return fail()
	}
	type span struct{ lo, hi uint64 }
	var used []span
	for i := uint64(0); i < count; i++ {
		offset := uint64(u.Uint32(chains[4+i*4:]))
		if offset == 0 {
			continue
		}
		if offset < 4+count*4 || offset > cs || cs-offset < 22 {
			return fail()
		}
		size := uint64(u.Uint32(chains[offset:]))
		if size < 22 || size > cs-offset {
			return fail()
		}
		for _, v := range used {
			if offset < v.hi && v.lo < offset+size {
				return fail()
			}
		}
		used = append(used, span{offset, offset + size})
		s := chains[offset : offset+size]
		pages := uint64(u.Uint16(s[20:]))
		rel := u.Uint64(s[8:])
		if u.Uint16(s[4:]) != uint16(vmPage) || u.Uint16(s[6:]) != 6 || pages > (size-22)/2 || rel%vmPage != 0 || p.imageAddress > userAddressLimit || rel > userAddressLimit-p.imageAddress {
			return fail()
		}
		address := p.imageAddress + rel
		if pages > (userAddressLimit-address)/vmPage {
			return fail()
		}
		seg := dyldLinkSegment{address: address}
		for j := uint64(0); j < pages; j++ {
			start := u.Uint16(s[22+j*2:])
			if start != 0xffff && (start >= uint16(vmPage)-7 || start&0x8000 != 0) {
				return fail()
			}
			seg.starts = append(seg.starts, start)
		}
		for _, v := range p.segments {
			if address < v.address+uint64(len(v.starts))*vmPage && v.address < address+pages*vmPage {
				return fail()
			}
		}
		p.segments = append(p.segments, seg)
	}
	return p, nil
}

// linkedFilePages fixes each source page before AddressSpace installs a private
// resident copy. Malformed chains fault on page-in, without leaking partial
// results into guest memory. The file and request snapshot never change.
type linkedFilePages struct {
	file    filePages
	address uint64
	info    *dyldLinkInfo
}

func (p linkedFilePages) page(offset uint64) ([]byte, error) {
	b := make([]byte, vmPage)
	if n, e := p.file.ReadAt(b, int64(offset)); n != len(b) || (e != nil && e != io.EOF) {
		return nil, fmt.Errorf("dyld pager backing read: %v", e)
	}
	address := p.address + offset
	var start uint16 = 0xffff
	matched := false
	for _, s := range p.info.segments {
		if address >= s.address && address-s.address < uint64(len(s.starts))*vmPage {
			matched = true
			start = s.starts[(address-s.address)/vmPage]
			break
		}
	}
	if !matched {
		return nil, fmt.Errorf("dyld pager page has no chain segment")
	}
	if start == 0xffff {
		return b, nil
	}
	u := binary.LittleEndian
	at := uint64(start)
	for {
		if at > vmPage-8 {
			return nil, fmt.Errorf("dyld pointer chain leaves page")
		}
		raw := u.Uint64(b[at:])
		next := ((raw >> 51) & 0xfff) * 4
		var value uint64
		if raw>>63 != 0 {
			index := raw & 0xffffff
			if index >= uint64(len(p.info.binds)) {
				return nil, fmt.Errorf("dyld bind ordinal outside table")
			}
			value = p.info.binds[index] + ((raw >> 24) & 0xff)
		} else {
			value = (raw & 0xfffffffff) + p.info.imageAddress + (((raw >> 36) & 0xff) << 56)
		}
		u.PutUint64(b[at:], value)
		if next == 0 {
			break
		}
		if next < 8 {
			return nil, fmt.Errorf("overlapping dyld chain pointers unsupported")
		}
		at += next
	}
	return b, nil
}
func (p linkedFilePages) ReadAt(dst []byte, offset int64) (int, error) {
	if offset < 0 || uint64(offset) > p.file.size {
		return 0, io.EOF
	}
	n := min(uint64(len(dst)), p.file.size-uint64(offset))
	written := uint64(0)
	// Stage every page first so even direct ReaderAt callers see no partial fixup
	// bytes if a later chain is invalid.
	out := make([]byte, n)
	for written < n {
		pos := uint64(offset) + written
		page := pos &^ (vmPage - 1)
		b, e := p.page(page)
		if e != nil {
			return 0, e
		}
		within := pos - page
		amount := min(n-written, vmPage-within)
		copy(out[written:written+amount], b[within:within+amount])
		written += amount
	}
	copy(dst, out)
	if n < uint64(len(dst)) {
		return int(n), io.EOF
	}
	return int(n), nil
}

// mapWithLinking implements the file-backed data pager, not a shortcut through
// dyld. Its metadata is guest-produced; imports have already been resolved by
// guest instructions. Other file kinds and pointer formats remain unsupported.
func (k *kernelState) mapWithLinking(r *emu.Registers, m *usermem.AddressSpace) bool {
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	if k.mapLinkingDisabled {
		return finish(22)
	}
	allowed := false
	for _, rule := range k.sandboxSyscalls {
		if rule.Number == 550 && rule.Decision == 0 {
			allowed = true
		}
	}
	if !allowed {
		return false
	}
	count, size := uint64(uint32(r.GPR[1])), uint64(uint32(r.GPR[3]))
	if count == 0 || count > 5 {
		return finish(22)
	}
	if size < 40 || size > 1<<20 {
		return false
	}
	regions := make([]byte, count*32)
	blob := make([]byte, size)
	if m.Read(r.GPR[0], regions) != nil || m.Read(r.GPR[2], blob) != nil {
		return finish(22)
	}
	info, e := parseDyldLinkInfo(blob)
	if e != nil {
		return false
	}
	u := binary.LittleEndian
	fd := u.Uint32(regions)
	descriptor, exists := k.descriptors[fd]
	if !exists {
		return finish(22)
	}
	if descriptor.file == nil || descriptor.file.reader == nil || k.executableImage == nil || descriptor.file.reader != k.executableImage || k.amfi == nil || k.amfi.CodeSigningStatus&1 == 0 {
		return false
	}
	if descriptor.file.flags&3 == 1 {
		return finish(22)
	}
	type replacement struct {
		address, size uint64
		prot          uint32
		source        linkedFilePages
	}
	var replacements []replacement
	for i := uint64(0); i < count; i++ {
		v := regions[i*32:]
		prot := u.Uint32(v[4:])
		offset, address, length := u.Uint64(v[8:]), u.Uint64(v[16:]), u.Uint64(v[24:])
		if u.Uint32(v) != fd {
			return finish(22)
		}
		if prot&4 != 0 {
			return finish(22)
		}
		if prot&^uint32(3) != 0 || address%vmPage != 0 || offset%vmPage != 0 || length == 0 || length%vmPage != 0 || address >= userAddressLimit || length > userAddressLimit-address {
			return false
		}
		for _, old := range replacements {
			if address < old.address+old.size && old.address < address+length {
				return false
			}
		}
		if e := k.executableImage.VerifyCodeSignatureRange(offset, length, k.executableCDHash); e != nil {
			return false
		}
		source, relative, ok := m.ReaderBacking(address, length)
		if !ok {
			return false
		}
		var originalOffset uint64
		switch source := source.(type) {
		case imageSegment:
			if source.image != k.executableImage || relative > source.segment.FileSize || length > source.segment.FileSize-relative {
				return false
			}
			originalOffset = source.image.SliceOffset() + source.segment.FileOffset + relative
		case filePages:
			if source.source != k.executableImage {
				return false
			}
			originalOffset = source.offset + relative
		default:
			return false
		}
		if offset != originalOffset {
			return false
		}
		file := filePages{source: k.executableImage, offset: offset, size: length, fileSize: k.executableImage.FileSize}
		replacements = append(replacements, replacement{address, length, prot, linkedFilePages{file: file, address: address, info: info}})
	}
	// All supported replacement ranges have been checked before the first change.
	// The native API's partially failing multi-region replacement paths are not
	// admitted above; each operation here has no remaining fallible backing IO.
	for _, v := range replacements {
		if e := m.ReplaceReader(v.address, v.size, machPermissions(v.prot), machPermissions(v.prot), v.source, vmPage); e != nil {
			return false
		}

	}
	return finish(0)
}
