package emu

import (
	"encoding/binary"
	"math"
	"sort"
	"testing"
)

// knnSearchGolden computes k-NN in Go (golden reference).
func knnSearchGolden(queries, database []float64, nQueries, nDB, dim, k int) []float64 {
	results := make([]float64, nQueries*k)
	for q := 0; q < nQueries; q++ {
		query := queries[q*dim : (q+1)*dim]
		res := results[q*k : (q+1)*k]
		for i := range res {
			res[i] = 1e30
		}
		for d := 0; d < nDB; d++ {
			dbVec := database[d*dim : (d+1)*dim]
			dist := 0.0
			for j := 0; j < dim; j++ {
				diff := query[j] - dbVec[j]
				dist += diff * diff
			}
			if dist < res[k-1] {
				res[k-1] = dist
				sort.Float64s(res) // simple insertion
			}
		}
	}
	return results
}

func TestParsecFerret(t *testing.T) {
	// End-to-end test: PARSEC ferret k-NN distance computation.
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize
	//                -fno-slp-vectorize -fno-unroll-loops -fno-stack-protector
	//                -fno-builtin
	//
	// No external function calls (no BL instructions). Pure FP arithmetic.
	//
	// ARM64 ABI:
	//   X0 = results (double[nQueries * k])
	//   X1 = queries (double[nQueries * dim])
	//   X2 = database (double[nDB * dim])
	//   W3 = nQueries
	//   W4 = nDB
	//   W5 = dim
	//   W6 = k

	nQueries, nDB, dim, k := 4, 32, 8, 3

	// Generate query data
	queries := make([]float64, nQueries*dim)
	for i := range queries {
		queries[i] = float64(i)*0.1 + 0.5
	}

	// Generate database data
	database := make([]float64, nDB*dim)
	for i := range database {
		database[i] = float64(i%17)*0.2 - 1.0
	}

	// Compute golden reference
	golden := knnSearchGolden(queries, database, nQueries, nDB, dim, k)

	// Machine code: knn_search function (78 instructions)
	// Compiled from parsec/kernels/ferret.c
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

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load queries
	qBuf := make([]byte, nQueries*dim*8)
	for i, v := range queries {
		binary.LittleEndian.PutUint64(qBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(queriesBase, qBuf)

	// Load database
	dbBuf := make([]byte, nDB*dim*8)
	for i, v := range database {
		binary.LittleEndian.PutUint64(dbBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(databaseBase, dbBuf)

	// Output buffer (zeros)
	em.LoadProgram(resultsBase, make([]byte, nQueries*k*8))

	// Stack
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set up registers
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, resultsBase)   // results
	em.Regs.WriteX(1, queriesBase)   // queries
	em.Regs.WriteX(2, databaseBase)  // database
	em.Regs.WriteX(3, uint64(nQueries)) // nQueries (W3)
	em.Regs.WriteX(4, uint64(nDB))      // nDB (W4)
	em.Regs.WriteX(5, uint64(dim))      // dim (W5)
	em.Regs.WriteX(6, uint64(k))        // k (W6)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	// Run
	err := em.Run()
	if err != nil {
		t.Fatalf("emulator error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read output and compare
	maxRelErr := 0.0
	for q := 0; q < nQueries; q++ {
		for i := 0; i < k; i++ {
			idx := q*k + i
			addr := resultsBase + uint64(idx*8)
			bits := em.Mem.ReadU64(addr)
			got := math.Float64frombits(bits)
			want := golden[idx]

			denom := math.Max(1.0, math.Abs(want))
			relErr := math.Abs(got-want) / denom
			if relErr > maxRelErr {
				maxRelErr = relErr
			}
			if relErr > 1e-10 {
				t.Errorf("results[%d][%d]: got %v, want %v (relErr=%e)", q, i, got, want, relErr)
			}
		}
	}

	t.Logf("ferret k-NN: %d instructions executed, max relative error = %e",
		em.InstCount, maxRelErr)
}
