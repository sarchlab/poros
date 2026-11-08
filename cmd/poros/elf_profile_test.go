package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/elfsim"
)

func TestELFTimingProfileCLI(t *testing.T) {
	profile := elfsim.DefaultTimingProfile(timing.GHz)
	profile.Configuration.NumStores = 2
	profile.Configuration.NumMSHREntries = 24
	profile.CacheConfig.L1Latency = 7
	profile.CacheConfig.L2Latency = 21
	profile.CacheConfig.DRAMLatency = 400
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var out, diag bytes.Buffer
	status := runELF([]string{"--mode", "ooo", "--frequency-hz", "1000000000", "--timing-profile", path, "../../tests/elf/hello.elf"}, &out, &diag)
	if status != 0 || diag.Len() != 0 {
		t.Fatalf("valid profile CLI failed: status%d diagnostics%s", status, diag.String())
	}
	var result struct {
		Instructions uint64
		Stdout       []byte `json:"stdout_base64"`
		Stderr       []byte `json:"stderr_base64"`
		Timing       json.RawMessage
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Instructions != 51 || string(result.Stdout) != "hello, ELF\n" || string(result.Stderr) != "stderr\n" {
		t.Fatalf("profile changed functional output: %+v", result)
	}
	var effective elfsim.TimingProfile
	if err := json.Unmarshal(result.Timing, &effective); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effective, profile) {
		t.Fatalf("requested profile silently replaced: got%+v want%+v", effective, profile)
	}
	var stats elfsim.Statistics
	if err := json.Unmarshal(result.Timing, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.AssumedHz != 1e9 || stats.Retired != 51 || stats.Cycles == 0 {
		t.Fatalf("incorrect execution statistics: %+v", stats)
	}
	for _, tc := range []struct {
		name       string
		args       []string
		diagnostic string
	}{
		{"functional", []string{"--mode", "functional", "--timing-profile", path}, "require --mode ooo"},
		{"inorder", []string{"--mode", "inorder", "--timing-profile", path}, "require --mode ooo"},
		{"missing file", []string{"--mode", "ooo", "--timing-profile", filepath.Join(t.TempDir(), "missing.json")}, "missing.json"},
		{"frequency mismatch", []string{"--mode", "ooo", "--timing-profile", path}, "frequency differs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diag bytes.Buffer
			status := runELF(append(tc.args, "../../tests/elf/hello.elf"), &out, &diag)
			if status != 2 || out.Len() != 0 || !strings.Contains(diag.String(), tc.diagnostic) {
				t.Fatalf("invalid profile CLI: status%d output%s diagnostics%s", status, out.String(), diag.String())
			}
		})
	}
}
