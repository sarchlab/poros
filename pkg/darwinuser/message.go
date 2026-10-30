package darwinuser

import (
	"bytes"
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

var nativeNDR = [8]byte{0, 0, 0, 0, 1, 0, 0, 0}

// message2 performs a synchronous task-kobject RPC. Header fields come from
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
	if options != 0x200000003 || bits != 0x80001513 || size != 100 || voucher != 0 || descriptors != 1 || id != 4811 || receive != reply || priority != 0 || r.GPR[7] != 0 {
		return false
	}
	if !k.currentTaskRight(target) {
		return false
	}
	if _, ok := k.receivePorts[reply]; !ok {
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
	var response [52]byte
	responseSize := uint32(36)
	if result == 0 {
		responseSize = 44
		binary.LittleEndian.PutUint64(response[36:44], address)
	}
	binary.LittleEndian.PutUint32(response[:4], 0x1200)
	binary.LittleEndian.PutUint32(response[4:8], responseSize)
	binary.LittleEndian.PutUint32(response[12:16], reply)
	binary.LittleEndian.PutUint32(response[20:24], id+100)
	copy(response[24:32], nativeNDR[:])
	binary.LittleEndian.PutUint32(response[32:36], uint32(result))
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
