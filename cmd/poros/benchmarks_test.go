package main

import (
	"bytes"
	"encoding/json"
	"github.com/sarchlab/poros/internal/benchmarks"
	"github.com/sarchlab/poros/pkg/elfsim"
	"strings"
	"testing"
)

func TestNamedELFInventoryAndRun(t *testing.T) {
	var out, diag bytes.Buffer
	if command([]string{"list", "--root", "../.."}, &out, &diag) != 0 {
		t.Fatal(diag.String())
	}
	for _, name := range []string{"gemm", "facesim", "mvt", "integer", "memory", "branch", "Total: 44"} {
		if !strings.Contains(out.String(), name) {
			t.Fatal("missing", name)
		}
	}
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		out.Reset()
		diag.Reset()
		if command([]string{"run", "--root", "../..", "--mode", mode, "gemm", "2", "3", "4"}, &out, &diag) != 0 {
			t.Fatal(diag.String())
		}
		var r struct {
			Termination string
			Exit        *int   `json:"exit_status"`
			Output      []byte `json:"stdout_base64"`
		}
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		if r.Termination != "exit" || r.Exit == nil || *r.Exit != 0 || len(r.Output) != 48 {
			t.Fatalf("incomplete named ELF output %+v", r)
		}
	}
	out.Reset()
	diag.Reset()
	if command([]string{"run", "--root", "../..", "--cores", "2", "--case", "3", "canneal"}, &out, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var multi elfsim.MultiCoreResult
	if err := json.Unmarshal(out.Bytes(), &multi); err != nil {
		t.Fatal(err)
	}
	if len(multi.Cores) != 2 {
		t.Fatal("incorrect core count")
	}
	catalog, err := benchmarks.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	var workload benchmarks.Workload
	for _, w := range catalog.Workloads {
		if w.Name == "canneal" {
			workload = w
		}
	}
	var retired, max uint64
	for _, core := range multi.Cores {
		if core.Termination != "exit" || core.ExitStatus == nil || *core.ExitStatus != 0 || len(core.Stderr) != 0 {
			t.Fatal("incomplete core")
		}
		if err := benchmarks.CheckNumerical(workload, workload.Cases[2], core.Stdout); err != nil {
			t.Fatal(err)
		}
		retired += core.Instructions
		if core.Timing.Cycles > max {
			max = core.Timing.Cycles
		}
	}
	if multi.TotalInstructions != retired || multi.TotalCycles != max {
		t.Fatal("incorrect aggregate counters")
	}
	for _, args := range [][]string{{"run", "--root", "../..", "--case", "0", "gemm"}, {"run", "--root", "../..", "--cores", "2", "--mode", "functional", "gemm"}, {"run", "--root", "../..", "missing"}, {"run", "--root", "../..", "--all", "gemm"}} {
		out.Reset()
		diag.Reset()
		if command(args, &out, &diag) != 2 {
			t.Fatalf("accepted invalid args%v", args)
		}
	}
}
