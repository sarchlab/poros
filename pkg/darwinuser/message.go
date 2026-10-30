package darwinuser

import (
	"bytes"
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

var nativeNDR = [8]byte{0, 0, 0, 0, 1, 0, 0, 0}

// message2 performs a bounded synchronous kernel-object RPC. Header fields come from
// packed trap arguments; XNU copies only the descriptor/payload tail from user
// memory. The supported exchange makes and consumes a send-once reply right in
// this trap, leaving the receive right live. Other transports/services are
// explicit unsupported dependencies, not fabricated successful messages.
func (k *kernelState) message2(r *emu.Registers, m *usermem.AddressSpace) bool {
	data, options := r.GPR[0], r.GPR[1]
	bits, size := uint32(r.GPR[2]), uint32(r.GPR[2]>>32)
	target, reply := uint32(r.GPR[3]), uint32(r.GPR[3]>>32)
	voucher, id := uint32(r.GPR[4]), uint32(r.GPR[4]>>32)
	descriptors, receive := uint32(r.GPR[5]), uint32(r.GPR[5]>>32)
	receiveSize, priority := uint32(r.GPR[6]), uint32(r.GPR[6]>>32)
	if options != 0x200000003 || voucher != 0 || receive != reply || priority != 0 || r.GPR[7] != 0 {
		return false
	}
	if _, ok := k.receivePorts[reply]; !ok {
		return false
	}
	if id == 200 {
		if target != k.hostRightName() || k.hostSendRefs == 0 || bits != 0x1513 || descriptors != 0 {
			return false
		}
		return k.messageHostPriority(r, m, data, size, reply, receiveSize)
	}
	if id != 4811 || bits != 0x80001513 || size != 100 || descriptors != 1 || !k.currentTaskRight(target) {
		return false
	}
	var request [100]byte
	if data > ^uint64(0)-100 || m.Read(data+28, request[28:]) != nil {
		r.GPR[0] = 0x10000002
		return true
	}
	// mach_vm_map's single COPY_SEND port descriptor names a null memory object.
	if binary.LittleEndian.Uint32(request[28:32]) != 0 || request[38] != 19 || request[39] != 0 || !bytes.Equal(request[40:48], nativeNDR[:]) {
		return false
	}
	hint := binary.LittleEndian.Uint64(request[48:56])
	length := binary.LittleEndian.Uint64(request[56:64])
	mask := binary.LittleEndian.Uint64(request[64:72])
	flags := binary.LittleEndian.Uint32(request[72:76])
	offset := binary.LittleEndian.Uint64(request[76:84])
	copyOnWrite := binary.LittleEndian.Uint32(request[84:88])
	current := binary.LittleEndian.Uint32(request[88:92])
	maximum := binary.LittleEndian.Uint32(request[92:96])
	inherit := binary.LittleEndian.Uint32(request[96:100])
	if offset != 0 || copyOnWrite != 0 || inherit != 1 {
		return false
	}
	// Partial page-fault copyout needs an explicit byte-level kernel contract.
	// Handle complete writable buffers and faults at their first byte; reject a
	// mixed-permission output before any VM mutation rather than inventing atomic
	// kernel copyout semantics. Only bytes this receive could copy are checked.
	if count := min(receiveSize, uint32(52)); count >= 24 && m.Check(data, int(count), usermem.Write, "Mach reply") != nil && m.Check(data, 1, usermem.Write, "Mach reply") == nil {
		return false
	}
	address, result, supported := k.allocateAnonymous(m, hint, length, mask, flags, current, maximum)
	if !supported {
		return false
	}
	var payload []byte
	if result == 0 {
		payload = make([]byte, 8)
		binary.LittleEndian.PutUint64(payload, address)
	}
	return messageReply(r, m, data, reply, id, receiveSize, uint32(result), payload)
}

// messageReply is shared by the supported kernel services. The caller validates
// mixed-permission output before any kernel mutation; failed delivery does not
// roll back completed service work.
func messageReply(r *emu.Registers, m *usermem.AddressSpace, data uint64, reply, id, receiveSize, result uint32, payload []byte) bool {
	responseSize := uint32(36 + len(payload))
	response := make([]byte, responseSize+8)
	binary.LittleEndian.PutUint32(response[:4], 0x1200)
	binary.LittleEndian.PutUint32(response[4:8], responseSize)
	binary.LittleEndian.PutUint32(response[12:16], reply)
	binary.LittleEndian.PutUint32(response[20:24], id+100)
	copy(response[24:32], nativeNDR[:])
	binary.LittleEndian.PutUint32(response[32:36], result)
	copy(response[36:responseSize], payload)
	binary.LittleEndian.PutUint32(response[responseSize+4:responseSize+8], 8)
	// Kernel work precedes receive delivery, including a too-small receiver.
	// Without MACH_RCV_LARGE the reply is consumed and replaced by a bare header.
	count := responseSize + 8
	r.GPR[0] = 0
	if receiveSize < count {
		if receiveSize < 24 {
			r.GPR[0] = 0x10004008
			return true
		}
		clear(response[24:])
		binary.LittleEndian.PutUint32(response[4:8], 24)
		binary.LittleEndian.PutUint32(response[28:32], 8)
		count = min(receiveSize, uint32(32))
		r.GPR[0] = 0x10004004
	}
	if m.Write(data, response[:count]) != nil {
		r.GPR[0] = 0x10004008
	}
	return true
}

// HOST_PRIORITY_INFO is captured host policy. Other flavors may be valid kernel
// services, so they remain unsupported instead of receiving a guessed error.
func (k *kernelState) messageHostPriority(r *emu.Registers, m *usermem.AddressSpace, data uint64, size, reply, receiveSize uint32) bool {
	if k.hostPriorities == nil || (size != 36 && size != 40 && size != 44) {
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
	if size == 40 && binary.LittleEndian.Uint32(request[32:36]) != 5 {
		return false
	}
	if count := min(receiveSize, uint32(80)); count >= 24 && m.Check(data, int(count), usermem.Write, "Mach reply") != nil && m.Check(data, 1, usermem.Write, "Mach reply") == nil {
		return false
	}
	// MIG validates the fixed request size before calling host_info.
	result := uint32(0)
	var payload []byte
	if size != 40 {
		result = 0xfffffed0 // MIG_BAD_ARGUMENTS (-304)
	} else if binary.LittleEndian.Uint32(request[36:40]) < 8 {
		result = 5 // KERN_FAILURE
	} else {
		payload = make([]byte, 36)
		binary.LittleEndian.PutUint32(payload[:4], 8)
		for index, priority := range k.hostPriorities {
			binary.LittleEndian.PutUint32(payload[4+index*4:], uint32(priority))
		}
	}
	return messageReply(r, m, data, reply, 200, receiveSize, result, payload)
}
