package darwinuser

import (
	"bytes"
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// A semaphore is owned by the guest task, with its initial value and queue policy
// retained for future operations. Creating one does not run a host synchronizer.
// Blocking/waiting operations require a separate scheduling contract.
type semaphore struct {
	policy   uint32
	value    int32
	sendRefs uint32
}

func (k *kernelState) messageSemaphoreCreate(r *emu.Registers, m *usermem.AddressSpace, data uint64, size, reply, receiveSize uint32) bool {
	if size != 36 && size != 40 && size != 44 {
		return false
	}
	var request [44]byte
	if data > ^uint64(0)-uint64(size) || m.Read(data+24, request[24:size]) != nil {
		r.GPR[0] = 0x10000002
		return true
	}
	if !bytes.Equal(request[24:32], nativeNDR[:]) {
		return false
	}
	policy := binary.LittleEndian.Uint32(request[32:36])
	value := int32(binary.LittleEndian.Uint32(request[36:40]))
	result := uint32(0)
	if size != 40 {
		result = 0xfffffed0
	} else if value < 0 || policy & ^uint32(6) != 0 {
		result = 4
	}
	fullSize := uint32(48)
	if result != 0 {
		fullSize = 44
	}
	if count := messageCopyoutSize(receiveSize, fullSize); count >= 24 && m.Check(data, int(count), usermem.Write, "Mach reply") != nil && m.Check(data, 1, usermem.Write, "Mach reply") == nil {
		return false
	}
	if result != 0 {
		return messageReply(r, m, data, reply, 3418, receiveSize, result, nil)
	}
	response := portMessageReply(reply, 3418)
	// Undelivered messages drop their sole send right; XNU's no-senders path
	// destroys the now-unreferenced task object. Only a full receive installs it.
	if receiveSize >= 48 {
		name := k.allocatePortName()
		if name == 0 {
			return false
		}
		if k.semaphores == nil {
			k.semaphores = make(map[uint32]semaphore)
		}
		k.semaphores[name] = semaphore{policy: policy, value: value, sendRefs: 1}
		binary.LittleEndian.PutUint32(response[28:32], name)
	}
	return deliverMessageReply(r, m, data, receiveSize, response)
}
