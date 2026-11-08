package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/sarchlab/poros/pkg/elfsim"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"github.com/sarchlab/poros/pkg/loader"
)

type environmentFlags []string

func (e *environmentFlags) String() string     { return fmt.Sprint([]string(*e)) }
func (e *environmentFlags) Set(s string) error { *e = append(*e, s); return nil }

// runELF emits one JSON result. Guest bytes are base64 fields, never mixed with
// host diagnostics. CLI exit 0 means guest exit 0; 1 means guest failure and 2
// means configuration/load failure. The full guest exit value stays in JSON.
func runELF(args []string, out io.Writer, diagnostics io.Writer) int {
	fs := flag.NewFlagSet("elf", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	imageLimit := fs.Uint64("max-image-bytes", loader.MaxELFImageBytes, "page-aligned ELF image budget, at most 512 MiB (default 256 MiB)")
	profilePath := fs.String("timing-profile", "", "complete lightweight OoO CPU/cache JSON profile for calibration")
	mode := fs.String("mode", "functional", "functional, inorder or ooo")
	hz := fs.Float64("frequency-hz", 3.49e9, "assumed integer CPU Hz (1 MHz to 1 THz), not measured")
	steps := fs.Uint64("max-instructions", 1_000_000, "instruction budget")
	timeout := fs.Duration("timeout", 2*time.Second, "host execution timeout")
	output := fs.Int("max-output", 1<<20, "combined stdout/stderr byte budget")
	argv0 := fs.String("argv0", "program", "guest argv[0] and AT_EXECFN")
	var env environmentFlags
	fs.Var(&env, "env", "guest KEY=VALUE (repeatable; no inherited environment)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *imageLimit == 0 {
		fmt.Fprintln(diagnostics, "image budget must be positive")
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(diagnostics, "usage: poros elf [options] file.elf [guest arguments...]")
		return 2
	}
	if *mode != "functional" && *hz == 0 {
		fmt.Fprintln(diagnostics, "timing frequency must be positive integer CPU Hz (1 MHz to 1 THz)")
		return 2
	}
	var profile *elfsim.TimingProfile
	if *profilePath != "" {
		if *mode != "ooo" {
			fmt.Fprintln(diagnostics, "timing profiles require --mode ooo")
			return 2
		}
		pf, err := os.Open(*profilePath)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 2
		}
		profile, err = elfsim.ReadTimingProfile(io.LimitReader(pf, 1<<20))
		pf.Close()
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 2
		}
	}
	path := fs.Arg(0)
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	b, err := io.ReadAll(io.LimitReader(f, loader.MaxELFFileBytes+1))
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	options := linuxuser.Options{MaxImageBytes: *imageLimit, Args: append([]string{*argv0}, fs.Args()[1:]...), Env: []string(env), ExecName: *argv0}
	p, err := linuxuser.Parse(b, options)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	var result linuxuser.Result
	var timingStats *elfsim.Statistics
	limits := linuxuser.RunOptions{MaxInstructions: *steps, Timeout: *timeout, MaxOutput: *output}
	if *mode == "functional" {
		result, err = p.Run(context.Background(), limits)
	} else {
		var timed elfsim.Result
		timed, err = elfsim.Run(context.Background(), p, elfsim.Options{Model: *mode, TimingProfile: profile, FrequencyHz: *hz, Limits: limits})
		result = timed.Result
		timingStats = &timed.Timing
	}
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	sum := sha256.Sum256(b)
	build := buildMetadata()
	report := struct {
		Schema        int                `json:"schema"`
		MaxImageBytes uint64             `json:"max_image_bytes"`
		BinarySHA256  string             `json:"binary_sha256"`
		Args          []string           `json:"args"`
		Env           []string           `json:"env"`
		Build         map[string]string  `json:"build"`
		Timing        *elfsim.Statistics `json:"timing,omitempty"`
		linuxuser.Result
	}{1, *imageLimit, hex.EncodeToString(sum[:]), options.Args, append([]string{}, options.Env...), build, timingStats, result}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2
	}
	if result.Termination == "exit" && result.ExitStatus != nil && *result.ExitStatus == 0 {
		return 0
	}
	return 1
}

func buildMetadata() map[string]string {
	build := map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		build["go_version"] = info.GoVersion
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision", "vcs.modified", "vcs.time", "GOOS", "GOARCH":
				build[setting.Key] = setting.Value
			}
		}
	}
	return build
}
