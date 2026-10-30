package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/sarchlab/poros/pkg/darwinuser"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestMachOProfileRejectsUnknownTrailingAndOversizedData(t *testing.T) {
	for _, content := range []string{`{"unknown":1}`, `{"value":1} {}`, `{"value":1} trailing`, string(bytes.Repeat([]byte(" "), 33))} {
		p := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		var v struct {
			Value int `json:"value"`
		}
		if readMachOProfile(p, 32, &v) == nil {
			t.Fatalf("accepted %q", content)
		}
	}
}

func TestMachOCLIUsesAndReportsBoundStartupProfile(t *testing.T) {
	directory := t.TempDir()
	program, dyld := filepath.Join(directory, "program"), filepath.Join(directory, "dyld")
	a, b := machotest.Image(2), machotest.Image(7)
	for path, data := range map[string][]byte{program: a, dyld: b} {
		if e := os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	hash := func(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
	profile := darwinuser.StartupProfile{BinarySHA256: hash(a), DyldSHA256: hash(b), ExecutableFile: darwinuser.FileIdentity{FSID: 1, ObjectID: 2, Path: "/guest/program"}, DyldFile: darwinuser.FileIdentity{FSID: 1, ObjectID: 3, Path: "/usr/lib/dyld"}, CDHash: strings.Repeat("ab", 20), BootHash: strings.Repeat("cd", 20), PFZ: 0xfffe78000, StackSize: 0x7fc000, StackAllocationSize: 64 << 20}
	profilePath := filepath.Join(directory, "startup.json")
	encoded, e := json.Marshal(profile)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(profilePath, encoded, 0600); e != nil {
		t.Fatal(e)
	}
	var output, diagnostic bytes.Buffer
	code := runMachO([]string{"--dyld", dyld, "--startup-profile", profilePath, "--exec-name", "/guest/program", program}, &output, &diagnostic)
	var report struct {
		Startup      darwinuser.StartupProfile `json:"startup_profile"`
		Instructions uint64
	}
	if e := json.Unmarshal(output.Bytes(), &report); e != nil {
		t.Fatal(e, diagnostic.String())
	}
	if code != 1 || diagnostic.Len() != 0 || !reflect.DeepEqual(report.Startup, profile) || report.Instructions != 6 {
		t.Fatal(code, report, diagnostic.String())
	}
	profile.BinarySHA256 = strings.Repeat("0", 64)
	encoded, _ = json.Marshal(profile)
	os.WriteFile(profilePath, encoded, 0600)
	output.Reset()
	diagnostic.Reset()
	if runMachO([]string{"--dyld", dyld, "--startup-profile", profilePath, program}, &output, &diagnostic) != 2 || output.Len() != 0 || !strings.Contains(diagnostic.String(), "input hash mismatch") {
		t.Fatal("mismatched startup identity accepted", diagnostic.String())
	}
}

func TestMachORuntimeProfileSizeBound(t *testing.T) {
	directory := t.TempDir()
	profile := filepath.Join(directory, "runtime.json")
	// Whitespace is valid JSON padding; the limit applies to all input bytes.
	valid := append([]byte(`{}`), bytes.Repeat([]byte(" "), maxMachORuntimeProfileBytes-2)...)
	if err := os.WriteFile(profile, valid, 0600); err != nil {
		t.Fatal(err)
	}
	var p darwinuser.RuntimeProfile
	if err := readMachOProfile(profile, maxMachORuntimeProfileBytes, &p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, append(valid, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := runMachO([]string{"--dyld", "unused", "--runtime-profile", profile, "unused"}, &out, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "exceeds") || out.Len() != 0 {
		t.Fatal(code, diagnostic.String())
	}
}

func TestMachOVirtualQuiescencePolicyIsReported(t *testing.T) {
	directory := t.TempDir()
	program, dyld := filepath.Join(directory, "program"), filepath.Join(directory, "dyld")
	os.WriteFile(program, machotest.Image(2), 0600)
	os.WriteFile(dyld, machotest.Image(7), 0600)
	var out, diagnostic bytes.Buffer
	if code := runMachO([]string{"--dyld", dyld, "--quiescence", "single-thread-no-scheduling", program}, &out, &diagnostic); code != 1 {
		t.Fatal(code, diagnostic.String())
	}
	var report struct {
		Quiescence string `json:"quiescence_policy"`
	}
	if e := json.Unmarshal(out.Bytes(), &report); e != nil || report.Quiescence != "single-thread-no-scheduling" {
		t.Fatal(report, e)
	}
	out.Reset()
	diagnostic.Reset()
	if code := runMachO([]string{"--dyld", dyld, "--quiescence", "instruction-ticks", program}, &out, &diagnostic); code != 2 || !strings.Contains(diagnostic.String(), "quiescence") {
		t.Fatal(code, diagnostic.String())
	}
}

func TestMachOCLIReportsExplicitClockAnchor(t *testing.T) {
	directory := t.TempDir()
	program := filepath.Join(directory, "program")
	dyld := filepath.Join(directory, "dyld")
	runtime := darwinuser.RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, UserTimebase: 3, ContinuousHWClock: 1}
	anchor := darwinuser.ClockAnchor{BootTimeMicroseconds: 1_000_000, InitialCounter: 101, TimeOfDay: darwinuser.TimeOfDayAnchor{Tick: 100, Seconds: 10, Scale: 768614479732, TicksPerSecond: 24_000_000}}
	profilePath := filepath.Join(directory, "runtime.json")
	clockPath := filepath.Join(directory, "clock.json")
	rb, _ := json.Marshal(runtime)
	ab, _ := json.Marshal(anchor)
	for path, data := range map[string][]byte{program: machotest.Image(2), dyld: machotest.Image(7), profilePath: rb, clockPath: ab} {
		if e := os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	var out, diagnostic bytes.Buffer
	args := []string{"--dyld", dyld, "--runtime-profile", profilePath, "--clock-anchor", clockPath, program}
	if code := runMachO(args, &out, &diagnostic); code != 1 || diagnostic.Len() != 0 {
		t.Fatal(code, diagnostic.String())
	}
	var report struct {
		Clock *darwinuser.ClockAnchor `json:"clock_anchor"`
	}
	if e := json.Unmarshal(out.Bytes(), &report); e != nil || report.Clock == nil || *report.Clock != anchor {
		t.Fatal("clock provenance", e, report)
	}
	for _, bad := range [][]byte{[]byte(`{"unknown":1}`), []byte(`{}`), append(ab, []byte(` {}`)...), bytes.Repeat([]byte(" "), 4097)} {
		if e := os.WriteFile(clockPath, bad, 0600); e != nil {
			t.Fatal(e)
		}
		out.Reset()
		diagnostic.Reset()
		if code := runMachO(args, &out, &diagnostic); code != 2 || out.Len() != 0 {
			t.Fatal("invalid clock input", code, out.String(), diagnostic.String())
		}
	}
}
