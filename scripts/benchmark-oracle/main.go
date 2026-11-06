// benchmark-oracle checks complete native/QEMU output with independent equations.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/sarchlab/poros/internal/benchmarks"
	"io"
	"os"
	"runtime/debug"
)

func main() {
	if len(os.Args) != 2 {
		panic("expected one JSON workload/case request and raw stdin")
	}
	var request struct {
		Workload benchmarks.Workload
		Case     benchmarks.Case
	}
	if err := json.Unmarshal([]byte(os.Args[1]), &request); err != nil {
		panic(err)
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 64*1024*1024+1))
	if err == nil && len(raw) > 64*1024*1024 {
		err = fmt.Errorf("output exceeds bounded oracle input")
	}
	if err == nil {
		err = benchmarks.CheckNumerical(request.Workload, request.Case, raw)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	build := map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			build[setting.Key] = setting.Value
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "pass", "check_kind": benchmarks.NumericalCheckKind(request.Workload.Name), "build": build}); err != nil {
		panic(err)
	}
}
