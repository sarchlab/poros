package main

import (
	"bytes"
	"encoding/json"
	"github.com/sarchlab/poros/pkg/elfsim"
	"github.com/sarchlab/poros/pkg/target"
	"strings"
	"testing"
)

func TestELFCommand(t *testing.T) {
	var out, diag bytes.Buffer
	code := runELF([]string{"--argv0", "fixture", "../../tests/elf/hello.elf"}, &out, &diag)
	if code != 0 || diag.Len() != 0 {
		t.Fatalf("CLI failed: %d %s", code, diag.String())
	}
	var result struct {
		Args         []string
		Termination  string
		Instructions uint64
		Stdout       []byte `json:"stdout_base64"`
		Stderr       []byte `json:"stderr_base64"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Termination != "exit" || result.Instructions != 51 || string(result.Stdout) != "hello, ELF\n" || string(result.Stderr) != "stderr\n" || len(result.Args) != 1 || result.Args[0] != "fixture" {
		t.Fatalf("unexpected CLI result: %+v", result)
	}
	for _, args := range [][]string{{}, {"missing.elf"}, {"--max-output", "-1", "../../tests/elf/hello.elf"}} {
		out.Reset()
		diag.Reset()
		if runELF(args, &out, &diag) != 2 || out.Len() != 0 || diag.Len() == 0 {
			t.Fatalf("invalid input accepted: %v", args)
		}
	}
	out.Reset()
	diag.Reset()
	if runELF([]string{"../../tests/elf/exit-status.elf"}, &out, &diag) != 1 {
		t.Fatal("nonzero guest exit reported as CLI success")
	}
}

func TestELFTimingModesAndOptions(t *testing.T) {
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		t.Run(mode, func(t *testing.T) {
			var out, diag bytes.Buffer
			status := runELF([]string{"--mode", mode, "--frequency-hz", "1000000000", "../../tests/elf/hello.elf"}, &out, &diag)
			if status != 0 || diag.Len() != 0 {
				t.Fatalf("mode failed: %d %s", status, diag.String())
			}
			var r struct {
				Instructions uint64
				Stdout       []byte             `json:"stdout_base64"`
				Timing       *elfsim.Statistics `json:"timing"`
			}
			if err := json.Unmarshal(out.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			if r.Instructions != 51 || string(r.Stdout) != "hello, ELF\n" {
				t.Fatalf("wrong functional result: %+v", r)
			}
			if mode == "functional" {
				if r.Timing != nil {
					t.Fatal("functional mode reported predictions")
				}
				return
			}
			if r.Timing == nil || r.Timing.Model != mode || r.Timing.AssumedHz != 1e9 || r.Timing.Retired != 51 || r.Timing.Cycles == 0 || r.Timing.PredictedUserSeconds != float64(r.Timing.Cycles)/1e9 {
				t.Fatalf("wrong timing output: %+v", r.Timing)
			}
			if r.Timing.BranchMetricsAvailable != (mode == "ooo") {
				t.Fatal("wrong branch metrics availability")
			}
		})
	}
	for _, options := range [][]string{{"--mode", "invalid"}, {"--mode", "ooo", "--frequency-hz", "-1"}, {"--mode", "inorder", "--frequency-hz", "NaN"}, {"--mode", "ooo", "--frequency-hz", "+Inf"}, {"--mode", "ooo", "--frequency-hz", "0"}, {"--mode", "inorder", "--frequency-hz", "1000000.5"}, {"--mode", "ooo", "--frequency-hz", "999999"}} {
		var out, diag bytes.Buffer
		if runELF(append(options, "../../tests/elf/hello.elf"), &out, &diag) != 2 || out.Len() != 0 || diag.Len() == 0 {
			t.Fatalf("invalid timing options accepted: %v", options)
		}
	}
}

// Every target is a real command-line choice in every mode; the default command
// line remains M2 byte for byte, and an unknown name fails without output.
func TestELFTargetCLI(t *testing.T) {
	run := func(args ...string) (int, string, string) {
		var out, diag bytes.Buffer
		code := runELF(append(args, "../../tests/usertime/mvt.elf", "24"), &out, &diag)
		return code, out.String(), diag.String()
	}
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		code, legacy, diag := run("--mode", mode)
		if code != 0 || diag != "" {
			t.Fatalf("%s: %d %s", mode, code, diag)
		}
		for _, device := range target.All() {
			code, out, diag := run("--mode", mode, "--target", device.Name)
			if code != 0 || diag != "" {
				t.Fatalf("%s/%s: %d %s", mode, device.Name, code, diag)
			}
			var r struct {
				Instructions uint64
				Timing       *elfsim.Statistics `json:"timing"`
			}
			if err := json.Unmarshal([]byte(out), &r); err != nil {
				t.Fatal(err)
			}
			if device.Name == target.Default && out != legacy {
				t.Fatalf("%s: explicit default target changed the output", mode)
			}
			if mode == "functional" {
				if r.Timing != nil || out != legacy {
					t.Fatalf("functional result depends on target %s", device.Name)
				}
				continue
			}
			if r.Timing == nil || r.Timing.Target != device.Identity || r.Timing.Cycles == 0 || r.Timing.Retired != r.Instructions {
				t.Fatalf("%s/%s: timing %+v", mode, device.Name, r.Timing)
			}
		}
		for _, name := range []string{"m3", "M2", ""} {
			code, out, diag := run("--mode", mode, "--target", name)
			if code != 2 || out != "" || !strings.Contains(diag, "unknown target") || !strings.Contains(diag, "m5-max") {
				t.Fatalf("%s: target %q gave %d %q %q", mode, name, code, out, diag)
			}
		}
	}
	// The target supplies the clock unless one is given; an explicit clock
	// still overrides it and an explicit zero is still rejected.
	code, out, diag := run("--mode", "ooo", "--target", "m1", "--frequency-hz", "2000000000")
	var r struct {
		Timing elfsim.Statistics `json:"timing"`
	}
	if err := json.Unmarshal([]byte(out), &r); code != 0 || diag != "" || err != nil || r.Timing.AssumedHz != 2e9 || r.Timing.Target.Name != "m1" {
		t.Fatalf("frequency override: %d %s %+v", code, diag, r.Timing)
	}
	if code, out, _ := run("--mode", "ooo", "--target", "m1", "--frequency-hz", "0"); code != 2 || out != "" {
		t.Fatal("explicit zero frequency accepted")
	}
}

func TestELFImageBudgetCLI(t *testing.T) {
	for _, limit := range []string{"0", "-1", "4097", "8192", "536875008", "18446744073709551616"} {
		var out, diag bytes.Buffer
		if runELF([]string{"--max-image-bytes", limit, "../../tests/elf/hello.elf"}, &out, &diag) != 2 || out.Len() != 0 {
			t.Fatalf("invalid/insufficient image budget accepted: %s", limit)
		}
	}
	for limit, expected := range map[string]uint64{"268435456": 268435456, "536870912": 536870912} {
		var out, diag bytes.Buffer
		if runELF([]string{"--max-image-bytes", limit, "../../tests/elf/hello.elf"}, &out, &diag) != 0 {
			t.Fatal(diag.String())
		}
		var result struct {
			MaxImageBytes uint64 `json:"max_image_bytes"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.MaxImageBytes != expected {
			t.Fatal("image budget provenance missing")
		}
	}
}
