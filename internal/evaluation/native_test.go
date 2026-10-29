package evaluation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNativePilotIntegration(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("native code-identity pilot requires darwin/arm64; required by macOS CI")
	}
	if _, err := exec.LookPath("clang"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	report, err := Pilot(ctx, repo(t), "integration-test", []int{8}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "pass" || len(report.Cases) != 1 || report.Cases[0].Status != "pass" || report.Cases[0].Instructions == 0 || report.Hashes["executed_kernel"] == "" {
		t.Fatal("incomplete pilot evidence")
	}
}

func TestPilotCancellationAndBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// b . is a valid ARM64 infinite loop. It must not hang an evaluation.
	code := []byte{0, 0, 0, 0x14}
	if e := simulate(ctx, code, &PilotCase{N: 1, MedianNS: 1}); e == nil {
		t.Fatal("ignored cancellation")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e := simulate(ctx, code, &PilotCase{N: 1, MedianNS: 1}); e == nil || !strings.Contains(e.Error(), "cycle budget") {
		t.Fatalf("expected cycle budget error, got %v", e)
	}
}
func TestFunctionExtractionFailures(t *testing.T) {
	if _, e := FunctionBytes("does-not-exist"); e == nil {
		t.Fatal("accepted missing object")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("Mach-O fixture compilation requires macOS ARM64")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "other.c")
	obj := filepath.Join(dir, "other.o")
	if e := os.WriteFile(src, []byte("int other(void) { return 1; }\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if out, e := exec.Command("clang", "-c", src, "-o", obj).CombinedOutput(); e != nil {
		t.Fatalf("%s: %v", out, e)
	}
	if _, e := FunctionBytes(obj); e == nil {
		t.Fatal("accepted object missing kernel_mvt symbol")
	}
}

func TestKernelIdentityMismatch(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("Mach-O fixture compilation requires macOS ARM64")
	}
	dir := t.TempDir()
	paths := []string{}
	for i := 0; i < 2; i++ {
		src := filepath.Join(dir, fmt.Sprintf("kernel%d.c", i))
		obj := filepath.Join(dir, fmt.Sprintf("kernel%d.o", i))
		if e := os.WriteFile(src, []byte(fmt.Sprintf("int kernel_mvt(void) { return %d; }\n", i)), 0600); e != nil {
			t.Fatal(e)
		}
		if out, e := exec.Command("clang", "-O1", "-c", src, "-o", obj).CombinedOutput(); e != nil {
			t.Fatalf("%s: %v", out, e)
		}
		paths = append(paths, obj)
	}
	if _, e := matchingFunctionBytes(paths[0], paths[1]); e == nil {
		t.Fatal("accepted different native/simulated code")
	}
}
