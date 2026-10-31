package elfsim

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/sarchlab/poros/internal/benchmarks"
	"github.com/sarchlab/poros/pkg/linuxuser"
)

// Every supported kernel runs a complete ELF in every execution mode. Short CI
// uses case 3 (a nontrivial tail size); full CI requires all six declared sizes.
// A separate host compiler and host math library provide an independent ISA and
// compiler oracle without storing generated megabytes of float JSON.
func TestBenchmarkELF(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := benchmarks.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range manifest.Workloads {
		t.Run(w.Name, func(t *testing.T) {
			native := filepath.Join(t.TempDir(), w.Name)
			cmd := exec.Command("cc", "-O1", "-ffp-contract=off", "-DNATIVE_REFERENCE", filepath.Join(root, w.Source), "-lm", "-o", native)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("native reference build: %v\n%s", err, out)
			}
			image, err := os.ReadFile(filepath.Join(root, w.Binary))
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range w.Cases {
				if testing.Short() && c.ID != "3" {
					continue
				}
				t.Run(c.ID, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					cmd := exec.CommandContext(ctx, native, c.Args...)
					var stderr bytes.Buffer
					cmd.Stderr = &stderr
					reference, err := cmd.Output()
					if err != nil || stderr.Len() != 0 {
						t.Fatalf("native reference: %v %s", err, stderr.String())
					}
					if err := benchmarks.CheckOutput(c, reference, reference); err != nil {
						t.Fatalf("invalid native output: %v", err)
					}
					var functional linuxuser.Result
					for _, mode := range []string{"functional", "inorder", "ooo"} {
						t.Run(mode, func(t *testing.T) {
							p, err := linuxuser.Parse(image, linuxuser.Options{Args: append([]string{w.Name}, c.Args...)})
							if err != nil {
								t.Fatal(err)
							}
							limits := linuxuser.RunOptions{MaxInstructions: 200_000_000, Timeout: 5 * time.Minute, MaxOutput: 16 << 20}
							started := time.Now()
							var result linuxuser.Result
							if mode == "functional" {
								result, err = p.Run(context.Background(), limits)
							} else {
								var r Result
								r, err = Run(context.Background(), p, Options{Model: mode, Limits: limits})
								result = r.Result
							}
							if err != nil {
								t.Fatal(err)
							}
							if result.Termination != "exit" || result.ExitStatus == nil || *result.ExitStatus != 0 || len(result.Stderr) != 0 {
								t.Fatalf("incomplete execution: %+v", result)
							}
							if err := benchmarks.CheckOutput(c, result.Stdout, reference); err != nil {
								t.Fatal(err)
							}
							if err := benchmarks.CheckNumerical(w, c, result.Stdout); err != nil {
								t.Fatal(err)
							}
							if mode == "functional" {
								functional = result
							} else if !reflect.DeepEqual(functional, result) {
								t.Fatal("architectural result differs across modes")
							}
							t.Logf("%d guest instructions, %d complete output bytes, %.3fs", result.Instructions, len(result.Stdout), time.Since(started).Seconds())
						})
					}
				})
			}
			t.Run("invalid", func(t *testing.T) {
				base := append([]string(nil), w.Cases[0].Args...)
				bad := [][]string{nil, append(append([]string(nil), base...), "1")}
				for i, d := range w.Dimensions {
					for _, v := range []string{"", "-1", "0", "1x", "18446744073709551616", strconv.Itoa(d.Min - 1), strconv.Itoa(d.Max + 1)} {
						a := append([]string(nil), base...)
						a[i] = v
						bad = append(bad, a)
					}
				}
				if w.Name == "gramschmidt" {
					bad = append(bad, []string{"2", "3"})
				}
				if w.Name == "ferret" {
					bad = append(bad, []string{"1", "1", "1", "2"})
				}
				for i, args := range bad {
					t.Run(fmt.Sprint(i), func(t *testing.T) {
						p, err := linuxuser.Parse(image, linuxuser.Options{Args: append([]string{w.Name}, args...)})
						if err != nil {
							t.Fatal(err)
						}
						r, err := p.Run(context.Background(), linuxuser.RunOptions{MaxInstructions: 10000, Timeout: time.Second})
						if err != nil {
							t.Fatal(err)
						}
						if r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 2 || len(r.Stdout) != 0 || len(r.Stderr) != 0 {
							t.Fatalf("invalid input accepted: %+v", r)
						}
					})
				}
			})
		})
	}
}
