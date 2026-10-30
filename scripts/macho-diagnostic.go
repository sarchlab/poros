//go:build ignore

// Standalone failure-only diagnostic harness. The collector invokes this owned
// source explicitly with go run; it is not part of the production CLI/API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sarchlab/poros/pkg/darwinuser"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"golang.org/x/arch/arm64/arm64asm"
	"io"
	"os"
	"time"
)

type configuration struct {
	Binary, Dyld, Cache, ExecName string
	Args                          []string
	Runtime                       *darwinuser.RuntimeProfile
	Startup                       *darwinuser.StartupProfile
	MaxInstructions               uint64
	TimeoutSeconds                int
}
type call struct {
	PC, Retired uint64
	Number      int32
	Arguments   [6]uint64
	Returns     [2]uint64
	Carry       bool
	Error       string `json:",omitempty"`
}
type instruction struct {
	PC       uint64
	Encoding uint32
	Text     string
}

func main() {
	if len(os.Args) != 2 {
		panic("expected owned configuration path")
	}
	f, e := os.Open(os.Args[1])
	if e != nil {
		panic(e)
	}
	var c configuration
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	e = d.Decode(&c)
	f.Close()
	if e != nil {
		panic(e)
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 120 {
		panic("bounded timeout required")
	}
	var cache *loader.SharedCache
	if c.Cache != "" {
		cache, e = loader.OpenSharedCache(c.Cache)
		if e != nil {
			panic(e)
		}
		defer cache.Close()
	}
	p, e := darwinuser.Load(c.Binary, c.Dyld, darwinuser.Options{Args: c.Args, ExecName: c.ExecName, RuntimeProfile: c.Runtime, StartupProfile: c.Startup, SharedCache: cache})
	if e != nil {
		panic(e)
	}
	initial := p.Memory.Mappings()
	if len(initial) > 512 {
		panic("diagnostic mapping bound")
	}
	x, e := p.NewExecution(context.Background(), darwinuser.RunOptions{MaxInstructions: c.MaxInstructions, Timeout: time.Duration(c.TimeoutSeconds) * time.Second})
	if e != nil {
		panic(e)
	}
	defer x.Close()
	var calls []call
	var instructions []instruction
	var count, mainAt uint64
	mainReached := false
	for !x.Stopped() {
		if p.Registers.PC == p.MainEntry && !mainReached {
			mainReached = true
			mainAt = count
		}
		i, pe := x.Peek(p.Registers.PC)
		before := p.Registers
		if pe == nil {
			instructions = append(instructions, instruction{before.PC, i.Enc, emu.InstructionString(i)})
			if len(instructions) > 16 {
				instructions = instructions[1:]
			}
		}
		_, _, err := x.StepDetailed()
		if pe == nil && i.Op == arm64asm.SVC {
			entry := call{PC: before.PC, Retired: count, Number: int32(before.GPR[16]), Returns: [2]uint64{p.Registers.GPR[0], p.Registers.GPR[1]}, Carry: p.Registers.C}
			copy(entry.Arguments[:], before.GPR[:6])
			if err != nil {
				entry.Error = err.Error()
			}
			calls = append(calls, entry)
			if len(calls) > 16 {
				calls = calls[1:]
			}
		}
		if err == nil {
			count++
		}
	}
	current := p.Memory.Mappings()
	if len(current) > 512 {
		current = current[:512]
	}
	out := struct {
		Binary, Dyld                     darwinuser.Identity
		Result                           darwinuser.Result
		MainEntryReached                 bool
		MainEntryRetired                 uint64
		RecentSyscalls                   []call
		RecentInstructions               []instruction
		InitialMappings, CurrentMappings []usermem.Mapping
		GPR                              [31]uint64
		SP                               uint64
	}{p.Binary, p.Dyld, x.Result(), mainReached, mainAt, calls, instructions, initial, current, p.Registers.GPR, p.Registers.SP}
	if e := json.NewEncoder(os.Stdout).Encode(out); e != nil {
		panic(fmt.Sprintf("diagnostic output: %v", e))
	}
}
