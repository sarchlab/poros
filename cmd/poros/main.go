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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sarchlab/poros/internal/benchmarks"
	"github.com/sarchlab/poros/pkg/elfsim"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"github.com/sarchlab/poros/pkg/target"
)

func main() { os.Exit(command(os.Args[1:], os.Stdout, os.Stderr)) }
func command(args []string, out, diag io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(diag, "usage: poros list [--root path] | targets | run [options] name [guest dimensions...] | elf [options] file.elf [args...]")
		return 2
	}
	switch args[0] {
	case "elf":
		return runELF(args[1:], out, diag)
	case "targets":
		return listTargets(args[1:], out, diag)
	case "list", "run":
		return namedCommand(args[0], args[1:], out, diag)
	case "eval":
		return namedCommand("run", append([]string{"--all", "--case", "3"}, args[1:]...), out, diag)
	default:
		fmt.Fprintln(diag, "unknown command", args[0])
		return 2
	}
}

// listTargets prints every modeled device as one JSON document: its name, the
// hardware it stands for and the target whose parameters it currently carries.
func listTargets(args []string, out, diag io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(diag, "targets takes no arguments")
		return 2
	}
	report := struct {
		Default string            `json:"default"`
		Targets []target.Identity `json:"targets"`
	}{Default: target.Default}
	for _, t := range target.All() {
		report.Targets = append(report.Targets, t.Identity)
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(diag, err)
		return 2
	}
	return 0
}

func namedCommand(operation string, args []string, out, diag io.Writer) int {
	fs := flag.NewFlagSet(operation, flag.ContinueOnError)
	fs.SetOutput(diag)
	root := fs.String("root", ".", "repository root containing checked-in ELF programs")
	all := fs.Bool("all", false, "run every named ELF workload")
	suite := fs.String("suite", "", "polybench, parsec or micro")
	point := fs.Int("case", 6, "declared input point, 1 through 6; default is largest")
	mode := fs.String("mode", "ooo", "functional, inorder or ooo")
	device := fs.String("target", target.Default, "modeled device: "+strings.Join(target.Names(), ", "))
	cores := fs.Int("cores", 1, "independent OoO process copies with modeled shared L2 (1..64)")
	steps := fs.Uint64("max-instructions", 200_000_000, "per-process instruction budget")
	timeout := fs.Duration("timeout", 5*time.Minute, "host elapsed timeout (multicore deadlines include time spent simulating the other cores)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *point < 1 || *point > 6 || *cores < 1 || *cores > 64 || (*mode != "functional" && *mode != "inorder" && *mode != "ooo") || (*cores > 1 && *mode != "ooo") || (*all && *suite != "") {
		fmt.Fprintln(diag, "invalid case, core count, mode or selector")
		return 2
	}
	if _, err := target.Lookup(*device); err != nil {
		fmt.Fprintln(diag, err)
		return 2
	}
	inventory, err := benchmarks.Inventory(*root)
	if err != nil {
		fmt.Fprintln(diag, err)
		return 2
	}
	if operation == "list" {
		if fs.NArg() != 0 || *all || *suite != "" {
			fmt.Fprintln(diag, "list takes no workload selector")
			return 2
		}
		for _, w := range inventory {
			fmt.Fprintf(out, "%-16s %-10s %s (6 input points)\n", w.Name, w.Suite, w.Binary)
		}
		fmt.Fprintf(out, "Total: %d ELF workloads\n", len(inventory))
		return 0
	}
	var selected []benchmarks.Workload
	for _, w := range inventory {
		if *all || (*suite != "" && w.Suite == *suite) || (*suite == "" && fs.NArg() > 0 && w.Name == fs.Arg(0)) {
			selected = append(selected, w)
		}
	}
	if len(selected) == 0 || ((*all || *suite != "") && fs.NArg() != 0) {
		fmt.Fprintln(diag, "select a valid workload, --all, or --suite")
		return 2
	}
	status := 0
	for _, w := range selected {
		guestArgs := w.Cases[*point-1].Args
		if !*all && *suite == "" && fs.NArg() > 1 {
			guestArgs = fs.Args()[1:]
		}
		path := filepath.Join(*root, w.Binary)
		if *cores == 1 {
			a := []string{"--mode", *mode, "--target", *device, "--argv0", w.Name, "--max-instructions", strconv.FormatUint(*steps, 10), "--timeout", timeout.String(), "--max-output", "16777216", path}
			a = append(a, guestArgs...)
			if code := runELF(a, out, diag); code != 0 {
				status = code
			}
			continue
		}
		image, e := os.ReadFile(path)
		if e != nil {
			fmt.Fprintln(diag, e)
			return 2
		}
		processes := make([]*linuxuser.Process, *cores)
		for i := range processes {
			processes[i], e = linuxuser.Parse(image, linuxuser.Options{Args: append([]string{w.Name}, guestArgs...), ExecName: w.Name})
			if e != nil {
				fmt.Fprintln(diag, e)
				return 2
			}
		}
		r, e := elfsim.RunMultiCore(context.Background(), processes, elfsim.Options{Model: "ooo", Target: *device, Limits: linuxuser.RunOptions{MaxInstructions: *steps, Timeout: *timeout, MaxOutput: 16 << 20}})
		sum := sha256.Sum256(image)
		report := struct {
			Schema    int               `json:"schema"`
			Build     map[string]string `json:"build"`
			Name      string            `json:"workload"`
			BinarySHA string            `json:"binary_sha256"`
			Args      []string          `json:"args"`
			elfsim.MultiCoreResult
		}{1, buildMetadata(), w.Name, hex.EncodeToString(sum[:]), append([]string{w.Name}, guestArgs...), r}
		if err := json.NewEncoder(out).Encode(report); err != nil {
			fmt.Fprintln(diag, err)
			return 2
		}
		if e != nil {
			fmt.Fprintln(diag, e)
			status = 1
		}
		for _, c := range r.Cores {
			if c.Termination != "exit" || c.ExitStatus == nil || *c.ExitStatus != 0 {
				status = 1
			}
		}
	}
	return status
}
