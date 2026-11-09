package ooo

import "math/bits"

// PredictorSpec is the immutable predictor geometry.
type PredictorSpec struct {
	BimodalSize     int `json:"bimodal_size"`
	TaggedTableSize int `json:"tagged_table_size"`
	BTBSize         int `json:"btb_size"`
	BTBWays         int `json:"btb_ways"`
	RASSize         int `json:"ras_size"`
	// TaggedHistLen is the number of most recent outcomes used by this table.
	TaggedHistLen int `json:"tagged_hist_len"`
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
type predictor struct {
	*PredictorState
	spec PredictorSpec
}

func (bp *predictor) init() {
	v := bp.spec
	if v.BimodalSize < 1 || v.TaggedTableSize < 1 || v.BTBSize < 1 || v.BTBWays < 1 || v.BTBSize%v.BTBWays != 0 || v.RASSize < 1 || v.TaggedHistLen < 1 || v.TaggedHistLen > 64 {
		panic("ooo: invalid predictor configuration")
	}

	// Bimodal: init to weakly taken (2).
	bp.Bimodal = make([]uint8, bp.spec.BimodalSize)
	for i := range bp.Bimodal {
		bp.Bimodal[i] = 2
	}

	// Tagged table: init counters to 4.
	bp.Tagged = make([]taggedEntry, bp.spec.TaggedTableSize)

	// BTB.
	bp.BtbSets = bp.spec.BTBSize / bp.spec.BTBWays
	bp.Btb = make([]btbEntry, bp.spec.BTBSize)

	// RAS.
	bp.Ras = make([]uint64, bp.spec.RASSize)
}

// bimodalIndex computes the bimodal table index from a PC.
func (bp *predictor) bimodalIndex(pc uint64) int {
	return int((pc >> 2) % uint64(bp.spec.BimodalSize))
}

// taggedIndex computes the tagged table index using PC and folded history.
func (bp *predictor) taggedIndex(pc uint64) int {
	folded := bp.foldHistory()
	return int(((pc >> 2) ^ folded) % uint64(bp.spec.TaggedTableSize))
}

// A full PC and independently mixed history identify the indexed context.
// Shifting PC by the history length made ordinary low-address tags all zero.
func (bp *predictor) taggedTag(pc uint64) uint64 {
	return (pc >> 2) ^ bits.RotateLeft64(bp.history(), 32)
}

func (bp *predictor) history() uint64 {
	if bp.spec.TaggedHistLen == 64 {
		return bp.Ghr
	}
	return bp.Ghr & ((uint64(1) << uint(bp.spec.TaggedHistLen)) - 1)
}

// Fold every configured history bit into the actual index width. Folding to
// 32 bits before taking an 11-bit index silently discarded most history.
func (bp *predictor) foldHistory() uint64 {
	width := bits.Len(uint(bp.spec.TaggedTableSize - 1))
	if width == 0 {
		return 0
	}
	mask := (uint64(1) << uint(width)) - 1
	h, folded := bp.history(), uint64(0)
	for h != 0 {
		folded ^= h & mask
		h >>= uint(width)
	}
	return folded
}

// btbLookup returns (found, target) for a given PC in the BTB.
func (bp *predictor) btbLookup(pc uint64) (bool, uint64) {
	setIdx := int((pc >> 2) % uint64(bp.BtbSets))
	base := setIdx * bp.spec.BTBWays
	for w := 0; w < bp.spec.BTBWays; w++ {
		e := &bp.Btb[base+w]
		if e.Valid && e.Tag == pc {
			return true, e.Target
		}
	}
	return false, 0
}

// btbUpdate stores a target for the given PC in the BTB.
func (bp *predictor) btbUpdate(pc, target uint64) {
	setIdx := int((pc >> 2) % uint64(bp.BtbSets))
	base := setIdx * bp.spec.BTBWays

	// Look for existing entry or invalid slot.
	oldestIdx := base
	for w := 0; w < bp.spec.BTBWays; w++ {
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
func (bp *predictor) rasPush(addr uint64) {
	bp.Ras[bp.RasTOS] = addr
	bp.RasTOS = (bp.RasTOS + 1) % bp.spec.RASSize
}

// rasPop pops and returns the top of the RAS.
func (bp *predictor) rasPop() uint64 {
	bp.RasTOS = (bp.RasTOS - 1 + bp.spec.RASSize) % bp.spec.RASSize
	return bp.Ras[bp.RasTOS]
}

// Predict predicts whether a branch at pc is taken and its target.
func (bp *predictor) Predict(pc uint64, isCall, isReturn bool) (taken bool, target uint64) {
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
func (bp *predictor) Update(
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
