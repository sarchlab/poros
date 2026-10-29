package linuxuser

import (
	"context"
	"debug/elf"
	"errors"
	"fmt"
	"time"

	"github.com/sarchlab/poros/pkg/emu"
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

// Run executes this process once. It never inherits host environment, file
// descriptors, signals or privileges. Guest failures are structured results;
// invalid host configuration and repeated calls return errors.
func (p *Process) Run(ctx context.Context, o RunOptions) (Result, error) {
	r := Result{Stdout: []byte{}, Stderr: []byte{}}
	if p.ran {
		return r, fmt.Errorf("process has already run")
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
		return r, fmt.Errorf("invalid execution limits")
	}
	p.ran = true
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	finish := func(reason, detail string) (Result, error) {
		r.Termination = reason
		r.Diagnostic = detail
		r.PC = p.Registers.PC
		return r, nil
	}
	for r.Instructions < o.MaxInstructions {
		select {
		case <-ctx.Done():
			return finish("time_limit", ctx.Err().Error())
		default:
		}
		step, err := emu.StepChecked(&p.Registers, p.Memory)
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
	}
	return finish("instruction_limit", fmt.Sprintf("completed %d instructions", o.MaxInstructions))
}
func errno(n uint64) uint64 { return ^n + 1 }
