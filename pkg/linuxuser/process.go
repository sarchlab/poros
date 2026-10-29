package linuxuser

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
)

const (
	DefaultStackSize uint64 = 1 << 20
	MaxStackSize     uint64 = 8 << 20
	StackTop         uint64 = 0x0000fffffffff000
	MaxArgumentBytes        = 128 << 10
	MaxArgumentCount        = 4096
)

// Linux uapi/linux/auxvec.h tags. Keep these local to startup construction;
// tests use independent numeric ABI values.
const (
	atNull       uint64 = 0
	atPHDR       uint64 = 3
	atPHENT      uint64 = 4
	atPHNUM      uint64 = 5
	atPageSize   uint64 = 6
	atBase       uint64 = 7
	atFlags      uint64 = 8
	atEntry      uint64 = 9
	atUID        uint64 = 11
	atEUID       uint64 = 12
	atGID        uint64 = 13
	atEGID       uint64 = 14
	atPlatform   uint64 = 15
	atHWCap      uint64 = 16
	atClockTicks uint64 = 17
	atSecure     uint64 = 23
	atRandom     uint64 = 25
	atHWCap2     uint64 = 26
	atExecName   uint64 = 31
)

// Options describes deterministic simulated startup inputs. Random supplies the
// sixteen AT_RANDOM bytes; the default all-zero seed is reproducible, not entropy.
// Empty Args becomes one empty argument, matching Linux execve normalization.
// No host arguments or environment leak into the simulated process.
type Options struct {
	Args, Env []string
	ExecName  string
	StackSize uint64
	Random    [16]byte
}

// Process owns the authoritative architectural state and checked address space.
// Run is one-shot; final registers and memory remain available for inspection.
type Process struct {
	Registers emu.Registers
	Memory    *AddressSpace
	ran       bool
}

// StartupError describes rejected startup configuration or image/stack collision.
type StartupError struct{ Detail string }

func (e *StartupError) Error() string { return "Linux startup: " + e.Detail }
func startupError(s string) error     { return &StartupError{s} }

// Load opens and validates an ELF, then constructs fresh initial process state.
// Errors return no partially initialized process. The path defaults ExecName.
func Load(path string, o Options) (*Process, error) {
	image, e := loader.LoadELF(path)
	if e != nil {
		return nil, e
	}
	if o.ExecName == "" {
		o.ExecName = path
	}
	return start(image, o)
}

// Parse is Load for caller-owned ELF bytes; ExecName defaults to "program".
func Parse(b []byte, o Options) (*Process, error) {
	image, e := loader.ParseELF(b)
	if e != nil {
		return nil, e
	}
	if o.ExecName == "" {
		o.ExecName = "program"
	}
	return start(image, o)
}

func start(image *loader.ELFImage, o Options) (*Process, error) {
	if len(o.Args) == 0 {
		o.Args = []string{""}
	}
	size := o.StackSize
	if size == 0 {
		size = DefaultStackSize
	}
	if size > MaxStackSize || size < loader.ELFPageSize || size%loader.ELFPageSize != 0 {
		return nil, startupError("stack size must be page-aligned, between 4 KiB and 8 MiB")
	}
	if len(o.Args) > MaxArgumentCount || len(o.Env) > MaxArgumentCount {
		return nil, startupError("too many arguments/environment entries")
	}
	// Bound payload before allocating or adding lengths; strings cannot contain NUL.
	payload := 0
	for _, ss := range [][]string{o.Args, o.Env, {o.ExecName}} {
		for _, s := range ss {
			if strings.IndexByte(s, 0) >= 0 {
				return nil, startupError("argument/environment/exec name contains NUL")
			}
			if len(s) >= MaxArgumentBytes-payload {
				return nil, startupError("argument/environment payload exceeds 128 KiB")
			}
			payload += len(s) + 1
		}
	}
	base := StackTop - size
	for _, s := range image.Segments {
		end := (s.Address + s.MemorySize + loader.ELFPageSize - 1) &^ (loader.ELFPageSize - 1)
		if end > base && s.Address < StackTop {
			return nil, startupError("ELF overlaps initial stack")
		}
	}
	stack := make([]byte, size)
	sp := size
	put := func(b []byte) (uint64, error) {
		if uint64(len(b)) > sp {
			return 0, startupError("initial stack overflow")
		}
		sp -= uint64(len(b))
		copy(stack[sp:], b)
		return base + sp, nil
	}
	putString := func(s string) (uint64, error) { return put(append([]byte(s), 0)) }
	execName, e := putString(o.ExecName)
	if e != nil {
		return nil, e
	}
	random, e := put(o.Random[:])
	if e != nil {
		return nil, e
	}
	platform, e := putString("aarch64")
	if e != nil {
		return nil, e
	}
	argPtrs := make([]uint64, len(o.Args))
	envPtrs := make([]uint64, len(o.Env))
	for i, s := range o.Args {
		argPtrs[i], e = putString(s)
		if e != nil {
			return nil, e
		}
	}
	for i, s := range o.Env {
		envPtrs[i], e = putString(s)
		if e != nil {
			return nil, e
		}
	}
	// Numeric tags follow Linux uapi/linux/auxvec.h. No vDSO or unsupported CPU
	// capabilities are advertised. Identity is a fixed nonprivileged simulation.
	aux := [][2]uint64{
		{atPHDR, image.ProgramHeaderAddress},
		{atPHENT, 56},
		{atPHNUM, uint64(image.ProgramHeaderCount)},
		{atPageSize, loader.ELFPageSize},
		{atBase, 0},
		{atFlags, 0},
		{atEntry, image.Entry},
		{atUID, 1000}, {atEUID, 1000}, {atGID, 1000}, {atEGID, 1000},
		{atPlatform, platform},
		{atHWCap, 0},
		{atClockTicks, 100},
		{atSecure, 0},
		{atRandom, random},
		{atHWCap2, 0},
		{atExecName, execName},
		{atNull, 0},
	}
	words := []uint64{uint64(len(argPtrs))}
	words = append(words, argPtrs...)
	words = append(words, 0)
	words = append(words, envPtrs...)
	words = append(words, 0)
	for _, a := range aux {
		words = append(words, a[:]...)
	}
	bytes := uint64(len(words)) * 8
	if bytes > sp {
		return nil, startupError("initial stack overflow")
	}
	sp = (sp - bytes) &^ 15
	for i, w := range words {
		binary.LittleEndian.PutUint64(stack[sp+uint64(i)*8:], w)
	}
	memory := &AddressSpace{}
	for _, s := range image.Segments {
		lo := s.Address &^ (loader.ELFPageSize - 1)
		hi := (s.Address + s.MemorySize + loader.ELFPageSize - 1) &^ (loader.ELFPageSize - 1)
		data := s.Data // fresh validated parser output, exclusively owned by this process
		memory.regions = append(memory.regions, mappedRegion{Mapping{lo, hi, s.Flags}, data})
	}
	memory.regions = append(memory.regions, mappedRegion{Mapping{base, StackTop, elf.PF_R | elf.PF_W}, stack})
	sort.Slice(memory.regions, func(i, j int) bool { return memory.regions[i].Start < memory.regions[j].Start })
	return &Process{Registers: emu.Registers{PC: image.Entry, SP: base + sp}, Memory: memory}, nil
}

// String returns a concise startup witness suitable for diagnostics.
func (p *Process) String() string {
	return fmt.Sprintf("PC=%#x SP=%#x mappings=%d", p.Registers.PC, p.Registers.SP, len(p.Memory.regions))
}
