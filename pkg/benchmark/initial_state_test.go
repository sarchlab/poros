package benchmark

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// This fixture defines inputs, not correct outputs. It was captured before
// initializer canonicalization on Go 1.27/darwin/arm64 at commit 9269bca.
// Contract: SHA256(JSON(initial registers) || declared data-region bytes in
// registry order), with no execution, hook calls, or timing-model involvement.
func TestInitialStateIdentity(t *testing.T) {
	data, err := os.ReadFile("testdata/initial-state-go127-arm64.sha256")
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[1]) != 64 || expected[fields[0]] != "" {
			t.Fatalf("invalid input fixture row %q", line)
		}
		expected[fields[0]] = fields[1]
	}
	kernels := All()
	if len(expected) != len(kernels) || len(kernels) != 41 {
		t.Fatal("input inventory mismatch")
	}
	for _, b := range kernels {
		t.Run(b.Name, func(t *testing.T) {
			em := b.Setup()
			h := sha256.New()
			registers, err := json.Marshal(em.Regs)
			if err != nil {
				t.Fatal(err)
			}
			h.Write(registers)
			for _, region := range b.Regions {
				data := make([]byte, region.Size)
				for i := range data {
					data[i] = em.Mem.ReadU8(region.Addr + uint64(i))
				}
				h.Write(data)
			}
			if got := fmt.Sprintf("%x", h.Sum(nil)); got != expected[b.Name] {
				t.Fatalf("initial input identity changed: got %s, want %s", got, expected[b.Name])
			}
		})
	}
}
