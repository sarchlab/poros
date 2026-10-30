package darwinuser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
)

// SysctlName captures a selected native kernel name/OID association. It grants
// no host access. Optional values snapshot selected read-only system integers
// or strings; unlisted settings and absent values remain unsupported.
type SysctlName struct {
	Name   string  `json:"name"`
	MIB    []int32 `json:"mib"`
	Int32  *int32  `json:"int32,omitempty"`
	String *string `json:"string,omitempty"`
}

func validateSysctlNames(names []SysctlName) error {
	if len(names) > 32 {
		return fmt.Errorf("too many captured sysctl names")
	}
	seen := map[string]bool{}
	seenMIB := map[string]bool{}
	for _, entry := range names {
		if len(entry.Name) == 0 || len(entry.Name) > 1023 || strings.IndexByte(entry.Name, 0) >= 0 || strings.HasPrefix(entry.Name, ".") || strings.HasSuffix(entry.Name, ".") || strings.Contains(entry.Name, "..") || seen[entry.Name] || len(entry.MIB) == 0 || len(entry.MIB) > 24 {
			return fmt.Errorf("invalid/duplicate captured sysctl name")
		}
		for _, v := range entry.MIB {
			if v <= 0 {
				return fmt.Errorf("unsupported sysctl OID component")
			}
		}
		if entry.Int32 != nil && entry.Name != "security.mac.lockdown_mode_state" {
			return fmt.Errorf("unsupported captured integer sysctl")
		}
		key := fmt.Sprint(entry.MIB)
		if seenMIB[key] {
			return fmt.Errorf("duplicate captured sysctl OID")
		}
		seenMIB[key] = true
		if entry.String != nil && (entry.Name != "kern.bootargs" || entry.Int32 != nil || len(*entry.String) > 1023 || strings.IndexByte(*entry.String, 0) >= 0) {
			return fmt.Errorf("unsupported captured string sysctl")
		}
		seen[entry.Name] = true
	}
	return nil
}

func (k *kernelState) sysctl(r *emu.Registers, m *usermem.AddressSpace) bool {
	if len(k.sysctlNames) == 0 || r.GPR[1] == 0 || r.GPR[1] > 24 {
		return false
	}
	finish := func(value uint64) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = value != 0; return true }
	// Invalid non-null oldlen pointers have no established bounded native
	// contract. Reject before any output instead of guessing an error/side effect.
	var length [8]byte
	if r.GPR[3] != 0 {
		if m.Read(r.GPR[3], length[:]) != nil || m.Check(r.GPR[3], 8, usermem.Write, "sysctl length") != nil {
			return false
		}
	}
	mib := make([]byte, int(r.GPR[1])*4)
	if m.Read(r.GPR[0], mib) != nil {
		return finish(14)
	}
	if len(mib) != 8 || binary.LittleEndian.Uint32(mib) != 0 || binary.LittleEndian.Uint32(mib[4:]) != 3 {
		return k.sysctlValue(r, m, mib, length)
	}
	if r.GPR[5] >= 1024 {
		return finish(63)
	}
	if r.GPR[5] == 0 {
		return finish(2)
	}
	input := make([]byte, int(r.GPR[5]))
	if m.Read(r.GPR[4], input) != nil {
		return finish(14)
	}
	if i := bytes.IndexByte(input, 0); i >= 0 {
		input = input[:i]
	}
	name := strings.TrimSuffix(string(input), ".")
	if name == "" {
		return finish(2)
	}
	var entry *SysctlName
	for i := range k.sysctlNames {
		if k.sysctlNames[i].Name == name {
			entry = &k.sysctlNames[i]
			break
		}
	}
	if entry == nil {
		return false
	} // unobserved namespace membership is not ENOENT evidence
	result := make([]byte, len(entry.MIB)*4)
	for i, v := range entry.MIB {
		binary.LittleEndian.PutUint32(result[i*4:], uint32(v))
	}
	size := uint64(len(result))
	if r.GPR[2] != 0 {
		if binary.LittleEndian.Uint64(length[:]) < size {
			if r.GPR[3] != 0 {
				clear(length[:])
				if m.Write(r.GPR[3], length[:]) != nil {
					return finish(14)
				}
			}
			return finish(12)
		}
		if m.Check(r.GPR[2], len(result), usermem.Write, "sysctl OID") != nil && m.Check(r.GPR[2], 1, usermem.Write, "sysctl OID") == nil {
			return false
		}
		if m.Write(r.GPR[2], result) != nil {
			return finish(14)
		}
	} else if r.GPR[3] == 0 {
		return false
	}
	if r.GPR[3] != 0 {
		binary.LittleEndian.PutUint64(length[:], size)
		if m.Write(r.GPR[3], length[:]) != nil {
			return finish(14)
		}
	}
	return finish(0)
}

// sysctlValue serves captured read-only system settings. It never forwards a
// guest query to the host. The lockdown value is a global XNU SYSCTL_INT, unlike
// the per-process Sandbox permission captured in StartupProfile.
func (k *kernelState) sysctlValue(r *emu.Registers, m *usermem.AddressSpace, mib []byte, length [8]byte) bool {
	var selected *SysctlName
	for _, entry := range k.sysctlNames {
		if (entry.Int32 == nil && entry.String == nil) || len(entry.MIB)*4 != len(mib) {
			continue
		}
		match := true
		for i, component := range entry.MIB {
			if binary.LittleEndian.Uint32(mib[i*4:]) != uint32(component) {
				match = false
				break
			}
		}
		if match {
			copy := entry
			selected = &copy
			break
		}
	}
	if selected == nil || r.GPR[4] != 0 {
		return false
	}
	finish := func(value uint64) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = value != 0; return true }
	var value []byte
	if selected.Int32 != nil {
		value = make([]byte, 4)
		binary.LittleEndian.PutUint32(value, uint32(*selected.Int32))
	} else {
		value = append([]byte(*selected.String), 0)
	}
	size := uint64(len(value))
	if r.GPR[2] != 0 {
		if binary.LittleEndian.Uint64(length[:]) < size {
			if r.GPR[3] != 0 {
				clear(length[:])
				if m.Write(r.GPR[3], length[:]) != nil {
					return finish(14)
				}
			}
			return finish(12)
		}
		if m.Check(r.GPR[2], len(value), usermem.Write, "sysctl value") != nil && m.Check(r.GPR[2], 1, usermem.Write, "sysctl value") == nil {
			return false
		}
		if m.Write(r.GPR[2], value) != nil {
			return finish(14)
		}
	}
	if r.GPR[3] != 0 {
		binary.LittleEndian.PutUint64(length[:], size)
		if m.Write(r.GPR[3], length[:]) != nil {
			return finish(14)
		}
	}
	return finish(0)
}
