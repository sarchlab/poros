package darwinuser

import (
	"encoding/hex"
	"fmt"
	"github.com/sarchlab/poros/pkg/loader"
	"path"
	"strings"
)

// FileIdentity names an explicitly supplied guest file. Decimal strings retain
// all 64 bits when capture JSON passes through tools that use floating numbers.
// Path is the native canonical identity path, not permission to open a host file.
type FileIdentity struct {
	FSID     uint64 `json:"fsid,string"`
	ObjectID uint64 `json:"object_id,string"`
	Path     string `json:"path"`
}

// StartupProfile is specific to one executable and native boot. It captures
// nonsecret kernel launch metadata before dyld consumes the apple vector, plus
// separately captured native startup policy observations.
// Guest addresses, port names, and entropy remain explicitly virtual state.
type SandboxSyscallPermission struct {
	Number   uint32 `json:"number"`
	Decision uint64 `json:"decision,string"`
}

// AMFIPolicy snapshots one read-only dyld-policy query for this executable.
// It is not a global allow policy and never changes host enforcement.
type AMFIPolicy struct {
	CodeSigningStatus uint32 `json:"code_signing_status"`
	InputFlags        uint64 `json:"input_flags,string"`
	OutputFlags       uint64 `json:"output_flags,string"`
}

// DTracePolicy records the observed per-image lazy helper mode. No tracing
// consumer or host device is exposed by this profile.
type DTracePolicy struct {
	Mode              string `json:"mode"`
	CodeSigningStatus uint32 `json:"code_signing_status"`
}

type StartupProfile struct {
	Pthread *PthreadPolicy `json:"pthread_registration,omitempty"`
	// ThreadProtectionRegister is observed at the actual native TPRO startup
	// call entry/return. Earlier guest reads/writes remain audited explicitly.
	ThreadProtectionRegister *uint64                    `json:"thread_protection_register,omitempty,string"`
	DTrace                   *DTracePolicy              `json:"dtrace_helper,omitempty"`
	ExecutableStat           *FileStat                  `json:"executable_stat,omitempty"`
	StandardIO               []InitialDescriptor        `json:"standard_io,omitempty"`
	AMFI                     *AMFIPolicy                `json:"amfi_dyld_policy,omitempty"`
	SandboxSyscalls          []SandboxSyscallPermission `json:"sandbox_syscall_permissions,omitempty"`
	BinarySHA256             string                     `json:"binary_sha256"`
	DyldSHA256               string                     `json:"dyld_sha256"`
	ExecutableFile           FileIdentity               `json:"executable_file"`
	DyldFile                 FileIdentity               `json:"dyld_file"`
	CDHash                   string                     `json:"cdhash"`
	BootHash                 string                     `json:"boot_hash"`
	PFZ                      uint64                     `json:"pfz"`
	StackSize                uint64                     `json:"stack_size"`
	StackAllocationSize      uint64                     `json:"stack_allocation_size"`
	SecurityConfig           uint64                     `json:"security_config"`
	ARM64eABI                string                     `json:"arm64e_abi,omitempty"`
}

const maxStackReservation = 128 << 20

// XNU arm64 vm_map reserves [63 GiB,64 GiB) for commpage/PFZ mappings.
// Reserve the submap without inventing bytes or permissions in its holes.
const commpageRegionStart uint64 = 63 << 30
const commpageRegionEnd uint64 = 64 << 30

func nextEntropyWord(state *uint64) uint64 {
	*state += 0x9e3779b97f4a7c15
	v := *state
	v = (v ^ (v >> 30)) * 0xbf58476d1ce4e5b9
	v = (v ^ (v >> 27)) * 0x94d049bb133111eb
	return v ^ (v >> 31)
}

func (s *StartupProfile) validate(image, dyld *loader.MachOImage) error {
	if s.DTrace != nil && (s.DTrace.Mode != "lazy-untraced" || s.AMFI == nil || s.DTrace.CodeSigningStatus != s.AMFI.CodeSigningStatus) {
		return fmt.Errorf("DTrace helper policy is unsupported or does not match executable signing state")
	}
	if err := validateInitialDescriptors(s.StandardIO); err != nil {
		return err
	}
	if s.ExecutableStat != nil {
		stat := s.ExecutableStat
		if err := stat.validate(); err != nil {
			return err
		}
		if stat.Mode&0xf000 != 0x8000 || stat.Size < 0 || uint64(stat.Size) != image.FileSize || stat.Inode != s.ExecutableFile.ObjectID {
			return fmt.Errorf("executable stat does not match supplied image identity/size")
		}
	}
	if len(s.SandboxSyscalls) > 32 {
		return fmt.Errorf("too many captured sandbox syscall rules")
	}
	seen := map[uint32]bool{}
	for _, rule := range s.SandboxSyscalls {
		if seen[rule.Number] || rule.Decision != 0 {
			return fmt.Errorf("unsupported or duplicate sandbox syscall decision")
		}
		seen[rule.Number] = true
	}
	if s.BinarySHA256 != image.SHA256 || s.DyldSHA256 != dyld.SHA256 {
		return fmt.Errorf("startup profile input hash mismatch")
	}
	for _, f := range []FileIdentity{s.ExecutableFile, s.DyldFile} {
		if f.FSID == 0 || f.ObjectID == 0 || !path.IsAbs(f.Path) || path.Clean(f.Path) != f.Path || len(f.Path) >= 8192 || strings.IndexByte(f.Path, 0) >= 0 {
			return fmt.Errorf("invalid startup file identity/path")
		}
	}
	if uint32(s.ExecutableFile.FSID) == uint32(s.DyldFile.FSID) && s.ExecutableFile.ObjectID == s.DyldFile.ObjectID {
		return fmt.Errorf("duplicate startup file identities")
	}
	for _, h := range []string{s.CDHash, s.BootHash} {
		b, e := hex.DecodeString(h)
		if e != nil || len(b) != 20 || hex.EncodeToString(b) != h {
			return fmt.Errorf("invalid startup code/boot hash")
		}
	}
	if s.StackSize < loader.MachOPageSize || s.StackSize > MaxStackSize || s.StackSize%loader.MachOPageSize != 0 || s.StackAllocationSize < s.StackSize+loader.MachOPageSize || s.StackAllocationSize > maxStackReservation || s.StackAllocationSize%loader.MachOPageSize != 0 {
		return fmt.Errorf("unsupported startup stack geometry")
	}
	if s.PFZ < commpageRegionStart || s.PFZ%loader.MachOPageSize != 0 || s.PFZ >= commpageRegionEnd {
		return fmt.Errorf("invalid startup PFZ address")
	}
	if s.ARM64eABI != "" && s.ARM64eABI != "all" && s.ARM64eABI != "os" {
		return fmt.Errorf("unsupported startup arm64e ABI")
	}
	return nil
}

func (s *StartupProfile) apple(execName string, seed uint64) ([]string, uint64) {
	guard := nextEntropyWord(&seed) &^ uint64(0xff00)
	malloc0, malloc1 := nextEntropyWord(&seed), nextEntropyWord(&seed)
	munge := nextEntropyWord(&seed)
	file := func(f FileIdentity) string { return fmt.Sprintf("0x%x,0x%x", f.FSID, f.ObjectID) }
	values := []string{"executable_path=" + execName, fmt.Sprintf("pfz=0x%x", s.PFZ), fmt.Sprintf("stack_guard=0x%x", guard), fmt.Sprintf("malloc_entropy=0x%x,0x%x", malloc0, malloc1), fmt.Sprintf("ptr_munge=0x%x", munge), fmt.Sprintf("main_stack=0x%x,0x%x,0x%x,0x%x", StackTop, s.StackSize, StackTop-s.StackAllocationSize, s.StackAllocationSize), "executable_file=" + file(s.ExecutableFile), "dyld_file=" + file(s.DyldFile), "executable_cdhash=" + s.CDHash, "executable_boothash=" + s.BootHash}
	if s.ARM64eABI != "" {
		values = append(values, "arm64e_abi="+s.ARM64eABI)
	}
	values = append(values, "th_port=0x10b", fmt.Sprintf("security_config=0x%x", s.SecurityConfig))
	return values, seed
}
