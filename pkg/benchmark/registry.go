package benchmark

import (
	"encoding/binary"
	"math"

	"github.com/sarchlab/m2sim3/pkg/emu"
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

	return []Benchmark{
		{Name: "swaptions", Suite: "parsec", Setup: setupSwaptionsEmu, NativeTime: 1e-4, Regions: []DataRegion{
			{Addr: 0x100000, Size: 50 * 50 * 8}, {Addr: 0x200000, Size: 50 * 8}, {Addr: 0x300000, Size: 50 * 8}, {Addr: 0x400000, Size: 8192},
		}, FuncHookCycles: map[uint64]int{0x40000: 25}},
		{Name: "blackscholes", Suite: "parsec", Setup: setupBlackScholesEmu, NativeTime: 1e-4, Regions: []DataRegion{
			{Addr: 0x1000, Size: 173 * 4}, {Addr: 0x100000, Size: 1024 * 8}, {Addr: 0x200000, Size: 1024 * 8},
			{Addr: 0x300000, Size: 1024 * 8}, {Addr: 0x400000, Size: 1024 * 8}, {Addr: 0x500000, Size: 1024 * 8},
			{Addr: 0x600000, Size: 1024 * 8}, {Addr: 0x700000, Size: 1024 * 4}, {Addr: 0x800000, Size: 8192},
		}, FuncHookCycles: map[uint64]int{0xF000: 20, 0xF100: 25}},
		{Name: "streamcluster", Suite: "parsec", Setup: setupStreamclusterEmu, NativeTime: 1e-4, Regions: []DataRegion{
			{Addr: 0x100000, Size: 512 * 32 * 8},
			{Addr: 0x200000, Size: 512 * 32 * 8},
			{Addr: 0x300000, Size: 512 * 8},
		}},
		{Name: "canneal", Suite: "parsec", Setup: setupCannealEmu, NativeTime: 1e-4, Regions: []DataRegion{
			{Addr: 0x100000, Size: 8192 * 2 * 4}, // netlist (int32)
			{Addr: 0x200000, Size: 512 * 2 * 4},  // locations (int32)
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

