// Package benchmark provides reusable benchmark kernel definitions for the
// m2sim3 simulator. Each benchmark encapsulates the setup logic that creates a
// configured emulator ready to run through the pipeline models.
package benchmark

import (
	"encoding/binary"

	"github.com/sarchlab/m2sim3/pkg/emu"
	"github.com/sarchlab/m2sim3/pkg/mem"
)

// DataRegion describes a contiguous memory region to warm in the cache.
type DataRegion struct {
	Addr uint64
	Size int
}

// Benchmark defines a benchmark kernel with its setup closure, native
// execution time, and data regions for cache warm-up.
type Benchmark struct {
	Name       string
	Suite      string
	Setup      func() *emu.Emulator
	NativeTime float64
	Regions    []DataRegion
}

// EncodeInsts encodes a slice of ARM64 instruction words into a byte slice
// (little-endian).
func EncodeInsts(insts []uint32) []byte {
	buf := make([]byte, len(insts)*4)
	for i, inst := range insts {
		binary.LittleEndian.PutUint32(buf[i*4:], inst)
	}
	return buf
}

// WarmCache pre-populates the cache with the given data regions by issuing
// read lookups every 64 bytes (L1 line size). This simulates the
// initialization phase that occurs before the kernel runs on real hardware.
func WarmCache(cache *mem.Cache, regions []DataRegion) {
	const lineSize = 64
	for _, r := range regions {
		for off := 0; off < r.Size; off += lineSize {
			cache.Lookup(r.Addr+uint64(off), 8, false)
		}
	}
}
