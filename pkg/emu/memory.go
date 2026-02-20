package emu

import (
	"encoding/binary"
	"math"
)

// Memory provides a flat byte-addressable memory model using a page table.
// Each page is 4096 bytes. Pages are allocated on demand.
type Memory struct {
	pages map[uint64][]byte // page address -> 4096 byte page
}

const pageSize = 4096
const pageMask = pageSize - 1

// NewMemory creates an empty memory.
func NewMemory() *Memory {
	return &Memory{pages: make(map[uint64][]byte)}
}

func (m *Memory) getPage(addr uint64) []byte {
	pageAddr := addr &^ uint64(pageMask)
	page, ok := m.pages[pageAddr]
	if !ok {
		page = make([]byte, pageSize)
		m.pages[pageAddr] = page
	}
	return page
}

// ReadU8 reads a single byte.
func (m *Memory) ReadU8(addr uint64) uint8 {
	page := m.getPage(addr)
	return page[addr&uint64(pageMask)]
}

// WriteU8 writes a single byte.
func (m *Memory) WriteU8(addr uint64, val uint8) {
	page := m.getPage(addr)
	page[addr&uint64(pageMask)] = val
}

// ReadU16 reads a 16-bit little-endian value.
func (m *Memory) ReadU16(addr uint64) uint16 {
	var buf [2]byte
	for i := range buf {
		buf[i] = m.ReadU8(addr + uint64(i))
	}
	return binary.LittleEndian.Uint16(buf[:])
}

// WriteU16 writes a 16-bit little-endian value.
func (m *Memory) WriteU16(addr uint64, val uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], val)
	for i, b := range buf {
		m.WriteU8(addr+uint64(i), b)
	}
}

// ReadU32 reads a 32-bit little-endian value.
func (m *Memory) ReadU32(addr uint64) uint32 {
	var buf [4]byte
	for i := range buf {
		buf[i] = m.ReadU8(addr + uint64(i))
	}
	return binary.LittleEndian.Uint32(buf[:])
}

// WriteU32 writes a 32-bit little-endian value.
func (m *Memory) WriteU32(addr uint64, val uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], val)
	for i, b := range buf {
		m.WriteU8(addr+uint64(i), b)
	}
}

// ReadU64 reads a 64-bit little-endian value.
func (m *Memory) ReadU64(addr uint64) uint64 {
	var buf [8]byte
	for i := range buf {
		buf[i] = m.ReadU8(addr + uint64(i))
	}
	return binary.LittleEndian.Uint64(buf[:])
}

// WriteU64 writes a 64-bit little-endian value.
func (m *Memory) WriteU64(addr uint64, val uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], val)
	for i, b := range buf {
		m.WriteU8(addr+uint64(i), b)
	}
}

// ReadF32 reads a float32 from memory.
func (m *Memory) ReadF32(addr uint64) float32 {
	return math.Float32frombits(m.ReadU32(addr))
}

// WriteF32 writes a float32 to memory.
func (m *Memory) WriteF32(addr uint64, val float32) {
	m.WriteU32(addr, math.Float32bits(val))
}

// ReadF64 reads a float64 from memory.
func (m *Memory) ReadF64(addr uint64) float64 {
	return math.Float64frombits(m.ReadU64(addr))
}

// WriteF64 writes a float64 to memory.
func (m *Memory) WriteF64(addr uint64, val float64) {
	m.WriteU64(addr, math.Float64bits(val))
}

// Write bulk-loads bytes into memory starting at addr.
func (m *Memory) Write(addr uint64, data []byte) {
	for i, b := range data {
		m.WriteU8(addr+uint64(i), b)
	}
}

// ReadS8 reads a signed 8-bit value.
func (m *Memory) ReadS8(addr uint64) int8 {
	return int8(m.ReadU8(addr))
}

// ReadS32 reads a signed 32-bit value.
func (m *Memory) ReadS32(addr uint64) int32 {
	return int32(m.ReadU32(addr))
}
