package darwinuser

import (
	"bytes"
	"encoding/binary"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// ReclaimPolicy identifies the observed kernel/user ABI, including a deadline
// rather than a duration in the second MIG output. Native absolute timestamps
// are not profile state; each reply uses this process's virtual absolute clock.
type ReclaimPolicy struct {
	ABI           string `json:"abi"`
	IntervalTicks uint64 `json:"interval_ticks,string"`
}

func (p ReclaimPolicy) valid(frequency uint64) bool {
	return p.ABI == "deadline-128-v1" && frequency > 0 && frequency <= ^uint64(0)/10 && p.IntervalTicks == 10*frequency
}

type reclaimState struct {
	policy        ReclaimPolicy
	clock         *virtualClock
	address, size uint64
}

// Only registration is implemented. The real guest wrapper initializes the
// zero-filled ring. Accounting, resize, flush and ambient reclamation remain
// unsupported; no background reclaim behavior is invented by registration.
func (k *kernelState) messageReclaimAllocate(r *emu.Registers, m *usermem.AddressSpace, data uint64, size, reply, receiveSize uint32) bool {
	state := k.reclaim
	if state == nil || state.clock == nil || !r.Counter.Enabled || !state.policy.valid(r.Counter.Frequency) || (size != 36 && size != 40 && size != 44) {
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
	result := uint32(0)
	length, maximum := binary.LittleEndian.Uint32(request[32:36]), binary.LittleEndian.Uint32(request[36:40])
	if size != 40 {
		result = 0xfffffed0
	} else if length == 0 || maximum == 0 || maximum < length {
		result = 4
	} else if maximum > (128*1024*1024-128)/16 {
		result = 3
	} else if state.address != 0 {
		result = 0x20004006
	}
	fullReply := uint32(44)
	if result == 0 {
		fullReply = 60
	}
	if count := messageCopyoutSize(receiveSize, fullReply); count >= 24 && m.Check(data, int(count), usermem.Write, "reclaim reply") != nil && m.Check(data, 1, usermem.Write, "reclaim reply") == nil {
		return false
	}
	var payload []byte
	if result == 0 {
		// Hardware-to-absolute offset is a modular addition, as in native MRS
		// paths. An unrepresentable future deadline is explicitly unsupported.
		now := r.Counter.Value + state.clock.anchor.AbsoluteOffset
		if now > ^uint64(0)-state.policy.IntervalTicks {
			return false
		}
		span := (128 + uint64(maximum)*16 + vmPage - 1) &^ (vmPage - 1)
		address, code, ok := k.allocateAnonymous(m, 0, span, 0, 1, 3, 3)
		if !ok {
			return false
		}
		result = uint32(code)
		if result == 0 {
			state.address, state.size = address, span
			// XNU's permanent allocation has special unmap semantics. Existing
			// generic deallocation/protection must not silently remove it.
			k.protected = append(k.protected, addressSpan{address, address + span})
			k.reserved = append(k.reserved, addressSpan{address, address + span})
			payload = make([]byte, 16)
			binary.LittleEndian.PutUint64(payload, address)
			binary.LittleEndian.PutUint64(payload[8:], now+state.policy.IntervalTicks)
		}
	}
	return messageReply(r, m, data, reply, 4822, receiveSize, result, payload)
}

// Permanent reclaim mappings have native tombstone/permission semantics that
// ordinary VM operations do not model. Stop before changing any overlapping page.
func (k *kernelState) overlapsReclaim(start, end uint64) bool {
	s := k.reclaim
	return s != nil && s.size != 0 && start < s.address+s.size && s.address < end
}
