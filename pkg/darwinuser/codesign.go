package darwinuser

import (
	"encoding/binary"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// codeSigningStatus implements the read-only CS_OPS_STATUS query for this guest.
// XNU bsd/kern/kern_proc.c csops_internal ignores usersize for this operation;
// a null output is a successful query without copyout. The status was captured
// for this executable before dyld and checked again by its AMFI diagnostic.
func (k *kernelState) codeSigningStatus(r *emu.Registers, m *usermem.AddressSpace) bool {
	if k.amfi == nil || uint32(r.GPR[1]) != 0 || (uint32(r.GPR[0]) != 0 && uint32(r.GPR[0]) != 100) {
		return false
	}
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	output := r.GPR[2]
	if output == 0 {
		return finish(0)
	}
	if m.Check(output, 4, usermem.Write, "csops status copyout") != nil && m.Check(output, 1, usermem.Write, "csops status copyout") == nil {
		return false
	}
	var status [4]byte
	binary.LittleEndian.PutUint32(status[:], k.amfi.CodeSigningStatus)
	if m.Write(output, status[:]) != nil {
		return finish(14)
	}
	return finish(0)
}
