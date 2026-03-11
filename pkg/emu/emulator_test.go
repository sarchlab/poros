package emu

import (
	"encoding/binary"
	"math"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestEmulatorArraySum(t *testing.T) {
	// Integration test: Sum an array of 5 uint64 values.
	//
	// C equivalent:
	//   uint64_t sum_array(uint64_t *arr, uint64_t n) {
	//       uint64_t sum = 0;
	//       for (uint64_t i = 0; i < n; i++) {
	//           sum += arr[i];
	//       }
	//       return sum;
	//   }
	//
	// ARM64 assembly (hand-assembled):
	//   0x1000: MOV X2, #0          ; sum = 0           (MOVZ X2, #0)
	//   0x1004: MOV X3, #0          ; i = 0             (MOVZ X3, #0)
	//   0x1008: CMP X3, X1          ; compare i, n
	//   0x100C: B.GE #0x20          ; if i >= n, goto done (offset=0x20-0x0C=0x14... actually relative)
	//   0x1010: LDR X4, [X0, X3, LSL #3] ; load arr[i]
	//   0x1014: ADD X2, X2, X4      ; sum += arr[i]
	//   0x1018: ADD X3, X3, #1      ; i++
	//   0x101C: B #-0x14            ; goto loop (back to CMP)
	//   0x1020: MOV X0, X2          ; return sum
	//   0x1024: RET                  ; return

	// Hand-assembled encodings:
	code := []uint32{
		0xD2800002, // MOVZ X2, #0
		0xD2800003, // MOVZ X3, #0
		0xEB01007F, // CMP X3, X1
		0x5400008A, // B.GE #+0x10 (to offset 0x1C from B.GE at 0x0C, 4 instructions forward)
		// Wait, I need to be more careful with offsets.
		// B.GE #+16 means 4 instructions ahead = to offset 0x1C from 0x0C = 0x10 + 0x0C = 0x1C... nope
		// B.cond uses imm19*4, from current PC.
		// At PC=0x100C, target=0x1020. Offset = 0x1020-0x100C = 0x14 = 20 bytes = 5 instructions.
		// Hmm, but I'm loading at base 0x1000. Let me adjust.
	}
	_ = code

	// Let me use absolute addresses. Load code at 0x1000.
	// Instructions are at 0x1000, 0x1004, 0x1008, ...
	//
	// 0x1000: MOVZ X2, #0           => 0xD2800002
	// 0x1004: MOVZ X3, #0           => 0xD2800003
	// 0x1008: CMP X3, X1            => 0xEB01007F
	// 0x100C: B.GE #0x14            => 0x540000AA (target = 0x100C + 0x14 = 0x1020)
	//         imm19 = 0x14/4 = 5, cond = 0xA (GE)
	//         encoding: 0101_0100 0000_0000 0000_0000 1010_1010
	//         Actually: B.cond format is 0101_0100 imm19[18:0] 0 cond[3:0]
	//         imm19 = 5 = 0x00005
	//         0x54 | (5 << 5) | 0xA = 0x540000AA
	// 0x1010: LDR X4, [X0, X3, LSL #3] => 0xF8637804
	// 0x1014: ADD X2, X2, X4        => 0x8B040042
	// 0x1018: ADD X3, X3, #1        => 0x91000463
	// 0x101C: B #-0x14              => 0x17FFFFFB (target = 0x101C + (-0x14) = 0x1008)
	//         imm26 = -0x14/4 = -5 = 0x3FFFFFB
	// 0x1020: MOV X0, X2            => ORR X0, XZR, X2 => 0xAA0203E0
	// 0x1024: RET                   => 0xD65F03C0

	program := []uint32{
		0xD2800002, // MOVZ X2, #0
		0xD2800003, // MOVZ X3, #0
		0xEB01007F, // CMP X3, X1
		0x540000AA, // B.GE #0x14 -> 0x1020
		0xF8637804, // LDR X4, [X0, X3, LSL #3]
		0x8B040042, // ADD X2, X2, X4
		0x91000463, // ADD X3, X3, #1
		0x17FFFFFB, // B #-0x14 -> 0x1008
		0xAA0203E0, // MOV X0, X2 (ORR X0, XZR, X2)
		0xD65F03C0, // RET
	}

	codeBase := uint64(0x1000)
	dataBase := uint64(0x2000)
	haltAddr := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 1000

	// Load program
	buf := make([]byte, len(program)*4)
	for i, inst := range program {
		binary.LittleEndian.PutUint32(buf[i*4:], inst)
	}
	em.LoadProgram(codeBase, buf)

	// Load data: array of 5 elements [10, 20, 30, 40, 50]
	arr := []uint64{10, 20, 30, 40, 50}
	dataBuf := make([]byte, len(arr)*8)
	for i, v := range arr {
		binary.LittleEndian.PutUint64(dataBuf[i*8:], v)
	}
	em.LoadProgram(dataBase, dataBuf)

	// Set up registers
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, dataBase) // X0 = array pointer
	em.Regs.WriteX(1, 5)       // X1 = length
	em.Regs.WriteX(30, haltAddr) // LR = halt address

	em.SetHaltPC(haltAddr)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Check result: sum should be 10+20+30+40+50 = 150
	result := em.Regs.ReadX(0)
	if result != 150 {
		t.Errorf("array sum = %d, want 150", result)
	}

	t.Logf("Array sum executed in %d instructions", em.InstCount)
}

func TestEmulatorSimpleSequence(t *testing.T) {
	// Simple test: MOV X0, #42; RET
	program := []uint32{
		0xD2800540, // MOVZ X0, #42
		0xD65F03C0, // RET
	}

	em := NewEmulator()
	buf := make([]byte, len(program)*4)
	for i, inst := range program {
		binary.LittleEndian.PutUint32(buf[i*4:], inst)
	}
	em.LoadProgram(0, buf)
	em.Regs.WriteX(30, 0xDEAD)
	em.SetHaltPC(0xDEAD)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	assertX(t, em, 0, 42)
}

func TestEmulatorFibonacci(t *testing.T) {
	// Compute fibonacci(10) iteratively:
	//   X0 = n (input)
	//   X1 = a = 0
	//   X2 = b = 1
	// loop:
	//   CBZ X0, done
	//   X3 = a + b
	//   a = b
	//   b = X3
	//   X0--
	//   B loop
	// done:
	//   X0 = a
	//   RET

	// PC layout:
	// 0x00: MOV X1, #0
	// 0x04: MOV X2, #1
	// 0x08: CBZ X0, #+0x18 -> 0x20 (done)
	// 0x0C: ADD X3, X1, X2
	// 0x10: MOV X1, X2
	// 0x14: MOV X2, X3
	// 0x18: SUB X0, X0, #1
	// 0x1C: B #-0x14 -> 0x08 (loop)
	// 0x20: MOV X0, X1
	// 0x24: RET
	program := []uint32{
		0xD2800001, // MOV X1, #0          (a = 0)
		0xD2800022, // MOV X2, #1          (b = 1)
		0xB40000C0, // CBZ X0, #+0x18 -> done at 0x20
		0x8B020023, // ADD X3, X1, X2      (X3 = a + b)
		0xAA0203E1, // MOV X1, X2          (a = b)
		0xAA0303E2, // MOV X2, X3          (b = X3)
		0xD1000400, // SUB X0, X0, #1      (n--)
		0x17FFFFFB, // B #-0x14 -> 0x08    (loop back to CBZ)
		0xAA0103E0, // MOV X0, X1          (return a)
		0xD65F03C0, // RET
	}

	em := NewEmulator()
	buf := make([]byte, len(program)*4)
	for i, inst := range program {
		binary.LittleEndian.PutUint32(buf[i*4:], inst)
	}
	em.LoadProgram(0, buf)
	em.Regs.WriteX(0, 10) // fib(10)
	em.Regs.WriteX(30, 0xFFFF)
	em.SetHaltPC(0xFFFF)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// fib(10) = 55
	result := em.Regs.ReadX(0)
	if result != 55 {
		t.Errorf("fibonacci(10) = %d, want 55", result)
	}
	t.Logf("Fibonacci executed in %d instructions", em.InstCount)
}

func TestBLIntercept_Run(t *testing.T) {
	// Program at 0x1000:
	//   0x1000: FMOV D0, D1         ; set up input (D0 = D1)
	//   0x1004: BL #0x1000          ; call hook at 0x2004 (offset = 0x2004 - 0x1004 = 0x1000)
	//   0x1008: RET                 ; return

	// BL encoding: imm26 = offset/4 = 0x1000/4 = 0x400
	// BL: 1001_01 imm26 => 0x94000400
	blEnc := uint32(0x94000400)
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], blEnc)
	inst, err := arm64asm.Decode(buf[:])
	if err != nil {
		t.Fatalf("decode BL: %v", err)
	}
	if inst.Op != arm64asm.BL {
		t.Fatalf("expected BL, got %s", inst.Op)
	}

	program := []uint32{
		0x1E604020, // FMOV D0, D1 (copy D1 to D0)
		blEnc,      // BL #0x1000 → target = 0x1004 + 0x1000 = 0x2004
		0xD65F03C0, // RET
	}

	codeBase := uint64(0x1000)
	haltAddr := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 100
	codeBuf := make([]byte, len(program)*4)
	for i, inst := range program {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	em.Regs.PC = codeBase
	em.Regs.WriteX(30, haltAddr) // LR
	em.SetHaltPC(haltAddr)

	// Set D1 = 2.0 (the input for exp)
	em.Regs.WriteD(1, 2.0)

	// Register hook at 0x2004: exp(D0) → D0
	hookAddr := uint64(0x2004) // target of BL at 0x1004
	hookCalled := false
	em.RegisterFuncHook(hookAddr, func(em *Emulator) {
		hookCalled = true
		input := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Exp(input))
	})

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !hookCalled {
		t.Fatal("hook was not called")
	}
	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	got := em.Regs.ReadD(0)
	want := math.Exp(2.0)
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("D0 = %f, want %f (exp(2.0))", got, want)
	}
}

func TestBLIntercept_StepDetailed(t *testing.T) {
	// Same program as above, but use StepDetailed().
	blEnc := uint32(0x94000400) // BL #0x1000

	program := []uint32{
		0x1E604020, // FMOV D0, D1
		blEnc,      // BL → 0x2004
		0xD65F03C0, // RET
	}

	codeBase := uint64(0x1000)
	haltAddr := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 100
	codeBuf := make([]byte, len(program)*4)
	for i, inst := range program {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	em.Regs.PC = codeBase
	em.Regs.WriteX(30, haltAddr)
	em.SetHaltPC(haltAddr)
	em.Regs.WriteD(1, 1.0) // exp(1.0) = e ≈ 2.71828

	hookAddr := uint64(0x2004)
	hookCalled := false
	em.RegisterFuncHook(hookAddr, func(em *Emulator) {
		hookCalled = true
		input := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Exp(input))
	})

	// Step 1: FMOV D0, D1
	inst1, branched1, err := em.StepDetailed()
	if err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if inst1.Op != arm64asm.FMOV {
		t.Errorf("step 1: expected FMOV, got %s", inst1.Op)
	}
	if branched1 {
		t.Error("step 1: should not branch")
	}

	// Step 2: BL (intercepted)
	inst2, branched2, err := em.StepDetailed()
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if inst2.Op != arm64asm.BL {
		t.Errorf("step 2: expected BL, got %s", inst2.Op)
	}
	if !hookCalled {
		t.Fatal("hook was not called during StepDetailed")
	}
	// After hook: PC should be LR (the return address after BL), which is 0x1008
	// The BL sets LR = PC+4 = 0x1008, then hook runs, PC = LR = 0x1008
	// branched should be false since PC = pcBefore + 4 = 0x1004 + 4 = 0x1008
	if branched2 {
		t.Error("step 2: intercepted BL should not appear as branched (PC = next inst)")
	}

	// Step 3: RET → halts
	_, _, err = em.StepDetailed()
	if err != nil {
		t.Fatalf("step 3: %v", err)
	}

	got := em.Regs.ReadD(0)
	want := math.Exp(1.0)
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("D0 = %f, want %f (exp(1.0))", got, want)
	}
}
