package darwinuser

import (
	"errors"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
)

// machProtect preserves Mach return registers and flags. COPY is bounded to
// existing private data pages whose maximum already permits write; it never
// recreates their backing or discards private dirty bytes.
func (k *kernelState) machProtect(r *emu.Registers, m *usermem.AddressSpace) bool {
	if !k.currentTaskRight(uint32(r.GPR[0])) {
		r.GPR[0] = 0x10000003 // MACH_SEND_INVALID_DEST; kernelrpc rejects foreign task rights.
		return true
	}
	addr, size, prot := r.GPR[1], r.GPR[2], uint64(uint32(r.GPR[4]))
	copyPrivate := prot&0x10 != 0
	setMaximum := uint32(r.GPR[3]) != 0
	if prot&^uint64(0x17) != 0 || size > ^uint64(0)-addr || addr+size > ^uint64(0)-0x3fff {
		r.GPR[0] = 4
		return true
	}
	end := (addr + size + 0x3fff) &^ uint64(0x3fff)
	addr &^= 0x3fff
	if size == 0 {
		r.GPR[0] = 0
		return true
	}
	if k.overlapsReclaim(addr, end) {
		return false
	}
	if copyPrivate {
		// Cache DATA uses validated private reader pages; synthetic kernel
		// fields and executable mappings need different OS semantics.
		for _, span := range k.protected {
			if addr < span.hi && span.lo < end {
				source, offset, ok := m.ReaderBacking(addr, end-addr)
				cache, isCache := source.(loader.CacheMapping)
				if !ok || !isCache || cache.Protection&4 != 0 || cache.MaxProtection&4 != 0 || cache.MaxProtection&2 == 0 || offset > cache.Size || end-addr > cache.Size-offset || cache.Address+offset != addr {
					return false
				}
			}
		}
		for _, mapping := range m.Mappings() {
			if addr < mapping.End && mapping.Start < end && mapping.Flags&usermem.Execute != 0 {
				return false
			}
		}
		prot &^= 0x10
		if prot&4 != 0 || (setMaximum && prot != 3) {
			return false
		}
		// Native COPY can raise maximum write permission. This bounded
		// path only covers mappings whose maximum already allows it.
		if e := m.CheckProtection(addr, end-addr, machPermissions(uint32(prot))|usermem.Write); errors.Is(e, usermem.ErrProtection) {
			return false
		}
	}
	if prot&4 != 0 {
		for _, span := range k.ownedRanges {
			if addr < span.hi && span.lo < end {
				return false
			}
		}
	} // Owned memory needs an explicit signing/JIT policy before executable transitions.
	err := m.Protect(addr, end-addr, machPermissions(uint32(prot)), setMaximum)
	r.GPR[0] = 0
	if err != nil {
		r.GPR[0] = 1
		if errors.Is(err, usermem.ErrProtection) {
			r.GPR[0] = 2
		}
	}
	return true
}
