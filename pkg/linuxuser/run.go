package linuxuser

import (
	"context"
	"debug/elf"
	"errors"
	"fmt"
	"time"

	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

const MaxOutputBytes = 16 << 20

// RunOptions bounds host work separately from the deterministic instruction
// budget. Zero values select 1M instructions, 2 seconds and 1 MiB total output.
type RunOptions struct {
	MaxInstructions uint64
	Timeout         time.Duration
	MaxOutput       int
}

// Result captures bytes without text conversion (JSON encodes []byte as base64).
// ExitStatus is present only for a guest exit syscall. Instructions counts
// completed guest instructions, including successful/errno-returning syscalls.
// Faulting and unsupported instructions/syscalls are not counted.
type Result struct {
	Termination  string `json:"termination"`
	ExitStatus   *int   `json:"exit_status"`
	Instructions uint64 `json:"instructions"`
	PC           uint64 `json:"pc"`
	Stdout       []byte `json:"stdout_base64"`
	Stderr       []byte `json:"stderr_base64"`
	Diagnostic   string `json:"diagnostic,omitempty"`
}

// Execution owns one run and is a live resource for either functional or timing
// driving. Close releases its deadline; it never shares mutable state across runs.
type Execution struct {
	process *Process
	options RunOptions
	ctx     context.Context
	cancel  context.CancelFunc
	result  Result
	done    bool
	memory  trackedMemory
}

// NewExecution claims this process exactly once; all execution modes share it.
func (p *Process) NewExecution(ctx context.Context, o RunOptions) (*Execution, error) {
	r := Result{Stdout: []byte{}, Stderr: []byte{}}
	if p.ran {
		return nil, fmt.Errorf("process has already run")
	}
	if o.MaxInstructions == 0 {
		o.MaxInstructions = 1_000_000
	}
	if o.Timeout == 0 {
		o.Timeout = 2 * time.Second
	}
	if o.MaxOutput == 0 {
		o.MaxOutput = 1 << 20
	}
	if o.Timeout < 0 || o.MaxOutput < 0 || o.MaxOutput > MaxOutputBytes {
		return nil, fmt.Errorf("invalid execution limits")
	}
	p.ran = true
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)

	x := &Execution{process: p, options: o, ctx: ctx, cancel: cancel, result: r}
	x.memory.space = p.Memory
	return x, nil
}
func (x *Execution) Close()                          { x.cancel() }
func (x *Execution) Result() Result                  { return x.result }
func (x *Execution) CPURegisters() *emu.Registers    { return &x.process.Registers }
func (x *Execution) Stopped() bool                   { return x.done }
func (x *Execution) MemoryAccesses() []emu.MemAccess { return x.memory.accesses }
func (x *Execution) Peek(pc uint64) (arm64asm.Inst, error) {
	var b [4]byte
	if err := x.process.Memory.Fetch(pc, b[:]); err != nil {
		return arm64asm.Inst{}, err
	}
	return arm64asm.Decode(b[:])
}
func (x *Execution) StepDetailed() (arm64asm.Inst, bool, error) {
	pc := x.process.Registers.PC
	inst, _ := x.Peek(pc)
	before := x.result.Instructions
	x.memory.accesses = x.memory.accesses[:0]
	_, err := x.step()
	if err != nil {
		return inst, false, err
	}
	if x.result.Instructions == before {
		x.memory.accesses = x.memory.accesses[:0]
		return inst, false, fmt.Errorf("%s: %s", x.result.Termination, x.result.Diagnostic)
	}
	return inst, x.process.Registers.PC != pc+4, nil
}
func (x *Execution) step() (Result, error) {
	p, o := x.process, x.options
	r := &x.result
	ctx := x.ctx
	finish := func(reason, detail string) (Result, error) {
		x.done = true
		r.Termination = reason
		r.Diagnostic = detail
		r.PC = p.Registers.PC
		return *r, nil
	}
	if x.done {
		return *r, fmt.Errorf("execution already stopped")
	}
	if r.Instructions >= o.MaxInstructions {
		return finish("instruction_limit", fmt.Sprintf("completed %d instructions", o.MaxInstructions))
	}
	select {
	case <-ctx.Done():
		return finish("time_limit", ctx.Err().Error())
	default:
	}
	step, err := emu.StepChecked(&p.Registers, &x.memory)
	if err != nil {
		var fault *emu.StepError
		if errors.As(err, &fault) {
			return finish(fault.Kind, err.Error())
		}
		return finish("execution_fault", err.Error())
	}
	if step.Syscall {
		number := p.Registers.GPR[8]
		switch number {
		case 93, 94:
			code := int(p.Registers.GPR[0] & 255)
			r.ExitStatus = &code
			r.Instructions++
			p.Registers.PC += 4
			return finish("exit", "")
		case 64:
			fd := uint32(p.Registers.GPR[0])
			addr := p.Registers.GPR[1]
			count := min(p.Registers.GPR[2], uint64(0x7ffff000))
			if fd != 1 && fd != 2 {
				p.Registers.GPR[0] = errno(9)
			} else if err := p.Memory.check(addr, int(count), elf.PF_R, "read"); err != nil {
				p.Registers.GPR[0] = errno(14)
			} else {
				if count > uint64(o.MaxOutput-len(r.Stdout)-len(r.Stderr)) {
					return finish("output_limit", fmt.Sprintf("write would exceed %d output bytes", o.MaxOutput))
				}
				b := make([]byte, int(count))
				if err := p.Memory.Read(addr, b); err != nil {
					return finish("execution_fault", err.Error())
				}
				if fd == 1 {
					r.Stdout = append(r.Stdout, b...)
				} else {
					r.Stderr = append(r.Stderr, b...)
				}
				p.Registers.GPR[0] = count
			}
			p.Registers.PC += 4
		default:
			return finish("unsupported_syscall", fmt.Sprintf("syscall %d at PC=%#x is unsupported", number, p.Registers.PC))
		}
	}
	r.Instructions++
	return *r, nil
}

func (p *Process) Run(ctx context.Context, o RunOptions) (Result, error) {
	x, err := p.NewExecution(ctx, o)
	if err != nil {
		return Result{}, err
	}
	defer x.Close()
	for !x.Stopped() {
		_, _, _ = x.StepDetailed()
	}
	return x.Result(), nil
}

type trackedMemory struct {
	space    *AddressSpace
	accesses []emu.MemAccess
}

func (m *trackedMemory) Fetch(a uint64, b []byte) error { return m.space.Fetch(a, b) }
func (m *trackedMemory) Read(a uint64, b []byte) error {
	if err := m.space.Read(a, b); err != nil {
		return err
	}
	m.record(a, len(b), false)
	return nil
}
func (m *trackedMemory) Write(a uint64, b []byte) error {
	if err := m.space.Write(a, b); err != nil {
		return err
	}
	m.record(a, len(b), true)
	return nil
}
func (m *trackedMemory) record(a uint64, n int, w bool) {
	for n > 0 {
		// Split at natural word boundaries so an unaligned operation crossing
		// a cache line is represented on both sides.
		width := min(n, 8-int(a%8))
		m.accesses = append(m.accesses, emu.MemAccess{Addr: a, Size: width, IsWrite: w})
		a += uint64(width)
		n -= width
	}
}
func errno(n uint64) uint64 { return ^n + 1 }
