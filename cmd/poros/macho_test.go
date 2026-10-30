package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sarchlab/poros/internal/machotest"
)

func TestMachOCLIReportsIncompleteRuntime(t *testing.T) {
	directory := t.TempDir()
	program, dyld := filepath.Join(directory, "program"), filepath.Join(directory, "dyld")
	if e := os.WriteFile(program, machotest.Image(2), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(dyld, machotest.Image(7), 0600); e != nil {
		t.Fatal(e)
	}
	var output, diagnostic bytes.Buffer
	code := runMachO([]string{"--dyld", dyld, program, "63"}, &output, &diagnostic)
	var r map[string]any
	if e := json.Unmarshal(output.Bytes(), &r); e != nil {
		t.Fatal(e)
	}
	if code != 1 || diagnostic.Len() != 0 || r["termination"] != "unsupported_instruction" || r["instructions"] != float64(6) {
		t.Fatalf("%d %s %s", code, output.String(), diagnostic.String())
	}
	if _, ok := r["timing"]; ok {
		t.Fatal("partial runtime emitted accuracy prediction")
	}
	if runMachO([]string{program}, &output, &diagnostic) != 2 {
		t.Fatal("implicit host runtime accepted")
	}
}
