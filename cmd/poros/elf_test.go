package main

import (
	"bytes"
	"encoding/json"
	"github.com/sarchlab/poros/pkg/elfsim"
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
	for _, options := range [][]string{{"--mode", "invalid"}, {"--mode", "ooo", "--frequency-hz", "-1"}, {"--mode", "inorder", "--frequency-hz", "NaN"}, {"--mode", "ooo", "--frequency-hz", "+Inf"}} {
		var out, diag bytes.Buffer
		if runELF(append(options, "../../tests/elf/hello.elf"), &out, &diag) != 2 || out.Len() != 0 || diag.Len() == 0 {
			t.Fatalf("invalid timing options accepted: %v", options)
		}
	}
}
