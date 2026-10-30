package darwinuser

import (
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// access answers only explicitly captured F_OK observations. Other permission
// queries and unknown paths are not inferred from stat or a host filesystem.
func (k *kernelState) access(r *emu.Registers, m *usermem.AddressSpace) bool {
	if uint32(r.GPR[1]) != 0 {
		return false
	}
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	name, errno, handled := k.directoryPath(m, r.GPR[0], 0, false)
	if !handled {
		return false
	}
	if errno != 0 {
		return finish(errno)
	}
	for _, absent := range k.accessENOENTPaths {
		if name == absent {
			return finish(2)
		}
	}
	return false
}
