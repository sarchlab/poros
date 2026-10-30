// evaluate validates the M0 contract and produces reproducible evaluation reports.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/sarchlab/poros/internal/evaluation"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func write(path string, v any) (err error) {
	var w io.Writer = os.Stdout
	if path != "" {
		f, e := os.Create(path)
		if e != nil {
			return e
		}
		defer func() {
			if e := f.Close(); err == nil {
				err = e
			}
		}()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: evaluate manifest|regression|capture|seal [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := fs.String("root", ".", "repository root")
	out := fs.String("out", "", "JSON report path (default stdout)")
	var tier, log, capture *string
	var exit *int
	switch args[0] {
	case "manifest", "capture":
	case "seal":
		capture = fs.String("capture", "", "unsealed capture path")
		log = fs.String("log", "", "test JSON log")
		exit = fs.Int("test-exit", -1, "go test process exit code")
	case "regression":
		tier = fs.String("tier", "full", "short or full")
		log = fs.String("log", "", "go test JSON log")
		capture = fs.String("capture", "", "sealed tested provenance; otherwise imported log provenance is unknown")
	default:
		return fmt.Errorf("unknown command %s", args[0])
	}
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	commit := ""
	if args[0] == "regression" {
		b, e := exec.Command("git", "-C", *root, "rev-parse", "HEAD").Output()
		if e != nil {
			return e
		}
		commit = strings.TrimSpace(string(b))
	}

	switch args[0] {
	case "manifest":
		m, e := evaluation.LoadManifest(*root)
		if e != nil {
			return e
		}
		return write(*out, m)
	case "capture":
		if *out == "" {
			return fmt.Errorf("capture needs -out outside tracked source")
		}
		if e := evaluation.CheckCapturePath(*root, *out); e != nil {
			return e
		}
		c, e := evaluation.BeginCapture(*root)
		if e != nil {
			return e
		}
		return evaluation.SaveCapture(*out, c)
	case "seal":
		if *exit < 0 || *capture == "" || *log == "" {
			return fmt.Errorf("seal requires capture, log and test-exit")
		}
		c, e := evaluation.SealCapture(*root, *capture, *log, *exit)
		if e != nil {
			return e
		}
		return evaluation.SaveCapture(*capture, c)
	case "regression":
		data, e := os.ReadFile(*log)
		if e != nil {
			return e
		}
		r, check := evaluation.Regression(*root, *tier, commit, bytes.NewReader(data))
		if *capture != "" {
			r.Tested, e = evaluation.ReadCapture(*capture, data)
			if e == nil {
				e = evaluation.MatchCapture(*root, r.Tested)
			}
			if e != nil {
				r.Status = "fail"
				r.Acceptance = "fail"
				r.Provenance = "invalid"
				r.ProvenanceError = e.Error()
				check = e
			} else {
				r.Provenance = "bound"
				if *r.Tested.ExitCode != 0 {
					r.Status = "fail"
					check = fmt.Errorf("captured test process failed")
				}
				r.Acceptance = r.Status
			}
		}

		if e = write(*out, r); e != nil {
			return e
		}
		return check
	}
	return nil
}
