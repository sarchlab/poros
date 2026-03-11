package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

// rayTriangleIntersectGolden implements Möller-Trumbore in Go (golden reference).
func rayTriangleIntersectGolden(
	ox, oy, oz, dx, dy, dz float64,
	v0x, v0y, v0z, v1x, v1y, v1z, v2x, v2y, v2z float64,
) float64 {
	e1x := v1x - v0x
	e1y := v1y - v0y
	e1z := v1z - v0z
	e2x := v2x - v0x
	e2y := v2y - v0y
	e2z := v2z - v0z

	hx := dy*e2z - dz*e2y
	hy := dz*e2x - dx*e2z
	hz := dx*e2y - dy*e2x

	a := e1x*hx + e1y*hy + e1z*hz
	if a > -1e-10 && a < 1e-10 {
		return -1.0
	}

	f := 1.0 / a
	sx := ox - v0x
	sy := oy - v0y
	sz := oz - v0z
	u := f * (sx*hx + sy*hy + sz*hz)
	if u < 0.0 || u > 1.0 {
		return -1.0
	}

	qx := sy*e1z - sz*e1y
	qy := sz*e1x - sx*e1z
	qz := sx*e1y - sy*e1x

	v := f * (dx*qx + dy*qy + dz*qz)
	if v < 0.0 || u+v > 1.0 {
		return -1.0
	}

	t := f * (e2x*qx + e2y*qy + e2z*qz)
	if t > 1e-10 {
		return t
	}
	return -1.0
}

// rayAABBIntersectGolden tests ray-AABB intersection (slab method).
func rayAABBIntersectGolden(
	ox, oy, oz, idx, idy, idz float64,
	bminx, bminy, bminz, bmaxx, bmaxy, bmaxz float64,
	tmaxLimit float64,
) bool {
	t1x := (bminx - ox) * idx
	t2x := (bmaxx - ox) * idx
	t1y := (bminy - oy) * idy
	t2y := (bmaxy - oy) * idy
	t1z := (bminz - oz) * idz
	t2z := (bmaxz - oz) * idz

	tminX := math.Min(t1x, t2x)
	tmaxX := math.Max(t1x, t2x)
	tminY := math.Min(t1y, t2y)
	tmaxY := math.Max(t1y, t2y)
	tminZ := math.Min(t1z, t2z)
	tmaxZ := math.Max(t1z, t2z)

	tmin := math.Max(math.Max(tminX, tminY), tminZ)
	tmax := math.Min(math.Min(tmaxX, tmaxY), tmaxZ)

	return tmax >= tmin && tmax > 0.0 && tmin < tmaxLimit
}

// bvhTraceGolden is the Go golden reference for the BVH traversal.
func bvhTraceGolden(
	ray [6]float64, // ox, oy, oz, dx, dy, dz
	bbox []float64, // [minx, miny, minz, maxx, maxy, maxz] * numNodes
	children []int32, // [left, right] * numNodes
	triVerts []float64, // [v0x..v2z] * numTris (9 each)
	triIdx []int32, // triangle index per node
	numNodes int,
) float64 {
	ox, oy, oz := ray[0], ray[1], ray[2]
	dx, dy, dz := ray[3], ray[4], ray[5]
	idx := 1.0 / dx
	idy := 1.0 / dy
	idz := 1.0 / dz

	closestT := 1e20

	stack := make([]int, 64)
	sp := 0
	stack[sp] = 0
	sp++

	for sp > 0 {
		sp--
		node := stack[sp]
		if node < 0 || node >= numNodes {
			continue
		}

		bi := node * 6
		if !rayAABBIntersectGolden(ox, oy, oz, idx, idy, idz,
			bbox[bi], bbox[bi+1], bbox[bi+2],
			bbox[bi+3], bbox[bi+4], bbox[bi+5],
			closestT) {
			continue
		}

		left := int(children[node*2])
		right := int(children[node*2+1])

		if left == -1 && right == -1 {
			ti := int(triIdx[node])
			if ti >= 0 {
				vi := ti * 9
				t := rayTriangleIntersectGolden(
					ox, oy, oz, dx, dy, dz,
					triVerts[vi], triVerts[vi+1], triVerts[vi+2],
					triVerts[vi+3], triVerts[vi+4], triVerts[vi+5],
					triVerts[vi+6], triVerts[vi+7], triVerts[vi+8],
				)
				if t > 0.0 && t < closestT {
					closestT = t
				}
			}
		} else {
			if right >= 0 {
				stack[sp] = right
				sp++
			}
			if left >= 0 {
				stack[sp] = left
				sp++
			}
		}
	}

	if closestT >= 1e20 {
		return -1.0
	}
	return closestT
}

func TestParsecRaytrace(t *testing.T) {
	// BVH with 7 nodes:
	//   node 0: root (children 1,2)
	//   node 1: internal (children 3,4)
	//   node 2: internal (children 5,6)
	//   node 3: leaf, tri 0
	//   node 4: leaf, tri 1
	//   node 5: leaf, tri 2
	//   node 6: leaf, tri 3
	numNodes := 7
	numTris := 4

	// 4 triangles forming a ground plane at z=0 from (-2,-2) to (2,2)
	triVerts := []float64{
		// tri 0: (-2,-2,0), (0,-2,0), (0,0,0)
		-2, -2, 0, 0, -2, 0, 0, 0, 0,
		// tri 1: (-2,-2,0), (0,0,0), (-2,0,0)
		-2, -2, 0, 0, 0, 0, -2, 0, 0,
		// tri 2: (0,0,0), (2,0,0), (2,2,0)
		0, 0, 0, 2, 0, 0, 2, 2, 0,
		// tri 3: (0,0,0), (2,2,0), (0,2,0)
		0, 0, 0, 2, 2, 0, 0, 2, 0,
	}

	// BBox for each node [minx, miny, minz, maxx, maxy, maxz]
	bbox := []float64{
		// node 0: root  [-2,-2,-0.1, 2,2,0.1]
		-2, -2, -0.1, 2, 2, 0.1,
		// node 1: left  [-2,-2,-0.1, 0,0,0.1]
		-2, -2, -0.1, 0, 0, 0.1,
		// node 2: right [0,0,-0.1, 2,2,0.1]
		0, 0, -0.1, 2, 2, 0.1,
		// node 3: leaf [-2,-2,-0.1, 0,0,0.1]
		-2, -2, -0.1, 0, 0, 0.1,
		// node 4: leaf [-2,-2,-0.1, 0,0,0.1]
		-2, -2, -0.1, 0, 0, 0.1,
		// node 5: leaf [0,0,-0.1, 2,2,0.1]
		0, 0, -0.1, 2, 2, 0.1,
		// node 6: leaf [0,0,-0.1, 2,2,0.1]
		0, 0, -0.1, 2, 2, 0.1,
	}

	// Children: [left, right] per node
	children := []int32{
		1, 2, // node 0
		3, 4, // node 1
		5, 6, // node 2
		-1, -1, // node 3 (leaf)
		-1, -1, // node 4 (leaf)
		-1, -1, // node 5 (leaf)
		-1, -1, // node 6 (leaf)
	}

	// Triangle index per node (-1 for internal nodes)
	triIdx := []int32{
		-1, // node 0
		-1, // node 1
		-1, // node 2
		0,  // node 3 → tri 0
		1,  // node 4 → tri 1
		2,  // node 5 → tri 2
		3,  // node 6 → tri 3
	}

	// Test rays
	type testCase struct {
		name string
		ray  [6]float64 // ox, oy, oz, dx, dy, dz
	}

	rays := []testCase{
		{"hit_center", [6]float64{-1, -1, 5, 0, 0, -1}},      // straight down at (-1,-1)
		{"hit_right", [6]float64{1, 1, 3, 0, 0, -1}},          // straight down at (1,1)
		{"miss_above", [6]float64{5, 5, 3, 0, 0, -1}},         // misses entirely
		{"hit_angled", [6]float64{-1, -1, 2, 0.1, 0.1, -1}},   // angled ray hitting plane
	}

	_ = numTris

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
	resultBase := uint64(0x700000)
	haltPC := uint64(0xFFFF0000)

	for _, tc := range rays {
		t.Run(tc.name, func(t *testing.T) {
			// Compute golden
			golden := bvhTraceGolden(tc.ray, bbox, children, triVerts, triIdx, numNodes)

			em := NewEmulator()
			em.MaxInst = 10_000_000

			// Load code
			codeBuf := make([]byte, len(code)*4)
			for i, w := range code {
				binary.LittleEndian.PutUint32(codeBuf[i*4:], w)
			}
			em.LoadProgram(codeBase, codeBuf)

			// Load ray data
			rayBuf := make([]byte, 6*8)
			for i, v := range tc.ray {
				binary.LittleEndian.PutUint64(rayBuf[i*8:], math.Float64bits(v))
			}
			em.LoadProgram(rayBase, rayBuf)

			// Load bbox
			bboxBuf := make([]byte, len(bbox)*8)
			for i, v := range bbox {
				binary.LittleEndian.PutUint64(bboxBuf[i*8:], math.Float64bits(v))
			}
			em.LoadProgram(bboxBase, bboxBuf)

			// Load children (int32 array)
			childBuf := make([]byte, len(children)*4)
			for i, v := range children {
				binary.LittleEndian.PutUint32(childBuf[i*4:], uint32(v))
			}
			em.LoadProgram(childrenBase, childBuf)

			// Load tri_verts
			tvBuf := make([]byte, len(triVerts)*8)
			for i, v := range triVerts {
				binary.LittleEndian.PutUint64(tvBuf[i*8:], math.Float64bits(v))
			}
			em.LoadProgram(triVertsBase, tvBuf)

			// Load tri_idx (int32 array)
			tiBuf := make([]byte, len(triIdx)*4)
			for i, v := range triIdx {
				binary.LittleEndian.PutUint32(tiBuf[i*4:], uint32(v))
			}
			em.LoadProgram(triIdxBase, tiBuf)

			// Stack and result area
			em.LoadProgram(stackBase, make([]byte, 16384))
			em.LoadProgram(resultBase, make([]byte, 64))

			// Set registers per ABI:
			// X0 = ray_data, X1 = bbox, X2 = children, X3 = tri_verts, X4 = tri_idx, W5 = num_nodes
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

			err := em.Run()
			if err != nil {
				t.Fatalf("emulator error: %v", err)
			}

			// Result is in D0
			result := em.Regs.ReadD(0)

			if math.IsNaN(golden) || math.IsNaN(result) {
				t.Fatalf("NaN: golden=%v, result=%v", golden, result)
			}

			if golden == -1.0 {
				if result != -1.0 {
					t.Fatalf("expected miss (-1), got %v", result)
				}
			} else {
				diff := math.Abs(result - golden)
				if diff > 1e-6 {
					t.Fatalf("mismatch: golden=%v, result=%v, diff=%v", golden, result, diff)
				}
			}
			t.Logf("ray=%v golden=%v result=%v", tc.ray, golden, result)
		})
	}
}
