package darwinuser

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

type RunOptions struct {
	MaxInstructions uint64
	Timeout         time.Duration
}

// Result distinguishes an unsupported runtime dependency from successful exit.
// No performance prediction is emitted for incomplete runtime bring-up.
type Result struct {
	Termination  string `json:"termination"`
	Instructions uint64 `json:"instructions"`
	PC           uint64 `json:"pc"`
	Encoding     uint32 `json:"instruction_encoding"`
	Diagnostic   string `json:"diagnostic,omitempty"`
}
type Execution struct {
	process *Process
	memory  emu.TrackedMemory
	limit   uint64
	ctx     context.Context
	cancel  context.CancelFunc
	result  Result
	done    bool
}

var _ emu.Execution = (*Execution)(nil)

func (p *Process) NewExecution(ctx context.Context, o RunOptions) (*Execution, error) {
	if p.ran {
		return nil, fmt.Errorf("process has already run")
	}
	if o.MaxInstructions == 0 {
		o.MaxInstructions = 1_000_000
	}
	if o.Timeout == 0 {
		o.Timeout = 2 * time.Second
	}
	if o.Timeout < 0 {
		return nil, fmt.Errorf("negative timeout")
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	p.ran = true
	return &Execution{process: p, memory: emu.TrackedMemory{Space: p.Memory}, limit: o.MaxInstructions, ctx: ctx, cancel: cancel}, nil
}
func (x *Execution) Close()                          { x.cancel() }
func (x *Execution) Result() Result                  { return x.result }
func (x *Execution) Stopped() bool                   { return x.done }
func (x *Execution) CPURegisters() *emu.Registers    { return &x.process.Registers }
func (x *Execution) MemoryAccesses() []emu.MemAccess { return x.memory.Accesses }
func (x *Execution) Peek(pc uint64) (arm64asm.Inst, error) {
	var b [4]byte
	if e := x.process.Memory.Fetch(pc, b[:]); e != nil {
		return arm64asm.Inst{}, e
	}
	return emu.DecodeChecked(b[:], x.process.Registers.PointerAuth)
}
func (x *Execution) StepDetailed() (arm64asm.Inst, bool, error) {
	x.memory.Accesses = x.memory.Accesses[:0]
	if x.done {
		return arm64asm.Inst{}, false, fmt.Errorf("execution already stopped")
	}
	finish := func(reason string, e error) (arm64asm.Inst, bool, error) {
		x.done = true
		x.result.Termination = reason
		x.result.PC = x.process.Registers.PC
		x.result.Diagnostic = e.Error()
		var bytes [4]byte
		if err := x.process.Memory.Fetch(x.result.PC, bytes[:]); err == nil {
			x.result.Encoding = binary.LittleEndian.Uint32(bytes[:])
		}
		return arm64asm.Inst{}, false, e
	}
	if x.result.Instructions >= x.limit {
		return finish("instruction_limit", fmt.Errorf("completed %d instructions", x.limit))
	}
	select {
	case <-x.ctx.Done():
		return finish("time_limit", x.ctx.Err())
	default:
	}
	step, e := emu.StepChecked(&x.process.Registers, &x.memory)
	if e != nil {
		kind := "execution_fault"
		var se *emu.StepError
		if errors.As(e, &se) {
			kind = se.Kind
		}
		x.memory.Accesses = x.memory.Accesses[:0]
		return finish(kind, e)
	}
	if step.Syscall {
		return finish("unsupported_syscall", fmt.Errorf("Darwin trap %d at PC=%#x is not implemented", int64(x.process.Registers.GPR[16]), x.process.Registers.PC))
	}
	x.result.Instructions++
	if x.process.Registers.Counter.Enabled {
		x.process.Registers.Counter.Value++
	}
	x.result.PC = x.process.Registers.PC
	return step.Instruction, step.Branched, nil
}
func (p *Process) Run(ctx context.Context, o RunOptions) (Result, error) {
	x, e := p.NewExecution(ctx, o)
	if e != nil {
		return Result{}, e
	}
	defer x.Close()
	for !x.Stopped() {
		if _, _, e = x.StepDetailed(); e != nil && !x.Stopped() {
			return x.Result(), e
		}
	}
	return x.Result(), nil
}
