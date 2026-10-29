package elfsim

import (
	"bytes"
	"context"
	"debug/elf"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarchlab/poros/pkg/linuxuser"
	"golang.org/x/arch/arm64/arm64asm"
)

func TestELFModeParity(t *testing.T) {
	var manifest struct {
		Cases []struct {
			Name, File      string
			Args, Env       []string
			MaxInstructions uint64 `json:"max_instructions"`
			MaxOutput       int    `json:"max_output"`
		}
	}
	data, err := os.ReadFile("../../tests/elf/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	names := []string{"hello", "startup", "memory", "compute", "syscall-errors", "bad-load", "bad-store", "nx", "unsupported-syscall", "unsupported-instruction", "loop", "output-limit", "exit-status", "arithmetic-loop"}
	if len(manifest.Cases) != len(names) {
		t.Fatal("incomplete required inventory")
	}
	for i, c := range manifest.Cases {
		if c.Name != names[i] {
			t.Fatal("changed inventory")
		}
		t.Run(c.Name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("../../tests/elf", c.File))
			if err != nil {
				t.Fatal(err)
			}
			options := linuxuser.Options{Args: c.Args, Env: c.Env, ExecName: "program"}
			limits := linuxuser.RunOptions{MaxInstructions: c.MaxInstructions, MaxOutput: c.MaxOutput, Timeout: 30 * time.Second}
			reference, err := linuxuser.Parse(input, options)
			if err != nil {
				t.Fatal(err)
			}
			want, err := reference.Run(context.Background(), limits)
			if err != nil {
				t.Fatal(err)
			}
			for _, model := range []string{"inorder", "ooo"} {
				t.Run(model, func(t *testing.T) {
					var previous uint64
					for repeat := 0; repeat < 2; repeat++ {
						p, err := linuxuser.Parse(input, options)
						if err != nil {
							t.Fatal(err)
						}
						got, err := Run(context.Background(), p, Options{Model: model, Limits: limits})
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got.Result, want) || p.Registers != reference.Registers {
							t.Fatalf("functional parity failed: got %+v want %+v", got.Result, want)
						}
						for _, m := range p.Memory.Mappings() {
							if m.Flags&elf.PF_R == 0 {
								continue
							}
							a, b := make([]byte, m.End-m.Start), make([]byte, m.End-m.Start)
							if err = p.Memory.Read(m.Start, a); err != nil {
								t.Fatal(err)
							}
							if err = reference.Memory.Read(m.Start, b); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(a, b) {
								t.Fatal("memory differs")
							}
						}
						if got.Timing.Cycles == 0 || got.Timing.PredictedUserSeconds != float64(got.Timing.Cycles)/got.Timing.AssumedHz {
							t.Fatal("invalid timing")
						}
						if repeat > 0 && previous != got.Timing.Cycles {
							t.Fatal("nondeterministic timing")
						}
						previous = got.Timing.Cycles
					}
				})
			}
		})
	}
}

// The minimal exit program permits a hand-derived timing oracle: two independent
// MOVs issue together then SVC after their retirement in OoO (3 cycles); five
// stage inorder drains both MOVs before SVC (8 cycles).
func TestExitBoundaryCycles(t *testing.T) {
	input, err := os.ReadFile("../../pkg/loader/testdata/elf/startup.elf")
	if err != nil {
		t.Fatal(err)
	}
	image, err := elf.NewFile(bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var offset uint64
	for _, p := range image.Progs {
		if p.Type == elf.PT_LOAD && image.Entry >= p.Vaddr && image.Entry < p.Vaddr+p.Filesz {
			offset = p.Off + image.Entry - p.Vaddr
		}
	}
	for i, word := range []uint32{0xd2800000, 0xd2800ba8, 0xd4000001} {
		binary.LittleEndian.PutUint32(input[offset+uint64(4*i):], word)
	}
	for model, cycles := range map[string]uint64{"ooo": 3, "inorder": 8} {
		p, err := linuxuser.Parse(input, linuxuser.Options{})
		if err != nil {
			t.Fatal(err)
		}
		r, err := Run(context.Background(), p, Options{Model: model})
		if err != nil {
			t.Fatal(err)
		}
		if r.Timing.Cycles != cycles || r.Instructions != 3 || r.Timing.Cache.L1Hits+r.Timing.Cache.L1Misses != 0 {
			t.Fatalf("%s: %+v", model, r)
		}
	}
}

func TestCancellationAndDeadline(t *testing.T) {
	input, err := os.ReadFile("../../tests/elf/loop.elf")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"inorder", "ooo"} {
		for _, preCancelled := range []bool{true, false} {
			t.Run(fmt.Sprint(model, preCancelled), func(t *testing.T) {
				p, err := linuxuser.Parse(input, linuxuser.Options{})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if preCancelled {
					cancel()
				}
				started := time.Now()
				r, err := Run(ctx, p, Options{Model: model, Limits: linuxuser.RunOptions{Timeout: time.Millisecond, MaxInstructions: 100000000}})
				if err != nil {
					t.Fatal(err)
				}
				if r.Termination != "time_limit" || time.Since(started) > time.Second {
					t.Fatalf("deadline not bounded: %+v", r)
				}
				if preCancelled && r.Instructions != 0 {
					t.Fatal("cancelled process executed")
				}
			})
		}
	}
}
func TestCompleteWorkloadShapesAndResults(t *testing.T) {
	const n = 1024
	for _, name := range []string{"integer", "memory", "branch"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile("../../tests/usertime/" + name + ".elf")
			if err != nil {
				t.Fatal(err)
			}
			if name == "branch" {
				f, err := elf.NewFile(bytes.NewReader(input))
				if err != nil {
					t.Fatal(err)
				}
				code, err := f.Section(".text").Data()
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for i := 0; i+4 <= len(code); i += 4 {
					inst, e := arm64asm.Decode(code[i : i+4])
					if e == nil && inst.Op == arm64asm.TBZ && strings.Contains(inst.String(), "#16") {
						found = true
					}
				}
				if !found {
					t.Fatal("branch workload lost its data-dependent TBZ #16 (if-converted?)")
				}
			}
			var want uint64 = 1
			var data [8192]uint64
			for i := range data {
				data[i] = uint64(i)
			}
			for i := uint64(0); i < n; i++ {
				if name == "memory" {
					j := i & 8191
					data[j] += i
					want += data[j]
				} else {
					want = (want*1664525 + 1013904223) & 0xffffffff
					if name == "branch" {
						if want&0x10000 != 0 {
							want ^= 0x1234
						} else {
							want += 7
						}
					}
				}
			}
			p, err := linuxuser.Parse(input, linuxuser.Options{Args: []string{"program", "1024"}})
			if err != nil {
				t.Fatal(err)
			}
			r, err := Run(context.Background(), p, Options{Model: "ooo"})
			if err != nil {
				t.Fatal(err)
			}
			if r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 0 || len(r.Stdout) != 8 || binary.LittleEndian.Uint64(r.Stdout) != want {
				t.Fatalf("wrong complete-program result: %+v", r)
			}
			if name == "branch" && (r.Timing.BranchPredictions < 2*n || r.Timing.BranchDirectionHits == r.Timing.BranchPredictions) {
				t.Fatal("branch workload has no varied branch activity")
			}
		})
	}
}
