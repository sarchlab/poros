package elfsim

import (
	"bytes"
	"context"
	"debug/elf"
	"encoding/binary"
	"os"
	"testing"

	"github.com/sarchlab/poros/pkg/linuxuser"
)

// storeRunProcess is a Linux process whose loop stores an ascending stream of
// eight-byte words and, after each of them, eight bytes at one fixed address
// elsewhere:
//
//	loop: str x3, [x1], #8
//	      str x3, [x2]
//	      subs x4, x4, #1
//	      b.ne loop
//	      mov x0, #0 ; mov x8, #93 ; svc #0
//
// Both addresses lie in the stack mapping, below anything the startup wrote.
// The fixed store starts elsewhereOffset bytes into a line of the 16 KiB page
// that no line of the stream shares.
func storeRunProcess(t *testing.T, trips, elsewhereOffset uint64) *linuxuser.Process {
	t.Helper()
	image, err := os.ReadFile("../../tests/elf/hello.elf")
	if err != nil {
		t.Fatal(err)
	}
	file, err := elf.NewFile(bytes.NewReader(image))
	if err != nil {
		t.Fatal(err)
	}
	code := []uint32{0xf8008423, 0xf9000043, 0xf1000484, 0x54ffffa1, 0xd2800000, 0xd2800ba8, 0xd4000001}
	text := file.Section(".text")
	if text == nil || text.Addr != file.Entry || text.Size < uint64(4*len(code)) {
		t.Fatal("fixture has no entry section to hold the loop")
	}
	for i, word := range code {
		binary.LittleEndian.PutUint32(image[text.Offset+uint64(4*i):], word)
	}
	p, err := linuxuser.Parse(image, linuxuser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	stream := linuxuser.StackTop - linuxuser.DefaultStackSize
	p.Registers.GPR[1] = stream
	p.Registers.GPR[2] = stream + 0x41000 + elsewhereOffset
	p.Registers.GPR[3] = 0x5a5a5a5a5a5a5a5a
	p.Registers.GPR[4] = trips
	return p
}

// On the M1 target one store instruction is one store to the store-run rule,
// also in the Linux process backend, which records an unaligned store as two
// accesses cut at the word boundary. A stream with one eight-byte store
// elsewhere per trip stays an interleaved run whether that store is aligned,
// crosses a word inside its line or crosses a cache line: every stream store
// from the one that completes the first line waits for its line, and the
// loop takes the same time (within 1 %; the store across a line fetches one
// more line). Counted per access or per line, the unaligned store would be
// two stores elsewhere, end the run every trip and let the stream through in
// a third of the cycles.
func TestM1StoreRunCountsAnUnalignedStoreOnce(t *testing.T) {
	const trips = 512 // 64 lines of the stream
	run := func(device string, elsewhereOffset uint64) Statistics {
		t.Helper()
		r, err := Run(context.Background(), storeRunProcess(t, trips, elsewhereOffset), Options{Target: device})
		if err != nil || r.Termination != "exit" || r.Instructions != 4*trips+3 {
			t.Fatalf("%s, store elsewhere at +%d: %+v, %v", device, elsewhereOffset, r.Result, err)
		}
		return r.Timing
	}
	aligned := run("m1", 0)
	if aligned.Cache.StoreRunWaits != trips-7 {
		t.Fatalf("aligned store elsewhere: %d stream stores wait, want %d", aligned.Cache.StoreRunWaits, trips-7)
	}
	for _, offset := range []uint64{20, 60} {
		unaligned := run("m1", offset)
		slower, faster := max(unaligned.Cycles, aligned.Cycles), min(unaligned.Cycles, aligned.Cycles)
		if unaligned.Cache.StoreRunWaits != aligned.Cache.StoreRunWaits || 100*(slower-faster) > aligned.Cycles {
			t.Errorf("store elsewhere at +%d: %d stream stores wait in %d cycles, aligned %d in %d", offset,
				unaligned.Cache.StoreRunWaits, unaligned.Cycles, aligned.Cache.StoreRunWaits, aligned.Cycles)
		}
	}
	// M2 has no such rule: the same loop takes less than half the cycles there.
	if m2 := run("m2", 60); m2.Cache.StoreRunWaits != 0 || 2*m2.Cycles > aligned.Cycles {
		t.Errorf("m2: %d waits, %d cycles against M1's %d", m2.Cache.StoreRunWaits, m2.Cycles, aligned.Cycles)
	}
}
