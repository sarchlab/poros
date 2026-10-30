package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// XNU mach_timebase_info_trap ignores copyout's error and returns success.
// Mixed writable/nonwritable output may be partially modified natively; that
// form remains unsupported because guest partial copyout is not modeled.
func (k *kernelState) timebaseInfo(r *emu.Registers, m *usermem.AddressSpace) bool {
	if k.timebase[0] == 0 || k.timebase[1] == 0 {
		return false
	}
	address := r.GPR[0]
	if m.Check(address, 8, usermem.Write, "timebase copyout") != nil {
		if m.Check(address, 1, usermem.Write, "timebase copyout") == nil {
			return false
		}
	} else {
		var data [8]byte
		binary.LittleEndian.PutUint32(data[:], k.timebase[0])
		binary.LittleEndian.PutUint32(data[4:], k.timebase[1])
		if m.Write(address, data[:]) != nil {
			return false
		}
	}
	r.GPR[0] = 0
	return true
}
