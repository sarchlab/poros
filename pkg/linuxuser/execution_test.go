package linuxuser

import (
	"context"
	"golang.org/x/arch/arm64/arm64asm"
	"os"
	"testing"
)

func TestSyscallCopyDoesNotGenerateGuestAccesses(t *testing.T) {
	b, err := os.ReadFile("../../tests/elf/hello.elf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	x, err := p.NewExecution(context.Background(), RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	svc := 0
	for !x.Stopped() {
		inst, _, err := x.StepDetailed()
		if err != nil {
			t.Fatal(err)
		}
		if inst.Op == arm64asm.SVC {
			svc++
			if len(x.MemoryAccesses()) != 0 {
				t.Fatal("host syscall copy entered guest memory stream")
			}
		}
	}
	if svc != 3 || len(x.Result().Stdout) == 0 {
		t.Fatal("write/exit not exercised")
	}
}
func TestFaultClearsMemoryEvents(t *testing.T) {
	b, err := os.ReadFile("../../tests/elf/bad-load.elf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	x, err := p.NewExecution(context.Background(), RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	for !x.Stopped() {
		_, _, err = x.StepDetailed()
		if err != nil && len(x.MemoryAccesses()) != 0 {
			t.Fatal("fault retained memory events")
		}
	}
	if x.Result().Termination != "memory_fault" {
		t.Fatal(x.Result())
	}
}
