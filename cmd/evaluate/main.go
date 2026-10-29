// evaluate validates the M0 contract and produces reproducible evaluation reports.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sarchlab/poros/internal/evaluation"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func write(path string, v any) error {
	var w io.Writer = os.Stdout
	if path != "" {
		f, e := os.Create(path)
		if e != nil {
			return e
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: evaluate manifest|regression|pilot [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := fs.String("root", ".", "repository root")
	out := fs.String("out", "", "JSON report path (default stdout)")
	tier := fs.String("tier", "full", "short or full regression")
	log := fs.String("log", "", "go test -json log")
	sizes := fs.String("sizes", "8,63,120,129,400,1536", "native MVT dimensions")
	reps := fs.Int("repetitions", 13, "native samples per size")
	timeout := fs.Duration("timeout", 30*time.Minute, "pilot wall-time budget")
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	b, e := exec.Command("git", "-C", *root, "rev-parse", "HEAD").Output()
	if e != nil {
		return e
	}
	commit := strings.TrimSpace(string(b))
	switch args[0] {
	case "manifest":
		m, e := evaluation.LoadManifest(*root)
		if e != nil {
			return e
		}
		return write(*out, m)
	case "regression":
		f, e := os.Open(*log)
		if e != nil {
			return e
		}
		defer f.Close()
		r, check := evaluation.Regression(*root, *tier, commit, f)
		if e = write(*out, r); e != nil {
			return e
		}
		return check
	case "pilot":
		if _, e := evaluation.LoadManifest(*root); e != nil {
			return e
		}
		ns := []int{}
		for _, s := range strings.Split(*sizes, ",") {
			n, e := strconv.Atoi(s)
			if e != nil {
				return e
			}
			ns = append(ns, n)
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		r, check := evaluation.Pilot(ctx, *root, commit, ns, *reps)
		if e = write(*out, r); e != nil {
			return e
		}
		return check
	default:
		return fmt.Errorf("unknown command %s", args[0])
	}
}
