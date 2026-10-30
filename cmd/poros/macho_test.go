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
