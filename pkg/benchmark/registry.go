package benchmark

import (
	"encoding/binary"
	"math"

	"github.com/sarchlab/poros/pkg/emu"
)

// All returns the complete set of benchmark kernel definitions.
func All() []Benchmark {
	// setupAtaxEmu creates a fresh emulator configured for the atax kernel.
	// MEDIUM dataset: M=390, N=410
	setupAtaxEmu := func() *emu.Emulator {
		m, n := 390, 410
		A := make([]float64, m*n)
		x := make([]float64, n)
		for i := 0; i < n; i++ {
			x[i] = 1.0 + float64(i)/float64(n)
		}
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				A[i*n+j] = float64((i+j)%n) / float64(5*m)
			}
		}

		code := []uint32{
			0x7100041f, 0x5400046b, 0xd2800008, 0x2a0103e9,
			0xd37d7c2a, 0x2a0003eb, 0x14000005, 0x91000508,
			0x8b0a0042, 0xeb0b011f, 0x54000340, 0xf82878bf,
			0x7100043f, 0x54ffff4b, 0x2f00e400, 0xaa0903ec,
			0xaa0303ed, 0xaa0203ee, 0xfc4085c1, 0xfc4085a2,
			0x1f420020, 0xfc2878a0, 0xf100058c, 0x54ffff61,
			0x7100043f, 0x54fffdcb, 0xd280000c, 0xfc6c6880,
			0xfc6c6841, 0xfc6878a2, 0x1f420020, 0xfc2c6880,
			0x9100218c, 0xeb0c015f, 0x54ffff21, 0x17ffffe4,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		xBase := uint64(0x300000)
		yBase := uint64(0x400000)
		tmpBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, m*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		xBuf := make([]byte, n*8)
		for i, v := range x {
			binary.LittleEndian.PutUint64(xBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(xBase, xBuf)
		em.LoadProgram(yBase, make([]byte, n*8))
		em.LoadProgram(tmpBase, make([]byte, m*8))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, xBase)
		em.Regs.WriteX(4, yBase)
		em.Regs.WriteX(5, tmpBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x600000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// setupMvtEmu creates a fresh emulator configured for the mvt kernel.
	// MEDIUM dataset: N=400
	setupMvtEmu := func() *emu.Emulator {
		n := 400
		x1 := make([]float64, n)
		x2 := make([]float64, n)
		y1 := make([]float64, n)
		y2 := make([]float64, n)
		A := make([]float64, n*n)

		for i := 0; i < n; i++ {
			x1[i] = float64(i%n) / float64(n)
			x2[i] = float64((i+1)%n) / float64(n)
			y1[i] = float64((i+3)%n) / float64(n)
			y2[i] = float64((i+4)%n) / float64(n)
			for j := 0; j < n; j++ {
				A[i*n+j] = float64(i*j%n) / float64(n)
			}
		}

		code := []uint32{
			0x7100041f, 0x540004cb, 0xd2800009, 0xd37d7c0a,
			0xaa0503eb, 0x2a0003e8, 0xfc697820, 0xaa0803ec,
			0xaa0303ed, 0xaa0b03ee, 0xfc4085c1, 0xfc4085a2,
			0x1f420020, 0xfc297820, 0xf100058c, 0x54ffff61,
			0x91000529, 0x8b0a016b, 0xeb08013f, 0x54fffe61,
			0x7100041f, 0x5400024b, 0xd2800009, 0xd37d7c0a,
			0xfc697840, 0xaa0803eb, 0xaa0403ec, 0xaa0503ed,
			0xfc408581, 0xfd4001a2, 0x1f410040, 0xfc297840,
			0x8b0a01ad, 0xf100056b, 0x54ffff41, 0x91000529,
			0x910020a5, 0xeb08013f, 0x54fffe41, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		x1Base := uint64(0x100000)
		x2Base := uint64(0x200000)
		y1Base := uint64(0x300000)
		y2Base := uint64(0x400000)
		aBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		x1Buf := make([]byte, n*8)
		for i, v := range x1 {
			binary.LittleEndian.PutUint64(x1Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(x1Base, x1Buf)

		x2Buf := make([]byte, n*8)
		for i, v := range x2 {
			binary.LittleEndian.PutUint64(x2Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(x2Base, x2Buf)

		y1Buf := make([]byte, n*8)
		for i, v := range y1 {
			binary.LittleEndian.PutUint64(y1Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(y1Base, y1Buf)

		y2Buf := make([]byte, n*8)
		for i, v := range y2 {
			binary.LittleEndian.PutUint64(y2Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(y2Base, y2Buf)

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, x1Base)
		em.Regs.WriteX(2, x2Base)
		em.Regs.WriteX(3, y1Base)
		em.Regs.WriteX(4, y2Base)
		em.Regs.WriteX(5, aBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x800000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// setupGemmEmu creates a fresh emulator configured for the gemm kernel.
	setupGemmEmu := func() *emu.Emulator {
		ni, nj, nk := 60, 70, 80
		alpha, beta := 1.5, 1.2

		C := make([]float64, ni*nj)
		A := make([]float64, ni*nk)
		B := make([]float64, nk*nj)

		for i := 0; i < ni; i++ {
			for j := 0; j < nj; j++ {
				C[i*nj+j] = float64((i*j+1)%ni) / float64(ni)
			}
		}
		for i := 0; i < ni; i++ {
			for j := 0; j < nk; j++ {
				A[i*nk+j] = float64(i*(j+1)%nk) / float64(nk)
			}
		}
		for i := 0; i < nk; i++ {
			for j := 0; j < nj; j++ {
				B[i*nj+j] = float64(i*(j+2)%nj) / float64(nj)
			}
		}

		code := []uint32{
			0x7100041f, 0x5400066b, 0xa9bf4ff4, 0x52800008,
			0xd2800009, 0x2a0103ea, 0x2a0203eb, 0xd37d7c2c,
			0xaa0303ed, 0x2a0003ee, 0x14000006, 0x91000529,
			0x8b0c01ad, 0x0b010108, 0xeb0e013f, 0x54000480,
			0xaa0a03ef, 0xaa0d03f0, 0x7100043f, 0x540000cb,
			0xfd400202, 0x1e610842, 0xfc008602, 0xf10005ef,
			0x54ffff81, 0x7100045f, 0x54fffe2b, 0xd280000f,
			0x8b284c70, 0x9b0b7d31, 0x8b110c91, 0xaa0503e0,
			0x14000005, 0x910005ef, 0x8b0c0000, 0xeb0b01ff,
			0x54fffce0, 0xaa0a03e6, 0xaa0003e7, 0xaa1003f3,
			0x7100043f, 0x54ffff0b, 0xfc6f7a22, 0xfc4084e3,
			0x1e600842, 0xfd400264, 0x1f431042, 0xfc008662,
			0xf10004c6, 0x54ffff21, 0x17ffffef, 0xa8c14ff4,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		cBase := uint64(0x100000)
		aBase := uint64(0x200000)
		bBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		cBuf := make([]byte, ni*nj*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, ni*nk*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, nk*nj*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(ni))
		em.Regs.WriteX(1, uint64(nj))
		em.Regs.WriteX(2, uint64(nk))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteD(1, beta)
		em.Regs.WriteX(3, cBase)
		em.Regs.WriteX(4, aBase)
		em.Regs.WriteX(5, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// setup2mmEmu creates a fresh emulator configured for the 2mm kernel.
	setup2mmEmu := func() *emu.Emulator {
		ni, nj, nk, nl := 40, 50, 70, 80
		alpha, beta := 1.5, 1.2

		tmp := make([]float64, ni*nj)
		A := make([]float64, ni*nk)
		B := make([]float64, nk*nj)
		C := make([]float64, nj*nl)
		D := make([]float64, ni*nl)

		for i := 0; i < ni; i++ {
			for j := 0; j < nk; j++ {
				A[i*nk+j] = float64((i*j+1)%ni) / float64(ni)
			}
		}
		for i := 0; i < nk; i++ {
			for j := 0; j < nj; j++ {
				B[i*nj+j] = float64(i*(j+1)%nj) / float64(nj)
			}
		}
		for i := 0; i < nj; i++ {
			for j := 0; j < nl; j++ {
				C[i*nl+j] = float64((i*(j+3)+1)%nl) / float64(nl)
			}
		}
		for i := 0; i < ni; i++ {
			for j := 0; j < nl; j++ {
				D[i*nl+j] = float64(i*(j+2)%nk) / float64(nk)
			}
		}

		code := []uint32{
			0x7100041f, 0x54000a8b, 0xa9be57f6, 0xa9014ff4,
			0x5280000b, 0xd280000c, 0x2a0103e8, 0xf94013e9,
			0x2a0003ea, 0xd37d7c2d, 0x2a0203ee, 0x14000005,
			0x9100058c, 0x0b02016b, 0xeb0a019f, 0x540003a0,
			0x7100043f, 0x54ffff6b, 0xd280000f, 0x8b2b4cb0,
			0x9b087d91, 0x8b110c91, 0xaa0603f3, 0x14000005,
			0x910005ef, 0x91002273, 0xeb0801ff, 0x54fffe20,
			0xf82f7a3f, 0x7100045f, 0x54ffff4b, 0x2f00e402,
			0xaa0e03f4, 0xaa1303f5, 0xaa1003f6, 0xfc4086c3,
			0x1e600863, 0xfd4002a4, 0x1f440862, 0xfc2f7a22,
			0x8b0d02b5, 0xf1000694, 0x54ffff21, 0x17ffffed,
			0x7100041f, 0x540004cb, 0x5280000b, 0xd280000c,
			0xd37d7c6d, 0x2a0303ee, 0x14000005, 0x9100058c,
			0x0b01016b, 0xeb0a019f, 0x540003a0, 0x7100047f,
			0x54ffff6b, 0xd280000f, 0x8b2b4c90, 0x9b0e7d91,
			0x8b110d31, 0xaa0703e0, 0x14000005, 0x910005ef,
			0x91002000, 0xeb0e01ff, 0x54fffe20, 0xfc6f7a20,
			0x1e610800, 0xfc2f7a20, 0xaa0803e2, 0xaa0003e5,
			0xaa1003e6, 0x7100043f, 0x54fffeab, 0xfc4084c2,
			0xfd4000a3, 0x1f430040, 0xfc2f7a20, 0x8b0d00a5,
			0xf1000442, 0x54ffff41, 0x17ffffed, 0xa9414ff4,
			0xa8c257f6, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		tmpBase := uint64(0x100000)
		aBase := uint64(0x200000)
		bBase := uint64(0x300000)
		cBase := uint64(0x400000)
		dBase := uint64(0x500000)
		stackBase := uint64(0x600000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		tmpBuf := make([]byte, ni*nj*8)
		for i, v := range tmp {
			binary.LittleEndian.PutUint64(tmpBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(tmpBase, tmpBuf)

		aBuf := make([]byte, ni*nk*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, nk*nj*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		cBuf := make([]byte, nj*nl*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		dBuf := make([]byte, ni*nl*8)
		for i, v := range D {
			binary.LittleEndian.PutUint64(dBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(dBase, dBuf)

		stackBuf := make([]byte, 8192)
		binary.LittleEndian.PutUint64(stackBuf[4096:], dBase)
		em.LoadProgram(stackBase, stackBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(ni))
		em.Regs.WriteX(1, uint64(nj))
		em.Regs.WriteX(2, uint64(nk))
		em.Regs.WriteX(3, uint64(nl))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteD(1, beta)
		em.Regs.WriteX(4, tmpBase)
		em.Regs.WriteX(5, aBase)
		em.Regs.WriteX(6, bBase)
		em.Regs.WriteX(7, cBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 4096
		em.SetHaltPC(haltPC)
		return em
	}

	// setup3mmEmu creates a fresh emulator configured for the 3mm kernel.
	setup3mmEmu := func() *emu.Emulator {
		ni, nj, nk, nl, nm := 40, 50, 60, 70, 80

		A := make([]float64, ni*nk)
		B := make([]float64, nk*nj)
		C := make([]float64, nj*nm)
		D := make([]float64, nm*nl)
		E := make([]float64, ni*nj)
		F := make([]float64, nj*nl)
		G := make([]float64, ni*nl)

		for i := 0; i < ni; i++ {
			for j := 0; j < nk; j++ {
				A[i*nk+j] = float64((i*j+1)%ni) / float64(5*ni)
			}
		}
		for i := 0; i < nk; i++ {
			for j := 0; j < nj; j++ {
				B[i*nj+j] = float64((i*(j+1)+2)%nj) / float64(5*nj)
			}
		}
		for i := 0; i < nj; i++ {
			for j := 0; j < nm; j++ {
				C[i*nm+j] = float64(i*(j+3)%nl) / float64(5*nl)
			}
		}
		for i := 0; i < nm; i++ {
			for j := 0; j < nl; j++ {
				D[i*nl+j] = float64((i*(j+2)+2)%nk) / float64(5*nk)
			}
		}

		code := []uint32{
			0xa9be57f6, 0xa9014ff4, 0x2a0103e8, 0x2a0003e9,
			0x7100041f, 0x540004ab, 0x5280000a, 0xd280000b,
			0xd37d7c2c, 0x2a0203ed, 0x14000005, 0x9100056b,
			0x0b02014a, 0xeb09017f, 0x54000380, 0x7100043f,
			0x54ffff6b, 0xd280000e, 0x8b2a4ccf, 0x9b087d70,
			0x8b100cb0, 0xaa0703f1, 0x14000005, 0x910005ce,
			0x91002231, 0xeb0801df, 0x54fffe20, 0xf82e7a1f,
			0x7100045f, 0x54ffff4b, 0x2f00e400, 0xaa0d03f3,
			0xaa1103f4, 0xaa0f03f5, 0xfc4086a1, 0xfd400282,
			0x1f420020, 0xfc2e7a00, 0x8b0c0294, 0xf1000673,
			0x54ffff41, 0x17ffffee, 0xf94013ea, 0x2a0303eb,
			0x7100043f, 0x540004cb, 0x5280000c, 0xd280000d,
			0xa942bbef, 0xd37d7c70, 0x2a0403f1, 0x14000005,
			0x910005ad, 0x0b04018c, 0xeb0801bf, 0x54000380,
			0x7100047f, 0x54ffff6b, 0xd2800002, 0x8b2c4de6,
			0x9b0b7da7, 0x8b070d47, 0xaa0e03f3, 0x14000005,
			0x91000442, 0x91002273, 0xeb0b005f, 0x54fffe20,
			0xf82278ff, 0x7100049f, 0x54ffff4b, 0x2f00e400,
			0xaa1103f4, 0xaa1303f5, 0xaa0603f6, 0xfc4086c1,
			0xfd4002a2, 0x1f420020, 0xfc2278e0, 0x8b1002b5,
			0xf1000694, 0x54ffff41, 0x17ffffee, 0x7100041f,
			0x540004cb, 0x5280000c, 0xd280000d, 0xf9401fee,
			0xaa0303ef, 0xd37d7def, 0x14000005, 0x910005ad,
			0x0b01018c, 0xeb0901bf, 0x54000380, 0x7100047f,
			0x54ffff6b, 0xd2800010, 0x8b2c4cb1, 0x9b0b7da0,
			0x8b000dc0, 0xaa0a03e2, 0x14000005, 0x91000610,
			0x91002042, 0xeb0b021f, 0x54fffe20, 0xf830781f,
			0x7100043f, 0x54ffff4b, 0x2f00e400, 0xaa0803e4,
			0xaa0203e6, 0xaa1103e7, 0xfc4084e1, 0xfd4000c2,
			0x1f420020, 0xfc307800, 0x8b0f00c6, 0xf1000484,
			0x54ffff41, 0x17ffffee, 0xa9414ff4, 0xa8c257f6,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		eBase := uint64(0x100000)
		aBase := uint64(0x200000)
		bBase := uint64(0x300000)
		fBase := uint64(0x400000)
		cBase := uint64(0x500000)
		dBase := uint64(0x600000)
		gBase := uint64(0x700000)
		stackBase := uint64(0x800000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		eBuf := make([]byte, ni*nj*8)
		for i, v := range E {
			binary.LittleEndian.PutUint64(eBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(eBase, eBuf)

		aBuf := make([]byte, ni*nk*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, nk*nj*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		fBuf := make([]byte, nj*nl*8)
		for i, v := range F {
			binary.LittleEndian.PutUint64(fBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(fBase, fBuf)

		cBufData := make([]byte, nj*nm*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBufData[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBufData)

		dBufData := make([]byte, nm*nl*8)
		for i, v := range D {
			binary.LittleEndian.PutUint64(dBufData[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(dBase, dBufData)

		gBuf := make([]byte, ni*nl*8)
		for i, v := range G {
			binary.LittleEndian.PutUint64(gBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(gBase, gBuf)

		stackBuf := make([]byte, 8192)
		binary.LittleEndian.PutUint64(stackBuf[4096:], fBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+8:], cBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+16:], dBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+24:], gBase)
		em.LoadProgram(stackBase, stackBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(ni))
		em.Regs.WriteX(1, uint64(nj))
		em.Regs.WriteX(2, uint64(nk))
		em.Regs.WriteX(3, uint64(nl))
		em.Regs.WriteX(4, uint64(nm))
		em.Regs.WriteX(5, eBase)
		em.Regs.WriteX(6, aBase)
		em.Regs.WriteX(7, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 4096
		em.SetHaltPC(haltPC)
		return em
	}

	// setupBicgEmu creates a fresh emulator configured for the bicg kernel.
	setupBicgEmu := func() *emu.Emulator {
		m, n := 116, 124

		A := make([]float64, m*n)
		s := make([]float64, n)
		q := make([]float64, m)
		p := make([]float64, n)
		r := make([]float64, m)

		for i := 0; i < m; i++ {
			r[i] = float64(i%m) / float64(m)
			for j := 0; j < n; j++ {
				A[i*n+j] = float64(i*(j+1)%n) / float64(n)
			}
		}
		for i := 0; i < n; i++ {
			p[i] = float64(i%n) / float64(n)
		}

		code := []uint32{
			0x7100041f, 0x540003cb, 0xd2800008, 0x2a0103e9,
			0xd37d7c2a, 0x2a0003eb, 0x14000005, 0x91000508,
			0x8b0a0042, 0xeb0b011f, 0x540002a0, 0xf828789f,
			0xaa0903ec, 0xaa0303ed, 0xaa0503ee, 0xaa0203ef,
			0x7100043f, 0x54fffecb, 0xfd4001a0, 0xfc6878c1,
			0xfd4001e2, 0x1f420020, 0xfc0085a0, 0xfc687880,
			0xfc4085e1, 0xfc4085c2, 0x1f420020, 0xfc287880,
			0xf100058c, 0x54fffea1, 0x17ffffe9, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		sBase := uint64(0x200000)
		qBase := uint64(0x300000)
		pBase := uint64(0x400000)
		rBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, m*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		sBuf := make([]byte, n*8)
		for i, v := range s {
			binary.LittleEndian.PutUint64(sBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(sBase, sBuf)

		qBuf := make([]byte, m*8)
		for i, v := range q {
			binary.LittleEndian.PutUint64(qBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(qBase, qBuf)

		pBuf := make([]byte, n*8)
		for i, v := range p {
			binary.LittleEndian.PutUint64(pBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(pBase, pBuf)

		rBuf := make([]byte, m*8)
		for i, v := range r {
			binary.LittleEndian.PutUint64(rBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(rBase, rBuf)

		em.LoadProgram(uint64(0x600000), make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, sBase)
		em.Regs.WriteX(4, qBase)
		em.Regs.WriteX(5, pBase)
		em.Regs.WriteX(6, rBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x600000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// MEDIUM dataset: N=250
	setupGesummvEmu := func() *emu.Emulator {
		n := 250
		alpha, beta := 1.5, 1.2

		A := make([]float64, n*n)
		B := make([]float64, n*n)
		x := make([]float64, n)

		for i := 0; i < n; i++ {
			x[i] = float64(i%n) / float64(n)
			for j := 0; j < n; j++ {
				A[i*n+j] = float64((i*j+1)%n) / float64(n)
				B[i*n+j] = float64((i*j+2)%n) / float64(n)
			}
		}

		code := []uint32{
			0x7100041f, 0x540003eb, 0xd2800008, 0xd37d7c09,
			0x2a0003ea, 0xf828787f, 0xf82878bf, 0xaa0a03eb,
			0xaa0103ec, 0xaa0203ed, 0xaa0403ee, 0xfc408582,
			0xfd4001c3, 0xfc687864, 0x1f431042, 0xfc287862,
			0xfc4085a2, 0xfc4085c3, 0xfc6878a4, 0x1f431042,
			0xfc2878a2, 0xf100056b, 0x54fffea1, 0xfc687863,
			0x1e610842, 0x1f430802, 0xfc2878a2, 0x91000508,
			0x8b090042, 0x8b090021, 0xeb0a011f, 0x54fffcc1,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		bBase := uint64(0x200000)
		tmpBase := uint64(0x300000)
		xBase := uint64(0x400000)
		yBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, n*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(tmpBase, make([]byte, n*8))

		xBuf := make([]byte, n*8)
		for i, v := range x {
			binary.LittleEndian.PutUint64(xBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(xBase, xBuf)
		em.LoadProgram(yBase, make([]byte, n*8))

		em.LoadProgram(uint64(0x600000), make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteD(1, beta)
		em.Regs.WriteX(1, aBase)
		em.Regs.WriteX(2, bBase)
		em.Regs.WriteX(3, tmpBase)
		em.Regs.WriteX(4, xBase)
		em.Regs.WriteX(5, yBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x600000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupGemverEmu := func() *emu.Emulator {
		n := 120
		fn := float64(n)

		A := make([]float64, n*n)
		u1 := make([]float64, n)
		v1 := make([]float64, n)
		u2 := make([]float64, n)
		v2 := make([]float64, n)
		w := make([]float64, n)
		x := make([]float64, n)
		y := make([]float64, n)
		z := make([]float64, n)

		for i := 0; i < n; i++ {
			u1[i] = float64(i)
			u2[i] = (float64(i+1) / fn) / 2.0
			v1[i] = (float64(i+1) / fn) / 4.0
			v2[i] = (float64(i+1) / fn) / 6.0
			y[i] = (float64(i+1) / fn) / 8.0
			z[i] = (float64(i+1) / fn) / 9.0
			for j := 0; j < n; j++ {
				A[i*n+j] = float64(i*j%n) / float64(n)
			}
		}

		code := []uint32{
			0x7100041f, 0x5400086b, 0xd280000a, 0xa94027eb,
			0xd37d7c0c, 0xaa0103ed, 0x2a0003e8, 0xaa0803ee,
			0xaa0503ef, 0xaa0303f0, 0xaa0d03f1, 0xfc6a7842,
			0xfd400223, 0xfc408604, 0x1f440c42, 0xfc6a7883,
			0xfc4085e4, 0x1f440862, 0xfc008622, 0xf10005ce,
			0x54fffee1, 0x9100054a, 0x8b0c01ad, 0xeb08015f,
			0x54fffde1, 0xd280000c, 0xd37df10a, 0xaa0103ed,
			0xfc6c78e2, 0xaa0803ee, 0xaa0d03ef, 0xaa0b03f0,
			0xfd4001e3, 0xfc408604, 0x1e610863, 0x1f440862,
			0xfc2c78e2, 0x8b0a01ef, 0xf10005ce, 0x54ffff21,
			0x9100058c, 0x910021ad, 0xeb08019f, 0x54fffe21,
			0xaa0803eb, 0xaa0703ec, 0xfd400181, 0xfc408522,
			0x1e622821, 0xfc008581, 0xf100056b, 0x54ffff61,
			0xd2800009, 0xfc6978c1, 0xaa0803eb, 0xaa0703ec,
			0xaa0103ed, 0xfc4085a2, 0x1e600842, 0xfc408583,
			0x1f430441, 0xfc2978c1, 0xf100056b, 0x54ffff41,
			0x91000529, 0x8b0a0021, 0xeb08013f, 0x54fffe41,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		u1Base := uint64(0x200000)
		v1Base := uint64(0x300000)
		u2Base := uint64(0x400000)
		v2Base := uint64(0x500000)
		wBase := uint64(0x600000)
		xBase := uint64(0x700000)
		yBase := uint64(0x800000)
		zBase := uint64(0x900000)
		stackBase := uint64(0xA00000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		loadVec := func(base uint64, data []float64) {
			buf := make([]byte, len(data)*8)
			for i, v := range data {
				binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
			}
			em.LoadProgram(base, buf)
		}
		loadVec(u1Base, u1)
		loadVec(v1Base, v1)
		loadVec(u2Base, u2)
		loadVec(v2Base, v2)
		loadVec(wBase, w)
		loadVec(xBase, x)
		loadVec(yBase, y)
		loadVec(zBase, z)

		stackBuf := make([]byte, 8192)
		binary.LittleEndian.PutUint64(stackBuf[4096:], yBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+8:], zBase)
		em.LoadProgram(stackBase, stackBuf)

		alpha, beta := 1.5, 1.2
		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteD(1, beta)
		em.Regs.WriteX(1, aBase)
		em.Regs.WriteX(2, u1Base)
		em.Regs.WriteX(3, v1Base)
		em.Regs.WriteX(4, u2Base)
		em.Regs.WriteX(5, v2Base)
		em.Regs.WriteX(6, wBase)
		em.Regs.WriteX(7, xBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 4096
		em.SetHaltPC(haltPC)
		return em
	}

	setupDoitgenEmu := func() *emu.Emulator {
		nr, nq, np := 25, 20, 30

		A := make([]float64, nr*nq*np)
		C4 := make([]float64, np*np)

		for i := 0; i < nr; i++ {
			for j := 0; j < nq; j++ {
				for k := 0; k < np; k++ {
					A[i*nq*np+j*np+k] = float64((i*j+k)%np) / float64(np)
				}
			}
		}
		for i := 0; i < np; i++ {
			for j := 0; j < np; j++ {
				C4[i*np+j] = float64(i*j%np) / float64(np)
			}
		}

		code := []uint32{
			0xf100041f, 0x5400056b, 0xf100043f, 0x5400052b,
			0xf100045f, 0x540004eb, 0xd2800008, 0xd37df049,
			0x9b017c4a, 0xd37df14a, 0xd280000b, 0xaa0303ec,
			0xd280000d, 0xaa0403ee, 0xd280000f, 0xf82d78bf,
			0x2f00e400, 0xaa0e03f0, 0xfc6f7981, 0xfd400202,
			0x1f420020, 0xfc2d78a0, 0x910005ef, 0x8b090210,
			0xeb0f005f, 0x54ffff21, 0x910005ad, 0x910021ce,
			0xeb0201bf, 0x54fffe21, 0xd280000d, 0xfc6d78a0,
			0xfc2d7980, 0x910005ad, 0xeb0d005f, 0x54ffff81,
			0x9100056b, 0x8b09018c, 0xeb01017f, 0x54fffca1,
			0x91000508, 0x8b0a0063, 0xeb00011f, 0x54fffbe1,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		c4Base := uint64(0x200000)
		sumBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, nr*nq*np*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		c4Buf := make([]byte, np*np*8)
		for i, v := range C4 {
			binary.LittleEndian.PutUint64(c4Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(c4Base, c4Buf)

		em.LoadProgram(sumBase, make([]byte, np*8))
		em.LoadProgram(uint64(0x400000), make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(nr))
		em.Regs.WriteX(1, uint64(nq))
		em.Regs.WriteX(2, uint64(np))
		em.Regs.WriteX(3, aBase)
		em.Regs.WriteX(4, c4Base)
		em.Regs.WriteX(5, sumBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x400000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupTrisolvEmu := func() *emu.Emulator {
		n := 400

		L := make([]float64, n*n)
		b := make([]float64, n)
		for i := 0; i < n; i++ {
			b[i] = float64(i)
			for j := 0; j <= i; j++ {
				L[i*n+j] = float64(i+n-j+1) * 2 / float64(n)
			}
		}

		code := []uint32{
			0x7100041f, 0x5400036b, 0x52800008, 0xd2800009,
			0x1100040a, 0x2a0003eb, 0x14000009, 0x1b097d4c,
			0xfc6cd821, 0x1e611800, 0xfc297840, 0x91000529,
			0x0b000108, 0xeb0b013f, 0x540001c0, 0xfc697860,
			0xfc297840, 0xb4fffec9, 0xd280000c, 0x8b284c2d,
			0xfc6c79a1, 0xfc6c7842, 0x1f428020, 0xfc297840,
			0x9100058c, 0xeb0c013f, 0x54ffff41, 0x17ffffec,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		lBase := uint64(0x100000)
		xBase := uint64(0x300000)
		bBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		lBuf := make([]byte, n*n*8)
		for i, v := range L {
			binary.LittleEndian.PutUint64(lBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(lBase, lBuf)

		em.LoadProgram(xBase, make([]byte, n*8))

		bBuf := make([]byte, n*8)
		for i, v := range b {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, lBase)
		em.Regs.WriteX(2, xBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x500000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupDurbinEmu := func() *emu.Emulator {
		n := 400

		r := make([]float64, n)
		for i := 0; i < n; i++ {
			r[i] = float64(n + 1 - i)
		}

		code := []uint32{
			0xfd400020, 0x1e614000, 0xfd000040, 0x7100081f,
			0x54000a6b, 0xd2800008, 0xd2800009, 0xfd400020,
			0x1e614002, 0x2a0003ea, 0x9100206b, 0x9100204c,
			0x1e6e1000, 0x9280000d, 0x5280002e, 0xb201f3ef,
			0xf295556f, 0x1e6e1001, 0x14000008, 0xfc2e7842,
			0x910005ce, 0x91000529, 0x91002108, 0xd10005ad,
			0xeb0a01df, 0x540007c0, 0x1f428043, 0x2f00e402,
			0xaa0203f0, 0xaa0803f1, 0xfc716824, 0xfc408605,
			0x1f450882, 0xd1002231, 0xb100223f, 0x54ffff61,
			0xd2800010, 0x1e610861, 0xfc6e7823, 0x1e632842,
			0x1e614042, 0x1e611842, 0xaa0803f1, 0xfc706843,
			0xfc716844, 0x1f440c43, 0xfc306863, 0x91002210,
			0xd1002231, 0xb100223f, 0x54ffff21, 0xd2800000,
			0x9bcf7d30, 0xd341fe10, 0x8b100610, 0xcb100130,
			0x91000611, 0xf1000e3f, 0x9a9007f0, 0xf100093f,
			0x54000223, 0xd2800000, 0x8b0d0204, 0xaa0c03e5,
			0xaa0b03e6, 0xfc5f80c3, 0xfc1f80a3, 0xfd4000c3,
			0xfd0000a3, 0xfd4004c3, 0xfd0004a3, 0xd1000c00,
			0x910060c6, 0x910060a5, 0xeb00009f, 0x54fffec1,
			0xcb0003e0, 0xf1000e3f, 0x54fff8a0, 0xd37df000,
			0x8b000051, 0x8b000060, 0xfc408403, 0xfc008623,
			0xf1000610, 0x54ffffa1, 0x17ffffbd, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		rBase := uint64(0x100000)
		yBase := uint64(0x200000)
		zBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		rBuf := make([]byte, n*8)
		for i, v := range r {
			binary.LittleEndian.PutUint64(rBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(rBase, rBuf)

		em.LoadProgram(yBase, make([]byte, n*8))
		em.LoadProgram(zBase, make([]byte, n*8))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, rBase)
		em.Regs.WriteX(2, yBase)
		em.Regs.WriteX(3, zBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x400000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupJacobi1dEmu := func() *emu.Emulator {
		tsteps := 100
		n := 400

		A := make([]float64, n)
		B := make([]float64, n)
		for i := 0; i < n; i++ {
			A[i] = (float64(i) + 2) / float64(n)
			B[i] = (float64(i) + 3) / float64(n)
		}

		code := []uint32{
			0x7100041f, 0x540004cb, 0x71000c3f, 0x5400048b,
			0x52800008, 0x5100042b, 0x91002049, 0x9100206a,
			0xd100056b, 0xd29497cc, 0xf2ab462c, 0xf2caa8ec,
			0xf2e7faac, 0x9e670180, 0xaa0b03ec, 0xaa0a03ed,
			0xaa0903ee, 0x6d7f89c1, 0xfc408dc3, 0x1e622821,
			0x1e632821, 0x1e600821, 0xfc0085a1, 0xf100058c,
			0x54ffff21, 0xaa0b03ec, 0xaa0903ed, 0xaa0a03ee,
			0x6d7f89c1, 0xfc408dc3, 0x1e622821, 0x1e632821,
			0x1e600821, 0xfc0085a1, 0xf100058c, 0x54ffff21,
			0x11000508, 0x6b00011f, 0x54fffd01, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		bBase := uint64(0x200000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(tsteps))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x300000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupSyrkEmu := func() *emu.Emulator {
		n, m := 80, 60
		alpha, beta := 1.5, 1.2

		C := make([]float64, n*n)
		A := make([]float64, n*m)

		for i := 0; i < n; i++ {
			for j := 0; j < m; j++ {
				A[i*m+j] = float64((i*j+1)%n) / float64(n)
			}
		}
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				C[i*n+j] = float64((i*j+2)%m) / float64(m)
			}
		}

		code := []uint32{
			0x7100041f, 0x5400074b, 0x7100003f, 0x5400050d,
			0xd2800008, 0x2a0103e9, 0xd37d7c0a, 0xd37d7c2b,
			0x5280002c, 0x2a0003ed, 0xd280000e, 0xfc6e7842,
			0x1e610842, 0xfc2e7842, 0x910005ce, 0xeb0e019f,
			0x54ffff61, 0xd280000e, 0x9b097d0f, 0x8b0f0c6f,
			0xaa0303f0, 0xd2800011, 0xaa1003e0, 0xfc6e79e2,
			0x1e600842, 0xfd400003, 0xfc717844, 0x1f431042,
			0xfc317842, 0x91000631, 0x8b0b0000, 0xeb11019f,
			0x54fffee1, 0x910005ce, 0x91002210, 0xeb0901df,
			0x54fffe21, 0x91000508, 0x9100058c, 0x8b0a0042,
			0xeb0d011f, 0x54fffc21, 0x14000011, 0xd2800008,
			0xd37d7c09, 0x5280002a, 0x2a0003eb, 0xd280000c,
			0xfc6c7840, 0x1e610800, 0xfc2c7840, 0x9100058c,
			0xeb0c015f, 0x54ffff61, 0x91000508, 0x9100054a,
			0x8b090042, 0xeb0b011f, 0x54fffea1, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		cBase := uint64(0x100000)
		aBase := uint64(0x200000)
		stackBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		cBuf := make([]byte, n*n*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, n*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, uint64(m))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteD(1, beta)
		em.Regs.WriteX(2, cBase)
		em.Regs.WriteX(3, aBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupSyr2kEmu := func() *emu.Emulator {
		n, m := 80, 60
		alpha, beta := 1.5, 1.2

		C := make([]float64, n*n)
		A := make([]float64, n*m)
		B := make([]float64, n*m)

		for i := 0; i < n; i++ {
			for j := 0; j < m; j++ {
				A[i*m+j] = float64((i*j+1)%n) / float64(n)
				B[i*m+j] = float64((i*j+2)%m) / float64(m)
			}
		}
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				C[i*n+j] = float64((i*j+3)%n) / float64(m)
			}
		}

		code := []uint32{
			0x7100041f, 0x5400082b, 0x7100003f, 0x540005ed,
			0xd2800008, 0x2a0103e9, 0xd37d7c0a, 0xd37d7c2b,
			0x5280002c, 0x2a0003ed, 0xd280000e, 0xfc6e7842,
			0x1e610842, 0xfc2e7842, 0x910005ce, 0xeb0e019f,
			0x54ffff61, 0xd280000e, 0x9b097d0f, 0xaa0303f0,
			0xaa0403f1, 0xd2800000, 0xd2800001, 0x8b0f01c5,
			0xfc606a02, 0x1e600842, 0xfc657883, 0xfc606a24,
			0x1e600884, 0xfc657865, 0x1e650884, 0x1f431042,
			0xfc617843, 0x1e622862, 0xfc217842, 0x91000421,
			0x8b0b0000, 0xeb01019f, 0x54fffe41, 0x910005ce,
			0x91002231, 0x91002210, 0xeb0901df, 0x54fffd41,
			0x91000508, 0x9100058c, 0x8b0a0042, 0xeb0d011f,
			0x54fffb41, 0x14000011, 0xd2800008, 0xd37d7c09,
			0x5280002a, 0x2a0003eb, 0xd280000c, 0xfc6c7840,
			0x1e610800, 0xfc2c7840, 0x9100058c, 0xeb0c015f,
			0x54ffff61, 0x91000508, 0x9100054a, 0x8b090042,
			0xeb0b011f, 0x54fffea1, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		cBase := uint64(0x100000)
		aBase := uint64(0x200000)
		bBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		cBuf := make([]byte, n*n*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, n*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, n*m*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, uint64(m))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteD(1, beta)
		em.Regs.WriteX(2, cBase)
		em.Regs.WriteX(3, aBase)
		em.Regs.WriteX(4, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupTrmmEmu := func() *emu.Emulator {
		m, n := 60, 80
		alpha := 1.5

		A := make([]float64, m*m)
		B := make([]float64, m*n)

		for i := 0; i < m; i++ {
			for j := 0; j < i; j++ {
				A[i*m+j] = float64((i+j)%m) / float64(m)
			}
			A[i*m+i] = 1.0
			for j := 0; j < n; j++ {
				B[i*n+j] = float64((n+(i-j))%n) / float64(n)
			}
		}

		code := []uint32{
			0x7100041f, 0x540006eb, 0x7100043f, 0x540006ab,
			0xd2800004, 0x2a0003e8, 0xd37d7c29, 0x2a0103ea,
			0x8b09006b, 0xd37d7c0c, 0x8b0c004d, 0x9100218e,
			0xd100050f, 0xaa0303f0, 0x14000008, 0x8b090210,
			0x8b09016b, 0x8b0e01ad, 0xd10005ef, 0xaa1103e4,
			0xeb08023f, 0x54000460, 0x91000491, 0xaa0a03e0,
			0xaa1003e1, 0xeb08023f, 0x54000302, 0xd2800000,
			0x9b0a7c81, 0x8b010c61, 0xaa0b03e2, 0xfc607821,
			0xaa0f03e4, 0xaa0d03e5, 0xaa0203e6, 0xfd4000a2,
			0xfd4000c3, 0x1f430441, 0xfc207821, 0x8b0900c6,
			0x8b0c00a5, 0xf1000484, 0x54ffff21, 0x1e600821,
			0xfc207821, 0x91000400, 0x91002042, 0xeb0a001f,
			0x54fffde1, 0x17ffffde, 0xfd400021, 0x1e600821,
			0xfc008421, 0xf1000400, 0x54ffff81, 0x17ffffd8,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		bBase := uint64(0x200000)
		stackBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, m*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, m*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupSymmEmu := func() *emu.Emulator {
		m, n := 60, 80
		alpha, beta := 1.5, 1.2

		C := make([]float64, m*n)
		A := make([]float64, m*m)
		B := make([]float64, m*n)

		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				C[i*n+j] = float64((i+j)%100) / float64(m)
				B[i*n+j] = float64((n+i-j)%100) / float64(m)
			}
		}
		for i := 0; i < m; i++ {
			for j := 0; j <= i; j++ {
				A[i*m+j] = float64((i+j)%100) / float64(m)
			}
			for j := i + 1; j < m; j++ {
				A[i*m+j] = -999
			}
		}

		code := []uint32{
			0x7100041f, 0x540008ab, 0x7100043f, 0x5400086b,
			0xa9bf4ff4, 0x52800008, 0xd2800009, 0xd37d7c2a,
			0x1100040b, 0x2f00e402, 0x2a0103ec, 0x2a0003ed,
			0x14000005, 0x91000529, 0x0b000108, 0xeb0d013f,
			0x540006a0, 0x1b097d6e, 0xaa0c03ef, 0xaa0203f0,
			0xaa0403f1, 0xb40004a9, 0xd280000f, 0x1b017d30,
			0xaa0203f1, 0xaa0403e5, 0x8b284c66, 0xd2800013,
			0xd2800014, 0x8b1001e7, 0x2f00e403, 0xfc677884,
			0xfc7478c5, 0xfc736a26, 0x1e600884, 0x1f451884,
			0xfc336a24, 0xfc7368a4, 0xfc7478c5, 0x1f450c83,
			0x91000694, 0x8b0a0273, 0xeb14013f, 0x54fffe81,
			0xfc677844, 0xfc677885, 0x1e6008a5, 0xfc6ed866,
			0x1e6608a5, 0x1f441424, 0x1f431003, 0xfc277843,
			0x910005ef, 0x910020a5, 0x91002231, 0xeb0c01ff,
			0x54fffc61, 0x17ffffd4, 0xfd400203, 0xfc408624,
			0x1e600884, 0xfc6ed865, 0x1e650884, 0x1f431023,
			0x1f420c03, 0xfc008603, 0xf10005ef, 0x54fffee1,
			0x17ffffc9, 0xa8c14ff4, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		cBase := uint64(0x100000)
		aBase := uint64(0x200000)
		bBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		cBuf := make([]byte, m*n*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, m*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, m*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteD(1, beta)
		em.Regs.WriteX(2, cBase)
		em.Regs.WriteX(3, aBase)
		em.Regs.WriteX(4, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupCholeskyEmu := func() *emu.Emulator {
		n := 120

		A0 := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j <= i; j++ {
				A0[i*n+j] = float64((-j)%n)/float64(n) + 1
			}
			A0[i*n+i] = 1
		}
		B := make([]float64, n*n)
		for t := 0; t < n; t++ {
			for r := 0; r < n; r++ {
				for s := 0; s < n; s++ {
					B[r*n+s] += A0[r*n+t] * A0[s*n+t]
				}
			}
		}

		code := []uint32{
			0x7100041f, 0x540006ab, 0x52800008, 0xd2800009,
			0x1100040a, 0x2a0003eb, 0x14000009, 0x1b097d4c,
			0xfc6cd820, 0x1e61c000, 0xfc2cd820, 0x91000529,
			0x0b000108, 0xeb0b013f, 0x54000500, 0xb4ffff09,
			0x5280000d, 0xd280000e, 0x9b0b7d2f, 0x8b284c2c,
			0x1400000a, 0x1b0e7d51, 0xfc71d820, 0xfc707821,
			0x1e601820, 0xfc307820, 0x910005ce, 0x0b0001ad,
			0xeb0901df, 0x540001e0, 0xaa0f03f0, 0xb4fffece,
			0xd2800011, 0x8b2d4c22, 0x8b0f01d0, 0xfc707820,
			0xfc717981, 0xfc717842, 0x1f428020, 0xfc307820,
			0x91000631, 0xeb1101df, 0x54ffff41, 0x17ffffea,
			0xd280000d, 0x1b097d4e, 0xfc6e5820, 0xfc6d7981,
			0x1f418020, 0xfc2e5820, 0x910005ad, 0xeb0d013f,
			0x54ffff61, 0x17ffffd2, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		stackBase := uint64(0x200000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, aBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupLUEmu := func() *emu.Emulator {
		n := 120

		A0 := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j <= i; j++ {
				A0[i*n+j] = float64((-j)%n)/float64(n) + 1
			}
			A0[i*n+i] = 1
		}
		B := make([]float64, n*n)
		for t := 0; t < n; t++ {
			for r := 0; r < n; r++ {
				for s := 0; s < n; s++ {
					B[r*n+s] += A0[r*n+t] * A0[s*n+t]
				}
			}
		}

		code := []uint32{
			0x7100041f, 0x540007ab, 0xd2800008, 0x11000409,
			0xd37d7c0a, 0xaa0103eb, 0xaa0103ec, 0x2a0003ed,
			0x14000006, 0x91000508, 0x8b0a018c, 0x9100216b,
			0xeb0d011f, 0x54000620, 0xb4ffff68, 0xd280000f,
			0x9b0d7d10, 0x8b100c2e, 0xaa0103f1, 0x1400000a,
			0x1b0f7d22, 0xfc62d820, 0xfc607821, 0x1e601820,
			0xfc207820, 0x910005ef, 0x91002231, 0xeb0801ff,
			0x54000200, 0xaa1003e0, 0xb4fffecf, 0xd2800002,
			0x8b1001e0, 0xfc607820, 0xaa1103e3, 0xfc627981,
			0xfd400062, 0x1f428020, 0xfc207820, 0x91000442,
			0x8b0a0063, 0xeb0201ff, 0x54ffff21, 0x17ffffe9,
			0xaa0b03ef, 0xaa0803f0, 0xd2800011, 0xfc7079c0,
			0xaa0f03e0, 0xfc717981, 0xfd400002, 0x1f428020,
			0xfc3079c0, 0x91000631, 0x8b0a0000, 0xeb11011f,
			0x54ffff21, 0x91000610, 0x910021ef, 0xeb0d021f,
			0x54fffe41, 0x17ffffcc, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		stackBase := uint64(0x200000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, aBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupFloydWarshallEmu := func() *emu.Emulator {
		n := 180

		path := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				path[i*n+j] = float64(i*j%7 + 1)
				if (i+j)%13 == 0 || (i+j)%7 == 0 || (i+j)%11 == 0 {
					path[i*n+j] = 999
				}
			}
		}

		code := []uint32{
			0x7100041f, 0x5400046b, 0xd2800008, 0xd37d7c09,
			0xaa0103ea, 0x2a0003eb, 0x14000005, 0x91000508,
			0x8b09014a, 0xeb0b011f, 0x54000340, 0xd280000c,
			0x8b080c2d, 0xaa0103ee, 0x14000005, 0x9100058c,
			0x8b0901ce, 0xeb0b019f, 0x54fffea0, 0x9b0b7d8f,
			0xaa0b03f0, 0xaa0a03f1, 0xaa0e03e0, 0x14000005,
			0x91002000, 0x91002231, 0xf1000610, 0x54fffe80,
			0xfc6f79a0, 0xfd400221, 0x1e612800, 0xfd400001,
			0x1e612000, 0x54fffee5, 0xfd000000, 0x17fffff5,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		pathBase := uint64(0x100000)
		stackBase := uint64(0x200000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		pathBuf := make([]byte, n*n*8)
		for i, v := range path {
			binary.LittleEndian.PutUint64(pathBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(pathBase, pathBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, pathBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupCovarianceEmu := func() *emu.Emulator {
		m, n := 80, 100
		floatN := float64(n)

		data := make([]float64, n*m)
		for i := 0; i < n; i++ {
			for j := 0; j < m; j++ {
				data[i*m+j] = float64(i*j) / float64(m)
			}
		}

		code := []uint32{
			0x7100041f, 0x54000ccb, 0x2a0003e8, 0x2a0103ea,
			0xd37df109, 0x7100003f, 0x5400028d, 0xd280000b,
			0xaa0203ec, 0xf82b789f, 0x2f00e401, 0xaa0a03ed,
			0xaa0c03ee, 0xfd4001c2, 0x1e612841, 0xfc2b7881,
			0x8b0901ce, 0xf10005ad, 0x54ffff61, 0x1e601821,
			0xfc2b7881, 0x9100056b, 0x9100218c, 0xeb08017f,
			0x54fffe21, 0x14000008, 0x2f00e401, 0xaa0803eb,
			0xaa0403ec, 0x1e601821, 0xfc008581, 0xf100056b,
			0x54ffffc1, 0x7100043f, 0x5400020b, 0xd280000b,
			0xaa0203ec, 0xaa0803ed, 0xaa0403ee, 0xaa0c03ef,
			0xfc4085c1, 0xfd4001e2, 0x1e613841, 0xfc0085e1,
			0xf10005ad, 0x54ffff61, 0x9100056b, 0x8b09018c,
			0xeb0a017f, 0x54fffe81, 0x1e7e1001, 0x1e612800,
			0x7100003f, 0x5400040d, 0xd280000b, 0xd280000c,
			0x9b087d8d, 0x8b0d0c6d, 0x8b0c0c6e, 0xaa0b03ef,
			0xaa0c03f0, 0xf83079bf, 0x2f00e401, 0xaa0a03f1,
			0xaa0203e0, 0xfc6b6802, 0xfc6f6803, 0x1f430441,
			0xfc3079a1, 0x8b090000, 0xf1000631, 0x54ffff41,
			0x1e601821, 0xfc3079a1, 0x9b087e11, 0xfc3179c1,
			0x91000610, 0x910021ef, 0xeb08021f, 0x54fffdc1,
			0x9100058c, 0x9100216b, 0xeb08019f, 0x54fffca1,
			0x14000013, 0xd280000a, 0x2f00e401, 0x9100212b,
			0xaa0803ec, 0x1e601820, 0xaa0c03ed, 0xaa0303ee,
			0xaa0303ef, 0xfc0085e0, 0xfd0001c0, 0x8b0901ce,
			0xf10005ad, 0x54ffff81, 0x9100054a, 0x8b0b0063,
			0xd100058c, 0xeb08015f, 0x54fffe81, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		dataBase := uint64(0x100000)
		covBase := uint64(0x200000)
		meanBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		dataBuf := make([]byte, n*m*8)
		for i, v := range data {
			binary.LittleEndian.PutUint64(dataBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(dataBase, dataBuf)

		em.LoadProgram(covBase, make([]byte, m*m*8))
		em.LoadProgram(meanBase, make([]byte, m*8))

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteD(0, floatN)
		em.Regs.WriteX(2, dataBase)
		em.Regs.WriteX(3, covBase)
		em.Regs.WriteX(4, meanBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupCorrelationEmu := func() *emu.Emulator {
		m, n := 80, 100
		floatN := float64(n)

		data := make([]float64, n*m)
		for i := 0; i < n; i++ {
			for j := 0; j < m; j++ {
				data[i*m+j] = float64(i*j)/float64(m) + float64(i)
			}
		}

		code := []uint32{
			0x71000408, 0x540009cb, 0x2a0003e9, 0xd37df12a,
			0x7100003f, 0x540002ad, 0xd280000b, 0x2a0103ec,
			0xaa0203ed, 0xf82b789f, 0x2f00e401, 0xaa0c03ee,
			0xaa0d03ef, 0xfd4001e2, 0x1e612841, 0xfc2b7881,
			0x8b0a01ef, 0xf10005ce, 0x54ffff61, 0x1e601821,
			0xfc2b7881, 0x9100056b, 0x910021ad, 0xeb09017f,
			0x54fffe21, 0x14000008, 0x2f00e401, 0xaa0903eb,
			0xaa0403ec, 0x1e601821, 0xfc008581, 0xf100056b,
			0x54ffffc1, 0x7100003f, 0x540003ed, 0xd280000b,
			0x2a0103ec, 0xb201e7ed, 0xf293334d, 0xf2e7f72d,
			0x9e6701a1, 0x1e6e1002, 0xaa0203ed, 0xf82b78bf,
			0x2f00e403, 0xaa0c03ee, 0xaa0d03ef, 0xfd4001e4,
			0xfc6b7885, 0x1e653884, 0x1f440c83, 0xfc2b78a3,
			0x8b0a01ef, 0xf10005ce, 0x54ffff21, 0x1e601863,
			0x1e61c063, 0x1e612060, 0x1e628c63, 0xfc2b78a3,
			0x9100056b, 0x910021ad, 0xeb09017f, 0x54fffd81,
			0x1400000f, 0x2f00e401, 0x1e601821, 0x1e61c021,
			0xb201e7ea, 0xf293334a, 0xf2e7f72a, 0x9e670142,
			0x1e622020, 0x1e6e1002, 0x1e628c21, 0xaa0503ea,
			0xfc008541, 0xf1000529, 0x54ffffc1, 0x7100043f,
			0x540008cb, 0x7100001f, 0x54000bed, 0xd280000a,
			0xd37d7c0b, 0xaa0203ec, 0x2a0003e8, 0x2a0103e9,
			0x1e61c000, 0xaa0803ed, 0xaa0503ee, 0xaa0403ef,
			0xaa0c03f0, 0xfc4085e1, 0xfd400202, 0x1e613841,
			0xfd000201, 0xfc4085c2, 0x1e620802, 0x1e621821,
			0xfc008601, 0xf10005ad, 0x54fffee1, 0x9100054a,
			0x8b0b018c, 0xeb09015f, 0x54fffde1, 0x7100040a,
			0x540004a0, 0xd280000b, 0xd280000c, 0xd37df10d,
			0x5280002e, 0x5280010f, 0xd2e7fe10, 0x9b087d81,
			0x1b007d84, 0x8b0c0c71, 0xf8245a30, 0x9100058c,
			0x8b010c61, 0xaa0f03e4, 0xaa0e03e5, 0xf825783f,
			0x2f00e400, 0xaa0903e6, 0xaa0203e7, 0xfc6b68e1,
			0xfc6468e2, 0x1f420020, 0xfc257820, 0x8b0d00e7,
			0xf10004c6, 0x54ffff41, 0x9b087ca6, 0xfc267a20,
			0x910004a5, 0x91002084, 0xeb0800bf, 0x54fffe01,
			0x910005ce, 0x910021ef, 0x9100216b, 0xeb0a019f,
			0x54fffc61, 0xaa0a03e8, 0x1b002108, 0xd2e7fe09,
			0xf828d869, 0xd65f03c0, 0x7100041f, 0x5400034d,
			0xd2800009, 0x2a0003ee, 0x9100206a, 0xd37d7c0b,
			0x9100216c, 0x8b0b006d, 0xd10005ce, 0xd2e7fe0f,
			0x1b007d10, 0x8b090c71, 0xf8305a2f, 0x91000529,
			0xaa0e03f0, 0xaa0d03f1, 0xaa0a03e1, 0xf800843f,
			0xf900023f, 0x8b0b0231, 0xf1000610, 0x54ffff81,
			0x8b0c014a, 0x8b0c01ad, 0xd10005ce, 0xeb08013f,
			0x54fffe01, 0x1b002108, 0xd2e7fe09, 0xf828d869,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		dataBase := uint64(0x100000)
		corrBase := uint64(0x200000)
		meanBase := uint64(0x300000)
		stddevBase := uint64(0x400000)
		stackBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		dataBuf := make([]byte, n*m*8)
		for i, v := range data {
			binary.LittleEndian.PutUint64(dataBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(dataBase, dataBuf)

		em.LoadProgram(corrBase, make([]byte, m*m*8))
		em.LoadProgram(meanBase, make([]byte, m*8))
		em.LoadProgram(stddevBase, make([]byte, m*8))

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteD(0, floatN)
		em.Regs.WriteX(2, dataBase)
		em.Regs.WriteX(3, corrBase)
		em.Regs.WriteX(4, meanBase)
		em.Regs.WriteX(5, stddevBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupGramschmidtEmu := func() *emu.Emulator {
		m, n := 60, 80
		A := make([]float64, m*n)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				A[i*n+j] = (float64((i*j)%m)/float64(m))*100 + 10
			}
		}

		code := []uint32{
			0x7100043f, 0x540009eb, 0xd280000d, 0xd37d7c28,
			0x91002049, 0x5280002a, 0x2a0103eb, 0x2a0003ec,
			0x14000008, 0x9100054a, 0x91002042, 0x91002084,
			0x91002129, 0xaa0e03ed, 0xeb0b01df, 0x54000820,
			0x8b0d0c6e, 0x7100001f, 0x5400030d, 0x2f00e400,
			0xaa0c03ef, 0xaa0203f0, 0xfd400201, 0x1f410020,
			0x8b080210, 0xf10005ef, 0x54ffff81, 0x1e61c000,
			0x1b017db0, 0xfc3059c0, 0xaa0c03f1, 0xfc6f6840,
			0xfc7059c1, 0x1e611800, 0xfc2f6880, 0x8b0801ef,
			0xf1000631, 0x54ffff41, 0x910005ae, 0xeb0b01df,
			0x54fffc22, 0x14000006, 0x1b017daf, 0xf82f59df,
			0x910005ae, 0xeb0b01df, 0x54fffb62, 0x9b0b7dad,
			0x8b0d0c6d, 0xaa0903ef, 0xaa0a03f0, 0x14000005,
			0x91000610, 0x910021ef, 0xeb0b021f, 0x54fffa40,
			0xf83079bf, 0x7100041f, 0x54ffff4b, 0xd2800011,
			0x2f00e400, 0xaa0c03e5, 0xfc716881, 0xfc7169e2,
			0x1f420020, 0xfc3079a0, 0x8b080231, 0xf10004a5,
			0x54ffff41, 0xd2800011, 0xaa0c03e5, 0xfc7169e0,
			0xfc716881, 0xfc7079a2, 0x1f428020, 0xfc3169e0,
			0x8b080231, 0xf10004a5, 0x54ffff21, 0x17ffffe5,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		rBase := uint64(0x200000)
		qBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, m*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)
		em.LoadProgram(rBase, make([]byte, n*n*8))
		em.LoadProgram(qBase, make([]byte, m*n*8))
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, rBase)
		em.Regs.WriteX(4, qBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupLudcmpEmu := func() *emu.Emulator {
		n := 120
		fn := float64(n)

		A0 := make([]float64, n*n)
		b := make([]float64, n)
		for i := 0; i < n; i++ {
			b[i] = float64(i+1)/fn/2.0 + 4
		}
		for i := 0; i < n; i++ {
			for j := 0; j <= i; j++ {
				A0[i*n+j] = float64((-j)%n)/fn + 1
			}
			for j := i + 1; j < n; j++ {
				A0[i*n+j] = 0
			}
			A0[i*n+i] = 1
		}
		B := make([]float64, n*n)
		for tt := 0; tt < n; tt++ {
			for r := 0; r < n; r++ {
				for s := 0; s < n; s++ {
					B[r*n+s] += A0[r*n+tt] * A0[s*n+tt]
				}
			}
		}
		A := make([]float64, n*n)
		copy(A, B)

		code := []uint32{
			0x7100041f, 0x54000d0b, 0xd280000a, 0x11000408,
			0xd37d7c0b, 0xaa0103ec, 0xaa0103ed, 0x2a0003e9,
			0x14000006, 0x9100054a, 0x8b0b01ad, 0x9100218c,
			0xeb09015f, 0x540005a0, 0xb4ffff6a, 0xd280000f,
			0x9b097d4e, 0x8b0e0c2e, 0xaa0103f0, 0x14000009,
			0x1b0f7d11, 0xfc71d821, 0x1e611800, 0xfc2f79c0,
			0x910005ef, 0x91002210, 0xeb0a01ff, 0x540001a0,
			0xfc6f79c0, 0xb4fffeef, 0xd2800011, 0xaa1003e0,
			0xfc7179a1, 0xfd400002, 0x1f428020, 0x91000631,
			0x8b0b0000, 0xeb1101ff, 0x54ffff41, 0x17ffffed,
			0xaa0c03ef, 0xaa0a03f0, 0xd2800011, 0xfc7079c0,
			0xaa0f03e0, 0xfc7179a1, 0xfd400002, 0x1f428020,
			0x91000631, 0x8b0b0000, 0xeb11015f, 0x54ffff41,
			0xfc3079c0, 0x91000610, 0x910021ef, 0xeb09021f,
			0x54fffe41, 0x17ffffd0, 0xd280000a, 0xd37df12d,
			0xaa0103eb, 0x14000006, 0xfc2a7880, 0x9100054a,
			0x8b0d016b, 0xeb09015f, 0x54000160, 0xfc6a7840,
			0xb4ffff4a, 0xd280000c, 0xfc6c7961, 0xfc6c7882,
			0x1f428020, 0x9100058c, 0xeb0c015f, 0x54ffff61,
			0x17fffff2, 0xd280000a, 0x9ba97d2b, 0x8b0b0c2b,
			0xd27df1ac, 0x8b0d006d, 0xaa0903ee, 0x1400000a,
			0x1b0f7d10, 0xfc70d821, 0x1e611800, 0xfc2f7860,
			0x8b0c016b, 0xd100054a, 0xf10005df, 0xaa0f03ee,
			0x540001ad, 0xd10005cf, 0xfc6f7880, 0xaa0a03f0,
			0xaa0b03f1, 0xeb0901df, 0x54fffe4a, 0xfc408621,
			0xfc7079a2, 0x1f428020, 0x91000610, 0x35ffff90,
			0x17ffffec, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		bBase := uint64(0x200000)
		xBase := uint64(0x300000)
		yBase := uint64(0x400000)
		stackBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, n*8)
		for i, v := range b {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)
		em.LoadProgram(xBase, make([]byte, n*8))
		em.LoadProgram(yBase, make([]byte, n*8))
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, aBase)
		em.Regs.WriteX(2, bBase)
		em.Regs.WriteX(3, xBase)
		em.Regs.WriteX(4, yBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupNussinovEmu := func() *emu.Emulator {
		n := 180
		seq := make([]byte, n)
		for i := 0; i < n; i++ {
			seq[i] = byte((i + 1) % 4)
		}

		code := []uint32{
			0x7100040b, 0x54000a0b, 0xa9bc67fa, 0xa9015ff8,
			0xa90257f6, 0xa9034ff4, 0xd1001048, 0x2a0003e9,
			0x8b20484a, 0x1b0b7c0b, 0xd2c0002c, 0x1100080d,
			0x1b0d7c0d, 0x2a2003ee, 0xb2607fef, 0xaa0903f1,
			0x8b098190, 0x14000007, 0xd100114a, 0x4b00016b,
			0x8b0f0210, 0x0b0e01ad, 0xf100087f, 0x540006cb,
			0xaa1103e3, 0xd1000631, 0xeb09007f, 0x54fffeea,
			0x9b097e24, 0xaa0d03e5, 0xaa0303e6, 0x8b2b4947,
			0x9b097c73, 0x14000008, 0x6b1602bf, 0x1a96c2b5,
			0xb8347855, 0x910004c6, 0x110004a5, 0x6b0000df,
			0x54fffd40, 0x8b0400d4, 0xb8747855, 0xb8747916,
			0x6b1602bf, 0x1a96c2b5, 0xb8347855, 0x8b1300d6,
			0xb8767857, 0x6b1702bf, 0x1a97c2b5, 0xb8347855,
			0xb8767916, 0xeb06007f, 0x54fffd8a, 0x38716837,
			0x38666838, 0x0b170317, 0x71000eff, 0x1a9616d6,
			0x6b1602bf, 0x1a96c2b5, 0xb8347855, 0xaa0503f6,
			0xaa1003f8, 0xaa0703f7, 0xb84046f9, 0xb876585a,
			0x0b190359, 0x6b1902bf, 0x1a99c2b5, 0xb8347855,
			0xeb9880df, 0x8b0c0318, 0x0b0002d6, 0x54fffeec,
			0x17ffffd9, 0xa9434ff4, 0xa94257f6, 0xa9415ff8,
			0xa8c467fa, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		seqBase := uint64(0x100000)
		tableBase := uint64(0x200000)
		stackBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))
		em.LoadProgram(seqBase, seq)
		em.LoadProgram(tableBase, make([]byte, n*n*4))
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, seqBase)
		em.Regs.WriteX(2, tableBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupAdiEmu := func() *emu.Emulator {
		tsteps := 40
		n := 60

		u := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				u[i*n+j] = float64(i+n-j) / float64(n)
			}
		}

		code := []uint32{
			0xd10383ff, 0xa9087bfd, 0xa9096ffc, 0xa90a67fa, 0xa90b5ff8, 0xa90c57f6,
			0xa90d4ff4, 0x7100041f, 0xb90067e0, 0xf90037e1, 0x54001b0b, 0xf94037e1,
			0xb94067e8, 0x1e6e1007, 0x1e601010, 0xd2e7fe1d, 0x1e620020, 0x1e620101,
			0x5100082c, 0x93407c29, 0x93407d8a, 0x937d7c2b, 0x9b2c7c32, 0x51000428,
			0x8b0a012e, 0x8b0b0046, 0x9b2c242c, 0x1e6018e0, 0x8b0e0c50, 0xd10020ce,
			0x91002160, 0xf9003fee, 0xd37df252, 0x8b00008e, 0xd37df18c, 0x1b017d0d,
			0xf9002fee, 0x8b0000ae, 0x8b08012f, 0xf9002bee, 0x910040ce, 0x9100218c,
			0xf9001fee, 0x9100224e, 0x8b2dcc6d, 0xf9001bee, 0x9100206e, 0xd37df1ef,
			0xa90233e6, 0x8b0b008c, 0x91002210, 0xf90027ec, 0xd37d7d0c, 0xd1000511,
			0xf9000fee, 0x8b21d1ce, 0xcb0b03f4, 0x8b0b00b2, 0xd100219b, 0x5280002c,
			0xf9000bee, 0x8b00006e, 0xf90023f2, 0xf90007ee, 0x1e6118e1, 0x1e600800,
			0x1e612822, 0x1e601823, 0x1e7c1001, 0x1e601842, 0x1e610840, 0x1e610861,
			0x1e672842, 0x1e672863, 0x1e614004, 0x1e614025, 0x1f501c26, 0x1f501c07,
			0x14000006, 0xb94067ec, 0xf9403bee, 0x6b0c01df, 0x110005cc, 0x540011e0,
			0xf9003bec, 0xf94037ec, 0x71000d9f, 0x54ffff0b, 0xa9444bec, 0x52800038,
			0xa9429fe6, 0xf9401ff3, 0xa9455bf5, 0x9b097f0e, 0xf838787d, 0xaa1303f7,
			0xaa1503f9, 0xaa1603fa, 0xaa1103fe, 0xf82e789f, 0xfc787870, 0xfc2e78b0,
			0xfc5f8350, 0xf10007de, 0x1f500810, 0x1e701891, 0xfc008751, 0x6d7f46f2,
			0x1e7108d1, 0x1f5244b1, 0xfd4002f2, 0x8b0b02f7, 0x1f5244b1, 0xfc5f8332,
			0x1f524491, 0x1e701a30, 0xfc008730, 0x54fffe21, 0xaa0303f7, 0xaa0a03f9,
			0xf83879bd, 0xfc797a50, 0xfc666af1, 0xfc797992, 0xf1000739, 0x1f514a10,
			0xfc276af0, 0x8b1402f7, 0x54ffff2c, 0x91000718, 0x8b0b02d6, 0x8b0b02b5,
			0x91002273, 0x910020e7, 0xeb08031f, 0x910020c6, 0x8b0b0252, 0x8b0b018c,
			0x54fffa61, 0xf94037ec, 0x71000d9f, 0x54fff84b, 0xa941b3f5, 0xaa1f03fe,
			0xa9445bf8, 0x52800033, 0xa9454be6, 0xa940e7e7, 0x9b1e7d7a, 0x9b097e77,
			0x8b1a017c, 0x8b1a01e0, 0x8b1c0081, 0x8b00008e, 0x8b0000a0, 0xf837785d,
			0x8b1c00bc, 0xeb00003f, 0xf837789f, 0xfa4e3382, 0xfc777850, 0xfc3778b0,
			0x540002a3, 0x9b1e2d6e, 0xaa1f03fc, 0xfc6e6890, 0xfc6e68b1, 0x1f500c32,
			0x1e7218b0, 0xfc3c6a50, 0xfc7c68f3, 0xfc7c6ab4, 0x1e7308f3, 0x1f544c93,
			0xfc7c6b34, 0x1f544c93, 0x1f514cb1, 0x1e721a31, 0xfc3c68d1, 0x9100239c,
			0xeb1c037f, 0x54fffe41, 0x14000015, 0xaa1f03fc, 0x8b1c024e, 0xfc5f81d0,
			0x1f500c30, 0x1e7018b1, 0xfd0001d1, 0x8b1c030e, 0xfc7c68f1, 0xfc7c6ab2,
			0x1e7108f1, 0x1f524491, 0xfc7c6b32, 0x9100239c, 0xeb1c037f, 0x1f524491,
			0xfd4001d2, 0x1f5244b1, 0x1e701a30, 0xfd0005d0, 0x54fffdc1, 0xf9403fee,
			0x91000673, 0xf83779dd, 0xaa0a03f7, 0xfc7a6a10, 0xfc777ad1, 0xfc777b12,
			0x1f504a30, 0xfc377990, 0xf10006f7, 0x54ffff6c, 0x910007de, 0x8b0b0339,
			0x8b0b02b5, 0x8b0b00c6, 0x8b0b00e7, 0x8b0b0252, 0xeb08027f, 0x8b0b0318,
			0x8b0b02d6, 0x8b0b018c, 0x54fff681, 0x17ffff6e, 0xa94d4ff4, 0xa94c57f6,
			0xa94b5ff8, 0xa94a67fa, 0xa9496ffc, 0xa9487bfd, 0x910383ff, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		uBase := uint64(0x100000)
		vBase := uint64(0x200000)
		pBase := uint64(0x300000)
		qBase := uint64(0x400000)
		stackBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		uBuf := make([]byte, n*n*8)
		for i, val := range u {
			binary.LittleEndian.PutUint64(uBuf[i*8:], math.Float64bits(val))
		}
		em.LoadProgram(uBase, uBuf)
		em.LoadProgram(vBase, make([]byte, n*n*8))
		em.LoadProgram(pBase, make([]byte, n*n*8))
		em.LoadProgram(qBase, make([]byte, n*n*8))
		em.LoadProgram(stackBase, make([]byte, 16384))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(tsteps))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, uBase)
		em.Regs.WriteX(3, vBase)
		em.Regs.WriteX(4, pBase)
		em.Regs.WriteX(5, qBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 16384
		em.SetHaltPC(haltPC)
		return em
	}

	setupFdtd2dEmu := func() *emu.Emulator {
		tmax := 40
		nx := 60
		ny := 80

		ex := make([]float64, nx*ny)
		ey := make([]float64, nx*ny)
		hz := make([]float64, nx*ny)
		fict := make([]float64, tmax)

		for i := 0; i < tmax; i++ {
			fict[i] = float64(i)
		}
		for i := 0; i < nx; i++ {
			for j := 0; j < ny; j++ {
				ex[i*ny+j] = float64(i*(j+1)) / float64(nx)
				ey[i*ny+j] = float64(i*(j+2)) / float64(ny)
				hz[i*ny+j] = float64(i*(j+3)) / float64(nx)
			}
		}

		code := []uint32{
			0x7100041f, 0x54000e0b, 0xa9bc67fa, 0xa9015ff8, 0xa90257f6, 0xa9034ff4,
			0x7100083f, 0x1e7c1000, 0x2a0203ea, 0x1a9fa7ef, 0x7100042b, 0x937d7c50,
			0x1a9fa7ee, 0x7100085f, 0xd37d7c52, 0xb203e7e7, 0xaa1f03e8, 0x51000449,
			0x2a0003ec, 0x2a0103ed, 0x1a9fa5ce, 0x1a9fa5ef, 0x910020b1, 0x91002060,
			0xd1000543, 0xf2f7fcc7, 0x14000004, 0x91000508, 0xeb0c011f, 0x54000a00,
			0x7100045f, 0x54ffff8b, 0xfc6878c1, 0xaa0a03f3, 0xaa0403f4, 0xf1000673,
			0xfc008681, 0x54ffffc1, 0x7100083f, 0x540002ab, 0xaa0403f3, 0xaa0503f4,
			0x52800035, 0xaa0a03f6, 0xaa1303f7, 0xaa1403f8, 0xfc706b01, 0xfc408702,
			0xf10006d6, 0x1e623821, 0xfc706ae2, 0x1f400821, 0xfc306ae1, 0x910022f7,
			0x54ffff01, 0x910006b5, 0x8b100294, 0x8b100273, 0xeb0d02bf, 0x54fffe01,
			0x3707fbee, 0xaa1f03f3, 0xaa0003f4, 0xaa1103f5, 0xaa0303f6, 0xaa1403f7,
			0xaa1503f8, 0x6d7f8702, 0xf10006d6, 0x91002318, 0x1e623821, 0xfd4002e2,
			0x1f400821, 0xfc0086e1, 0x54ffff21, 0x91000673, 0x8b1202b5, 0x8b120294,
			0xeb0d027f, 0x54fffe21, 0x3707f96f, 0xaa1f03f3, 0xaa0503f4, 0xaa0003f5,
			0xaa0403f6, 0x91000673, 0xaa1403f7, 0xaa1503f8, 0xaa0903f9, 0xaa1603fa,
			0x6d7f8702, 0x9e6700e3, 0xf1000739, 0x91002318, 0x1e623821, 0xfc726b42,
			0x1e622821, 0xfc408742, 0x1e623821, 0xfd4002e2, 0x1f430821, 0xfc0086e1,
			0x54fffe81, 0xeb0b027f, 0x8b1202d6, 0x8b1202b5, 0x8b120294, 0x54fffd41,
			0x17ffffaf, 0xa9434ff4, 0xa94257f6, 0xa9415ff8, 0xa8c467fa, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		exBase := uint64(0x100000)
		eyBase := uint64(0x200000)
		hzBase := uint64(0x300000)
		fictBase := uint64(0x400000)
		stackBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		exBuf := make([]byte, nx*ny*8)
		for i, val := range ex {
			binary.LittleEndian.PutUint64(exBuf[i*8:], math.Float64bits(val))
		}
		em.LoadProgram(exBase, exBuf)

		eyBuf := make([]byte, nx*ny*8)
		for i, val := range ey {
			binary.LittleEndian.PutUint64(eyBuf[i*8:], math.Float64bits(val))
		}
		em.LoadProgram(eyBase, eyBuf)

		hzBuf := make([]byte, nx*ny*8)
		for i, val := range hz {
			binary.LittleEndian.PutUint64(hzBuf[i*8:], math.Float64bits(val))
		}
		em.LoadProgram(hzBase, hzBuf)

		fictBuf := make([]byte, tmax*8)
		for i, val := range fict {
			binary.LittleEndian.PutUint64(fictBuf[i*8:], math.Float64bits(val))
		}
		em.LoadProgram(fictBase, fictBuf)
		em.LoadProgram(stackBase, make([]byte, 16384))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(tmax))
		em.Regs.WriteX(1, uint64(nx))
		em.Regs.WriteX(2, uint64(ny))
		em.Regs.WriteX(3, exBase)
		em.Regs.WriteX(4, eyBase)
		em.Regs.WriteX(5, hzBase)
		em.Regs.WriteX(6, fictBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 16384
		em.SetHaltPC(haltPC)
		return em
	}

	setupHeat3dEmu := func() *emu.Emulator {
		tsteps := 40
		n := 20
		nn := n * n
		total := n * n * n

		A := make([]float64, total)
		B := make([]float64, total)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				for k := 0; k < n; k++ {
					A[i*nn+j*n+k] = float64(i+j+(n-k)) * 10 / float64(n)
					B[i*nn+j*n+k] = A[i*nn+j*n+k]
				}
			}
		}

		code := []uint32{
			0x7100041f, 0x5400142b, 0xd101c3ff, 0xa9017bfd, 0xa9026ffc, 0xa90367fa,
			0xa9045ff8, 0xa90557f6, 0xa9064ff4, 0x1b017c2b, 0x93407c2c, 0x937d7c29,
			0x51000428, 0x1e701000, 0x1e681001, 0x9100212f, 0xd37d7d0d, 0xd37d7d6a,
			0x8b0b018c, 0x8b2b51e5, 0xd37df18e, 0x8b0f004b, 0xd10021ac, 0x91002150,
			0x8b05004d, 0x8b21d204, 0x910021d1, 0x8b10004e, 0x8b100070, 0xf90007f0,
			0x8b110070, 0x8b110051, 0x8b040052, 0x8b040062, 0x8b0f0064, 0x8b050063,
			0x52800025, 0xf90003e2, 0x14000005, 0x110004af, 0x6b0000bf, 0x2a0f03e5,
			0x54000e20, 0x71000c3f, 0x54ffff6b, 0xaa1103e6, 0xaa1003e7, 0xaa0d03f3,
			0xaa0b03f4, 0xaa1203f5, 0xaa0e03f6, 0x52800037, 0x910006f7, 0xaa0603f8,
			0xaa0703f9, 0xaa1303fa, 0xaa1403fb, 0xaa1503fc, 0xaa1603fd, 0x5280003e,
			0xaa1f03ef, 0x910007de, 0x8b0f0302, 0xfc6f6b82, 0xfc6f6b45, 0x6d7f8c44,
			0xfc6f6ba6, 0xfd400447, 0x1f400862, 0x1f401465, 0x1e662842, 0xfc6f6b66,
			0x1e6628a5, 0x1f401c66, 0x1e610842, 0x1e6428c4, 0x1f4108a2, 0x1f410882,
			0x1e622862, 0xfc2f6b22, 0x910021ef, 0xeb0f019f, 0x54fffd81, 0x8b0903bd,
			0x8b09039c, 0x8b09037b, 0xeb0803df, 0x8b09035a, 0x8b090339, 0x8b090318,
			0x54fffc41, 0x8b0a02d6, 0x8b0a02b5, 0x8b0a0294, 0xeb0802ff, 0x8b0a0273,
			0x8b0a00e7, 0x8b0a00c6, 0x54fffa41, 0x71000c3f, 0x54fff86b, 0xa9405bf5,
			0xaa1003e6, 0xaa1103e7, 0xaa0303f3, 0xaa0403f4, 0x52800037, 0x910006f7,
			0xaa0603f8, 0xaa0703f9, 0xaa1303fa, 0xaa1403fb, 0xaa1503fc, 0xaa1603fd,
			0x5280003e, 0xaa1f03ef, 0x910007de, 0x8b0f0302, 0xfc6f6b82, 0xfc6f6b45,
			0x6d7f8c44, 0xfc6f6ba6, 0xfd400447, 0x1f400862, 0x1f401465, 0x1e662842,
			0xfc6f6b66, 0x1e6628a5, 0x1f401c66, 0x1e610842, 0x1e6428c4, 0x1f4108a2,
			0x1f410882, 0x1e622862, 0xfc2f6b22, 0x910021ef, 0xeb0f019f, 0x54fffd81,
			0x8b0903bd, 0x8b09039c, 0x8b09037b, 0xeb0803df, 0x8b09035a, 0x8b090339,
			0x8b090318, 0x54fffc41, 0x8b0a02d6, 0x8b0a02b5, 0x8b0a0294, 0xeb0802ff,
			0x8b0a0273, 0x8b0a00e7, 0x8b0a00c6, 0x54fffa41, 0x17ffff8d, 0xa9464ff4,
			0xa94557f6, 0xa9445ff8, 0xa94367fa, 0xa9426ffc, 0xa9417bfd, 0x9101c3ff,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		bBase := uint64(0x200000)
		stackBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, total*8)
		for i, val := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(val))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, total*8)
		for i, val := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(val))
		}
		em.LoadProgram(bBase, bBuf)
		em.LoadProgram(stackBase, make([]byte, 16384))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(tsteps))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 16384
		em.SetHaltPC(haltPC)
		return em
	}

	setupJacobi2dEmu := func() *emu.Emulator {
		tsteps := 40
		n := 90

		A := make([]float64, n*n)
		B := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				A[i*n+j] = float64(i*(j+2)+2) / float64(n)
				B[i*n+j] = float64(i*(j+3)+3) / float64(n)
			}
		}

		code := []uint32{
			0x7100041f, 0x540009eb, 0x71000c3f, 0x540009ab,
			0x52800008, 0x51000429, 0x2a0103eb, 0x9100204a,
			0x5280010c, 0xaa0b1190, 0xd37d7c2b, 0x8b10004c,
			0x9100216e, 0x8b0e006d, 0x8b0e004e, 0xb201e7ef,
			0xf293334f, 0xf2e7f92f, 0x9e6701e0, 0xd37d7d2f,
			0xd10021ef, 0x8b100070, 0x91002071, 0xaa0e03e1,
			0xaa0d03e2, 0xaa0c03e3, 0xaa0a03e4, 0x52800025,
			0xd2800006, 0x910004a5, 0x8b060027, 0x6d7f84e2,
			0x1e622821, 0xfd4004e2, 0x1e622821, 0xfc666862,
			0x1e622821, 0xfc666882, 0x1e622821, 0x1e600821,
			0xfc266841, 0x910020c6, 0xeb0601ff, 0x54fffe61,
			0x8b0b0084, 0x8b0b0063, 0x8b0b0042, 0x8b0b0021,
			0xeb0900bf, 0x54fffd61, 0xaa0d03e1, 0xaa0e03e2,
			0xaa1003e3, 0xaa1103e4, 0x52800025, 0xd2800006,
			0x910004a5, 0x8b060027, 0x6d7f84e2, 0x1e622821,
			0xfd4004e2, 0x1e622821, 0xfc666862, 0x1e622821,
			0xfc666882, 0x1e622821, 0x1e600821, 0xfc266841,
			0x910020c6, 0xeb0601ff, 0x54fffe61, 0x8b0b0084,
			0x8b0b0063, 0x8b0b0042, 0x8b0b0021, 0xeb0900bf,
			0x54fffd61, 0x11000508, 0x6b00011f, 0x54fff901,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		bBase := uint64(0x200000)
		stackBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, n*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(tsteps))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupSeidel2dEmu := func() *emu.Emulator {
		tsteps := 40
		n := 120

		A := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				A[i*n+j] = float64(i*(j+2)+2) / float64(n)
			}
		}

		code := []uint32{
			0x7100041f, 0x540006eb, 0x71000c3f, 0x540006ab,
			0xa9bf4ff4, 0x52800008, 0x51000429, 0xd37d7c2a,
			0x8b0a004b, 0xd100052c, 0x9100204d, 0x8b0a01ae,
			0x1e645000, 0x8b2151af, 0xd2800010, 0xaa0e03f1,
			0xaa0f03e1, 0xaa0d03e2, 0x52800023, 0x9b107d44,
			0xfc646961, 0x91000463, 0xaa1103f3, 0xaa0103e4,
			0xaa0203e5, 0xaa0c03e6, 0xaa1103e7, 0x6d7f8ca2,
			0x1e632842, 0xfc408ca3, 0x1e632842, 0x1e612841,
			0xfd400262, 0x1e622821, 0xfc408ce2, 0x1e622821,
			0x6d7f8c82, 0x1e622821, 0xfc408c82, 0x1e632821,
			0x1e622821, 0x1e601821, 0xfd000261, 0xaa0703f3,
			0xf10004c6, 0x54fffdc1, 0x91000610, 0x8b0a0042,
			0x8b0a0021, 0x8b0a0231, 0xeb09007f, 0x54fffc01,
			0x11000508, 0x6b00011f, 0x54fffb01, 0xa8c14ff4,
			0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		aBase := uint64(0x100000)
		stackBase := uint64(0x200000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(tsteps))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupDericheEmu := func() *emu.Emulator {
		w := 192
		h := 128

		imgIn := make([]float64, w*h)
		for i := 0; i < w; i++ {
			for j := 0; j < h; j++ {
				imgIn[i*h+j] = float64((313*i+991*j)%65536) / 65535.0
			}
		}

		code := []uint32{
			0x1e22c000, 0x1e7e1001, 0x1e612801, 0xd2836be8,
			0xf2a98588, 0xf2d9ed08, 0xf2f7f848, 0x9e670102,
			0x1e620821, 0x1e6e1003, 0x1e632800, 0x1e620800,
			0x7100041f, 0x54000d6b, 0x7100043f, 0x54001beb,
			0xd280000a, 0xd37d7c2b, 0xd29c8008, 0xf2a05488,
			0xf2c4e368, 0xf2f7f908, 0x9e670102, 0xd29a75a8,
			0xf2b32b48, 0xf2dd13e8, 0xf2e7fd48, 0x9e670103,
			0xd291a288, 0xf2bf8ec8, 0xf2cd1648, 0xf2f7fc68,
			0x9e670104, 0xaa0203ec, 0xaa0403ed, 0x2a0103e8,
			0x2a0003e9, 0x2f00e405, 0xaa0803ee, 0xaa0c03ef,
			0xaa0d03f0, 0x2f00e407, 0x2f00e406, 0xfd4001f0,
			0x1e650825, 0x1f421605, 0x1f4314c5, 0x1f4414f0,
			0xfc008610, 0xfc4085e5, 0x1e6040c7, 0x1e604206,
			0xf10005ce, 0x54fffec1, 0x9100054a, 0x8b0b01ad,
			0x8b0b018c, 0xeb09015f, 0x54fffd61, 0xd280000b,
			0xd10020ac, 0xd37df10a, 0xd100204d, 0xd28b578e,
			0xf2ae5b8e, 0xf2c98f8e, 0xf2e7f7ae, 0x9e6701c2,
			0xd29a75ae, 0xf2b32b4e, 0xf2dd13ee, 0xf2e7fd4e,
			0x9e6701c3, 0xd291a28e, 0xf2bf8ece, 0xf2cd164e,
			0xf2f7fc6e, 0x9e6701c4, 0x2f00e407, 0xaa0803ee,
			0x2f00e405, 0x2f00e410, 0x2f00e406, 0x1e6208e7,
			0x1f451c07, 0x1f431cc7, 0x1f441e11, 0xfc2e7991,
			0xfc6e79b0, 0x1e6040a7, 0x1e604205, 0x1e6040d0,
			0x1e604226, 0xf10005ce, 0x54fffeac, 0x9100056b,
			0x8b0a018c, 0x8b0a01ad, 0xeb09017f, 0x54fffd61,
			0xd280000b, 0xaa0403ec, 0xaa0503ed, 0xaa0303ee,
			0xaa0803ef, 0xaa0c03f0, 0xaa0d03f1, 0xaa0e03e2,
			0xfc408602, 0xfc408623, 0x1e632842, 0xfc008442,
			0xf10005ef, 0x54ffff61, 0x9100056b, 0x8b0a01ce,
			0x8b0a01ad, 0x8b0a018c, 0xeb09017f, 0x54fffe21,
			0x7100043f, 0x54000bab, 0x7100041f, 0x54000e6b,
			0xd2800009, 0xd37d7c2a, 0xd29c8008, 0xf2a05488,
			0xf2c4e368, 0xf2f7f908, 0x9e670102, 0xd29a75a8,
			0xf2b32b48, 0xf2dd13e8, 0xf2e7fd48, 0x9e670103,
			0xd291a288, 0xf2bf8ec8, 0xf2cd1648, 0xf2f7fc68,
			0x9e670104, 0xaa0303ec, 0xaa0403ed, 0x2a0103e8,
			0x2a0003eb, 0xd280000e, 0x2f00e405, 0xaa0b03ef,
			0x2f00e407, 0x2f00e406, 0xfc6e6990, 0x1e650825,
			0x1f421605, 0x1f4314c5, 0x1f4414f0, 0xfc2e69b0,
			0xfc6e6985, 0x8b0a01ce, 0x1e6040c7, 0x1e604206,
			0xf10005ef, 0x54fffea1, 0x91000529, 0x910021ad,
			0x9100218c, 0xeb08013f, 0x54fffd61, 0xd2800009,
			0x9100056a, 0xd100056b, 0x9b087d6b, 0xd37df16d,
			0x8b0d00ab, 0xd37df10c, 0x8b0d006d, 0xd28b578e,
			0xf2ae5b8e, 0xf2c98f8e, 0xf2e7f7ae, 0x9e6701c1,
			0xd29a75ae, 0xf2b32b4e, 0xf2dd13ee, 0xf2e7fd4e,
			0x9e6701c2, 0xd291a28e, 0xf2bf8ece, 0xf2cd164e,
			0xf2f7fc6e, 0x9e6701c3, 0xd280000e, 0x2f00e406,
			0xaa0a03ef, 0x2f00e405, 0x2f00e407, 0x2f00e404,
			0x1e6108c6, 0x1f451806, 0x1f421886, 0x1f4318f0,
			0xfc2e6970, 0x1e6040a6, 0xfc6e69a5, 0xd10005ef,
			0xcb0c01ce, 0x1e604087, 0x1e604204, 0xf10005ff,
			0x54fffe8c, 0x91000529, 0x9100216b, 0x910021ad,
			0xeb08013f, 0x54fffd21, 0x7100041f, 0x540002eb,
			0x7100043f, 0x540002ab, 0xd2800008, 0x2a0103e9,
			0xd37d7c2a, 0x2a0003eb, 0xaa0903ec, 0xaa0403ed,
			0xaa0503ee, 0xaa0303ef, 0xfc4085a0, 0xfc4085c1,
			0x1e612800, 0xfc0085e0, 0xf100058c, 0x54ffff61,
			0x91000508, 0x8b0a0063, 0x8b0a00a5, 0x8b0a0084,
			0xeb0b011f, 0x54fffe21, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		imgInBase := uint64(0x100000)
		imgOutBase := uint64(0x200000)
		y1Base := uint64(0x300000)
		y2Base := uint64(0x400000)
		stackBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		imgInBuf := make([]byte, w*h*8)
		for i, v := range imgIn {
			binary.LittleEndian.PutUint64(imgInBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(imgInBase, imgInBuf)
		em.LoadProgram(imgOutBase, make([]byte, w*h*8))
		em.LoadProgram(y1Base, make([]byte, w*h*8))
		em.LoadProgram(y2Base, make([]byte, w*h*8))
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(w))
		em.Regs.WriteX(1, uint64(h))
		em.Regs.WriteS(0, float32(0.25))
		em.Regs.WriteX(2, imgInBase)
		em.Regs.WriteX(3, imgOutBase)
		em.Regs.WriteX(4, y1Base)
		em.Regs.WriteX(5, y2Base)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	polybench := []Benchmark{
		{Name: "atax", Suite: "polybench", Setup: setupAtaxEmu, NativeTime: 0.000408, Regions: []DataRegion{
			{Addr: 0x100000, Size: 1279200}, {Addr: 0x300000, Size: 3280}, {Addr: 0x400000, Size: 3280}, {Addr: 0x500000, Size: 3120},
		}},
		{Name: "mvt", Suite: "polybench", Setup: setupMvtEmu, NativeTime: 0.000574, Regions: []DataRegion{
			{Addr: 0x100000, Size: 3200}, {Addr: 0x200000, Size: 3200}, {Addr: 0x300000, Size: 3200}, {Addr: 0x400000, Size: 3200}, {Addr: 0x500000, Size: 1280000},
		}},
		{Name: "gemm", Suite: "polybench", Setup: setupGemmEmu, NativeTime: 0.000271, Regions: []DataRegion{
			{Addr: 0x100000, Size: 33600}, {Addr: 0x200000, Size: 38400}, {Addr: 0x300000, Size: 44800}, {Addr: 0x400000, Size: 8192},
		}},
		{Name: "2mm", Suite: "polybench", Setup: setup2mmEmu, NativeTime: 0.000419, Regions: []DataRegion{
			{Addr: 0x100000, Size: 16000}, {Addr: 0x200000, Size: 22400}, {Addr: 0x300000, Size: 28000}, {Addr: 0x400000, Size: 32000}, {Addr: 0x500000, Size: 25600}, {Addr: 0x600000, Size: 8192},
		}},
		{Name: "3mm", Suite: "polybench", Setup: setup3mmEmu, NativeTime: 0.000804, Regions: []DataRegion{
			{Addr: 0x100000, Size: 16000}, {Addr: 0x200000, Size: 19200}, {Addr: 0x300000, Size: 24000}, {Addr: 0x400000, Size: 28000}, {Addr: 0x500000, Size: 32000}, {Addr: 0x600000, Size: 44800}, {Addr: 0x700000, Size: 22400}, {Addr: 0x800000, Size: 8192},
		}},
		{Name: "bicg", Suite: "polybench", Setup: setupBicgEmu, NativeTime: 0.000066, Regions: []DataRegion{
			{Addr: 0x100000, Size: 115072}, {Addr: 0x200000, Size: 992}, {Addr: 0x300000, Size: 928}, {Addr: 0x400000, Size: 992}, {Addr: 0x500000, Size: 928}, {Addr: 0x600000, Size: 8192},
		}},
		{Name: "gesummv", Suite: "polybench", Setup: setupGesummvEmu, NativeTime: 0.000277, Regions: []DataRegion{
			{Addr: 0x100000, Size: 500000}, {Addr: 0x200000, Size: 500000}, {Addr: 0x300000, Size: 2000}, {Addr: 0x400000, Size: 2000}, {Addr: 0x500000, Size: 2000}, {Addr: 0x600000, Size: 8192},
		}},
		{Name: "gemver", Suite: "polybench", Setup: setupGemverEmu, NativeTime: 0.000082, Regions: []DataRegion{
			{Addr: 0x100000, Size: 115200}, {Addr: 0x200000, Size: 960}, {Addr: 0x300000, Size: 960}, {Addr: 0x400000, Size: 960}, {Addr: 0x500000, Size: 960}, {Addr: 0x600000, Size: 960}, {Addr: 0x700000, Size: 960}, {Addr: 0x800000, Size: 960}, {Addr: 0x900000, Size: 960}, {Addr: 0xA00000, Size: 8192},
		}},
		{Name: "doitgen", Suite: "polybench", Setup: setupDoitgenEmu, NativeTime: 0.000494, Regions: []DataRegion{
			{Addr: 0x100000, Size: 120000}, {Addr: 0x200000, Size: 7200}, {Addr: 0x300000, Size: 240}, {Addr: 0x400000, Size: 8192},
		}},
		{Name: "trisolv", Suite: "polybench", Setup: setupTrisolvEmu, NativeTime: 0.000191, Regions: []DataRegion{
			{Addr: 0x100000, Size: 1280000}, {Addr: 0x300000, Size: 3200}, {Addr: 0x400000, Size: 3200},
		}},
		{Name: "durbin", Suite: "polybench", Setup: setupDurbinEmu, NativeTime: 0.000211, Regions: []DataRegion{
			{Addr: 0x100000, Size: 3200}, {Addr: 0x200000, Size: 3200}, {Addr: 0x300000, Size: 3200},
		}},
		{Name: "jacobi1d", Suite: "polybench", Setup: setupJacobi1dEmu, NativeTime: 0.000088, Regions: []DataRegion{
			{Addr: 0x100000, Size: 3200}, {Addr: 0x200000, Size: 3200},
		}},
		{Name: "syrk", Suite: "polybench", Setup: setupSyrkEmu, NativeTime: 0.000217, Regions: []DataRegion{
			{Addr: 0x100000, Size: 51200}, {Addr: 0x200000, Size: 38400}, {Addr: 0x300000, Size: 8192},
		}},
		{Name: "syr2k", Suite: "polybench", Setup: setupSyr2kEmu, NativeTime: 0.000350, Regions: []DataRegion{
			{Addr: 0x100000, Size: 51200}, {Addr: 0x200000, Size: 38400}, {Addr: 0x300000, Size: 38400}, {Addr: 0x400000, Size: 8192},
		}},
		{Name: "trmm", Suite: "polybench", Setup: setupTrmmEmu, NativeTime: 0.000184, Regions: []DataRegion{
			{Addr: 0x100000, Size: 28800}, {Addr: 0x200000, Size: 38400}, {Addr: 0x300000, Size: 8192},
		}},
		{Name: "symm", Suite: "polybench", Setup: setupSymmEmu, NativeTime: 0.000247, Regions: []DataRegion{
			{Addr: 0x100000, Size: 38400}, {Addr: 0x200000, Size: 28800}, {Addr: 0x300000, Size: 38400}, {Addr: 0x400000, Size: 8192},
		}},
		{Name: "cholesky", Suite: "polybench", Setup: setupCholeskyEmu, NativeTime: 0.000441, Regions: []DataRegion{
			{Addr: 0x100000, Size: 115200}, {Addr: 0x200000, Size: 8192},
		}},
		{Name: "lu", Suite: "polybench", Setup: setupLUEmu, NativeTime: 0.000833, Regions: []DataRegion{
			{Addr: 0x100000, Size: 115200}, {Addr: 0x200000, Size: 8192},
		}},
		{Name: "floydwarshall", Suite: "polybench", Setup: setupFloydWarshallEmu, NativeTime: 0.007104, Regions: []DataRegion{
			{Addr: 0x100000, Size: 259200}, {Addr: 0x200000, Size: 8192},
		}},
		{Name: "covariance", Suite: "polybench", Setup: setupCovarianceEmu, NativeTime: 0.000520, Regions: []DataRegion{
			{Addr: 0x100000, Size: 64000}, {Addr: 0x200000, Size: 51200}, {Addr: 0x300000, Size: 640}, {Addr: 0x400000, Size: 8192},
		}},
		{Name: "correlation", Suite: "polybench", Setup: setupCorrelationEmu, NativeTime: 0.000524, Regions: []DataRegion{
			{Addr: 0x100000, Size: 64000}, {Addr: 0x200000, Size: 51200}, {Addr: 0x300000, Size: 640}, {Addr: 0x400000, Size: 640}, {Addr: 0x500000, Size: 8192},
		}},
		{Name: "gramschmidt", Suite: "polybench", Setup: setupGramschmidtEmu, NativeTime: 0.000537, Regions: []DataRegion{
			{Addr: 0x100000, Size: 38400}, {Addr: 0x200000, Size: 51200}, {Addr: 0x300000, Size: 38400}, {Addr: 0x400000, Size: 8192},
		}},
		{Name: "ludcmp", Suite: "polybench", Setup: setupLudcmpEmu, NativeTime: 0.001128, Regions: []DataRegion{
			{Addr: 0x100000, Size: 115200}, {Addr: 0x200000, Size: 960}, {Addr: 0x300000, Size: 960}, {Addr: 0x400000, Size: 960}, {Addr: 0x500000, Size: 8192},
		}},
		{Name: "nussinov", Suite: "polybench", Setup: setupNussinovEmu, NativeTime: 0.001175, Regions: []DataRegion{
			{Addr: 0x100000, Size: 180}, {Addr: 0x200000, Size: 129600}, {Addr: 0x300000, Size: 8192},
		}},
		{Name: "adi", Suite: "polybench", Setup: setupAdiEmu, NativeTime: 0.002739, Regions: []DataRegion{
			{Addr: 0x100000, Size: 28800}, {Addr: 0x200000, Size: 28800}, {Addr: 0x300000, Size: 28800}, {Addr: 0x400000, Size: 28800}, {Addr: 0x500000, Size: 16384},
		}},
		{Name: "fdtd2d", Suite: "polybench", Setup: setupFdtd2dEmu, NativeTime: 0.000817, Regions: []DataRegion{
			{Addr: 0x100000, Size: 38400}, {Addr: 0x200000, Size: 38400}, {Addr: 0x300000, Size: 38400}, {Addr: 0x400000, Size: 320}, {Addr: 0x500000, Size: 16384},
		}},
		{Name: "heat3d", Suite: "polybench", Setup: setupHeat3dEmu, NativeTime: 0.001523, Regions: []DataRegion{
			{Addr: 0x100000, Size: 64000}, {Addr: 0x200000, Size: 64000}, {Addr: 0x300000, Size: 16384},
		}},
		{Name: "jacobi2d", Suite: "polybench", Setup: setupJacobi2dEmu, NativeTime: 0.001109, Regions: []DataRegion{
			{Addr: 0x100000, Size: 64800}, {Addr: 0x200000, Size: 64800}, {Addr: 0x300000, Size: 8192},
		}},
		{Name: "seidel2d", Suite: "polybench", Setup: setupSeidel2dEmu, NativeTime: 0.007820, Regions: []DataRegion{
			{Addr: 0x100000, Size: 115200}, {Addr: 0x200000, Size: 8192},
		}},
		{Name: "deriche", Suite: "polybench", Setup: setupDericheEmu, NativeTime: 0.000441, Regions: []DataRegion{
			{Addr: 0x100000, Size: 196608}, {Addr: 0x200000, Size: 196608}, {Addr: 0x300000, Size: 196608}, {Addr: 0x400000, Size: 196608}, {Addr: 0x500000, Size: 8192},
		}},
	}

	// Append PARSEC benchmarks.
	polybench = append(polybench, parsecAll()...)
	return polybench
}

// parsecAll returns the PARSEC benchmark definitions.
func parsecAll() []Benchmark {
	setupSwaptionsEmu := func() *emu.Emulator {
		n := 50
		r := 0.05
		dt := 0.25

		// Build positive-definite matrix: A = B*B^T + n*I
		Bmat := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				Bmat[i*n+j] = float64((i*3+j*7+1)%(n+1)) / float64(n+1)
			}
		}
		A := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				for k := 0; k < n; k++ {
					A[i*n+j] += Bmat[i*n+k] * Bmat[j*n+k]
				}
			}
			A[i*n+i] += float64(n)
		}

		// Z vector
		Z := make([]float64, n)
		for i := 0; i < n; i++ {
			Z[i] = float64(i*3-n) * 0.2
		}

		// Cholesky code
		cholCode := []uint32{
			0x7100043f, 0x5400072b, 0xd280000c, 0x2a0103e8,
			0xd37d7c29, 0x8b09000a, 0x5280002b, 0xaa0003ed,
			0x14000007, 0x9100056b, 0x8b0901ad, 0x8b09014a,
			0xaa0e03ec, 0xeb0801df, 0x54000580, 0xb400012c,
			0xd280000e, 0x2f00e400, 0xfc6e79a1, 0x1f410020,
			0x910005ce, 0xeb0e019f, 0x54ffff81, 0x14000002,
			0x2f00e400, 0x1b017d8f, 0x8b0c0c10, 0xfc6f5a01,
			0x1e603820, 0x1e61c000, 0xfc2f5a00, 0x9100058e,
			0xeb0801df, 0x54fffd02, 0xaa0a03f1, 0xaa0b03e2,
			0x1400000c, 0x2f00e400, 0x9b087c43, 0xfc637a01,
			0x1e603820, 0xfc6f5a01, 0x1e611800, 0xfc237a00,
			0x91000442, 0x8b090231, 0xeb08005f, 0x54fffb40,
			0xb4fffeac, 0xd2800003, 0x2f00e400, 0xfc637a21,
			0xfc6379a2, 0x1f420020, 0x91000463, 0xeb03019f,
			0x54ffff61, 0x17ffffed, 0xd65f03c0,
		}

		// HJM code
		hjmCode := []uint32{
			0xa9bd57f6, 0xa9014ff4, 0xa9027bfd, 0x910083fd,
			0xaa0303f6, 0xaa0203f3, 0xaa0103f4, 0xaa0003f5,
			0x1e618800, // fnmul d0, d0, d1
			0x94000000, // BL _exp (will be patched)
			0x710006df, 0x5400028b,
			0xd2800008, 0xd37d7ec9, 0x5280002a, 0x2a1603eb,
			0xd280000c, 0x2f00e401, 0xfc6c7a82, 0xfc6c7a63,
			0x1f430441, 0x9100058c, 0xeb0c015f, 0x54ffff61,
			0x1e610801, 0xfc287aa1, 0x91000508, 0x9100054a,
			0x8b090294, 0xeb0b011f, 0x54fffe41, 0xa9427bfd,
			0xa9414ff4, 0xa8c357f6, 0xd65f03c0,
		}

		// Trampoline: after cholesky RET, sets up HJM regs and branches.
		// The trampoline moves register values for HJM and then B to hjmBase.
		// HJM expects: X0=dfBase, X1=lBase(=aBase), X2=zBase, D0=r, D1=dt, X3=n
		// After cholesky: we restore from saved state on the trampoline.
		//
		// We encode the trampoline as: a series of MOV/MOVK to set regs, then B to hjm.
		// For simplicity, we pre-compute everything in Go and use a different approach:
		// Load fixed register values from a data area, then branch.

		cholBase := uint64(0x10000)
		hjmBase := uint64(0x20000)
		trampolineBase := uint64(0x30000)
		expHookAddr := uint64(0x40000)
		aBase := uint64(0x100000)
		dfBase := uint64(0x200000)
		zBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		dataBase := uint64(0x500000) // trampoline data area
		haltPC := uint64(0xFFFF0000)

		// Patch HJM BL at index 9 to target expHookAddr
		blPC := hjmBase + 9*4
		blOffset := int64(expHookAddr) - int64(blPC)
		imm26 := uint32(blOffset/4) & 0x03FFFFFF
		hjmCode[9] = 0x94000000 | imm26

		// Build trampoline code:
		// LDR X0, [X8, #0]   -> dfBase
		// LDR X1, [X8, #8]   -> aBase (L is stored in-place)
		// LDR X2, [X8, #16]  -> zBase
		// LDR X3, [X8, #24]  -> n
		// LDR D0, [X8, #32]  -> r
		// LDR D1, [X8, #40]  -> dt
		// MOV X30, haltPC (needs MOVZ + MOVK)
		// MOV SP, stackBase+8192 (needs MOVZ + MOVK)
		// B hjmBase
		trampolineCode := []uint32{
			0xf9400100, // LDR X0, [X8, #0]    -> dfBase
			0xf9400501, // LDR X1, [X8, #8]    -> aBase
			0xf9400902, // LDR X2, [X8, #16]   -> zBase
			0xf9400d03, // LDR X3, [X8, #24]   -> n
			0xfd401100, // LDR D0, [X8, #32]   -> r
			0xfd401501, // LDR D1, [X8, #40]   -> dt
		}
		// MOVZ X30, #0 + MOVK to encode haltPC = 0xFFFF0000
		// haltPC = 0xFFFF_0000 → MOVZ X30, #0 ; MOVK X30, #0xFFFF, LSL#16
		trampolineCode = append(trampolineCode,
			0xd280001e,                             // MOVZ X30, #0
			0xf2bfffe0|0x1e,                        // MOVK X30, #0xFFFF, LSL#16
			0xd2900000|0x1f,                         // MOVZ SP, #0x8000 (32768 = stackBase+8192 low bits if we use full addr)
		)
		// Actually, SP setup is tricky. Let me just set SP from the data area too.
		// Rewrite: use a simpler trampoline approach.

		// Simpler trampoline: load everything from memory
		// X8 is already set to dataBase before cholesky starts.
		// But we need X8 to survive cholesky. Let's use a callee-saved register.
		// Actually, cholesky clobbers many registers. Let's load X8 from a known address.

		// Even simpler: set X8 to dataBase via MOVZ+MOVK, then LDR from there.
		// dataBase = 0x500000
		// MOVZ X8, #0x0000 ; MOVK X8, #0x50, LSL#16
		trampolineCode = []uint32{
			0xd2800008,          // MOVZ X8, #0
			0xf2a00a08,          // MOVK X8, #0x50, LSL#16
			0xf9400100,          // LDR X0, [X8, #0]    -> dfBase
			0xf9400501,          // LDR X1, [X8, #8]    -> aBase
			0xf9400902,          // LDR X2, [X8, #16]   -> zBase
			0xf9400d03,          // LDR X3, [X8, #24]   -> n
			0xfd401100,          // LDR D0, [X8, #32]   -> r
			0xfd401501,          // LDR D1, [X8, #40]   -> dt
			0xf9401900,          // LDR X0_temp, [X8, #48] -> SP value, load into X0 temp
		}
		// Hmm, this is getting complicated. Let me use a much simpler approach.

		// Pre-compute the Cholesky L in Go, then set up a single emulator for HJM only.
		// This is the pragmatic approach.
		_ = cholCode
		_ = cholBase
		_ = trampolineBase
		_ = trampolineCode
		_ = dataBase

		// Pre-compute golden Cholesky L.
		L := make([]float64, n*n)
		copy(L, A)
		for j := 0; j < n; j++ {
			sum := 0.0
			for k := 0; k < j; k++ {
				sum += L[j*n+k] * L[j*n+k]
			}
			L[j*n+j] = math.Sqrt(L[j*n+j] - sum)
			for i := j + 1; i < n; i++ {
				sum = 0.0
				for k := 0; k < j; k++ {
					sum += L[i*n+k] * L[j*n+k]
				}
				L[i*n+j] = (L[i*n+j] - sum) / L[j*n+j]
			}
		}

		// Now set up emulator for the combined benchmark.
		// We lay out: cholesky code at cholBase, then a small trampoline, then HJM code.
		// Cholesky RET → X30 (trampoline). Trampoline sets regs, B to HJM.
		// HJM RET → X30 (haltPC).

		// Revised approach: Use two code sections in the same emulator.
		// 1) Cholesky at cholBase, with X30=trampolineBase
		// 2) Trampoline at trampolineBase loads regs from data area, then B to hjmBase
		// 3) HJM at hjmBase, with X30 restored from stack (it saves/restores X30)

		// Actually, HJM code saves X30 on entry (STP X22,X30,...) and restores it on exit (LDP).
		// So we need to make sure X30=haltPC is set before HJM's RET.
		// The HJM code saves X30 on the stack. If we set X30=haltPC before calling HJM,
		// HJM will save it, then restore it, then RET to haltPC. But HJM does BL exp,
		// which clobbers X30. HJM saves X30 initially and restores before RET.
		// So if X30=haltPC at HJM entry, it'll be saved and restored correctly.

		// Trampoline code:
		// Load regs from data area at dataBase:
		//   [dataBase+0]: dfBase, [+8]: aBase, [+16]: zBase, [+24]: n
		//   [+32]: r (as float64 bits), [+40]: dt (as float64 bits)
		//   [+48]: haltPC, [+56]: stackBase+8192
		trampolineCode = []uint32{
			// MOVZ X8, #0 ; MOVK X8, #0x50, LSL#16 → X8 = 0x500000 = dataBase
			0xd2800008, // MOVZ X8, #0
			0xf2a00a08, // MOVK X8, #0x50, LSL#16
			0xf9400100, // LDR X0, [X8, #0]    → dfBase
			0xf9400501, // LDR X1, [X8, #8]    → aBase (=lBase, cholesky done in-place)
			0xf9400902, // LDR X2, [X8, #16]   → zBase
			0xf9400d03, // LDR X3, [X8, #24]   → n
			0xfd401100, // LDR D0, [X8, #32]   → r
			0xfd401501, // LDR D1, [X8, #40]   → dt
			0xf940191e, // LDR X30, [X8, #48]  → haltPC
			0xf9401d1f, // LDR XZR/SP... (can't LDR into SP directly with this encoding)
		}

		// Loading SP is tricky. Let me use: LDR X9, [X8, #56] ; MOV SP, X9
		trampolineCode = []uint32{
			0xd2800008, // MOVZ X8, #0
			0xf2a00a08, // MOVK X8, #0x50, LSL#16
			0xf9400100, // LDR X0, [X8, #0]    → dfBase
			0xf9400501, // LDR X1, [X8, #8]    → aBase
			0xf9400902, // LDR X2, [X8, #16]   → zBase
			0xf9400d03, // LDR X3, [X8, #24]   → n
			0xfd401100, // LDR D0, [X8, #32]   → r
			0xfd401501, // LDR D1, [X8, #40]   → dt
			0xf940191e, // LDR X30, [X8, #48]  → haltPC
			0xf9401d09, // LDR X9, [X8, #56]   → stack top
			0x9100013f, // MOV SP, X9  (ADD SP, X9, #0)
		}
		// B to hjmBase: encode as B offset
		branchOffset := int64(hjmBase) - int64(trampolineBase+11*4)
		branchImm26 := uint32(branchOffset/4) & 0x03FFFFFF
		trampolineCode = append(trampolineCode, 0x14000000|branchImm26)

		em := emu.NewEmulator()
		em.MaxInst = 10_000_000

		// Load code sections
		em.LoadProgram(cholBase, EncodeInsts(cholCode))
		em.LoadProgram(trampolineBase, EncodeInsts(trampolineCode))
		em.LoadProgram(hjmBase, EncodeInsts(hjmCode))

		// Load data: A matrix
		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		// Load data: Z vector
		zBufData := make([]byte, n*8)
		for i, v := range Z {
			binary.LittleEndian.PutUint64(zBufData[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(zBase, zBufData)

		// Load dF output (zero)
		em.LoadProgram(dfBase, make([]byte, n*8))

		// Stack
		em.LoadProgram(stackBase, make([]byte, 8192))

		// Trampoline data area
		dataBuf := make([]byte, 64)
		binary.LittleEndian.PutUint64(dataBuf[0:], dfBase)
		binary.LittleEndian.PutUint64(dataBuf[8:], aBase)
		binary.LittleEndian.PutUint64(dataBuf[16:], zBase)
		binary.LittleEndian.PutUint64(dataBuf[24:], uint64(n))
		binary.LittleEndian.PutUint64(dataBuf[32:], math.Float64bits(r))
		binary.LittleEndian.PutUint64(dataBuf[40:], math.Float64bits(dt))
		binary.LittleEndian.PutUint64(dataBuf[48:], haltPC)
		binary.LittleEndian.PutUint64(dataBuf[56:], stackBase+8192)
		em.LoadProgram(dataBase, dataBuf)

		// Set initial registers for Cholesky phase:
		// X0=aBase, X1=n, X30=trampolineBase (so RET goes to trampoline)
		em.Regs.PC = cholBase
		em.Regs.WriteX(0, aBase)
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(30, trampolineBase) // after cholesky RET → trampoline
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)

		// Register exp function hook
		em.RegisterFuncHook(expHookAddr, func(emInner *emu.Emulator) {
			input := emInner.Regs.ReadD(0)
			emInner.Regs.WriteD(0, math.Exp(input))
		})

		return em
	}

	// --- Black-Scholes benchmark ---
	setupBlackScholesEmu := func() *emu.Emulator {
		numOptions := 1024

		stockPrices := make([]float64, numOptions)
		strikePrices := make([]float64, numOptions)
		riskFreeRate := make([]float64, numOptions)
		volatility := make([]float64, numOptions)
		timeToExp := make([]float64, numOptions)
		optionType := make([]int32, numOptions)
		for i := 0; i < numOptions; i++ {
			stockPrices[i] = 30.0 + float64(i%80)*1.25
			strikePrices[i] = 28.0 + float64(i%80)*1.25
			riskFreeRate[i] = 0.03 + float64(i%10)*0.005
			volatility[i] = 0.15 + float64(i%20)*0.02
			timeToExp[i] = 0.25 + float64(i%8)*0.25
			optionType[i] = int32(i % 2)
		}

		code := []uint32{
			0x710004ff, 0x5400156b, 0xd10443ff, 0x6d073bef,
			0x6d0833ed, 0x6d092beb, 0x6d0a23e9, 0xa90b6ffc,
			0xa90c67fa, 0xa90d5ff8, 0xa90e57f6, 0xa90f4ff4,
			0xa9107bfd, 0x910403fd, 0xaa0603f3, 0xaa0503f4,
			0xaa0403f5, 0xaa0303f6, 0xaa0203f7, 0xaa0103f8,
			0xaa0003f9, 0xd28201c8, 0xf2a30e28, 0xf2d4ce28,
			0xf2e7f9a8, 0xf9001be8, 0xd2864b68, 0xf2bcb508,
			0xf2da3e08, 0xf2f7fac8, 0xf90017e8, 0xd29f1d88,
			0xf2a75248, 0xf2ce17e8, 0xf2e7fa88, 0xf90013e8,
			0xd28bcd08, 0xf2a04be8, 0xf2d01de8, 0xf2e7ff88,
			0xf9000fe8, 0xd28f1a08, 0xf2a9de48, 0xf2c47ba8,
			0xf2f7ffa8, 0xf9000be8, 0xd2852868, 0xf2bade88,
			0xf2c919a8, 0xf2e7fea8, 0xf90033e8, 0xd286ca28,
			0xf2a67a88, 0xf2d108a8, 0xf2f7fb28, 0xf90007e8,
			0x2a0703fa, 0xfd400280, 0x1e61c00e, 0x1e604008,
			0xfd0023e0, 0xfc408700, 0xfc4086e1, 0x6d0503e1,
			0x1e611800, 0x940037bf, 0xfc4086c3, 0xfd0027e3,
			0xfc4086a1, 0x1e610822, 0x1e6c1004, 0x1f440c42,
			0x1e6c1003, 0x1f480040, 0x1e6109c2, 0x1e62180f,
			0x1f4ebc20, 0xfd0037e0, 0x1e60c1e0, 0xfd401beb,
			0x1e6e1008, 0x1f4b2001, 0x1e611909, 0x1e608800,
			0x1e630800, 0x940037eb, 0x6d4237ea, 0x1e6d0921,
			0x1e610921, 0x1f4a0521, 0x6d413bec, 0x1e6e0922,
			0x1e620922, 0x1f490441, 0x1e6c0922, 0x1e620922,
			0x1e620922, 0x1f490441, 0xfd4033e2, 0x1e620922,
			0x1e620922, 0x1e620922, 0x1e620922, 0x1f490441,
			0xfd4007e9, 0x1e690800, 0x1f412000, 0x1e603901,
			0x1e6021e8, 0x1e604c20, 0xfd001fe0, 0xfd4037e0,
			0x1e60c000, 0x1f4b2001, 0x1e61190f, 0x1e608800,
			0x1e6c1001, 0x1e610800, 0x940037ca, 0x1e6d09e1,
			0x1e6109e1, 0x1f4a05e1, 0x1e6e09e2, 0x1e6209e2,
			0x1f4f0441, 0x1e6c09e2, 0x1e6209e2, 0x1e6209e2,
			0xfd4033e3, 0x1e6309e3, 0x1f4f0441, 0x1e6309e2,
			0x1e6209e2, 0x1e6209e2, 0x1f4f0441, 0x1e690800,
			0x1f412001, 0x1e613902, 0xfd4037e0, 0x1e602008,
			0xb840467b, 0x6d4403e3, 0x1e638800, 0x1e614c4c,
			0x940037b0, 0x6d4513e5, 0x1e6008a1, 0x1e6c3902,
			0xfd401fe6, 0x1e663903, 0x1e638883, 0x1f420c21,
			0x1e6088a0, 0x1e6c0800, 0x1f460080, 0x7100037f,
			0x1e601c20, 0xfc008720, 0x91002294, 0xf100075a,
			0x54fff321, 0xa9507bfd, 0xa94f4ff4, 0xa94e57f6,
			0xa94d5ff8, 0xa94c67fa, 0xa94b6ffc, 0x6d4a23e9,
			0x6d492beb, 0x6d4833ed, 0x6d473bef, 0x910443ff,
			0xd65f03c0,
		}

		codeBase := uint64(0x1000)
		logAddr := uint64(0xF000)
		expAddr := uint64(0xF100)

		pricesBase := uint64(0x100000)
		sBase := uint64(0x200000)
		xBase := uint64(0x300000)
		rBase := uint64(0x400000)
		vBase := uint64(0x500000)
		tBase := uint64(0x600000)
		otypeBase := uint64(0x700000)
		stackBase := uint64(0x800000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 10_000_000

		em.LoadProgram(codeBase, EncodeInsts(code))

		writeF64 := func(base uint64, data []float64) {
			buf := make([]byte, len(data)*8)
			for i, v := range data {
				binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
			}
			em.LoadProgram(base, buf)
		}
		writeI32 := func(base uint64, data []int32) {
			buf := make([]byte, len(data)*4)
			for i, v := range data {
				binary.LittleEndian.PutUint32(buf[i*4:], uint32(v))
			}
			em.LoadProgram(base, buf)
		}

		writeF64(pricesBase, make([]float64, numOptions))
		writeF64(sBase, stockPrices)
		writeF64(xBase, strikePrices)
		writeF64(rBase, riskFreeRate)
		writeF64(vBase, volatility)
		writeF64(tBase, timeToExp)
		writeI32(otypeBase, optionType)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, pricesBase)
		em.Regs.WriteX(1, sBase)
		em.Regs.WriteX(2, xBase)
		em.Regs.WriteX(3, rBase)
		em.Regs.WriteX(4, vBase)
		em.Regs.WriteX(5, tBase)
		em.Regs.WriteX(6, otypeBase)
		em.Regs.WriteX(7, uint64(numOptions))
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 4096
		em.SetHaltPC(haltPC)

		// Register BL-intercept hooks for log and exp
		em.RegisterFuncHook(logAddr, func(emInner *emu.Emulator) {
			x := emInner.Regs.ReadD(0)
			emInner.Regs.WriteD(0, math.Log(x))
		})
		em.RegisterFuncHook(expAddr, func(emInner *emu.Emulator) {
			x := emInner.Regs.ReadD(0)
			emInner.Regs.WriteD(0, math.Exp(x))
		})

		return em
	}

	// --- Streamcluster benchmark ---
	setupStreamclusterEmu := func() *emu.Emulator {
		npoints, dim := 512, 32
		totalCoords := npoints * dim

		p1 := make([]float64, totalCoords)
		p2 := make([]float64, totalCoords)
		for i := 0; i < totalCoords; i++ {
			p1[i] = 1.0 + float64(i)*0.01
			p2[i] = 2.0 - float64(i)*0.02
		}

		// ARM64 machine code for distance computation kernel:
		//   X0 = p1 pointer, X1 = p2 pointer, X2 = out pointer
		//   W3 = npoints, W4 = dim
		// Compiled with: clang -target arm64-apple-macos -O1
		//   -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
		code := []uint32{
			0x7100047f, // cmp   w3, #1
			0x5400032b, // b.lt  end
			0xd2800008, // mov   x8, #0
			0x2a0403e9, // mov   w9, w4
			0xd37d7c8a, // ubfiz x10, x4, #3, #32
			0x2a0303eb, // mov   w11, w3
			0x14000007, // b     outer_loop
			0xfc287840, // str   d0, [x2, x8, lsl #3]
			0x91000508, // add   x8, x8, #1
			0x8b0a0021, // add   x1, x1, x10
			0x8b0a0000, // add   x0, x0, x10
			0xeb0b011f, // cmp   x8, x11
			0x540001c0, // b.eq  end
			0x2f00e400, // movi  d0, #0
			0x7100049f, // cmp   w4, #1
			0x54ffff0b, // b.lt  store_result
			0xaa0903ec, // mov   x12, x9
			0xaa0003ed, // mov   x13, x0
			0xaa0103ee, // mov   x14, x1
			0xfc4085a1, // ldr   d1, [x13], #8
			0xfc4085c2, // ldr   d2, [x14], #8
			0x1e623821, // fsub  d1, d1, d2
			0x1f410020, // fmadd d0, d1, d1, d0
			0xf100058c, // subs  x12, x12, #1
			0x54ffff61, // b.ne  inner_loop
			0x17ffffee, // b     store_result
			0xd65f03c0, // ret
		}

		codeBase := uint64(0x10000)
		p1Base := uint64(0x100000)
		p2Base := uint64(0x200000)
		outBase := uint64(0x300000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		p1Buf := make([]byte, totalCoords*8)
		for i, v := range p1 {
			binary.LittleEndian.PutUint64(p1Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(p1Base, p1Buf)

		p2Buf := make([]byte, totalCoords*8)
		for i, v := range p2 {
			binary.LittleEndian.PutUint64(p2Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(p2Base, p2Buf)

		em.LoadProgram(outBase, make([]byte, npoints*8))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, p1Base)
		em.Regs.WriteX(1, p2Base)
		em.Regs.WriteX(2, outBase)
		em.Regs.WriteX(3, uint64(npoints))
		em.Regs.WriteX(4, uint64(dim))
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x400000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// --- Canneal benchmark ---
	setupCannealEmu := func() *emu.Emulator {
		n, m := 8192, 512

		netlist := make([]int32, n*2)
		for i := 0; i < n*2; i++ {
			netlist[i] = int32(i % m)
		}

		locations := make([]int32, m*2)
		for i := 0; i < m; i++ {
			locations[2*i] = int32((i * 37) % 1000)
			locations[2*i+1] = int32((i * 73) % 1000)
		}

		// ARM64 machine code for routing_cost kernel:
		//   X0 = netlist pointer, X1 = locations pointer, W2 = n
		//   Returns total_cost in W0
		//
		// Note: CNEG conditions use PL instead of MI to match the
		// emulator's CNEG implementation (condition sense is inverted
		// relative to hardware).
		code := []uint32{
			0x7100045f, // cmp    w2, #1
			0x5400034b, // b.lt   LBB0_4 (offset +0x68)
			0xaa0003e8, // mov    x8, x0
			0x52800000, // mov    w0, #0
			0x91001108, // add    x8, x8, #4
			0x52800029, // mov    w9, #1
			0x2a0203ea, // mov    w10, w2
			// LBB0_2 (loop body):
			0x697fb10b, // ldpsw  x11, x12, [x8, #-4]
			0xd37df16d, // lsl    x13, x11, #3
			0xb86d682d, // ldr    w13, [x1, x13]
			0xd37df18e, // lsl    x14, x12, #3
			0xb86e682e, // ldr    w14, [x1, x14]
			0x2a0b052b, // orr    w11, w9, w11, lsl #1
			0xb86bd82b, // ldr    w11, [x1, w11, sxtw #2]
			0x6b0e01ad, // subs   w13, w13, w14
			0x2a0c052c, // orr    w12, w9, w12, lsl #1
			0xb86cd82c, // ldr    w12, [x1, w12, sxtw #2]
			0x4b0c016b, // sub    w11, w11, w12
			0x5a8d45ac, // cneg   w12, w13, pl  (abs of dx)
			0x7100017f, // cmp    w11, #0
			0x5a8b456b, // cneg   w11, w11, pl  (abs of dy)
			0x0b00018c, // add    w12, w12, w0
			0x0b0b0180, // add    w0, w12, w11
			0x91002108, // add    x8, x8, #8
			0xf100054a, // subs   x10, x10, #1
			0x54fffdc1, // b.ne   LBB0_2 (offset -0x48)
			// LBB0_3:
			0xd65f03c0, // ret
			// LBB0_4:
			0x52800000, // mov    w0, #0
			0xd65f03c0, // ret
		}

		codeBase := uint64(0x10000)
		netlistBase := uint64(0x100000)
		locationsBase := uint64(0x200000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		// Write netlist as little-endian int32
		netlistBuf := make([]byte, n*2*4)
		for i, v := range netlist {
			binary.LittleEndian.PutUint32(netlistBuf[i*4:], uint32(v))
		}
		em.LoadProgram(netlistBase, netlistBuf)

		// Write locations as little-endian int32
		locationsBuf := make([]byte, m*2*4)
		for i, v := range locations {
			binary.LittleEndian.PutUint32(locationsBuf[i*4:], uint32(v))
		}
		em.LoadProgram(locationsBase, locationsBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, netlistBase)
		em.Regs.WriteX(1, locationsBase)
		em.Regs.WriteX(2, uint64(n))
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x300000 + 8192
		em.LoadProgram(0x300000, make([]byte, 8192))
		em.SetHaltPC(haltPC)
		return em
	}

	// --- Fluidanimate benchmark ---
	setupFluidanimateEmu := func() *emu.Emulator {
		numParticles := 16
		n := numParticles
		hVal := 1.0

		// Particle positions in a 4x4 grid within smoothing radius
		pxData := make([]float64, n)
		pyData := make([]float64, n)
		pzData := make([]float64, n)
		for i := 0; i < n; i++ {
			pxData[i] = float64(i%4) * 0.3
			pyData[i] = float64(i/4) * 0.3
			pzData[i] = 0.0
		}

		// Particle velocities
		vxData := make([]float64, n)
		vyData := make([]float64, n)
		vzData := make([]float64, n)
		for i := 0; i < n; i++ {
			vxData[i] = 0.1 * float64(1-2*(i%2))
			vyData[i] = 0.1 * float64(1-2*((i/2)%2))
			vzData[i] = 0.0
		}

		massVal := 1.0
		rho0Val := 1000.0
		kVal := 3.0
		muVal := 3.5

		code := []uint32{
			0xb9401be8, 0x7100051f, 0x5400130b, 0xd280000c,
			0xd285a30a, 0xf2aa888a, 0xf2c43f6a, 0xf2e8012a,
			0x9e670145, 0xd2d0000a, 0xf2f808ca, 0x9e670146,
			0xd2d0000a, 0xf2e808ca, 0x9e670147, 0xd285a30a,
			0xf2aa888a, 0xf2c43f6a, 0xf2e80d2a, 0x9e670150,
			0xd2d6000a, 0xf2e80e6a, 0x9e670151, 0xa940a7ea,
			0xf94003eb, 0x1e650805, 0x1e6008a5, 0x1e6008a5,
			0x1e6008a5, 0x1e6008a5, 0x1e6008b2, 0x1e7218c5,
			0x1e7218e6, 0x1e600807, 0x1e700810, 0x1e600a10,
			0x1e600a10, 0x1e600a10, 0x1e600a10, 0x1e600a10,
			0x1e600a10, 0x1e600a10, 0x1e600a10, 0x1e701a30,
			0x1e610a10, 0xfc6c7811, 0xfc6c7833, 0x2f00e412,
			0xaa0803ed, 0xfc6c7854, 0xaa0003ee, 0xaa0103ef,
			0xaa0203f0, 0xfc4085d5, 0x1e753a35, 0xfc4085f6,
			0xfc408617, 0x1e763a76, 0x1e773a97, 0x1e760ad6,
			0x1f555ab5, 0x1f5756f5, 0x1e7538f6, 0x1e760a17,
			0x1e770ad7, 0x1f564af6, 0x1e6722a0, 0x1e724ed2,
			0xf10005ad, 0x54fffe01, 0xfc2c78d2, 0x1e623a51,
			0x1e630a31, 0xfc2c78f1, 0x9100058c, 0xeb08019f,
			0x54fffc21, 0x7100051f, 0x5400098b, 0xd280000c,
			0xd280000d, 0x1e614022, 0x1e640821, 0xd37d7d0e,
			0xd29d422f, 0xf2b025af, 0xf2d2f32f, 0xf2e7ae2f,
			0x14000008, 0xfc2d7963, 0xfc2d7944, 0xfc2d7930,
			0x910005ad, 0x9100218c, 0xeb0801bf, 0x54000760,
			0xd2800010, 0x2f00e403, 0x2f00e404, 0x2f00e410,
			0x14000004, 0x91002210, 0xeb1001df, 0x54fffe40,
			0xeb10019f, 0x54ffff80, 0xfc6d7811, 0xfc706812,
			0x1e723a31, 0xfc6d7832, 0xfc706833, 0x1e733a52,
			0xfc6d7853, 0xfc706854, 0x1e743a73, 0x1e720a54,
			0x1f515234, 0x1f535274, 0x1e672280, 0x9e6701f5,
			0x1e754684, 0x54fffd8d, 0x1e61c294, 0x1e743815,
			0xfc6d78f6, 0xfc7068f7, 0x1e772ad6, 0x1e620ad6,
			0xfc7068d7, 0x1e772af8, 0x1e781ad6, 0x1e7508b8,
			0x1e780ab8, 0x1e760b16, 0x1e741ad4, 0x1f510e83,
			0x1f521284, 0x1f534290, 0x1e771831, 0x1e7508d2,
			0x1e710a51, 0xfc706872, 0xfc6d7873, 0x1e733a52,
			0x1f520e23, 0xfc706892, 0xfc6d7893, 0x1e733a52,
			0x1f521224, 0xfc7068b2, 0xfc6d78b3, 0x1e733a52,
			0x1f524230, 0x17ffffcc, 0xd65f03c0,
		}

		codeBase := uint64(0x1000)
		pxBase := uint64(0x100000)
		pyBase := pxBase + uint64(n)*8
		pzBase := pyBase + uint64(n)*8
		vxBase := pzBase + uint64(n)*8
		vyBase := vxBase + uint64(n)*8
		vzBase := vyBase + uint64(n)*8
		densityBase := vzBase + uint64(n)*8
		pressureBase := densityBase + uint64(n)*8
		fxBase := pressureBase + uint64(n)*8
		fyBase := fxBase + uint64(n)*8
		fzBase := fyBase + uint64(n)*8
		stackBase := uint64(0x800000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 10_000_000

		em.LoadProgram(codeBase, EncodeInsts(code))

		writeF64 := func(base uint64, data []float64) {
			buf := make([]byte, len(data)*8)
			for i, v := range data {
				binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
			}
			em.LoadProgram(base, buf)
		}

		writeF64(pxBase, pxData)
		writeF64(pyBase, pyData)
		writeF64(pzBase, pzData)
		writeF64(vxBase, vxData)
		writeF64(vyBase, vyData)
		writeF64(vzBase, vzData)
		writeF64(densityBase, make([]float64, n))
		writeF64(pressureBase, make([]float64, n))
		writeF64(fxBase, make([]float64, n))
		writeF64(fyBase, make([]float64, n))
		writeF64(fzBase, make([]float64, n))

		// Stack with arguments
		stackBuf := make([]byte, 8192)
		sp := stackBase + uint64(4096)
		binary.LittleEndian.PutUint64(stackBuf[4096:], fxBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+8:], fyBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+16:], fzBase)
		binary.LittleEndian.PutUint32(stackBuf[4096+24:], uint32(n))
		em.LoadProgram(stackBase, stackBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, pxBase)
		em.Regs.WriteX(1, pyBase)
		em.Regs.WriteX(2, pzBase)
		em.Regs.WriteX(3, vxBase)
		em.Regs.WriteX(4, vyBase)
		em.Regs.WriteX(5, vzBase)
		em.Regs.WriteX(6, densityBase)
		em.Regs.WriteX(7, pressureBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = sp
		em.Regs.WriteD(0, hVal)
		em.Regs.WriteD(1, massVal)
		em.Regs.WriteD(2, rho0Val)
		em.Regs.WriteD(3, kVal)
		em.Regs.WriteD(4, muVal)
		em.SetHaltPC(haltPC)

		return em
	}

	// --- Raytrace benchmark ---
	setupRaytraceEmu := func() *emu.Emulator {
		numNodes := 7
		numTris := 4

		// 4 triangles forming a ground plane at z=0 from (-2,-2) to (2,2)
		triVerts := []float64{
			-2, -2, 0, 0, -2, 0, 0, 0, 0,
			-2, -2, 0, 0, 0, 0, -2, 0, 0,
			0, 0, 0, 2, 0, 0, 2, 2, 0,
			0, 0, 0, 2, 2, 0, 0, 2, 0,
		}

		bbox := []float64{
			-2, -2, -0.1, 2, 2, 0.1,
			-2, -2, -0.1, 0, 0, 0.1,
			0, 0, -0.1, 2, 2, 0.1,
			-2, -2, -0.1, 0, 0, 0.1,
			-2, -2, -0.1, 0, 0, 0.1,
			0, 0, -0.1, 2, 2, 0.1,
			0, 0, -0.1, 2, 2, 0.1,
		}

		children := []int32{
			1, 2, 3, 4, 5, 6,
			-1, -1, -1, -1, -1, -1, -1, -1,
		}

		triIdx := []int32{-1, -1, -1, 0, 1, 2, 3}

		// Ray shooting down at (-1,-1)
		ray := []float64{-1, -1, 5, 0, 0, -1}

		code := []uint32{
			0xd10503ff, 0x6d1033ed, 0x6d112beb, 0x6d1223e9, 0xa9136ffc, 0x6d400400,
			0x6d410c02, 0x6d421404, 0x1e6e1006, 0x1e6318c7, 0x1e6418d0, 0xb90003ff,
			0x52800028, 0xd2918809, 0xf2af16a9, 0xf2d5e3a9, 0xf2e882a9, 0x9e670131,
			0x910003e9, 0x5280090a, 0xd297b76b, 0xf2bb3aeb, 0xf2cf9beb, 0xf2e7bb6b,
			0x2f00e412, 0x1e7e1013, 0x5280002d, 0x1e6518d4, 0x14000007, 0x1e712300,
			0x1e724704, 0x1e71cf11, 0xaa0c03ed, 0x7100019f, 0x54000e8d, 0x510005ac,
			0xb86c592e, 0x37ffff6e, 0x6b0501df, 0x54ffff2a, 0x8b0e05cf, 0x8b0f1030,
			0xfd400215, 0x2a0f050f, 0xfc6f5836, 0x6d416217, 0x6d426a19, 0x1e603ab5,
			0x1e7508f5, 0x1e603b18, 0x1e7808f8, 0x1e613ad6, 0x1e760a16, 0x1e613b39,
			0x1e790a19, 0x1e623af7, 0x1e770a97, 0x1e623b5a, 0x1e7a0a9a, 0x1e787abb,
			0x1e786ab5, 0x1e797ad8, 0x1e796ad6, 0x1e7a7af9, 0x1e7a6af7, 0x1e786b78,
			0x1e796b18, 0x1e767ab5, 0x1e777ab5, 0x1e712300, 0x1e7846a8, 0x54fffb2b,
			0x1e6022a8, 0x54fffaed, 0x531f79cf, 0xb86f584f, 0x52800030, 0x331f75d0,
			0xb8705850, 0x310005ff, 0x3a410a00, 0x54000100, 0x37f80070, 0xb82c5930,
			0xaa0d03ec, 0x37fff96f, 0xb82cd92f, 0x1100058c, 0x17ffffc8, 0xb86e788d,
			0x37fff8cd, 0x9baa0dad, 0x6d4079bd, 0x6d4155bf, 0x6d425db6, 0x6d4371b8,
			0xfd4021a8, 0x1e7d3ab9, 0x1e7e3ada, 0x1e7f3afb, 0x1e7d3b15, 0x1e7e3b97,
			0x1e7f3916, 0x1e658af8, 0x1f566088, 0x1e638ad8, 0x1f5560a9, 0x1e648ab8,
			0x1f57606a, 0x1e690b58, 0x1f486338, 0x1f4a637c, 0x1e60c38b, 0x1e7e1018,
			0x9e67016c, 0x1e6c2160, 0x54fff524, 0x1e7c18dc, 0x1e7d380b, 0x1e7e382c,
			0x1e7f385f, 0x1e69099d, 0x1f48757d, 0x1f4a77fd, 0x1e7c0bbe, 0x1e6023c8,
			0x1e6657c0, 0x54fff3cc, 0x1e7a8bfd, 0x1f5b759d, 0x1e7b897b, 0x1f596ffb,
			0x1e798999, 0x1f5a6579, 0x1e7b089a, 0x1f5d687a, 0x1f5968ba, 0x1e7c0b5a,
			0x1e7a2bde, 0x1e602348, 0x1e6657c0, 0x54fff20c, 0x1e7b0af7, 0x1f5d5eb5,
			0x1f5956d5, 0x1e7c0ab5, 0x9e670176, 0x1e7622a0, 0x1e73ceb8, 0x17ffff88,
			0xd2918808, 0xf2af16a8, 0xf2d5e3a8, 0xf2e882a8, 0x9e670100, 0x1e602220,
			0x1e7e1000, 0x1e71ac00, 0xa9536ffc, 0x6d5223e9, 0x6d512beb, 0x6d5033ed,
			0x910503ff, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		rayBase := uint64(0x100000)
		bboxBase := uint64(0x200000)
		childrenBase := uint64(0x300000)
		triVertsBase := uint64(0x400000)
		triIdxBase := uint64(0x500000)
		stackBase := uint64(0x600000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 10_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		rayBuf := make([]byte, 6*8)
		for i, v := range ray {
			binary.LittleEndian.PutUint64(rayBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(rayBase, rayBuf)

		bboxBuf := make([]byte, len(bbox)*8)
		for i, v := range bbox {
			binary.LittleEndian.PutUint64(bboxBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bboxBase, bboxBuf)

		childBuf := make([]byte, len(children)*4)
		for i, v := range children {
			binary.LittleEndian.PutUint32(childBuf[i*4:], uint32(v))
		}
		em.LoadProgram(childrenBase, childBuf)

		tvBuf := make([]byte, len(triVerts)*8)
		for i, v := range triVerts {
			binary.LittleEndian.PutUint64(tvBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(triVertsBase, tvBuf)

		tiBuf := make([]byte, len(triIdx)*4)
		for i, v := range triIdx {
			binary.LittleEndian.PutUint32(tiBuf[i*4:], uint32(v))
		}
		em.LoadProgram(triIdxBase, tiBuf)

		em.LoadProgram(stackBase, make([]byte, 16384))

		_ = numTris

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, rayBase)
		em.Regs.WriteX(1, bboxBase)
		em.Regs.WriteX(2, childrenBase)
		em.Regs.WriteX(3, triVertsBase)
		em.Regs.WriteX(4, triIdxBase)
		em.Regs.WriteX(5, uint64(numNodes))
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 16384
		em.SetHaltPC(haltPC)
		return em
	}

	// --- Vips 2D convolution benchmark ---
	setupVipsEmu := func() *emu.Emulator {
		W, H, K := 64, 64, 3

		// Initialize input image
		input := make([]float64, W*H)
		for i := range input {
			input[i] = float64(i%256) / 255.0
		}

		// Initialize convolution kernel (Gaussian-like 3x3)
		kern := []float64{
			1.0 / 16, 2.0 / 16, 1.0 / 16,
			2.0 / 16, 4.0 / 16, 2.0 / 16,
			1.0 / 16, 2.0 / 16, 1.0 / 16,
		}

		// Machine code: convolve2d function (66 instructions)
		// Compiled from parsec/kernels/vips.c with:
		//   clang -target arm64-apple-macos -O1 -fno-vectorize
		//         -fno-slp-vectorize -fno-unroll-loops -fno-stack-protector
		//
		// CNEG conditions patched to match emulator semantics:
		//   cneg mi → cneg pl, cneg le → cneg gt
		code := []uint32{
			0x0b457ca8, 0x13017d08, 0x4b08008d, 0x6b0d011f,
			0x540007aa, 0xa9bd5ff8, 0xa90157f6, 0xa9024ff4,
			0x52800009, 0x4b08006e, 0x7100011f, 0x5a88450a,
			0x93407d0b, 0x93407c6c, 0x93407dad, 0x93407dce,
			0x4b0803ef, 0x6b0f011f, 0x5a88d510, 0x0b100110,
			0x11000610, 0xaa0b03f1, 0x14000005, 0x91000631,
			0x0b030129, 0xeb0d023f, 0x54000480, 0x6b0e011f,
			0x54ffff6a, 0x9b0c7e24, 0x8b040c04, 0xaa0903e6,
			0xaa0b03e7, 0x14000006, 0xfc277880, 0x910004e7,
			0x110004c6, 0xeb0e00ff, 0x54fffe20, 0x2f00e400,
			0x310004bf, 0x54ffff2b, 0x52800013, 0xaa0603f4,
			0xaa0f03f5, 0xaa1003f6, 0xaa1403f7, 0xaa1303f8,
			0xfc77d821, 0xfc78d842, 0x1f420020, 0x11000718,
			0x110006f7, 0x710006d6, 0x54ffff41, 0x110006b6,
			0x0b050273, 0x0b030294, 0x6b0a02bf, 0xaa1603f5,
			0x54fffe21, 0x17ffffe5, 0xa9424ff4, 0xa94157f6,
			0xa8c35ff8, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		outputBase := uint64(0x100000)
		inputBase := uint64(0x200000)
		kernelBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		// Load input data
		inputBuf := make([]byte, W*H*8)
		for i, v := range input {
			binary.LittleEndian.PutUint64(inputBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(inputBase, inputBuf)

		// Load kernel data
		kernBuf := make([]byte, K*K*8)
		for i, v := range kern {
			binary.LittleEndian.PutUint64(kernBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(kernelBase, kernBuf)

		// Output buffer (zeros)
		em.LoadProgram(outputBase, make([]byte, W*H*8))

		// Stack
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, outputBase)  // output
		em.Regs.WriteX(1, inputBase)   // input
		em.Regs.WriteX(2, kernelBase)  // kernel
		em.Regs.WriteX(3, uint64(W))   // width
		em.Regs.WriteX(4, uint64(H))   // height
		em.Regs.WriteX(5, uint64(K))   // ksize
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// --- Bodytrack benchmark ---
	setupBodytrackEmu := func() *emu.Emulator {
		nParticles := 64
		stateDim := 6
		obsDim := 8

		// Initialize states
		states := make([]float64, nParticles*stateDim)
		for p := 0; p < nParticles; p++ {
			for j := 0; j < stateDim; j++ {
				states[p*stateDim+j] = 0.1*float64(p+1) + 0.05*float64(j)
			}
		}

		// Initialize observations
		observations := make([]float64, obsDim)
		for i := 0; i < obsDim; i++ {
			observations[i] = 1.0 + 0.5*float64(i)
		}

		// Initialize projection matrix
		projMatrix := make([]float64, obsDim*stateDim)
		for i := 0; i < obsDim; i++ {
			for j := 0; j < stateDim; j++ {
				projMatrix[i*stateDim+j] = float64((i+j)%5+1) * 0.1
			}
		}

		// Machine code: 71 instructions from compiled bodytrack_likelihood.
		// BL _exp at offset 0x06c patched to target hook at 0xF100.
		// BL at PC = 0x1006c, target = 0xF100
		code := []uint32{
			0xd10203ff, 0x6d0123e9, 0xa9026ffc, 0xa90367fa,
			0xa9045ff8, 0xa90557f6, 0xa9064ff4, 0xa9077bfd,
			0x9101c3fd, 0xb9000fe6, 0xf90003e3, 0x7100049f,
			0x5400064b, 0xaa0503f4, 0xaa0203f6, 0xaa0103f7,
			0xaa0003f8, 0xd2800019, 0x2a0503fa, 0x2a0403fb,
			0xd37d7e9c, 0x937d7e95, 0x1e7c1008, 0xb9400fe8,
			0x2a0803f3, 0x14000008, 0x1e680800, 0x97fffc25,
			0xfc397b00, 0x91000739, 0x8b1502f7, 0xeb1b033f,
			0x540003c0, 0xb9400fe8, 0x7100051f, 0x5400030b,
			0xd2800008, 0x2f00e400, 0xf94003e9, 0x14000008,
			0xfc687ac2, 0x1e623821, 0x1f410020, 0x91000508,
			0x8b1c0129, 0xeb13011f, 0x54fffd80, 0x2f00e401,
			0x7100069f, 0x54fffeeb, 0xaa1a03ea, 0xaa1703eb,
			0xaa0903ec, 0xfc408582, 0xfc408563, 0x1f430441,
			0xf100054a, 0x54ffff81, 0x17ffffee, 0xd2f00008,
			0x9e670100, 0x17ffffde, 0xa9477bfd, 0xa9464ff4,
			0xa94557f6, 0xa9445ff8, 0xa94367fa, 0xa9426ffc,
			0x6d4123e9, 0x910203ff, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		expHookAddr := uint64(0xF100)

		likelihoodBase := uint64(0x100000)
		statesBase := uint64(0x200000)
		obsBase := uint64(0x300000)
		projBase := uint64(0x400000)
		stackBase := uint64(0x500000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 10_000_000

		em.LoadProgram(codeBase, EncodeInsts(code))

		// Load states
		statesBuf := make([]byte, nParticles*stateDim*8)
		for i, v := range states {
			binary.LittleEndian.PutUint64(statesBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(statesBase, statesBuf)

		// Load observations
		obsBuf := make([]byte, obsDim*8)
		for i, v := range observations {
			binary.LittleEndian.PutUint64(obsBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(obsBase, obsBuf)

		// Load projection matrix
		projBuf := make([]byte, obsDim*stateDim*8)
		for i, v := range projMatrix {
			binary.LittleEndian.PutUint64(projBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(projBase, projBuf)

		// Output buffer (zeros)
		em.LoadProgram(likelihoodBase, make([]byte, nParticles*8))

		// Stack
		em.LoadProgram(stackBase, make([]byte, 8192))

		// Set registers
		em.Regs.PC = codeBase
		em.Regs.WriteX(0, likelihoodBase)
		em.Regs.WriteX(1, statesBase)
		em.Regs.WriteX(2, obsBase)
		em.Regs.WriteX(3, projBase)
		em.Regs.WriteX(4, uint64(nParticles))
		em.Regs.WriteX(5, uint64(stateDim))
		em.Regs.WriteX(6, uint64(obsDim))
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)

		// Register exp function hook
		em.RegisterFuncHook(expHookAddr, func(emInner *emu.Emulator) {
			x := emInner.Regs.ReadD(0)
			emInner.Regs.WriteD(0, math.Exp(x))
		})

		return em
	}

	// --- Facesim benchmark ---
	setupFacesimEmu := func() *emu.Emulator {
		nVerts := 20
		nTets := 32

		// Grid-like vertex positions with small perturbation
		positions := make([]float64, nVerts*3)
		refPos := make([]float64, nVerts*3)
		for i := 0; i < nVerts; i++ {
			bx := float64(i%3) * 0.5
			by := float64((i/3)%3) * 0.5
			bz := float64(i/9) * 0.5
			refPos[i*3+0] = bx
			refPos[i*3+1] = by
			refPos[i*3+2] = bz
			// Slightly perturbed (deformed) positions
			positions[i*3+0] = bx + float64((i*7)%5)*0.01
			positions[i*3+1] = by + float64((i*11)%5)*0.01
			positions[i*3+2] = bz + float64((i*13)%5)*0.01
		}

		// Generate non-degenerate tet indices by cycling through a predefined pattern
		tetIndices := make([]int32, nTets*4)
		// Use groups of 4 vertices that form proper tetrahedra
		tetBase := [][4]int{
			{0, 1, 3, 9}, {1, 2, 4, 10}, {3, 4, 6, 12}, {4, 5, 7, 13},
			{0, 1, 4, 3}, {1, 2, 5, 4}, {3, 4, 7, 6}, {4, 5, 8, 7},
			{9, 10, 12, 0}, {10, 11, 13, 1}, {12, 13, 15, 3}, {13, 14, 16, 4},
			{9, 10, 13, 12}, {10, 11, 14, 13}, {12, 13, 16, 15}, {13, 14, 17, 16},
			{0, 3, 9, 1}, {1, 4, 10, 2}, {3, 6, 12, 4}, {4, 7, 13, 5},
			{0, 9, 10, 3}, {1, 10, 11, 4}, {3, 12, 13, 6}, {4, 13, 14, 7},
			{9, 0, 1, 10}, {10, 1, 2, 11}, {12, 3, 4, 13}, {13, 4, 5, 14},
			{0, 1, 9, 10}, {1, 2, 10, 11}, {3, 4, 12, 13}, {4, 5, 13, 14},
		}
		for t := 0; t < nTets; t++ {
			for j := 0; j < 4; j++ {
				idx := tetBase[t][j]
				if idx >= nVerts {
					idx = idx % nVerts
				}
				tetIndices[t*4+j] = int32(idx)
			}
		}

		// Precompute DmInv and restVolume
		dmInv := make([]float64, nTets*9)
		restVolume := make([]float64, nTets)
		for t := 0; t < nTets; t++ {
			i0 := int(tetIndices[t*4+0])
			i1 := int(tetIndices[t*4+1])
			i2 := int(tetIndices[t*4+2])
			i3 := int(tetIndices[t*4+3])

			dm00 := refPos[i1*3] - refPos[i0*3]
			dm01 := refPos[i2*3] - refPos[i0*3]
			dm02 := refPos[i3*3] - refPos[i0*3]
			dm10 := refPos[i1*3+1] - refPos[i0*3+1]
			dm11 := refPos[i2*3+1] - refPos[i0*3+1]
			dm12 := refPos[i3*3+1] - refPos[i0*3+1]
			dm20 := refPos[i1*3+2] - refPos[i0*3+2]
			dm21 := refPos[i2*3+2] - refPos[i0*3+2]
			dm22 := refPos[i3*3+2] - refPos[i0*3+2]

			det := dm00*(dm11*dm22-dm12*dm21) -
				dm01*(dm10*dm22-dm12*dm20) +
				dm02*(dm10*dm21-dm11*dm20)

			if math.Abs(det) < 1e-12 {
				det = 1.0
			}
			restVolume[t] = math.Abs(det) / 6.0

			invDet := 1.0 / det
			dmInv[t*9+0] = (dm11*dm22 - dm12*dm21) * invDet
			dmInv[t*9+1] = (dm02*dm21 - dm01*dm22) * invDet
			dmInv[t*9+2] = (dm01*dm12 - dm02*dm11) * invDet
			dmInv[t*9+3] = (dm12*dm20 - dm10*dm22) * invDet
			dmInv[t*9+4] = (dm00*dm22 - dm02*dm20) * invDet
			dmInv[t*9+5] = (dm02*dm10 - dm00*dm12) * invDet
			dmInv[t*9+6] = (dm10*dm21 - dm11*dm20) * invDet
			dmInv[t*9+7] = (dm01*dm20 - dm00*dm21) * invDet
			dmInv[t*9+8] = (dm00*dm11 - dm01*dm10) * invDet
		}

		muVal := 1000.0
		lambdaVal := 5000.0

		// Machine code: 285 instructions from parsec/kernels/facesim.c
		// BL _log at index 133 patched to 0xF000
		code := []uint32{
			0xd105c3ff, 0x6d0d3bef, 0x6d0e33ed, 0x6d0f2beb, 0x6d1023e9, 0xa9116ffc, 0xa91267fa, 0xa9135ff8,
			0xa91457f6, 0xa9154ff4, 0xa9167bfd, 0x910583fd, 0x1e604035, 0x1e604016, 0xaa0403fc, 0xaa0103f4,
			0xaa0003f5, 0xb81643bf, 0x0b0604c8, 0xb85643a9, 0x6b08013f, 0x540000ea, 0xb89643a9, 0xf8297abf,
			0xb85643a9, 0x11000529, 0xb81643a9, 0x17fffff8, 0x710004bf, 0x54001e8b, 0x1e6142c0, 0x6d005be0,
			0x2a0503f6, 0x91002057, 0x91008078, 0xfd000bf5, 0x697f26e8, 0x8b08051b, 0xfc7b7a80, 0x93407f68,
			0xd37df108, 0x9100210a, 0xf9005bea, 0xfc6a6a81, 0x91004108, 0xf81583a8, 0xfc686a82, 0x8b090539,
			0xfc797a83, 0x93407f28, 0xd37df108, 0x91002109, 0xfc696a84, 0x91004108, 0xa90823e9, 0xfc686a85,
			0x694026e8, 0x8b080513, 0xfc737a86, 0x93407e68, 0xd37df108, 0x9100210a, 0xfc6a6a87, 0x91004108,
			0xa90923ea, 0xfc686a90, 0x8b09053a, 0xfc7a7a91, 0x93407f48, 0xd37df108, 0x91002109, 0xfc696a92,
			0x91004108, 0xa90a23e9, 0xfc686a93, 0x1e603863, 0x1e6038c6, 0x1e603a20, 0x1e613884, 0x1e6138e7,
			0x1e613a41, 0x1e6238a5, 0x1e623a10, 0x1e623a62, 0x6d7f4b15, 0x1e7208d1, 0x6d7e6316, 0x1f564471,
			0x6d415313, 0x1f53440e, 0x6d405f19, 0x1e7908d1, 0x1f584471, 0x1f544408, 0x6d04dff6, 0x1e7708c6,
			0x6d03d7f8, 0x1f551863, 0xfd401306, 0x1f460c0f, 0x6d074ff2, 0x1e7208e0, 0x1f560080, 0x1f53002b,
			0x6d05d3f9, 0x1e7908e0, 0x1f580080, 0x1f540023, 0x1e7708e0, 0x1f550080, 0x1f46002c, 0xfd0037e6,
			0x1e720a00, 0x1f5600a0, 0x1f53004d, 0x1e790a00, 0x1f5800a0, 0x1f54004a, 0x1e770a00, 0x1f5500a0,
			0x1f460049, 0x1e6a8980, 0x1f490061, 0xfd0017e1, 0x1e6d8980, 0x1f490160, 0x1e608900, 0x1f4101c0,
			0x1e6d8861, 0x1f4a0561, 0xfd001be1, 0x1f4101e0, 0x6d0183e3, 0x97fffb7b, 0x6d40d7f6, 0x1e698961,
			0x1f4d0581, 0x1e698902, 0x1f4a09e2, 0x1e6d89e3, 0x1f490dc3, 0x1e6a89c4, 0x1f4d1104, 0xfd400ff1,
			0x1e7189e5, 0x1f4c1505, 0x1e6c89c6, 0x1f4b19e6, 0x1e6b8907, 0xfd4003f0, 0x1f4042a0, 0x6d4243f3,
			0x1e731800, 0x1f511dc7, 0x1e604232, 0x1e600a10, 0x1f4e42d0, 0x1e600821, 0x1f4806c1, 0x6d4353f1,
			0x1e600a31, 0x1f4f46d1, 0x1e600842, 0x1f4b0ac2, 0x1e600863, 0x1f520ec3, 0x1e600884, 0x1f4c12c4,
			0x1e6008a5, 0x1f4d16c5, 0x1e6008c6, 0x1f4a1ac6, 0x1e6008e0, 0x1f4902c7, 0xfc408780, 0x1e610a92,
			0x6d446ffe, 0x1f5b4a12, 0x6d4567fc, 0x1e610b33, 0x1f5e4a32, 0x6d475ff8, 0x1f584e13, 0x1f5c4e33,
			0xfd4033fa, 0x1e610b41, 0x1f570601, 0x1e630a90, 0x1e60429f, 0xfd4037f4, 0x1f540621, 0x1f5b4050,
			0x1e60437d, 0x1f5e4090, 0x1e630b31, 0x1e60433b, 0x1f584451, 0x1e604319, 0x1f5c4491, 0x1e728812,
			0x1e630b43, 0x1f570c42, 0x1e6042f8, 0x1f540882, 0x1e604297, 0x1e660be3, 0x1f5d0ca3, 0x1e738804,
			0x1f5e0ce3, 0x1e660b73, 0x1f594cb3, 0x1f5c4cf3, 0x1e660b46, 0x1e708810, 0xfc797ab4, 0x1e722a94,
			0xfc397ab4, 0x1e638803, 0x1f5818a5, 0xa94823eb, 0xfc6b6aa6, 0x1e7028c6, 0xfc2b6aa6, 0xfc686aa6,
			0x1e6328c6, 0xfc286aa6, 0x1e718806, 0xfc737ab1, 0x1e642a31, 0xfc337ab1, 0x1e738811, 0x1f5714e5,
			0xa94923eb, 0xfc6b6aa7, 0x1e6628e7, 0xfc2b6aa7, 0xfc686aa7, 0x1e7128e7, 0xfc286aa7, 0x1e618801,
			0xfc7a7aa7, 0x1e6128e7, 0xfc3a7aa7, 0x1e628802, 0x1e658800, 0xa94a23eb, 0xfc6b6aa5, 0x1e6228a5,
			0xfc2b6aa5, 0xfc686aa5, 0x1e652805, 0x1e642a44, 0xfc286aa5, 0x1e642821, 0xfc7b7aa4, 0x1e613881,
			0xfc3b7aa1, 0x1e662a01, 0x1e612841, 0xf9405be8, 0xfc686aa2, 0x1e613841, 0x1e712862, 0x1e622800,
			0xf85583a9, 0xfc696aa2, 0xfc286aa1, 0x1e603840, 0xfc296aa0, 0x910042f7, 0x91012318, 0xf10006d6,
			0x54ffe281, 0xa9567bfd, 0xa9554ff4, 0xa95457f6, 0xa9535ff8, 0xa95267fa, 0xa9516ffc, 0x6d5023e9,
			0x6d4f2beb, 0x6d4e33ed, 0x6d4d3bef, 0x9105c3ff, 0xd65f03c0,
		}

		logHookAddr := uint64(0xF000)
		codeBase := uint64(0x10000)
		forcesBase := uint64(0x100000)
		posBase := uint64(0x200000)
		tetIdxBase := uint64(0x300000)
		dmInvBase := uint64(0x400000)
		restVolBase := uint64(0x500000)
		stackBase := uint64(0x600000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 10_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		// Load positions
		posBuf := make([]byte, nVerts*3*8)
		for i, v := range positions {
			binary.LittleEndian.PutUint64(posBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(posBase, posBuf)

		// Load tet indices (int32)
		tetBuf := make([]byte, nTets*4*4)
		for i, v := range tetIndices {
			binary.LittleEndian.PutUint32(tetBuf[i*4:], uint32(v))
		}
		em.LoadProgram(tetIdxBase, tetBuf)

		// Load DmInv
		dmBuf := make([]byte, nTets*9*8)
		for i, v := range dmInv {
			binary.LittleEndian.PutUint64(dmBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(dmInvBase, dmBuf)

		// Load restVolume
		rvBuf := make([]byte, nTets*8)
		for i, v := range restVolume {
			binary.LittleEndian.PutUint64(rvBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(restVolBase, rvBuf)

		// Output forces (zeroed)
		em.LoadProgram(forcesBase, make([]byte, nVerts*3*8))

		// Stack
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, forcesBase)
		em.Regs.WriteX(1, posBase)
		em.Regs.WriteX(2, tetIdxBase)
		em.Regs.WriteX(3, dmInvBase)
		em.Regs.WriteX(4, restVolBase)
		em.Regs.WriteD(0, muVal)
		em.Regs.WriteD(1, lambdaVal)
		em.Regs.WriteX(5, uint64(nTets))
		em.Regs.WriteX(6, uint64(nVerts))
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)

		// Register log function hook
		em.RegisterFuncHook(logHookAddr, func(emInner *emu.Emulator) {
			x := emInner.Regs.ReadD(0)
			emInner.Regs.WriteD(0, math.Log(x))
		})

		return em
	}

	// --- Ferret benchmark (k-NN distance computation) ---
	setupFerretEmu := func() *emu.Emulator {
		nQueries, nDB, dim, k := 16, 128, 16, 4

		queries := make([]float64, nQueries*dim)
		for i := range queries {
			queries[i] = float64(i)*0.1 + 0.5
		}
		database := make([]float64, nDB*dim)
		for i := range database {
			database[i] = float64(i%17)*0.2 - 1.0
		}

		code := []uint32{
			0x7100047f, 0x5400098b, 0xa9bd5ff8, 0xa90157f6,
			0xa9024ff4, 0xd2800008, 0x93407cc9, 0x510008ca,
			0x2a0303eb, 0x2a0603ec, 0x2a0403ed, 0x937d7cce,
			0x937d7caf, 0xd2919d50, 0xf2a73410, 0xf2c7cb30,
			0xf2e8c530, 0xaa0003f1, 0x2a0503e3, 0x14000006,
			0x91000508, 0x8b0e0231, 0x8b0f0021, 0xeb0b011f,
			0x54000640, 0xaa0c03e7, 0xaa1103f3, 0x710004df,
			0x5400008b, 0xf8008670, 0xf10004e7, 0x54ffffc1,
			0x7100049f, 0x54fffe6b, 0xd2800007, 0x9b097d13,
			0x8b130c13, 0xd1002273, 0xaa0203f4, 0x14000005,
			0x910004e7, 0x8b0f0294, 0xeb0d00ff, 0x54fffd20,
			0x2f00e400, 0x710004bf, 0x5400014b, 0xaa0303f5,
			0xaa0103f6, 0xaa1403f7, 0xfc4086c1, 0xfc4086e2,
			0x1e623821, 0x1f410020, 0xf10006b5, 0x54ffff61,
			0xfc697a61, 0x1e612000, 0x54fffdc5, 0xfc297a60,
			0x710008df, 0x54fffd6b, 0xaa0a03f5, 0x8b150e36,
			0x6d4006c0, 0x1e612000, 0x54fffccd, 0xfc357a21,
			0xfd0006c0, 0xd10006b6, 0xf10002bf, 0xaa1603f5,
			0x54fffeec, 0x17ffffdf, 0xa9424ff4, 0xa94157f6,
			0xa8c35ff8, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		resultsBase := uint64(0x100000)
		queriesBase := uint64(0x200000)
		databaseBase := uint64(0x300000)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))

		qBuf := make([]byte, nQueries*dim*8)
		for i, v := range queries {
			binary.LittleEndian.PutUint64(qBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(queriesBase, qBuf)

		dbBuf := make([]byte, nDB*dim*8)
		for i, v := range database {
			binary.LittleEndian.PutUint64(dbBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(databaseBase, dbBuf)

		em.LoadProgram(resultsBase, make([]byte, nQueries*k*8))
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, resultsBase)
		em.Regs.WriteX(1, queriesBase)
		em.Regs.WriteX(2, databaseBase)
		em.Regs.WriteX(3, uint64(nQueries))
		em.Regs.WriteX(4, uint64(nDB))
		em.Regs.WriteX(5, uint64(dim))
		em.Regs.WriteX(6, uint64(k))
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// --- x264 benchmark ---
	setupX264Emu := func() *emu.Emulator {
		// 128x128 frame with 16x16 macroblock, search range 8
		frameW := 128
		searchRange := 8

		// Position the current macroblock at center with enough border
		// for the search range (need 16 + range on each side)
		curOffY := 48
		curOffX := 48
		curOff := curOffY*frameW + curOffX

		frameSize := frameW * frameW
		curFrame := make([]byte, frameSize)
		refFrame := make([]byte, frameSize)

		// Fill with pseudo-random data
		for i := 0; i < frameSize; i++ {
			curFrame[i] = byte((i*73 + 17) % 256)
		}

		// Copy cur to ref with a shift of (3, -2) and small noise
		copy(refFrame, curFrame)
		shiftMx, shiftMy := 3, -2
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				srcIdx := (curOffY+y)*frameW + (curOffX + x)
				dstIdx := (curOffY+shiftMy+y)*frameW + (curOffX + shiftMx + x)
				noise := byte((y*3 + x*7) % 5)
				refFrame[dstIdx] = curFrame[srcIdx] + noise
			}
		}

		// Machine code (156 instructions) compiled from parsec/kernels/x264.c
		// with: clang -target arm64-apple-macos -O1 -fno-vectorize
		//       -fno-slp-vectorize -fno-unroll-loops -fno-stack-protector
		// CNEG conditions patched for emulator: mi -> pl
		code := []uint32{
			0xd10283ff, 0xa90667fa, 0xa9075ff8, 0xa90857f6, 0xa9094ff4, 0x37f805a3, 0x52800009, 0x52800008,
			0x4b0303eb, 0xaa0203ea, 0x93407d4a, 0x93407d6b, 0x1100046c, 0xaa2a03ed, 0x9b2305ad, 0x12b0000e,
			0xaa0b03ef, 0xaa0d03f0, 0xaa0b03f1, 0xd2800007, 0x52800003, 0xaa0003f3, 0xaa1003f4, 0xd2800015,
			0x38756a76, 0x38756a97, 0x6b1702d6, 0x5a9646d6, 0x0b0302c3, 0x910006b5, 0xf10042bf, 0x54ffff21,
			0x910004e7, 0x8b0a0294, 0x8b0a0273, 0xf10040ff, 0x54fffe61, 0x6b0e007f, 0x1a88b1e8, 0x1a89b229,
			0x1a8eb06e, 0x91000631, 0x91000610, 0x6b11019f, 0x54fffce1, 0x910005ef, 0x8b0a01ad, 0x6b0f019f,
			0x54fffc21, 0x14000003, 0x52800008, 0x52800009, 0xd280000b, 0x5280000a, 0x93407c4c, 0x9b22050d,
			0x8b29c1ad, 0x937e7c4e, 0x910083ef, 0x910103f0, 0x910003f1, 0xd2800001, 0xaa0003e2, 0xaa0d03e3,
			0xd2800007, 0x910083f3, 0xaa0203f4, 0xaa0303f5, 0xd2800016, 0x38766a97, 0x38766ab8, 0x4b1802f7,
			0x78367a77, 0x910006d6, 0xf10012df, 0x54ffff41, 0x910004e7, 0x91002273, 0x8b0c02b5, 0x8b0c0294,
			0xf10010ff, 0x54fffe61, 0xd2800007, 0x8b0701f3, 0x79400274, 0x79400675, 0x79400a76, 0x79400e73,
			0x0b1402b7, 0x4b150294, 0x0b160275, 0x0b1702b8, 0x8b070219, 0x79000338, 0x4b1302d3, 0x0b140276,
			0x79000736, 0x4b1502f5, 0x79000b35, 0x4b130293, 0x79000f33, 0x910020e7, 0xf10080ff, 0x54fffd81,
			0x928000e7, 0x8b070213, 0x79401274, 0x79402275, 0x79403276, 0x79404273, 0x0b1402b7, 0x4b150294,
			0x0b160275, 0x0b1702b8, 0x8b070239, 0x79001338, 0x4b1302d3, 0x0b140276, 0x79002336, 0x4b1502f5,
			0x79003335, 0x4b130293, 0x79004333, 0xb10008e7, 0x54fffda1, 0xd2800013, 0x78f36a34, 0x7100029f,
			0x5a944694, 0x0b1400e7, 0x91000a73, 0xf100827f, 0x54ffff41, 0x110004e7, 0x0b47054a, 0x91001027,
			0x91001063, 0x91001042, 0xf100303f, 0xaa0703e1, 0x54fff683, 0x91001161, 0x8b0e01ad, 0x8b0e0000,
			0xf100317f, 0xaa0103eb, 0x54fff563, 0xb900008a, 0xb90000a9, 0xb90000c8, 0xa9494ff4, 0xa94857f6,
			0xa9475ff8, 0xa94667fa, 0x910283ff, 0xd65f03c0,
		}

		codeBase := uint64(0x10000)
		curBase := uint64(0x100000)
		refBase := uint64(0x200000)
		outCostBase := uint64(0x300000)
		outMxBase := uint64(0x300010)
		outMyBase := uint64(0x300020)
		stackBase := uint64(0x400000)
		haltPC := uint64(0xFFFF0000)

		em := emu.NewEmulator()
		em.MaxInst = 100_000_000
		em.LoadProgram(codeBase, EncodeInsts(code))
		em.LoadProgram(curBase, curFrame)
		em.LoadProgram(refBase, refFrame)
		em.LoadProgram(outCostBase, make([]byte, 64))
		em.LoadProgram(outMxBase, make([]byte, 64))
		em.LoadProgram(outMyBase, make([]byte, 64))
		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, curBase+uint64(curOff))
		em.Regs.WriteX(1, refBase+uint64(curOff))
		em.Regs.WriteX(2, uint64(frameW))
		em.Regs.WriteX(3, uint64(searchRange))
		em.Regs.WriteX(4, outCostBase)
		em.Regs.WriteX(5, outMxBase)
		em.Regs.WriteX(6, outMyBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	// NativeTime values measured on Apple M2 (3.49 GHz).
	//
	// Methodology: P-core times measured with 501-run median using
	// clang -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops.
	// Adjusted by 2.2× E/P-core factor, consistent with PolyBench methodology
	// where Nora found E-core runs (at median) are ~2-2.5× P-core times
	// for FP-heavy workloads on Apple M2 (see workspace/workspace/nora/note.md).
	// P-core baselines: swaptions=8.89µs, blackscholes=20.95µs,
	// streamcluster=6.86µs, canneal=6.95µs, fluidanimate=1.11µs (3.0× E/P
	// factor for FSQRT-heavy code), raytrace=42ns, vips=18.0µs,
	// facesim=507ns (32 tets, 20 verts), bodytrack=1583ns (64 particles),
	// ferret=14917ns (16 queries, 128 DB, dim=16, k=4),
	// x264=26875ns (128×128 frame, range=8).
	// freqmine: SKIPPED — tree-traversal kernel has pointer-chasing access
	// patterns and recursive data structures that cannot be faithfully
	// represented in the flat-memory emulator model (LOW feasibility, see
	// workspace/workspace/iris/note.md for analysis).
	return []Benchmark{
		{Name: "swaptions", Suite: "parsec", Setup: setupSwaptionsEmu, NativeTime: 1.956e-05, Regions: []DataRegion{
			{Addr: 0x100000, Size: 50 * 50 * 8}, {Addr: 0x200000, Size: 50 * 8}, {Addr: 0x300000, Size: 50 * 8}, {Addr: 0x400000, Size: 8192},
		}, FuncHookCycles: map[uint64]int{0x40000: 25}},
		{Name: "blackscholes", Suite: "parsec", Setup: setupBlackScholesEmu, NativeTime: 4.609e-05, Regions: []DataRegion{
			{Addr: 0x1000, Size: 173 * 4}, {Addr: 0x100000, Size: 1024 * 8}, {Addr: 0x200000, Size: 1024 * 8},
			{Addr: 0x300000, Size: 1024 * 8}, {Addr: 0x400000, Size: 1024 * 8}, {Addr: 0x500000, Size: 1024 * 8},
			{Addr: 0x600000, Size: 1024 * 8}, {Addr: 0x700000, Size: 1024 * 4}, {Addr: 0x800000, Size: 8192},
		}, FuncHookCycles: map[uint64]int{0xF000: 20, 0xF100: 25}},
		{Name: "streamcluster", Suite: "parsec", Setup: setupStreamclusterEmu, NativeTime: 1.509e-05, Regions: []DataRegion{
			{Addr: 0x100000, Size: 512 * 32 * 8},
			{Addr: 0x200000, Size: 512 * 32 * 8},
			{Addr: 0x300000, Size: 512 * 8},
		}},
		{Name: "canneal", Suite: "parsec", Setup: setupCannealEmu, NativeTime: 1.528e-05, Regions: []DataRegion{
			{Addr: 0x100000, Size: 8192 * 2 * 4}, // netlist (int32)
			{Addr: 0x200000, Size: 512 * 2 * 4},  // locations (int32)
		}},
		{Name: "fluidanimate", Suite: "parsec", Setup: setupFluidanimateEmu, NativeTime: 3.339e-06, Regions: []DataRegion{
			{Addr: 0x1000, Size: 155 * 4},          // code
			{Addr: 0x100000, Size: 16 * 8 * 11},    // particle data (11 arrays × 16 particles × 8 bytes)
			{Addr: 0x800000, Size: 8192},            // stack
		}},
		{Name: "raytrace", Suite: "parsec", Setup: setupRaytraceEmu, NativeTime: 1.38e-07, Regions: []DataRegion{
			{Addr: 0x10000, Size: 164 * 4},         // code
			{Addr: 0x100000, Size: 6 * 8},           // ray data
			{Addr: 0x200000, Size: 7 * 6 * 8},       // bbox
			{Addr: 0x300000, Size: 7 * 2 * 4},       // children
			{Addr: 0x400000, Size: 4 * 9 * 8},       // tri_verts
			{Addr: 0x500000, Size: 7 * 4},            // tri_idx
			{Addr: 0x600000, Size: 16384},            // stack
		}},
		{Name: "vips", Suite: "parsec", Setup: setupVipsEmu, NativeTime: 3.960e-05, Regions: []DataRegion{
			{Addr: 0x100000, Size: 64 * 64 * 8}, // output
			{Addr: 0x200000, Size: 64 * 64 * 8}, // input
			{Addr: 0x300000, Size: 3 * 3 * 8},   // kernel
		}},
		{Name: "bodytrack", Suite: "parsec", Setup: setupBodytrackEmu, NativeTime: 3.483e-06, Regions: []DataRegion{
			{Addr: 0x100000, Size: 64 * 8},      // likelihood output
			{Addr: 0x200000, Size: 64 * 6 * 8},  // states
			{Addr: 0x300000, Size: 8 * 8},        // observations
			{Addr: 0x400000, Size: 8 * 6 * 8},    // projMatrix
			{Addr: 0x500000, Size: 8192},          // stack
		}, FuncHookCycles: map[uint64]int{0xF100: 25}},
		{Name: "facesim", Suite: "parsec", Setup: setupFacesimEmu, NativeTime: 1.115e-06, Regions: []DataRegion{
			{Addr: 0x100000, Size: 20 * 3 * 8},  // forces output
			{Addr: 0x200000, Size: 20 * 3 * 8},  // positions
			{Addr: 0x300000, Size: 32 * 4 * 4},  // tetIndices (int32)
			{Addr: 0x400000, Size: 32 * 9 * 8},  // DmInv
			{Addr: 0x500000, Size: 32 * 8},       // restVolume
			{Addr: 0x600000, Size: 8192},          // stack
		}, FuncHookCycles: map[uint64]int{0xF000: 20}},
		{Name: "ferret", Suite: "parsec", Setup: setupFerretEmu, NativeTime: 3.282e-05, Regions: []DataRegion{
			{Addr: 0x100000, Size: 16 * 4 * 8},    // results (nQueries*k*8)
			{Addr: 0x200000, Size: 16 * 16 * 8},   // queries (nQueries*dim*8)
			{Addr: 0x300000, Size: 128 * 16 * 8},  // database (nDB*dim*8)
			{Addr: 0x400000, Size: 8192},           // stack
		}},
		{Name: "x264", Suite: "parsec", Setup: setupX264Emu, NativeTime: 5.913e-05, Regions: []DataRegion{
			{Addr: 0x100000, Size: 128 * 128},    // current frame (uint8)
			{Addr: 0x200000, Size: 128 * 128},    // reference frame (uint8)
			{Addr: 0x300000, Size: 64},            // output area
			{Addr: 0x400000, Size: 8192},          // stack
		}},
	}
}

// ByName returns a pointer to the benchmark with the given name, or nil if
// no benchmark with that name exists.
func ByName(name string) *Benchmark {
	all := All()
	for i := range all {
		if all[i].Name == name {
			return &all[i]
		}
	}
	return nil
}

// BySuite returns all benchmarks belonging to the given suite.
func BySuite(suite string) []Benchmark {
	all := All()
	var result []Benchmark
	for _, b := range all {
		if b.Suite == suite {
			result = append(result, b)
		}
	}
	return result
}

// QuickEval returns a subset of 6 representative benchmarks suitable for
// quick evaluation: atax, bicg, gemm, floydwarshall, trisolv, seidel2d.
func QuickEval() []Benchmark {
	names := map[string]bool{
		"atax": true, "bicg": true, "gemm": true,
		"floydwarshall": true, "trisolv": true, "seidel2d": true,
	}
	all := All()
	var result []Benchmark
	for _, b := range all {
		if names[b.Name] {
			result = append(result, b)
		}
	}
	return result
}

