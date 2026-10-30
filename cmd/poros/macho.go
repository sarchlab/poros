package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sarchlab/poros/pkg/darwinuser"
	"github.com/sarchlab/poros/pkg/loader"
)

const maxMachORuntimeProfileBytes = 64 << 10

// runMachO reports actual runtime progress. Incomplete startup is a failing
// execution, never a benchmark result or a prediction of complete user time.
func runMachO(args []string, out, diagnostics io.Writer) int {
	fs := flag.NewFlagSet("macho", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	cachePath := fs.String("shared-cache", "", "explicit read-only dyld shared-cache root file")
	entropySeed := fs.Uint64("entropy-seed", 0, "deterministic guest kernel entropy seed (not cryptographic randomness)")
	startupPath := fs.String("startup-profile", "", "explicit per-executable native kernel launch metadata JSON")
	profilePath := fs.String("runtime-profile", "", "explicit JSON native commpage capability profile")
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
	options := darwinuser.Options{EntropySeed: *entropySeed, Args: append([]string{*argv0}, fs.Args()[1:]...), Env: []string(env), ExecName: *execName}
	if *profilePath != "" {
		var profile darwinuser.RuntimeProfile
		if err := readMachOProfile(*profilePath, maxMachORuntimeProfileBytes, &profile); err != nil {
			fmt.Fprintln(diagnostics, err)
			return 2
		}
		options.RuntimeProfile = &profile
	}
	if *startupPath != "" {
		var profile darwinuser.StartupProfile
		if err := readMachOProfile(*startupPath, 32768, &profile); err != nil {
			fmt.Fprintln(diagnostics, err)
			return 2
		}
		options.StartupProfile = &profile
	}
	if *cachePath != "" {
		cache, err := loader.OpenSharedCache(*cachePath)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 2
		}
		defer cache.Close()
		options.SharedCache = cache
	}
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
	var cacheFiles []loader.CacheFileIdentity
	if options.SharedCache != nil {
		cacheFiles = options.SharedCache.Files
	}
	report := struct {
		CacheFiles     []loader.CacheFileIdentity `json:"shared_cache_files,omitempty"`
		EntropySeed    uint64                     `json:"entropy_seed"`
		Schema         int                        `json:"schema"`
		Binary         darwinuser.Identity        `json:"binary"`
		Dyld           darwinuser.Identity        `json:"dyld"`
		Args           []string                   `json:"args"`
		Env            []string                   `json:"env"`
		ExecName       string                     `json:"executable_path"`
		Policy         string                     `json:"policy"`
		RuntimeProfile *darwinuser.RuntimeProfile `json:"runtime_profile,omitempty"`
		StartupProfile *darwinuser.StartupProfile `json:"startup_profile,omitempty"`
		darwinuser.Result
	}{cacheFiles, *entropySeed, 1, p.Binary, p.Dyld, options.Args, append([]string{}, options.Env...), options.ExecName, "actual dyld entry; functional clock is one native counter tick per retired instruction; runtime bring-up incomplete; no full-program timing prediction", options.RuntimeProfile, options.StartupProfile, result}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	return 1
}

// Decode bounded explicit inputs completely; truncating a longer file to an
// apparently valid prefix would hide malformed or mismatched provenance.
func readMachOProfile(path string, limit int64, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return err
	}
	if int64(len(b)) > limit {
		return fmt.Errorf("Mach-O profile exceeds %d bytes", limit)
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return fmt.Errorf("Mach-O profile contains trailing data")
	}
	return nil
}
