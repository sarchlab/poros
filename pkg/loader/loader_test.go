package loader

import (
	"os"
	"runtime"
	"testing"
)

func TestLoadTextSection(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("test requires darwin/arm64")
	}

	// /bin/ls is a Mach-O ARM64 binary on macOS ARM64
	text, err := LoadTextSection("/bin/ls")
	if err != nil {
		t.Fatalf("LoadTextSection(/bin/ls): %v", err)
	}

	if len(text.Bytes) == 0 {
		t.Fatal("text section is empty")
	}

	if text.BaseAddr == 0 {
		t.Fatal("base address is zero")
	}

	if text.Size == 0 {
		t.Fatal("size is zero")
	}

	if text.Size != uint64(len(text.Bytes)) {
		t.Errorf("size mismatch: Size=%d, len(Bytes)=%d", text.Size, len(text.Bytes))
	}
}

func TestLoadTextSectionInvalidFile(t *testing.T) {
	_, err := LoadTextSection("/nonexistent/file")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadTextSectionNonMacho(t *testing.T) {
	// Create a temporary non-Mach-O file
	f, err := os.CreateTemp("", "notmacho")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.Write([]byte("this is not a macho file"))
	f.Close()

	_, err = LoadTextSection(f.Name())
	if err == nil {
		t.Fatal("expected error for non-Mach-O file")
	}
}

func TestDisassembleN(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("test requires darwin/arm64")
	}

	text, err := LoadTextSection("/bin/ls")
	if err != nil {
		t.Fatalf("LoadTextSection: %v", err)
	}

	instructions := DisassembleN(text.Bytes, text.BaseAddr, 10)

	if len(instructions) != 10 {
		t.Fatalf("expected 10 instructions, got %d", len(instructions))
	}

	// Check that addresses are sequential (4-byte stride)
	for i := 1; i < len(instructions); i++ {
		expected := instructions[i-1].Addr + 4
		if instructions[i].Addr != expected {
			t.Errorf("instruction %d: expected addr 0x%x, got 0x%x",
				i, expected, instructions[i].Addr)
		}
	}

	// At least some instructions should be valid
	validCount := 0
	for _, inst := range instructions {
		if inst.IsValid {
			validCount++
		}
	}
	if validCount == 0 {
		t.Fatal("no valid instructions decoded")
	}
}

func TestDisassembleNEmpty(t *testing.T) {
	instructions := DisassembleN(nil, 0, 10)
	if len(instructions) != 0 {
		t.Fatalf("expected 0 instructions for nil data, got %d", len(instructions))
	}
}

func TestDisassembleNShortData(t *testing.T) {
	// Less than 4 bytes — can't decode any instruction
	instructions := DisassembleN([]byte{0x00, 0x01}, 0x1000, 10)
	if len(instructions) != 0 {
		t.Fatalf("expected 0 instructions for short data, got %d", len(instructions))
	}
}

func TestFormatInstruction(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("test requires darwin/arm64")
	}

	text, err := LoadTextSection("/bin/ls")
	if err != nil {
		t.Fatalf("LoadTextSection: %v", err)
	}

	instructions := DisassembleN(text.Bytes, text.BaseAddr, 1)
	if len(instructions) == 0 {
		t.Fatal("no instructions decoded")
	}

	s := FormatInstruction(instructions[0])
	if s == "" {
		t.Fatal("formatted instruction is empty")
	}
	t.Logf("formatted: %s", s)
}
