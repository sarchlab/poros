package emu

import (
	"math"

	"golang.org/x/arch/arm64/arm64asm"
)

// isFPDouble returns true if the FP instruction operates on D (64-bit) registers.
func isFPDouble(arg arm64asm.Arg) bool {
	if r, ok := arg.(arm64asm.Reg); ok {
		return dRegIndex(r) >= 0
	}
	return false
}

func (e *executor) execFADD(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteD(di, a+b)
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteS(si, a+b)
	}
	return nil
}

func (e *executor) execFSUB(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteD(di, a-b)
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteS(si, a-b)
	}
	return nil
}

func (e *executor) execFMUL(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteD(di, a*b)
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteS(si, a*b)
	}
	return nil
}

func (e *executor) execFDIV(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteD(di, a/b)
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteS(si, a/b)
	}
	return nil
}

func (e *executor) execFMADD(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		n := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		m := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		a := e.regs.ReadD(dRegIndex(inst.Args[3].(arm64asm.Reg)))
		e.regs.WriteD(di, a+n*m)
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		n := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		m := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		a := e.regs.ReadS(sRegIndex(inst.Args[3].(arm64asm.Reg)))
		e.regs.WriteS(si, a+n*m)
	}
	return nil
}

func (e *executor) execFMSUB(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		n := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		m := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		a := e.regs.ReadD(dRegIndex(inst.Args[3].(arm64asm.Reg)))
		e.regs.WriteD(di, a-n*m)
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		n := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		m := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		a := e.regs.ReadS(sRegIndex(inst.Args[3].(arm64asm.Reg)))
		e.regs.WriteS(si, a-n*m)
	}
	return nil
}

func (e *executor) execFSQRT(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		val := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		e.regs.WriteD(di, math.Sqrt(val))
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		val := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		e.regs.WriteS(si, float32(math.Sqrt(float64(val))))
	}
	return nil
}

func (e *executor) execFNEG(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		val := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		e.regs.WriteD(di, -val)
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		val := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		e.regs.WriteS(si, -val)
	}
	return nil
}

func (e *executor) execFABS(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		val := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		e.regs.WriteD(di, math.Abs(val))
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		val := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		e.regs.WriteS(si, float32(math.Abs(float64(val))))
	}
	return nil
}

func (e *executor) execFMOV(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	src := inst.Args[1]

	dstDI := -1
	dstSI := -1
	dstXI := -1
	dstWI := -1

	if r, ok := dst.(arm64asm.Reg); ok {
		dstDI = dRegIndex(r)
		dstSI = sRegIndex(r)
		dstXI = xRegIndex(r)
		dstWI = wRegIndex(r)
	}

	switch s := src.(type) {
	case arm64asm.Reg:
		srcDI := dRegIndex(s)
		srcSI := sRegIndex(s)
		srcXI := xRegIndex(s)
		srcWI := wRegIndex(s)

		if srcDI >= 0 && dstDI >= 0 {
			// FMOV Dd, Dn
			e.regs.WriteDRaw(dstDI, e.regs.ReadDRaw(srcDI))
		} else if srcSI >= 0 && dstSI >= 0 {
			// FMOV Sd, Sn
			e.regs.WriteSRaw(dstSI, e.regs.ReadSRaw(srcSI))
		} else if srcXI >= 0 && dstDI >= 0 {
			// FMOV Dd, Xn — move GPR bits to FP
			e.regs.WriteDRaw(dstDI, e.regs.ReadX(srcXI))
		} else if srcDI >= 0 && dstXI >= 0 {
			// FMOV Xd, Dn — move FP bits to GPR
			e.regs.WriteX(dstXI, e.regs.ReadDRaw(srcDI))
		} else if srcWI >= 0 && dstSI >= 0 {
			// FMOV Sd, Wn
			e.regs.WriteSRaw(dstSI, e.regs.ReadW(srcWI))
		} else if srcSI >= 0 && dstWI >= 0 {
			// FMOV Wd, Sn
			e.regs.WriteW(dstWI, e.regs.ReadSRaw(srcSI))
		}

	case arm64asm.Imm_fp:
		val := getImmFp(s)
		if dstDI >= 0 {
			e.regs.WriteD(dstDI, val)
		} else if dstSI >= 0 {
			e.regs.WriteS(dstSI, float32(val))
		}
	}
	return nil
}

func (e *executor) execFCMP(inst arm64asm.Inst) error {
	isDouble := isFPDouble(inst.Args[0])

	if isDouble {
		a := e.regs.ReadD(dRegIndex(inst.Args[0].(arm64asm.Reg)))
		var b float64
		if inst.Args[1] == nil {
			b = 0
		} else if _, ok := inst.Args[1].(arm64asm.Imm); ok {
			b = 0 // FCMP Dn, #0.0
		} else {
			b = e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		}
		e.setFPFlags64(a, b)
	} else {
		a := e.regs.ReadS(sRegIndex(inst.Args[0].(arm64asm.Reg)))
		var b float32
		if inst.Args[1] == nil {
			b = 0
		} else if _, ok := inst.Args[1].(arm64asm.Imm); ok {
			b = 0
		} else {
			b = e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		}
		e.setFPFlags32(a, b)
	}
	return nil
}

func (e *executor) setFPFlags64(a, b float64) {
	if math.IsNaN(a) || math.IsNaN(b) {
		e.regs.N = false
		e.regs.Z = false
		e.regs.C = true
		e.regs.V = true
	} else if a == b {
		e.regs.N = false
		e.regs.Z = true
		e.regs.C = true
		e.regs.V = false
	} else if a < b {
		e.regs.N = true
		e.regs.Z = false
		e.regs.C = false
		e.regs.V = false
	} else {
		e.regs.N = false
		e.regs.Z = false
		e.regs.C = true
		e.regs.V = false
	}
}

func (e *executor) setFPFlags32(a, b float32) {
	e.setFPFlags64(float64(a), float64(b))
}

func (e *executor) execFCSEL(inst arm64asm.Inst) error {
	cond := inst.Args[3].(arm64asm.Cond)
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		if e.regs.EvalCond(cond) {
			e.regs.WriteDRaw(di, e.regs.ReadDRaw(dRegIndex(inst.Args[1].(arm64asm.Reg))))
		} else {
			e.regs.WriteDRaw(di, e.regs.ReadDRaw(dRegIndex(inst.Args[2].(arm64asm.Reg))))
		}
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		if e.regs.EvalCond(cond) {
			e.regs.WriteSRaw(si, e.regs.ReadSRaw(sRegIndex(inst.Args[1].(arm64asm.Reg))))
		} else {
			e.regs.WriteSRaw(si, e.regs.ReadSRaw(sRegIndex(inst.Args[2].(arm64asm.Reg))))
		}
	}
	return nil
}

func (e *executor) execSCVTF(inst arm64asm.Inst) error {
	dst := inst.Args[0].(arm64asm.Reg)
	src := inst.Args[1].(arm64asm.Reg)

	dstDI := dRegIndex(dst)
	dstSI := sRegIndex(dst)
	srcXI := xRegIndex(src)
	srcWI := wRegIndex(src)

	if dstDI >= 0 {
		if srcXI >= 0 {
			e.regs.WriteD(dstDI, float64(int64(e.regs.ReadX(srcXI))))
		} else if srcWI >= 0 {
			e.regs.WriteD(dstDI, float64(int32(e.regs.ReadW(srcWI))))
		}
	} else if dstSI >= 0 {
		if srcXI >= 0 {
			e.regs.WriteS(dstSI, float32(int64(e.regs.ReadX(srcXI))))
		} else if srcWI >= 0 {
			e.regs.WriteS(dstSI, float32(int32(e.regs.ReadW(srcWI))))
		}
	}
	return nil
}

func (e *executor) execUCVTF(inst arm64asm.Inst) error {
	dst := inst.Args[0].(arm64asm.Reg)
	src := inst.Args[1].(arm64asm.Reg)

	dstDI := dRegIndex(dst)
	dstSI := sRegIndex(dst)
	srcXI := xRegIndex(src)
	srcWI := wRegIndex(src)

	if dstDI >= 0 {
		if srcXI >= 0 {
			e.regs.WriteD(dstDI, float64(e.regs.ReadX(srcXI)))
		} else if srcWI >= 0 {
			e.regs.WriteD(dstDI, float64(e.regs.ReadW(srcWI)))
		}
	} else if dstSI >= 0 {
		if srcXI >= 0 {
			e.regs.WriteS(dstSI, float32(e.regs.ReadX(srcXI)))
		} else if srcWI >= 0 {
			e.regs.WriteS(dstSI, float32(e.regs.ReadW(srcWI)))
		}
	}
	return nil
}

func (e *executor) execFCVTZS(inst arm64asm.Inst) error {
	dst := inst.Args[0].(arm64asm.Reg)
	src := inst.Args[1].(arm64asm.Reg)

	dstXI := xRegIndex(dst)
	dstWI := wRegIndex(dst)
	srcDI := dRegIndex(src)
	srcSI := sRegIndex(src)

	if dstXI >= 0 {
		if srcDI >= 0 {
			val := e.regs.ReadD(srcDI)
			e.regs.WriteX(dstXI, uint64(int64(val)))
		} else if srcSI >= 0 {
			val := e.regs.ReadS(srcSI)
			e.regs.WriteX(dstXI, uint64(int64(val)))
		}
	} else if dstWI >= 0 {
		if srcDI >= 0 {
			val := e.regs.ReadD(srcDI)
			e.regs.WriteW(dstWI, uint32(int32(val)))
		} else if srcSI >= 0 {
			val := e.regs.ReadS(srcSI)
			e.regs.WriteW(dstWI, uint32(int32(val)))
		}
	}
	return nil
}

func (e *executor) execFCVTZU(inst arm64asm.Inst) error {
	dst := inst.Args[0].(arm64asm.Reg)
	src := inst.Args[1].(arm64asm.Reg)

	dstXI := xRegIndex(dst)
	dstWI := wRegIndex(dst)
	srcDI := dRegIndex(src)
	srcSI := sRegIndex(src)

	if dstXI >= 0 {
		if srcDI >= 0 {
			val := e.regs.ReadD(srcDI)
			e.regs.WriteX(dstXI, uint64(val))
		} else if srcSI >= 0 {
			val := e.regs.ReadS(srcSI)
			e.regs.WriteX(dstXI, uint64(val))
		}
	} else if dstWI >= 0 {
		if srcDI >= 0 {
			val := e.regs.ReadD(srcDI)
			e.regs.WriteW(dstWI, uint32(val))
		} else if srcSI >= 0 {
			val := e.regs.ReadS(srcSI)
			e.regs.WriteW(dstWI, uint32(val))
		}
	}
	return nil
}
