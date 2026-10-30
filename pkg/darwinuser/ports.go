package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// Self traps copy out a send reference, preserving the name while it is live.
// Last task/thread-send pinned guards need a separate exception contract.
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
	refs := k.selfSendRefs(name)
	if refs == nil {
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

// The initial reserved host name is stable only while its send right lives.
// Reacquisition after the last release allocates a fresh private name, so stale
// names cannot silently regain authority.
func (k *kernelState) hostRightName() uint32 {
	if k.hostName == 0 {
		return 0x107
	}
	return k.hostName
}
func (k *kernelState) selfSendRefs(name uint32) *uint32 {
	if name == k.hostRightName() {
		return &k.hostSendRefs
	}
	switch name {
	case 0x103:
		return &k.taskSendRefs
	case 0x10b:
		return &k.threadSendRefs
	}
	return nil
}
func (k *kernelState) deallocatePort(r *emu.Registers) bool {
	if !k.currentTaskRight(uint32(r.GPR[0])) {
		r.GPR[0] = 0x10000003
		return true
	}
	name := uint32(r.GPR[1])
	if name == 0 || name == 0xffffffff {
		r.GPR[0] = 0
		return true
	}
	if refs := k.selfSendRefs(name); refs != nil {
		if *refs == 0 {
			r.GPR[0] = 15
			return true
		}
		// Last task/thread sends may be pinned and raise guarded-port exceptions.
		if name != k.hostRightName() && *refs == 1 {
			return false
		}
		if name == k.hostRightName() && k.hostName == 0 {
			k.hostName = name
		}
		(*refs)--
		r.GPR[0] = 0
		return true
	}
	if _, known := k.receivePorts[name]; known {
		return false
	}
	r.GPR[0] = 15 // no right exists under this private namespace name
	return true
}

// A receive right retains its port type and context for later IPC operations.
// These are guest objects; no host port is allocated.
type receivePort struct {
	typedReply bool
	context    uint64
	queueLimit uint32
}

func (k *kernelState) allocatePortName() uint32 {
	if k.nextPort == 0 {
		k.nextPort = 0x10f
	}
	if k.nextPort >= 0x10000 {
		return 0
	}
	name := k.nextPort
	k.nextPort += 4
	return name
}
func (k *kernelState) allocateReceivePort(port receivePort) uint32 {
	name := k.allocatePortName()
	if name == 0 {
		return 0
	}
	if k.receivePorts == nil {
		k.receivePorts = make(map[uint32]receivePort)
	}
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
