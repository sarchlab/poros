package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// fsgetpath resolves explicitly captured identities in the guest namespace.
// It does not query a host filesystem on behalf of guest code.
func (k *kernelState) fsgetpath(r *emu.Registers, m *usermem.AddressSpace) bool {
	if len(k.files) == 0 {
		return false
	}
	finish := func(value uint64, err bool) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = err; return true }
	var fsid [8]byte
	if m.Read(r.GPR[2], fsid[:]) != nil {
		return finish(14, true)
	}
	size := r.GPR[1]
	if size == 0 || size > 8192 {
		return finish(22, true)
	}
	id := binary.LittleEndian.Uint32(fsid[:4])
	for _, f := range k.files {
		// Ordinary fsgetpath uses the low volfs ID; fsgetpath_ext has other options.
		if uint32(f.FSID) != id {
			continue
		}
		if f.ObjectID != r.GPR[3] {
			continue
		}
		b := append([]byte(f.Path), 0)
		if uint64(len(b)) > size {
			return finish(28, true)
		}
		if m.Check(r.GPR[0], len(b), usermem.Write, "fsgetpath copyout") != nil && m.Check(r.GPR[0], 1, usermem.Write, "fsgetpath copyout") == nil {
			return false
		}
		if m.Write(r.GPR[0], b) != nil {
			return finish(14, true)
		}
		return finish(uint64(len(b)), false)
	}
	// A captured mount does not describe all of its inode namespace. Unknown
	// identities remain unsupported instead of inventing an ENOENT response.
	return false
}
