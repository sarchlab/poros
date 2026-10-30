package darwinuser

import (
	"encoding/binary"
	"fmt"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

type StatTime struct {
	Seconds     int64 `json:"seconds,string"`
	Nanoseconds int64 `json:"nanoseconds,string"`
}

// FileStat is the defined Darwin ARM64 stat64 payload for a captured file. Native capture verifies ABI padding and reserved fields are zero.
type FileStat struct {
	Device       uint32   `json:"device"`
	Mode         uint16   `json:"mode"`
	Links        uint16   `json:"links"`
	Inode        uint64   `json:"inode,string"`
	UID          uint32   `json:"uid"`
	GID          uint32   `json:"gid"`
	RDev         uint32   `json:"rdev"`
	Access       StatTime `json:"atime"`
	Modification StatTime `json:"mtime"`
	Change       StatTime `json:"ctime"`
	Birth        StatTime `json:"birthtime"`
	Size         int64    `json:"size,string"`
	Blocks       int64    `json:"blocks,string"`
	BlockSize    int32    `json:"block_size"`
	Flags        uint32   `json:"flags"`
	Generation   uint32   `json:"generation"`
}

func (s FileStat) validate() error {
	for _, stamp := range []StatTime{s.Access, s.Modification, s.Change, s.Birth} {
		if stamp.Nanoseconds < 0 || stamp.Nanoseconds >= 1_000_000_000 {
			return fmt.Errorf("invalid captured stat nanoseconds")
		}
	}
	return nil
}
func (s FileStat) bytes() [144]byte {
	var b [144]byte
	binary.LittleEndian.PutUint32(b[0:], s.Device)
	binary.LittleEndian.PutUint16(b[4:], s.Mode)
	binary.LittleEndian.PutUint16(b[6:], s.Links)
	binary.LittleEndian.PutUint64(b[8:], s.Inode)
	binary.LittleEndian.PutUint32(b[16:], s.UID)
	binary.LittleEndian.PutUint32(b[20:], s.GID)
	binary.LittleEndian.PutUint32(b[24:], s.RDev)
	for i, t := range []StatTime{s.Access, s.Modification, s.Change, s.Birth} {
		binary.LittleEndian.PutUint64(b[32+i*16:], uint64(t.Seconds))
		binary.LittleEndian.PutUint64(b[40+i*16:], uint64(t.Nanoseconds))
	}
	binary.LittleEndian.PutUint64(b[96:], uint64(s.Size))
	binary.LittleEndian.PutUint64(b[104:], uint64(s.Blocks))
	binary.LittleEndian.PutUint32(b[112:], uint32(s.BlockSize))
	binary.LittleEndian.PutUint32(b[116:], s.Flags)
	binary.LittleEndian.PutUint32(b[120:], s.Generation)
	return b
}
func (k *kernelState) statAt(r *emu.Registers, m *usermem.AddressSpace) bool {
	if uint32(r.GPR[3]) != 0 {
		return false
	}
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	name, errno, handled := k.directoryPath(m, r.GPR[1], uint32(r.GPR[0]), true)
	if !handled {
		return false
	}
	if errno != 0 {
		return finish(errno)
	}
	for _, entry := range k.directories {
		if name != entry.Path && name != entry.Path+"/" && name != entry.CanonicalPath && name != entry.CanonicalPath+"/" {
			continue
		}
		if entry.Stat == nil {
			return false
		}
		if m.Check(r.GPR[2], 144, usermem.Write, "stat64 copyout") != nil && m.Check(r.GPR[2], 1, usermem.Write, "stat64 copyout") == nil {
			return false
		}
		b := entry.Stat.bytes()
		if m.Write(r.GPR[2], b[:]) != nil {
			return finish(14)
		}
		return finish(0)
	}
	return false
}

// stat answers captured executable metadata and explicit stat error observations.
// Other host paths are not an implicit filesystem namespace.
func (k *kernelState) stat(r *emu.Registers, m *usermem.AddressSpace) bool {
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	name, errno, handled := k.directoryPath(m, r.GPR[0], 0, false)
	if !handled {
		return false
	}
	if errno != 0 {
		return finish(errno)
	}
	for _, absent := range k.statENOENTPaths {
		if name == absent {
			return finish(2)
		}
	}
	if k.executableStat == nil {
		return false
	}
	for _, known := range k.executablePaths {
		if name != known {
			continue
		}
		output := r.GPR[1]
		if m.Check(output, 144, usermem.Write, "stat64 copyout") != nil && m.Check(output, 1, usermem.Write, "stat64 copyout") == nil {
			return false
		}
		b := k.executableStat.bytes()
		if m.Write(output, b[:]) != nil {
			return finish(14)
		}
		return finish(0)
	}
	return false
}
