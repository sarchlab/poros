package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// PthreadPolicy captures the native first registration's kernel capabilities and
// main-thread policy for this executable. Guest addresses remain virtual state.
type PthreadPolicy struct {
	Version           uint64 `json:"version"`
	Features          uint32 `json:"features"`
	MainQoS           uint64 `json:"main_qos,string"`
	MutexPolicy       uint32 `json:"mutex_policy"`
	CodeSigningStatus uint32 `json:"code_signing_status"`
}

func (p PthreadPolicy) valid() bool {
	return (p.Version == 48 || p.Version == 52 || p.Version == 56 || p.Version == 60) && p.Features > 0 && p.Features < 1<<31
}

type pthreadRegistration struct {
	ThreadStart, WorkqueueStart uint64
	PthreadSize                 uint32
	Data                        [60]byte
}

// registerPthread stores actual process registration, including normalized TSD
// offsets. No guest thread entry point or host syscall is invoked here.
func (k *kernelState) registerPthread(r *emu.Registers, m *usermem.AddressSpace) bool {
	if k.pthreadPolicy == nil {
		return false
	}
	finish := func(value, errno uint64) bool {
		r.GPR[0] = value
		if errno != 0 {
			r.GPR[0] = errno
		}
		r.GPR[1] = 0
		r.C = errno != 0
		return true
	}
	size := int32(r.GPR[2])
	if size < 0 || size > 65536 {
		return finish(0, 22)
	}
	pointer, length := r.GPR[3], r.GPR[4]
	count := min(length, k.pthreadPolicy.Version)
	var data [60]byte
	if pointer != 0 {
		if length < 8 {
			return finish(0, 22)
		}
		if m.Read(pointer, data[:count]) != nil {
			return finish(0, 14)
		}
		if binary.LittleEndian.Uint64(data[:8]) != length {
			return finish(0, 22)
		}
	} else {
		binary.LittleEndian.PutUint64(data[8:16], r.GPR[5])
	}
	// XNU refreshes this hint before its duplicate-registration check. It derives
	// from the main stack top, not from a captured native ASLR address.
	hint := StackTop & ^(vmPage - 1)
	if k.pthreadRegistration != nil {
		k.pthreadStackHint = hint
		return finish(0, 22)
	}
	// A partial kernel copyout could alter a prefix. Keep that unmodeled case
	// unsupported before registration; wholly readonly copyout still commits it.
	if pointer != 0 && m.Check(pointer, int(count), usermem.Write, "pthread registration") != nil && m.Check(pointer, 1, usermem.Write, "pthread registration") == nil {
		return false
	}
	tsd := binary.LittleEndian.Uint32(data[24:28])
	maxOffset := uint32(0)
	if size >= 8 && tsd <= uint32(size-8) {
		maxOffset = uint32(size) - tsd - 8
	} else {
		binary.LittleEndian.PutUint32(data[24:28], 0)
	}
	if binary.LittleEndian.Uint64(data[8:16]) > uint64(maxOffset) {
		clear(data[8:16])
	}
	// Byte56 is an additional TSD-relative offset observed in the current kernel.
	// Its range rule is independently native-verified; no private semantic name
	// or future operation using it is invented.
	for _, offset := range []uint64{28, 32, 52, 56} {
		if offset+4 <= k.pthreadPolicy.Version && binary.LittleEndian.Uint32(data[offset:offset+4]) > maxOffset {
			clear(data[offset : offset+4])
		}
	}
	k.pthreadStackHint = hint
	k.pthreadRegistration = &pthreadRegistration{ThreadStart: r.GPR[0], WorkqueueStart: r.GPR[1], PthreadSize: uint32(size), Data: data}
	if pointer != 0 {
		binary.LittleEndian.PutUint64(data[:8], k.pthreadPolicy.Version)
		binary.LittleEndian.PutUint64(data[16:24], k.pthreadPolicy.MainQoS)
		binary.LittleEndian.PutUint64(data[36:44], hint)
		binary.LittleEndian.PutUint32(data[44:48], k.pthreadPolicy.MutexPolicy)
		if m.Write(pointer, data[:count]) != nil {
			return finish(0, 22)
		}
	}
	return finish(uint64(k.pthreadPolicy.Features), 0)
}
