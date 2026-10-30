package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// Self traps copy out a send reference, preserving the name while it is live.
// Destruction/pinned-port guards need a separate modeled exception contract.
func copySelfRight(refs *uint32) bool {
	if *refs == 65535 {
		return false
	}
	(*refs)++
	return true
}
func (k *kernelState) currentTaskRight(name uint32) bool {
	return name == 0x103 && k.taskSendRefs != 0
}
func (k *kernelState) modifyPortRefs(r *emu.Registers) bool {
	if !k.currentTaskRight(uint32(r.GPR[0])) {
		r.GPR[0] = 0x10000003
		return true
	}
	name, right, delta := uint32(r.GPR[1]), uint32(r.GPR[2]), int64(int32(r.GPR[3]))
	if right >= 6 {
		r.GPR[0] = 18
		return true
	} // KERN_INVALID_VALUE
	if name == 0 || name == 0xffffffff {
		r.GPR[0] = 15
		if right == 0 || right == 2 {
			r.GPR[0] = 0
		}
		return true
	}
	if right != 0 {
		return false
	}
	var refs *uint32
	switch name {
	case 0x103:
		refs = &k.taskSendRefs
	case 0x107:
		refs = &k.hostSendRefs
	case 0x10b:
		refs = &k.threadSendRefs
	default:
		return false
	}
	if *refs == 0 {
		return false
	}
	next := int64(*refs) + delta
	if next <= 0 || next > 65535 {
		return false
	}
	*refs = uint32(next)
	r.GPR[0] = 0
	return true
}

// A receive right retains its port type and context for later IPC operations.
// These are guest objects; no host port is allocated.
type receivePort struct {
	typedReply bool
	context    uint64
	queueLimit uint32
}

func (k *kernelState) allocateReceivePort(port receivePort) uint32 {
	if k.nextPort == 0 {
		k.nextPort = 0x10f
	}
	if k.nextPort >= 0x10000 {
		return 0
	}
	if k.receivePorts == nil {
		k.receivePorts = make(map[uint32]receivePort)
	}
	name := k.nextPort
	k.nextPort += 4
	k.receivePorts[name] = port
	return name
}

// XNU copies all 24 option bytes before construction, then copies out the name.
// A failed output write therefore leaves the newly allocated receive right live.
func (k *kernelState) constructPort(r *emu.Registers, m *usermem.AddressSpace) bool {
	if !k.currentTaskRight(uint32(r.GPR[0])) {
		r.GPR[0] = 0x10000003
		return true
	}
	var options [24]byte
	if m.Read(r.GPR[1], options[:]) != nil {
		r.GPR[0] = 10
		return true
	}
	flags := binary.LittleEndian.Uint32(options[:4])
	limit := binary.LittleEndian.Uint32(options[4:8])
	if flags & ^uint32(0x1ffff) != 0 {
		r.GPR[0] = 4
		return true
	}
	// Guards, send insertion, and other port families require separate semantics.
	if flags & ^uint32(0x1002) != 0 {
		return false
	}
	if flags&2 == 0 {
		limit = 5
	} else if limit > 1024 {
		r.GPR[0] = 18
		return true
	}
	name := k.allocateReceivePort(receivePort{typedReply: flags&0x1000 != 0, context: r.GPR[2], queueLimit: limit})
	if name == 0 {
		return false
	} // Bounded namespace exhaustion is not a native result.
	var output [4]byte
	binary.LittleEndian.PutUint32(output[:], name)
	err := m.Write(r.GPR[3], output[:])
	r.GPR[0] = 0
	if err != nil {
		r.GPR[0] = 10
	}
	return true
}
