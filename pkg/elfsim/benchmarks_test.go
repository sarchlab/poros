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

// The performance-only wrapper is exercised even in short/ordinary CI. Keep
// these tiny coordinates separate from both six-point benchmark inventories.
func TestStreamclusterRepeatELF(t *testing.T) {
	const root = "../../tests/benchmarks/"
	limits := linuxuser.RunOptions{MaxInstructions: 100_000, Timeout: 10 * time.Second, MaxOutput: 1024}
	run := func(name, mode string, args ...string) linuxuser.Result {
		t.Helper()
		info, err := os.Stat(root + name + ".elf")
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0111 == 0 {
			t.Fatalf("%s ELF is packaged without executable permission required by QEMU", name)
		}
		image, err := os.ReadFile(root + name + ".elf")
		if err != nil {
			t.Fatal(err)
		}
		p, err := linuxuser.Parse(image, linuxuser.Options{Args: append([]string{"program"}, args...)})
		if err != nil {
			t.Fatal(err)
		}
		var result linuxuser.Result
		if mode == "functional" {
			result, err = p.Run(context.Background(), limits)
		} else {
			var timed Result
			timed, err = Run(context.Background(), p, Options{Model: mode, Limits: limits})
			result = timed.Result
		}
		if err != nil || result.Termination != "exit" || result.ExitStatus == nil || *result.ExitStatus != 0 || len(result.Stderr) != 0 {
			t.Fatalf("%s %s incomplete: %v %+v", name, mode, err, result)
		}
		return result
	}
	original := run("streamcluster", "functional", "7", "3")
	if len(original.Stdout) != 7*8 {
		t.Fatal("original output length differs")
	}
	expected := bytes.Repeat(original.Stdout, 2)
	functional := run("streamcluster-repeat", "functional", "7", "3", "2")
	if !bytes.Equal(functional.Stdout, expected) {
		t.Fatal("repeat wrapper did not emit exactly two complete original outputs")
	}
	timed := run("streamcluster-repeat", "ooo", "7", "3", "2")
	if !reflect.DeepEqual(functional, timed) {
		t.Fatal("repeat wrapper functional/OoO architectural results differ")
	}
	t.Run("qemu", func(t *testing.T) {
		qemu, err := exec.LookPath("qemu-aarch64")
		if err != nil {
			t.Skip("QEMU is checked in the Linux ELF differential CI job")
		}
		for _, name := range []string{"streamcluster", "streamcluster-repeat"} {
			args := []string{root + name + ".elf", "7", "3"}
			want := original.Stdout
			if name == "streamcluster-repeat" {
				args = append(args, "2")
				want = expected
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, qemu, args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			actual, err := cmd.Output()
			cancel()
			if err != nil || stderr.Len() != 0 || !bytes.Equal(actual, want) {
				t.Fatalf("%s QEMU complete-output mismatch: %v %s", name, err, stderr.String())
			}
		}
	})
}
