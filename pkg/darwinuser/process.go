// Package darwinuser starts ordinary Mach-O programs at the real dyld entry.
// Runtime bring-up is incremental: missing ISA or OS interfaces stop explicitly.
package darwinuser

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
)

const (
	DyldBase         uint64 = 0x700000000
	StackTop         uint64 = 0x800000000
	DefaultStackSize uint64 = 1 << 20
	MaxStackSize     uint64 = 8 << 20
	MaxArgumentBytes        = 128 << 10
	MaxArgumentCount        = 4096
)

type Options struct {
	ClockAnchor *ClockAnchor
	Quiescence  QuiescencePolicy
	// SharedCache is borrowed and must remain open until all executions finish.
	SharedCache    *loader.SharedCache
	EntropySeed    uint64
	Args, Env      []string
	ExecName       string
	StackSize      uint64
	RuntimeProfile *RuntimeProfile
	StartupProfile *StartupProfile
}
type Identity struct {
	SHA256  string `json:"sha256"`
	UUID    string `json:"uuid"`
	Subtype uint32 `json:"subtype"`
}

// Process owns memory and registers. Runtime bytes are read-only host inputs;
// neither host environment nor host library calls are inherited by the guest.
type Process struct {
	Registers    emu.Registers
	Memory       *usermem.AddressSpace
	Binary, Dyld Identity
	MainEntry    uint64
	kernel       kernelState
	clock        *virtualClock
	ran          bool
}

// Load snapshots an executable and an explicitly supplied dyld file. It never
// follows a guest dylinker/dylib path onto the host filesystem automatically.
func Load(path, dyldPath string, o Options) (*Process, error) {
	image, e := loader.LoadMachO(path, loader.MachOExecutable)
	if e != nil {
		return nil, e
	}
	dyld, e := loader.LoadMachO(dyldPath, loader.MachODylinker)
	if e != nil {
		return nil, e
	}
	return start(image, dyld, o)
}
func Parse(binary, dyld []byte, o Options) (*Process, error) {
	image, e := loader.ParseMachO(binary, loader.MachOExecutable)
	if e != nil {
		return nil, e
	}
	runtime, e := loader.ParseMachO(dyld, loader.MachODylinker)
	if e != nil {
		return nil, e
	}
	return start(image, runtime, o)
}
func start(image, dyld *loader.MachOImage, o Options) (*Process, error) {
	if o.RuntimeProfile != nil && o.RuntimeProfile.BranchTarget == "non-enforced" && image.Subtype != 0 {
		return nil, fmt.Errorf("captured branch-target mode requires generic ARM64 main")
	}
	if o.RuntimeProfile != nil {
		if err := o.RuntimeProfile.validate(); err != nil {
			return nil, err
		}
	}
	if o.RuntimeProfile != nil && o.RuntimeProfile.NativeDyld != nil {
		n := o.RuntimeProfile.NativeDyld
		if n.UUID == "" || n.UUID != dyld.UUID || n.Subtype != dyld.Subtype {
			return nil, fmt.Errorf("supplied dyld does not match native runtime identity")
		}
	}
	if len(o.Args) == 0 {
		o.Args = []string{"program"}
	}
	if o.ExecName == "" {
		o.ExecName = o.Args[0]
	}
	if len(o.Args) > MaxArgumentCount || len(o.Env) > MaxArgumentCount {
		return nil, fmt.Errorf("Darwin startup: too many arguments/environment entries")
	}
	authMode := emu.PointerAuthDisabled
	if image.Subtype&0xffffff == 2 {
		authMode = emu.PointerAuthEnabled
	}
	apple := []string{"executable_path=" + o.ExecName}
	entropyState := o.EntropySeed
	if o.StartupProfile != nil {
		if err := o.StartupProfile.validate(image, dyld); err != nil {
			return nil, err
		}
		apple, entropyState = o.StartupProfile.apple(o.ExecName, o.EntropySeed)
	}
	total := 0
	for _, list := range [][]string{o.Args, o.Env, apple} {
		for _, s := range list {
			if strings.IndexByte(s, 0) >= 0 || len(s) >= MaxArgumentBytes-total {
				return nil, fmt.Errorf("Darwin startup: invalid/oversized string payload")
			}
			total += len(s) + 1
		}
	}
	size := o.StackSize
	if o.StartupProfile != nil {
		if size != 0 && size != o.StartupProfile.StackSize {
			return nil, fmt.Errorf("stack override differs from captured startup geometry")
		}
		size = o.StartupProfile.StackSize
	}
	if size == 0 {
		size = image.StackSize
	}
	if size == 0 {
		size = DefaultStackSize
	}
	if size < loader.MachOPageSize || size > MaxStackSize || size%loader.MachOPageSize != 0 {
		return nil, fmt.Errorf("Darwin startup: stack must be 16 KiB aligned, between 16 KiB and 8 MiB")
	}
	memory := &usermem.AddressSpace{}
	if err := o.Quiescence.mapCounter(memory); err != nil {
		return nil, err
	}
	reservation := size
	if o.StartupProfile != nil {
		reservation = o.StartupProfile.StackAllocationSize
	}
	reserved := []addressSpan{{StackTop - reservation, StackTop}, {commpageRegionStart, commpageRegionEnd}}
	mapImage := func(img *loader.MachOImage, slide uint64) error {
		for _, s := range img.Segments {
			if s.Size == 0 {
				continue
			}
			if slide > (1<<48)-s.Address-s.Size {
				return fmt.Errorf("Darwin startup: relocated segment overflows address space")
			}
			lo, hi := s.Address+slide, s.Address+slide+s.Size
			for _, r := range reserved {
				if lo < r.hi && r.lo < hi {
					return fmt.Errorf("Darwin startup: overlapping reserved/image/stack ranges")
				}
			}
			reserved = append(reserved, addressSpan{lo, hi})
			if s.Protection == 0 {
				continue
			}
			source := imageSegment{image: img, segment: s}
			if err := memory.MapReader(lo, s.Size, machPermissions(s.Protection), machPermissions(s.MaxProtection), source, vmPage); err != nil {
				return err
			}
		}
		return nil
	}
	if err := mapImage(image, 0); err != nil {
		return nil, err
	}
	// A fixed slide is explicit simulator startup state, not a host ASLR sample.
	if err := mapImage(dyld, DyldBase); err != nil {
		return nil, err
	}
	if o.SharedCache != nil {
		for _, r := range reserved {
			if o.SharedCache.Base < r.hi && r.lo < o.SharedCache.Base+o.SharedCache.Size {
				return nil, fmt.Errorf("shared cache region overlaps reserved process region")
			}
		}
		if !o.SharedCache.IsOpen() {
			return nil, fmt.Errorf("shared cache handle is closed")
		}
		if authMode != emu.PointerAuthDisabled {
			return nil, fmt.Errorf("shared cache with enabled pointer authentication is not implemented")
		}
		for _, m := range o.SharedCache.Mappings() {
			for _, r := range reserved {
				if m.Address < r.hi && r.lo < m.Address+m.Size {
					return nil, fmt.Errorf("shared cache overlaps reserved process region")
				}
			}
			if err := memory.MapReader(m.Address, m.Size, machPermissions(m.Protection), machPermissions(m.MaxProtection), m, 16384); err != nil {
				return nil, err
			}
		}
	}
	if o.SharedCache != nil && o.SharedCache.DynamicSize != 0 {
		lo := o.SharedCache.Base + o.SharedCache.DynamicOffset
		hi := lo + o.SharedCache.DynamicSize
		for _, r := range reserved {
			if lo < r.hi && r.lo < hi {
				return nil, fmt.Errorf("dynamic cache overlaps reserved process region")
			}
		}
		if o.RuntimeProfile == nil || o.RuntimeProfile.CacheDynamic == nil {
			return nil, fmt.Errorf("shared cache requires captured native dynamic-region fields")
		}
		reserved = append(reserved, addressSpan{lo, hi})
		if err := o.RuntimeProfile.CacheDynamic.mapRegion(memory, o.SharedCache); err != nil {
			return nil, err
		}
	}
	if o.RuntimeProfile != nil {
		if err := o.RuntimeProfile.mapCommpage(memory); err != nil {
			return nil, err
		}
	}
	stack := make([]byte, size)
	sp := size
	base := StackTop - size
	putStrings := func(ss []string) ([]uint64, error) {
		ptrs := make([]uint64, len(ss))
		for i, s := range ss {
			n := uint64(len(s) + 1)
			if n > sp {
				return nil, fmt.Errorf("Darwin startup: stack overflow")
			}
			sp -= n
			copy(stack[sp:], s)
			ptrs[i] = base + sp
		}
		return ptrs, nil
	}
	args, e := putStrings(o.Args)
	if e != nil {
		return nil, e
	}
	env, e := putStrings(o.Env)
	if e != nil {
		return nil, e
	}
	ap, e := putStrings(apple)
	if e != nil {
		return nil, e
	}
	// dyldStartup.s kernel contract: mh, argc, argv[], NULL, env[], NULL,
	// apple[], NULL. This is not the LC_MAIN C calling convention.
	words := []uint64{image.HeaderAddress, uint64(len(args))}
	words = append(words, args...)
	words = append(words, 0)
	words = append(words, env...)
	words = append(words, 0)
	words = append(words, ap...)
	words = append(words, 0)
	n := uint64(len(words)) * 8
	if n > sp {
		return nil, fmt.Errorf("Darwin startup: stack overflow")
	}
	sp = (sp - n) &^ 15
	for i, w := range words {
		binary.LittleEndian.PutUint64(stack[sp+uint64(i)*8:], w)
	}
	if err := memory.Map(base, size, usermem.Read|usermem.Write, stack); err != nil {
		return nil, err
	}
	// XNU load_main calls thread_state_initialize, whose USER64 default clears
	// PSTATE.DIT. Availability comes from captured commpage capabilities, not
	// from the host CPU executing this simulation.
	dit := emu.DITState{}
	speculationBarrier := false
	counter := emu.CounterState{}
	if o.RuntimeProfile != nil {
		dit.Available = o.RuntimeProfile.CPUCapabilities&(1<<57) != 0
		speculationBarrier = o.RuntimeProfile.CPUCapabilities&(1<<45) != 0
		counter.Enabled = true
		counter.Frequency = 1_000_000_000 * uint64(o.RuntimeProfile.TimebaseDenom) / uint64(o.RuntimeProfile.TimebaseNumer)
	}
	clock, clockError := mapVirtualClock(memory, o.RuntimeProfile, o.ClockAnchor)
	if clockError != nil {
		return nil, clockError
	}
	if clock != nil {
		counter.Value = clock.anchor.InitialCounter
	}
	identity := func(i *loader.MachOImage) Identity { return Identity{i.SHA256, i.UUID, i.Subtype} }
	kernel := kernelState{entropyState: entropyState, reserved: append([]addressSpan(nil), reserved...), protected: []addressSpan{{commpageRegionStart, commpageRegionEnd}}}
	if o.StartupProfile != nil {
		kernel.executableCDHash = o.StartupProfile.CDHash
		if o.StartupProfile.ExecutableStat != nil {
			stat := *o.StartupProfile.ExecutableStat
			kernel.executableStat = &stat
			kernel.executableImage = image
			kernel.executableCanonical = o.StartupProfile.ExecutableFile.Path
			kernel.executablePaths = []string{o.ExecName, o.StartupProfile.ExecutableFile.Path}
		}
		if len(o.StartupProfile.StandardIO) != 0 {
			kernel.standardIO = true
			kernel.descriptors = make(map[uint32]guestDescriptor)
			for _, entry := range o.StartupProfile.StandardIO {
				file := &guestOpenFile{nullDevice: entry.FD == 0, initialFD: entry.FD, stat: entry.Stat, flags: entry.Flags}
				kernel.descriptors[entry.FD] = guestDescriptor{flags: entry.Flags, file: file}
				if entry.FD > 0 {
					kernel.outputFiles[entry.FD-1] = file
				}
			}
		}
		kernel.files = []FileIdentity{o.StartupProfile.ExecutableFile, o.StartupProfile.DyldFile}
		kernel.sandboxSyscalls = append([]SandboxSyscallPermission(nil), o.StartupProfile.SandboxSyscalls...)
		if o.StartupProfile.AMFI != nil {
			policy := *o.StartupProfile.AMFI
			kernel.amfi = &policy
		}
		if o.StartupProfile.DTrace != nil {
			policy := *o.StartupProfile.DTrace
			kernel.dtrace = &policy
		}
		kernel.threadSendRefs = 1 // XNU installs one pinned main-thread send reference.
	}
	if o.SharedCache != nil && o.SharedCache.DynamicSize != 0 {
		file, present, err := o.RuntimeProfile.CacheDynamic.fileIdentity()
		if err != nil {
			return nil, err
		}
		if present {
			for _, existing := range kernel.files {
				if uint32(existing.FSID) == uint32(file.FSID) && existing.ObjectID == file.ObjectID {
					return nil, fmt.Errorf("conflicting cache/startup file identity")
				}
			}
			kernel.files = append(kernel.files, file)
		}
	}
	if dyld.AllImageInfoAddress != 0 || dyld.AllImageInfoSize != 0 {
		kernel.dyldInfoAddress = dyld.AllImageInfoAddress + DyldBase
		kernel.dyldInfoSize = dyld.AllImageInfoSize
	}

	if o.SharedCache != nil {
		kernel.sharedCacheBase = o.SharedCache.Base
		kernel.reserved = append(kernel.reserved, addressSpan{o.SharedCache.Base, o.SharedCache.Base + o.SharedCache.Size})
		kernel.protected = append(kernel.protected, addressSpan{o.SharedCache.Base, o.SharedCache.Base + o.SharedCache.Size})
		if o.SharedCache.DynamicSize != 0 {
			start := o.SharedCache.Base + o.SharedCache.DynamicOffset
			kernel.protected = append(kernel.protected, addressSpan{start, start + o.SharedCache.DynamicSize})
		}
	}
	exclusive := emu.ExclusiveState{}
	if o.RuntimeProfile != nil && o.RuntimeProfile.ExclusiveMonitor == "m2-local-64-byte" {
		exclusive.Policy = emu.ExclusiveM2Local64
	}
	strip := emu.PointerStripUnspecified
	if o.RuntimeProfile != nil && o.RuntimeProfile.PointerStrip == "darwin-47-bit" {
		strip = emu.PointerStripDarwin47Bit
	}
	branchTarget := emu.BranchTargetUnspecified
	if o.RuntimeProfile != nil && o.RuntimeProfile.BranchTarget == "non-enforced" {
		branchTarget = emu.BranchTargetNonEnforced
	}
	alignment := emu.AtomicNaturalAlignment
	if o.RuntimeProfile != nil && o.RuntimeProfile.AtomicAlignment == "within-16-byte" {
		alignment = emu.AtomicWithin16Bytes
	}
	if o.RuntimeProfile != nil {
		kernel.sharedMemory = copySharedMemory(o.RuntimeProfile.ReadOnlySharedMemory)
		kernel.statENOENTPaths = append([]string(nil), o.RuntimeProfile.StatENOENTPaths...)
		kernel.accessENOENTPaths = append([]string(nil), o.RuntimeProfile.AccessENOENTPaths...)
		for _, name := range append(append([]string(nil), kernel.statENOENTPaths...), kernel.accessENOENTPaths...) {
			if name == o.ExecName || name == image.Dylinker {
				return nil, fmt.Errorf("captured ENOENT conflicts with process image path")
			}
			for _, file := range kernel.files {
				if name == file.Path {
					return nil, fmt.Errorf("captured ENOENT conflicts with captured file identity")
				}
			}
		}
		kernel.directories = append([]RuntimeDirectory(nil), o.RuntimeProfile.Directories...)
		for i := range kernel.directories {
			if kernel.directories[i].Stat != nil {
				stat := *kernel.directories[i].Stat
				kernel.directories[i].Stat = &stat
			}
		}
		for _, name := range o.RuntimeProfile.SysctlNames {
			entry := SysctlName{Name: name.Name, MIB: append([]int32(nil), name.MIB...)}
			if name.Int32 != nil {
				value := *name.Int32
				entry.Int32 = &value
			}
			if name.String != nil {
				value := *name.String
				entry.String = &value
			}
			kernel.sysctlNames = append(kernel.sysctlNames, entry)
		}
	}
	if kernel.dtrace != nil {
		if o.ExecName == dtraceHelperPath || image.Dylinker == dtraceHelperPath {
			return nil, fmt.Errorf("DTrace helper conflicts with process image")
		}
		for _, file := range kernel.files {
			if file.Path == dtraceHelperPath {
				return nil, fmt.Errorf("DTrace helper conflicts with captured file")
			}
		}
		for _, node := range kernel.directories {
			if node.Path == dtraceHelperPath || node.CanonicalPath == dtraceHelperPath {
				return nil, fmt.Errorf("DTrace helper conflicts with directory")
			}
		}
		for _, name := range append(append([]string(nil), kernel.statENOENTPaths...), kernel.accessENOENTPaths...) {
			if name == dtraceHelperPath {
				return nil, fmt.Errorf("DTrace helper conflicts with captured absence")
			}
		}
	}
	if o.RuntimeProfile != nil && o.RuntimeProfile.CSRConfig != nil {
		config := *o.RuntimeProfile.CSRConfig
		kernel.csrConfig = &config
	}
	if o.RuntimeProfile != nil && o.RuntimeProfile.HostPriorities != nil {
		kernel.hostPriorities = new([8]int32)
		copy(kernel.hostPriorities[:], o.RuntimeProfile.HostPriorities)
	}
	if o.StartupProfile != nil && o.StartupProfile.Pthread != nil {
		policy := *o.StartupProfile.Pthread
		if o.StartupProfile.AMFI == nil || policy.CodeSigningStatus != o.StartupProfile.AMFI.CodeSigningStatus || !policy.valid() {
			return nil, fmt.Errorf("pthread registration policy is invalid or unbound to executable signing state")
		}
		kernel.pthreadPolicy = &policy
	}
	threadProtection := emu.AppleThreadProtectionState{}
	if o.StartupProfile != nil && o.StartupProfile.ThreadProtectionRegister != nil {
		if o.RuntimeProfile == nil || o.RuntimeProfile.ThreadProtectionByte == nil || *o.RuntimeProfile.ThreadProtectionByte != 3 {
			return nil, fmt.Errorf("supplied Apple thread-protection register requires captured capability 3")
		}
		if o.StartupProfile.SecurityConfig != 0 {
			return nil, fmt.Errorf("supplied Apple thread-protection initial state requires non-TPRO process security policy")
		}
		threadProtection = emu.AppleThreadProtectionState{Enabled: true, Value: *o.StartupProfile.ThreadProtectionRegister}
	}
	return &Process{clock: clock, kernel: kernel, Registers: emu.Registers{DIT: dit, SpeculationBarrier: speculationBarrier, AppleThreadProtection: threadProtection, BranchTarget: branchTarget, Exclusive: exclusive, PointerStrip: strip, SPAlignmentCheck: true, AtomicAlignment: alignment, PC: dyld.ThreadEntry + DyldBase, SP: base + sp, PointerAuth: authMode, Counter: counter}, Memory: memory, Binary: identity(image), Dyld: identity(dyld), MainEntry: image.MainEntry}, nil
}
