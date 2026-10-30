package darwinuser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// matchKernelString recognizes only the supported service/operation, without
// searching any host namespace. Other strings are explicit unsupported calls.
func matchKernelString(m *usermem.AddressSpace, address uint64, text string) (bool, error) {
	for i, c := range append([]byte(text), 0) {
		var got [1]byte
		if address > ^uint64(0)-uint64(i) {
			return false, fmt.Errorf("guest string address overflow")
		}
		if e := m.Read(address+uint64(i), got[:]); e != nil {
			return false, e
		}
		if got[0] != c {
			return false, nil
		}
	}
	return true, nil
}

// macSyscall models the observed read-only Sandbox syscall permission query.
// Decisions come from this executable's suspended native launch, not from a
// universal allow rule or a host request made by guest code. Other operations
// and nonzero decisions await their own native contracts.
func (k *kernelState) macSyscall(r *emu.Registers, m *usermem.AddressSpace) bool {
	if len(k.sandboxSyscalls) == 0 || r.GPR[1] != 2 {
		return false
	}
	finish := func(value uint64, carry bool) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = carry; return true }
	match, e := matchKernelString(m, r.GPR[0], "Sandbox")
	if e != nil {
		return finish(14, true)
	}
	if !match {
		return false
	}
	var request [160]byte
	if m.Read(r.GPR[2], request[:]) != nil {
		return finish(14, true)
	}
	word := func(off int) uint64 { return binary.LittleEndian.Uint64(request[off:]) }
	match, e = matchKernelString(m, word(16), "syscall-unix")
	if e != nil {
		return finish(14, true)
	}
	if !match {
		return false
	}
	if word(8) != 100 || word(24) != 0x41 || word(40) != 1 || !bytes.Equal(request[48:], make([]byte, 112)) {
		return false
	}
	for _, rule := range k.sandboxSyscalls {
		if word(32) != uint64(rule.Number) {
			continue
		}
		if rule.Decision != 0 {
			return false
		}
		output := word(0)
		if m.Check(output, 8, usermem.Write, "sandbox query copyout") != nil && m.Check(output, 1, usermem.Write, "sandbox query copyout") == nil {
			return false
		}
		var value [8]byte
		binary.LittleEndian.PutUint64(value[:], rule.Decision)
		if m.Write(output, value[:]) != nil {
			return finish(14, true)
		}
		return finish(0, false)
	}
	return false
}
