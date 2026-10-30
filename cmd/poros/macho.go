package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/sarchlab/poros/pkg/darwinuser"
)

// runMachO reports actual runtime progress. Incomplete startup is a failing
// execution, never a benchmark result or a prediction of complete user time.
func runMachO(args []string, out, diagnostics io.Writer) int {
	fs := flag.NewFlagSet("macho", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	dyld := fs.String("dyld", "", "explicit local runtime dyld file (read-only)")
	steps := fs.Uint64("max-instructions", 1_000_000, "instruction budget")
	timeout := fs.Duration("timeout", 2*time.Second, "host execution timeout")
	argv0 := fs.String("argv0", "program", "guest argv[0]")
	execName := fs.String("exec-name", "", "guest executable_path; defaults to absolute input path")
	var env environmentFlags
	fs.Var(&env, "env", "guest KEY=VALUE (repeatable; no inherited environment)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || *dyld == "" {
		fmt.Fprintln(diagnostics, "usage: poros macho --dyld runtime/dyld [options] program [arguments...]")
		return 2
	}
	if *execName == "" {
		var err error
		*execName, err = filepath.Abs(fs.Arg(0))
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 2
		}
	}
	options := darwinuser.Options{Args: append([]string{*argv0}, fs.Args()[1:]...), Env: []string(env), ExecName: *execName}
	p, err := darwinuser.Load(fs.Arg(0), *dyld, options)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	result, err := p.Run(context.Background(), darwinuser.RunOptions{MaxInstructions: *steps, Timeout: *timeout})
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	report := struct {
		Schema   int                 `json:"schema"`
		Binary   darwinuser.Identity `json:"binary"`
		Dyld     darwinuser.Identity `json:"dyld"`
		Args     []string            `json:"args"`
		Env      []string            `json:"env"`
		ExecName string              `json:"executable_path"`
		Policy   string              `json:"policy"`
		darwinuser.Result
	}{1, p.Binary, p.Dyld, options.Args, append([]string{}, options.Env...), options.ExecName, "actual dyld entry; runtime bring-up incomplete; no full-program timing prediction", result}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	return 1
}
