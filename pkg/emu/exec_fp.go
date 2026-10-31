package emu

import (
	"fmt"
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
		// Go may otherwise fuse this expression only on some host architectures.
		e.regs.WriteD(di, math.FMA(n, m, a))
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
		e.regs.WriteD(di, math.FMA(-n, m, a))
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
	if dst, ok := inst.Args[0].(arm64asm.RegisterWithArrangement); ok {
		if !checkedVectorBits(inst) {
			return &UnsupportedInstructionError{Instruction: inst}
		}
		dr, _ := getRegWithArrangement(dst)
		sr, _ := getRegWithArrangement(inst.Args[1].(arm64asm.RegisterWithArrangement))
		lo, hi := e.regs.ReadQ(vRegIndex(sr))
		e.regs.WriteQ(vRegIndex(dr), lo^(uint64(1)<<63), hi^(uint64(1)<<63))
		return nil
	}
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

func (e *executor) execFNMUL(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteD(di, -(a * b))
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		e.regs.WriteS(si, -(a * b))
	}
	return nil
}

func (e *executor) execFNMADD(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		n := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		m := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		a := e.regs.ReadD(dRegIndex(inst.Args[3].(arm64asm.Reg)))
		// Negate operands before the fused operation, including signed zeros.
		e.regs.WriteD(di, math.FMA(-n, m, -a))
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		n := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		m := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		a := e.regs.ReadS(sRegIndex(inst.Args[3].(arm64asm.Reg)))
		e.regs.WriteS(si, -(n*m + a))
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
	var v int64
	switch {
	case xRegIndex(src) >= 0:
		v = int64(e.regs.ReadX(xRegIndex(src)))
	case wRegIndex(src) >= 0:
		v = int64(int32(e.regs.ReadW(wRegIndex(src))))
	case dRegIndex(src) >= 0:
		v = int64(e.regs.ReadDRaw(dRegIndex(src)))
	case sRegIndex(src) >= 0:
		v = int64(int32(e.regs.ReadSRaw(sRegIndex(src))))
	default:
		return &UnsupportedInstructionError{Instruction: inst}
	}
	if dRegIndex(dst) >= 0 {
		e.regs.WriteD(dRegIndex(dst), float64(v))
	} else {
		e.regs.WriteS(sRegIndex(dst), float32(v))
	}
	return nil
}

func (e *executor) execUCVTF(inst arm64asm.Inst) error {
	dst := inst.Args[0].(arm64asm.Reg)
	src := inst.Args[1].(arm64asm.Reg)
	var v uint64
	switch {
	case xRegIndex(src) >= 0:
		v = e.regs.ReadX(xRegIndex(src))
	case wRegIndex(src) >= 0:
		v = uint64(e.regs.ReadW(wRegIndex(src)))
	case dRegIndex(src) >= 0:
		v = e.regs.ReadDRaw(dRegIndex(src))
	case sRegIndex(src) >= 0:
		v = uint64(e.regs.ReadSRaw(sRegIndex(src)))
	default:
		return &UnsupportedInstructionError{Instruction: inst}
	}
	if dRegIndex(dst) >= 0 {
		e.regs.WriteD(dRegIndex(dst), float64(v))
	} else {
		e.regs.WriteS(sRegIndex(dst), float32(v))
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

func (e *executor) execMOVI(inst arm64asm.Inst) error {
	// Check for vector arrangement form (e.g., MOVI V0.2D, #0)
	if ra, ok := inst.Args[0].(arm64asm.RegisterWithArrangement); ok {
		reg, arrangement := getRegWithArrangement(ra)
		vi := vRegIndex(reg)
		if vi < 0 {
			return fmt.Errorf("MOVI: unsupported vector register %v", reg)
		}
		val := resolveImm(inst.Args[1])
		const arrangement2D = 12 // arm64asm.Arrangement2D
		if arrangement == arrangement2D {
			// MOVI Vd.2D, #imm — write imm to both 64-bit lanes
			e.regs.WriteQ(vi, val, val)
			return nil
		}
		// For other arrangements, write the value to lo, clear hi
		e.regs.WriteQ(vi, val, 0)
		return nil
	}

	dst := inst.Args[0].(arm64asm.Reg)
	val := resolveImm(inst.Args[1])

	if di := dRegIndex(dst); di >= 0 {
		e.regs.WriteDRaw(di, val)
		return nil
	}
	if si := sRegIndex(dst); si >= 0 {
		e.regs.WriteSRaw(si, uint32(val))
		return nil
	}
	return fmt.Errorf("MOVI: unsupported destination register %v", dst)
}

func (e *executor) execSXTL(inst arm64asm.Inst) error {
	// SXTL is SSHLL with shift=0 (sign-extend long)
	// SXTL Vd.2D, Vn.2S — sign-extend each 32-bit element to 64-bit
	dstRA := inst.Args[0].(arm64asm.RegisterWithArrangement)
	srcRA := inst.Args[1].(arm64asm.RegisterWithArrangement)

	dstReg, _ := getRegWithArrangement(dstRA)
	srcReg, _ := getRegWithArrangement(srcRA)

	di := vRegIndex(dstReg)
	si := vRegIndex(srcReg)
	if di < 0 || si < 0 {
		return fmt.Errorf("SXTL: invalid registers dst=%v src=%v", dstReg, srcReg)
	}

	// Read source: two 32-bit lanes packed in the low 64 bits
	srcLo, _ := e.regs.ReadQ(si)
	lane0 := int32(uint32(srcLo))       // bits [31:0]
	lane1 := int32(uint32(srcLo >> 32)) // bits [63:32]

	// Write dest: two 64-bit lanes (sign-extended)
	e.regs.WriteQ(di, uint64(int64(lane0)), uint64(int64(lane1)))
	return nil
}

func (e *executor) execFCVT(inst arm64asm.Inst) error {
	dst := inst.Args[0].(arm64asm.Reg)
	src := inst.Args[1].(arm64asm.Reg)

	dstDI := dRegIndex(dst)
	dstSI := sRegIndex(dst)
	srcDI := dRegIndex(src)
	srcSI := sRegIndex(src)

	if dstDI >= 0 && srcSI >= 0 {
		// FCVT Dd, Sn — single to double
		val := e.regs.ReadS(srcSI)
		e.regs.WriteD(dstDI, float64(val))
	} else if dstSI >= 0 && srcDI >= 0 {
		// FCVT Sd, Dn — double to single
		val := e.regs.ReadD(srcDI)
		e.regs.WriteS(dstSI, float32(val))
	} else {
		return fmt.Errorf("FCVT: unsupported operand combination dst=%v src=%v", dst, src)
	}
	return nil
}

func (e *executor) execFCCMP(inst arm64asm.Inst) error {
	cond := inst.Args[3].(arm64asm.Cond)
	if e.regs.EvalCond(cond) {
		// Condition true: compare the two FP registers (same as FCMP)
		if isFPDouble(inst.Args[0]) {
			a := e.regs.ReadD(dRegIndex(inst.Args[0].(arm64asm.Reg)))
			b := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
			e.setFPFlags64(a, b)
		} else {
			a := e.regs.ReadS(sRegIndex(inst.Args[0].(arm64asm.Reg)))
			b := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
			e.setFPFlags32(a, b)
		}
	} else {
		// Condition false: set NZCV from immediate
		nzcv := inst.Args[2].(arm64asm.Imm).Imm
		e.regs.N = (nzcv & 8) != 0
		e.regs.Z = (nzcv & 4) != 0
		e.regs.C = (nzcv & 2) != 0
		e.regs.V = (nzcv & 1) != 0
	}
	return nil
}

func (e *executor) execFMAXNM(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		if math.IsNaN(a) && math.IsNaN(b) {
			e.regs.WriteD(di, math.NaN())
		} else if math.IsNaN(a) {
			e.regs.WriteD(di, b)
		} else if math.IsNaN(b) {
			e.regs.WriteD(di, a)
		} else {
			e.regs.WriteD(di, math.Max(a, b))
		}
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		if math.IsNaN(float64(a)) && math.IsNaN(float64(b)) {
			e.regs.WriteS(si, float32(math.NaN()))
		} else if math.IsNaN(float64(a)) {
			e.regs.WriteS(si, b)
		} else if math.IsNaN(float64(b)) {
			e.regs.WriteS(si, a)
		} else {
			e.regs.WriteS(si, float32(math.Max(float64(a), float64(b))))
		}
	}
	return nil
}

func (e *executor) execFMINNM(inst arm64asm.Inst) error {
	if isFPDouble(inst.Args[0]) {
		di := dRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadD(dRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadD(dRegIndex(inst.Args[2].(arm64asm.Reg)))
		if math.IsNaN(a) && math.IsNaN(b) {
			e.regs.WriteD(di, math.NaN())
		} else if math.IsNaN(a) {
			e.regs.WriteD(di, b)
		} else if math.IsNaN(b) {
			e.regs.WriteD(di, a)
		} else {
			e.regs.WriteD(di, math.Min(a, b))
		}
	} else {
		si := sRegIndex(inst.Args[0].(arm64asm.Reg))
		a := e.regs.ReadS(sRegIndex(inst.Args[1].(arm64asm.Reg)))
		b := e.regs.ReadS(sRegIndex(inst.Args[2].(arm64asm.Reg)))
		if math.IsNaN(float64(a)) && math.IsNaN(float64(b)) {
			e.regs.WriteS(si, float32(math.NaN()))
		} else if math.IsNaN(float64(a)) {
			e.regs.WriteS(si, b)
		} else if math.IsNaN(float64(b)) {
			e.regs.WriteS(si, a)
		} else {
			e.regs.WriteS(si, float32(math.Min(float64(a), float64(b))))
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
