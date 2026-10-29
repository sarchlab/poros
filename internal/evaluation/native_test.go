package evaluation

import (
	"context"
	"os/exec"
	"runtime"
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
