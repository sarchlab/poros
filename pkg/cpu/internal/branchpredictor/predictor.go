package branchpredictor

// PredictorSpec is the immutable predictor geometry.
type PredictorSpec struct {
	BimodalSize     int `json:"bimodal_size"`
	TaggedTableSize int `json:"tagged_table_size"`
	BTBSize         int `json:"btb_size"`
	BTBWays         int `json:"btb_ways"`
	RASSize         int `json:"ras_size"`
	TaggedHistLen   int `json:"tagged_hist_len"`
}

// PredictorState keeps all predictor runtime data checkpointable.
type PredictorState struct {
	Predictions, Hits uint64
	Bimodal           []uint8
	Tagged            []taggedEntry
	BtbSets           int
	Btb               []btbEntry
	Ras               []uint64
	RasTOS            int
	Ghr               uint64
}
type taggedEntry struct {
	Tag     uint64
	Counter uint8
	Valid   bool
}
type btbEntry struct {
	Valid       bool
	Tag, Target uint64
}
type Predictor struct {
	*PredictorState
	Spec PredictorSpec
}

func (bp *Predictor) Init() {
	v := bp.Spec
	if v.BimodalSize < 1 || v.TaggedTableSize < 1 || v.BTBSize < 1 || v.BTBWays < 1 || v.BTBSize%v.BTBWays != 0 || v.RASSize < 1 || v.TaggedHistLen < 1 || v.TaggedHistLen > 63 {
		panic("ooo: invalid predictor configuration")
	}

	// Bimodal: init to weakly taken (2).
	bp.Bimodal = make([]uint8, bp.Spec.BimodalSize)
	for i := range bp.Bimodal {
		bp.Bimodal[i] = 2
	}

	// Tagged table: init counters to 4.
	bp.Tagged = make([]taggedEntry, bp.Spec.TaggedTableSize)

	// BTB.
	bp.BtbSets = bp.Spec.BTBSize / bp.Spec.BTBWays
	bp.Btb = make([]btbEntry, bp.Spec.BTBSize)

	// RAS.
	bp.Ras = make([]uint64, bp.Spec.RASSize)
}

// bimodalIndex computes the bimodal table index from a PC.
func (bp *Predictor) bimodalIndex(pc uint64) int {
	return int((pc >> 2) % uint64(bp.Spec.BimodalSize))
}

// taggedIndex computes the tagged table index using PC and folded history.
func (bp *Predictor) taggedIndex(pc uint64) int {
	folded := bp.foldHistory()
	return int(((pc >> 2) ^ folded) % uint64(bp.Spec.TaggedTableSize))
}

// taggedTag computes the tag for the tagged table from the upper bits of PC.
func (bp *Predictor) taggedTag(pc uint64) uint64 {
	return (pc >> 2) >> uint(bp.Spec.TaggedHistLen)
}

// foldHistory XOR-folds the global history register to TaggedHistLen bits.
func (bp *Predictor) foldHistory() uint64 {
	h := bp.Ghr
	folded := uint64(0)
	bits := bp.Spec.TaggedHistLen
	for h != 0 {
		folded ^= h & ((1 << uint(bits)) - 1)
		h >>= uint(bits)
	}
	return folded
}

// btbLookup returns (found, target) for a given PC in the BTB.
func (bp *Predictor) btbLookup(pc uint64) (bool, uint64) {
	setIdx := int((pc >> 2) % uint64(bp.BtbSets))
	base := setIdx * bp.Spec.BTBWays
	for w := 0; w < bp.Spec.BTBWays; w++ {
		e := &bp.Btb[base+w]
		if e.Valid && e.Tag == pc {
			return true, e.Target
		}
	}
	return false, 0
}

// btbUpdate stores a target for the given PC in the BTB.
func (bp *Predictor) btbUpdate(pc, target uint64) {
	setIdx := int((pc >> 2) % uint64(bp.BtbSets))
	base := setIdx * bp.Spec.BTBWays

	// Look for existing entry or invalid slot.
	oldestIdx := base
	for w := 0; w < bp.Spec.BTBWays; w++ {
		e := &bp.Btb[base+w]
		if !e.Valid {
			e.Valid = true
			e.Tag = pc
			e.Target = target
			return
		}
		if e.Tag == pc {
			e.Target = target
			return
		}
	}

	// Evict first way (simple replacement).
	bp.Btb[oldestIdx] = btbEntry{Valid: true, Tag: pc, Target: target}
}

// rasPush pushes a return address onto the RAS.
func (bp *Predictor) rasPush(addr uint64) {
	bp.Ras[bp.RasTOS] = addr
	bp.RasTOS = (bp.RasTOS + 1) % bp.Spec.RASSize
}

// rasPop pops and returns the top of the RAS.
func (bp *Predictor) rasPop() uint64 {
	bp.RasTOS = (bp.RasTOS - 1 + bp.Spec.RASSize) % bp.Spec.RASSize
	return bp.Ras[bp.RasTOS]
}

// Predict predicts whether a branch at pc is taken and its target.
func (bp *Predictor) Predict(pc uint64, isCall, isReturn bool) (taken bool, target uint64) {
	bp.Predictions++

	// Check tagged table first.
	tIdx := bp.taggedIndex(pc)
	tEntry := &bp.Tagged[tIdx]
	if tEntry.Valid && tEntry.Tag == bp.taggedTag(pc) {
		taken = tEntry.Counter >= 4
	} else {
		// Fall back to bimodal.
		bIdx := bp.bimodalIndex(pc)
		taken = bp.Bimodal[bIdx] >= 2
	}

	// Determine target.
	if isReturn {
		target = bp.rasPop()
		taken = true // returns are always taken
		return
	}

	if isCall {
		bp.rasPush(pc + 4)
	}

	if taken {
		found, btbTarget := bp.btbLookup(pc)
		if found {
			target = btbTarget
		}
		// If BTB miss, target is 0 (unknown) — will mispreddict on target.
	}

	return
}

// Update updates predictor state after a branch resolves.
func (bp *Predictor) Update(
	pc uint64,
	actualTaken bool,
	actualTarget uint64,
	wasPredTaken bool,
	isCall, isReturn bool,
) {
	// Track hits.
	if wasPredTaken == actualTaken {
		bp.Hits++
	}

	// Update bimodal counter.
	bIdx := bp.bimodalIndex(pc)
	if actualTaken {
		if bp.Bimodal[bIdx] < 3 {
			bp.Bimodal[bIdx]++
		}
	} else {
		if bp.Bimodal[bIdx] > 0 {
			bp.Bimodal[bIdx]--
		}
	}

	// Update tagged table.
	tIdx := bp.taggedIndex(pc)
	tEntry := &bp.Tagged[tIdx]
	tagVal := bp.taggedTag(pc)

	if tEntry.Valid && tEntry.Tag == tagVal {
		// Tag matched: update counter.
		if actualTaken {
			if tEntry.Counter < 7 {
				tEntry.Counter++
			}
		} else {
			if tEntry.Counter > 0 {
				tEntry.Counter--
			}
		}
	} else {
		// Tag didn't match: allocate if mispredicted.
		mispredicted := wasPredTaken != actualTaken
		if mispredicted {
			*tEntry = taggedEntry{
				Tag:     tagVal,
				Counter: 4,
				Valid:   true,
			}
		}
	}

	// Update BTB if branch was taken.
	if actualTaken {
		bp.btbUpdate(pc, actualTarget)
	}

	// Shift global history register.
	bp.Ghr = (bp.Ghr << 1)
	if actualTaken {
		bp.Ghr |= 1
	}
}
