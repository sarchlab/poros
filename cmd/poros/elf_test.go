package main

import (
	"bytes"
	"encoding/json"
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
