// Package loader provides functionality for loading and disassembling
// ARM64 Mach-O binaries.
package loader

import (
	"debug/macho"
	"fmt"
	"io"
)

// TextSection holds the raw bytes and metadata of a Mach-O __TEXT,__text section.
type TextSection struct {
	Bytes    []byte
	BaseAddr uint64
	Size     uint64
}

// LoadTextSection opens a Mach-O file (thin or universal/fat) and extracts
// the __TEXT,__text section. For universal binaries, it selects the arm64 slice.
func LoadTextSection(path string) (*TextSection, error) {
	mf, closer, err := openMacho(path)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	sect := mf.Section("__text")
	if sect == nil {
		return nil, fmt.Errorf("no __text section found in %s", path)
	}

	data, err := io.ReadAll(sect.Open())
	if err != nil {
		return nil, fmt.Errorf("read __text section: %w", err)
	}

	return &TextSection{
		Bytes:    data,
		BaseAddr: sect.Addr,
		Size:     sect.Size,
	}, nil
}

// openMacho tries to open a Mach-O file. If it's a universal (fat) binary,
// it extracts the arm64 or arm64e slice. Returns the macho.File and an
// io.Closer that must be closed by the caller.
func openMacho(path string) (*macho.File, io.Closer, error) {
	// Try thin Mach-O first.
	f, err := macho.Open(path)
	if err == nil {
		return f, f, nil
	}

	// Try as a universal (fat) binary.
	fat, fatErr := macho.OpenFat(path)
	if fatErr != nil {
		return nil, nil, fmt.Errorf("open macho: %w", err)
	}

	for _, arch := range fat.Arches {
		if arch.Cpu == macho.CpuArm64 {
			return arch.File, fat, nil
		}
	}

	fat.Close()
	return nil, nil, fmt.Errorf("no arm64 architecture found in universal binary %s", path)
}
