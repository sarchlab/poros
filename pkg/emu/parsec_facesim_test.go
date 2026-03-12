package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParsecFacesim(t *testing.T) {
	// End-to-end test: FEM tetrahedron force computation (Neo-Hookean model).
	//
	// The kernel computes forces on vertices of a tetrahedral mesh using
	// the Neo-Hookean constitutive model. For each tetrahedron:
	//   1) Compute deformation gradient F = Ds * DmInv
	//   2) Compute Piola-Kirchhoff stress P (uses log(det(F)))
	//   3) Accumulate forces f = -V * P * DmInv^T on vertices
	//
	// ARM64 ABI:
	//   X0 = forces (double[nVerts*3])
	//   X1 = positions (double[nVerts*3])
	//   X2 = tetIndices (int[nTets*4])
	//   X3 = DmInv (double[nTets*9])
	//   X4 = restVolume (double[nTets])
	//   D0 = mu
	//   D1 = lambda
	//   W5 = nTets
	//   W6 = nVerts

	nVerts := 8
	nTets := 6

	// Cube vertices with slight deformation
	positions := []float64{
		0.0, 0.0, 0.0, // v0
		1.0, 0.0, 0.0, // v1
		0.0, 1.0, 0.0, // v2
		0.0, 0.0, 1.0, // v3
		1.1, 1.0, 0.0, // v4 (slightly deformed)
		1.0, 0.1, 1.0, // v5
		0.0, 1.0, 1.1, // v6
		1.0, 1.1, 1.0, // v7
	}

	// Tetrahedra indices (6 tets from a cube)
	tetIndices := []int32{
		0, 1, 2, 3,
		1, 4, 2, 7,
		1, 5, 3, 7,
		2, 6, 3, 7,
		1, 2, 3, 7,
		0, 1, 3, 2, // duplicate with different winding for coverage
	}

	// Reference positions (undeformed cube) for DmInv computation
	refPos := []float64{
		0.0, 0.0, 0.0,
		1.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		0.0, 0.0, 1.0,
		1.0, 1.0, 0.0,
		1.0, 0.0, 1.0,
		0.0, 1.0, 1.0,
		1.0, 1.0, 1.0,
	}

	// Precompute DmInv and restVolume for each tet
	DmInv := make([]float64, nTets*9)
	restVolume := make([]float64, nTets)
	for t := 0; t < nTets; t++ {
		i0 := tetIndices[t*4+0]
		i1 := tetIndices[t*4+1]
		i2 := tetIndices[t*4+2]
		i3 := tetIndices[t*4+3]

		x0 := refPos[i0*3]
		y0 := refPos[i0*3+1]
		z0 := refPos[i0*3+2]
		x1 := refPos[i1*3]
		y1 := refPos[i1*3+1]
		z1 := refPos[i1*3+2]
		x2 := refPos[i2*3]
		y2 := refPos[i2*3+1]
		z2 := refPos[i2*3+2]
		x3 := refPos[i3*3]
		y3 := refPos[i3*3+1]
		z3 := refPos[i3*3+2]

		// Dm = [x1-x0, x2-x0, x3-x0; y1-y0, y2-y0, y3-y0; z1-z0, z2-z0, z3-z0]
		dm00 := x1 - x0
		dm01 := x2 - x0
		dm02 := x3 - x0
		dm10 := y1 - y0
		dm11 := y2 - y0
		dm12 := y3 - y0
		dm20 := z1 - z0
		dm21 := z2 - z0
		dm22 := z3 - z0

		// det(Dm)
		det := dm00*(dm11*dm22-dm12*dm21) -
			dm01*(dm10*dm22-dm12*dm20) +
			dm02*(dm10*dm21-dm11*dm20)

		restVolume[t] = math.Abs(det) / 6.0

		// Inverse of Dm (3x3)
		invDet := 1.0 / det
		DmInv[t*9+0] = (dm11*dm22 - dm12*dm21) * invDet
		DmInv[t*9+1] = (dm02*dm21 - dm01*dm22) * invDet
		DmInv[t*9+2] = (dm01*dm12 - dm02*dm11) * invDet
		DmInv[t*9+3] = (dm12*dm20 - dm10*dm22) * invDet
		DmInv[t*9+4] = (dm00*dm22 - dm02*dm20) * invDet
		DmInv[t*9+5] = (dm02*dm10 - dm00*dm12) * invDet
		DmInv[t*9+6] = (dm10*dm21 - dm11*dm20) * invDet
		DmInv[t*9+7] = (dm01*dm20 - dm00*dm21) * invDet
		DmInv[t*9+8] = (dm00*dm11 - dm01*dm10) * invDet
	}

	mu := 1000.0
	lambda := 5000.0

	// Golden reference computation in Go
	goldenForces := make([]float64, nVerts*3)
	for t := 0; t < nTets; t++ {
		i0 := int(tetIndices[t*4+0])
		i1 := int(tetIndices[t*4+1])
		i2 := int(tetIndices[t*4+2])
		i3 := int(tetIndices[t*4+3])

		x0 := positions[i0*3]
		y0 := positions[i0*3+1]
		z0 := positions[i0*3+2]
		x1 := positions[i1*3]
		y1 := positions[i1*3+1]
		z1 := positions[i1*3+2]
		x2 := positions[i2*3]
		y2 := positions[i2*3+1]
		z2 := positions[i2*3+2]
		x3 := positions[i3*3]
		y3 := positions[i3*3+1]
		z3 := positions[i3*3+2]

		ds00 := x1 - x0
		ds01 := x2 - x0
		ds02 := x3 - x0
		ds10 := y1 - y0
		ds11 := y2 - y0
		ds12 := y3 - y0
		ds20 := z1 - z0
		ds21 := z2 - z0
		ds22 := z3 - z0

		dm := DmInv[t*9 : t*9+9]

		// F = Ds * DmInv
		f00 := ds00*dm[0] + ds01*dm[3] + ds02*dm[6]
		f01 := ds00*dm[1] + ds01*dm[4] + ds02*dm[7]
		f02 := ds00*dm[2] + ds01*dm[5] + ds02*dm[8]
		f10 := ds10*dm[0] + ds11*dm[3] + ds12*dm[6]
		f11 := ds10*dm[1] + ds11*dm[4] + ds12*dm[7]
		f12 := ds10*dm[2] + ds11*dm[5] + ds12*dm[8]
		f20 := ds20*dm[0] + ds21*dm[3] + ds22*dm[6]
		f21 := ds20*dm[1] + ds21*dm[4] + ds22*dm[7]
		f22 := ds20*dm[2] + ds21*dm[5] + ds22*dm[8]

		J := f00*(f11*f22-f12*f21) -
			f01*(f10*f22-f12*f20) +
			f02*(f10*f21-f11*f20)

		logJ := math.Log(J)

		// Cofactor
		c00 := f11*f22 - f12*f21
		c01 := f12*f20 - f10*f22
		c02 := f10*f21 - f11*f20
		c10 := f02*f21 - f01*f22
		c11 := f00*f22 - f02*f20
		c12 := f01*f20 - f00*f21
		c20 := f01*f12 - f02*f11
		c21 := f02*f10 - f00*f12
		c22 := f00*f11 - f01*f10

		coeff := (lambda*logJ - mu) / J

		p00 := mu*f00 + coeff*c00
		p01 := mu*f01 + coeff*c01
		p02 := mu*f02 + coeff*c02
		p10 := mu*f10 + coeff*c10
		p11 := mu*f11 + coeff*c11
		p12 := mu*f12 + coeff*c12
		p20 := mu*f20 + coeff*c20
		p21 := mu*f21 + coeff*c21
		p22 := mu*f22 + coeff*c22

		V := restVolume[t]
		h00 := -V * (p00*dm[0] + p01*dm[1] + p02*dm[2])
		h01 := -V * (p00*dm[3] + p01*dm[4] + p02*dm[5])
		h02 := -V * (p00*dm[6] + p01*dm[7] + p02*dm[8])
		h10 := -V * (p10*dm[0] + p11*dm[1] + p12*dm[2])
		h11 := -V * (p10*dm[3] + p11*dm[4] + p12*dm[5])
		h12 := -V * (p10*dm[6] + p11*dm[7] + p12*dm[8])
		h20 := -V * (p20*dm[0] + p21*dm[1] + p22*dm[2])
		h21 := -V * (p20*dm[3] + p21*dm[4] + p22*dm[5])
		h22 := -V * (p20*dm[6] + p21*dm[7] + p22*dm[8])

		goldenForces[i1*3+0] += h00
		goldenForces[i1*3+1] += h10
		goldenForces[i1*3+2] += h20
		goldenForces[i2*3+0] += h01
		goldenForces[i2*3+1] += h11
		goldenForces[i2*3+2] += h21
		goldenForces[i3*3+0] += h02
		goldenForces[i3*3+1] += h12
		goldenForces[i3*3+2] += h22
		goldenForces[i0*3+0] -= h00 + h01 + h02
		goldenForces[i0*3+1] -= h10 + h11 + h12
		goldenForces[i0*3+2] -= h20 + h21 + h22
	}

	// Machine code: 285 instructions compiled from parsec/kernels/facesim.c
	// BL _log at index 133 patched to target logHookAddr=0xF000
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

	codeBase := uint64(0x10000)
	logHookAddr := uint64(0xF000)
	forcesBase := uint64(0x100000)
	posBase := uint64(0x200000)
	tetIdxBase := uint64(0x300000)
	dmInvBase := uint64(0x400000)
	restVolBase := uint64(0x500000)
	stackBase := uint64(0x600000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

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
	for i, v := range DmInv {
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

	// Set registers
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, forcesBase)
	em.Regs.WriteX(1, posBase)
	em.Regs.WriteX(2, tetIdxBase)
	em.Regs.WriteX(3, dmInvBase)
	em.Regs.WriteX(4, restVolBase)
	em.Regs.WriteD(0, mu)
	em.Regs.WriteD(1, lambda)
	em.Regs.WriteX(5, uint64(nTets))
	em.Regs.WriteX(6, uint64(nVerts))
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	// Register log function hook
	em.RegisterFuncHook(logHookAddr, func(em *Emulator) {
		x := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Log(x))
	})

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Read output forces
	tol := 1e-6
	for i := 0; i < nVerts*3; i++ {
		got := math.Float64frombits(em.Mem.ReadU64(forcesBase + uint64(i*8)))
		want := goldenForces[i]
		diff := math.Abs(got - want)
		relDiff := diff
		if math.Abs(want) > 1e-10 {
			relDiff = diff / math.Abs(want)
		}
		if relDiff > tol && diff > tol {
			t.Errorf("forces[%d]: got %g, want %g (diff=%g, rel=%g)", i, got, want, diff, relDiff)
		}
	}

	t.Logf("Facesim test passed: %d tets, %d verts, %d instructions executed",
		nTets, nVerts, em.InstCount)
}
