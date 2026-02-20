package emu

import (
	"math"
	"testing"
)

func TestMemoryReadWriteU8(t *testing.T) {
	m := NewMemory()
	m.WriteU8(0x1000, 0xAB)
	if got := m.ReadU8(0x1000); got != 0xAB {
		t.Errorf("ReadU8 = %#x, want 0xAB", got)
	}
}

func TestMemoryReadWriteU16(t *testing.T) {
	m := NewMemory()
	m.WriteU16(0x2000, 0x1234)
	if got := m.ReadU16(0x2000); got != 0x1234 {
		t.Errorf("ReadU16 = %#x, want 0x1234", got)
	}
	// Little endian: low byte first
	if lo := m.ReadU8(0x2000); lo != 0x34 {
		t.Errorf("low byte = %#x, want 0x34", lo)
	}
	if hi := m.ReadU8(0x2001); hi != 0x12 {
		t.Errorf("high byte = %#x, want 0x12", hi)
	}
}

func TestMemoryReadWriteU32(t *testing.T) {
	m := NewMemory()
	m.WriteU32(0x3000, 0xDEADBEEF)
	if got := m.ReadU32(0x3000); got != 0xDEADBEEF {
		t.Errorf("ReadU32 = %#x, want 0xDEADBEEF", got)
	}
}

func TestMemoryReadWriteU64(t *testing.T) {
	m := NewMemory()
	m.WriteU64(0x4000, 0x123456789ABCDEF0)
	if got := m.ReadU64(0x4000); got != 0x123456789ABCDEF0 {
		t.Errorf("ReadU64 = %#x, want 0x123456789ABCDEF0", got)
	}
}

func TestMemoryReadWriteFloat(t *testing.T) {
	m := NewMemory()
	m.WriteF64(0x5000, 3.14)
	if got := m.ReadF64(0x5000); got != 3.14 {
		t.Errorf("ReadF64 = %f, want 3.14", got)
	}
	m.WriteF32(0x6000, 2.5)
	if got := m.ReadF32(0x6000); got != 2.5 {
		t.Errorf("ReadF32 = %f, want 2.5", got)
	}
}

func TestMemoryBulkWrite(t *testing.T) {
	m := NewMemory()
	data := []byte{0x01, 0x02, 0x03, 0x04}
	m.Write(0x7000, data)
	if got := m.ReadU32(0x7000); got != 0x04030201 {
		t.Errorf("bulk write ReadU32 = %#x, want 0x04030201", got)
	}
}

func TestMemoryPageBoundary(t *testing.T) {
	m := NewMemory()
	// Write U32 spanning page boundary (page = 4096 bytes)
	addr := uint64(4094) // last 2 bytes of page 0, first 2 bytes of page 1
	m.WriteU32(addr, 0xAABBCCDD)
	if got := m.ReadU32(addr); got != 0xAABBCCDD {
		t.Errorf("cross-page ReadU32 = %#x, want 0xAABBCCDD", got)
	}
}

func TestMemorySignedRead(t *testing.T) {
	m := NewMemory()
	m.WriteU8(0x8000, 0xFF) // -1 as signed
	if got := m.ReadS8(0x8000); got != -1 {
		t.Errorf("ReadS8 = %d, want -1", got)
	}
	m.WriteU32(0x9000, math.MaxUint32) // -1 as signed 32
	if got := m.ReadS32(0x9000); got != -1 {
		t.Errorf("ReadS32 = %d, want -1", got)
	}
}
