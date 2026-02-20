package cpu

// BranchPredictor implements a TAGE-like branch predictor with BTB and RAS.
type BranchPredictor struct {
	// Configuration (set before calling Init).
	BimodalSize     int // default 4096, must be power of 2
	TaggedTableSize int // default 2048, must be power of 2
	BTBSize         int // default 2048, must be power of 2
	BTBWays         int // default 4
	RASSize         int // default 32
	TaggedHistLen   int // default 32

	// Statistics.
	Predictions uint64
	Hits        uint64

	// Bimodal: 2-bit saturating counters.
	bimodal []uint8

	// Tagged table entries.
	tagged []taggedEntry

	// BTB: set-associative.
	btbSets int
	btb     []btbEntry

	// RAS: circular buffer.
	ras    []uint64
	rasTOS int // top-of-stack pointer

	// Global history register (shift register).
	ghr uint64
}

type taggedEntry struct {
	tag     uint64
	counter uint8 // 3-bit, range 0-7
	valid   bool
}

type btbEntry struct {
	valid  bool
	tag    uint64
	target uint64
}

// NewBranchPredictor creates a branch predictor with default configuration.
func NewBranchPredictor() *BranchPredictor {
	bp := &BranchPredictor{
		BimodalSize:     4096,
		TaggedTableSize: 2048,
		BTBSize:         2048,
		BTBWays:         4,
		RASSize:         32,
		TaggedHistLen:   32,
	}
	bp.init()
	return bp
}

func (bp *BranchPredictor) init() {
	// Bimodal: init to weakly taken (2).
	bp.bimodal = make([]uint8, bp.BimodalSize)
	for i := range bp.bimodal {
		bp.bimodal[i] = 2
	}

	// Tagged table: init counters to 4.
	bp.tagged = make([]taggedEntry, bp.TaggedTableSize)

	// BTB.
	bp.btbSets = bp.BTBSize / bp.BTBWays
	bp.btb = make([]btbEntry, bp.BTBSize)

	// RAS.
	bp.ras = make([]uint64, bp.RASSize)
}

// bimodalIndex computes the bimodal table index from a PC.
func (bp *BranchPredictor) bimodalIndex(pc uint64) int {
	return int((pc >> 2) % uint64(bp.BimodalSize))
}

// taggedIndex computes the tagged table index using PC and folded history.
func (bp *BranchPredictor) taggedIndex(pc uint64) int {
	folded := bp.foldHistory()
	return int(((pc >> 2) ^ folded) % uint64(bp.TaggedTableSize))
}

// taggedTag computes the tag for the tagged table from the upper bits of PC.
func (bp *BranchPredictor) taggedTag(pc uint64) uint64 {
	return (pc >> 2) >> uint(bp.TaggedHistLen)
}

// foldHistory XOR-folds the global history register to TaggedHistLen bits.
func (bp *BranchPredictor) foldHistory() uint64 {
	h := bp.ghr
	folded := uint64(0)
	bits := bp.TaggedHistLen
	for h != 0 {
		folded ^= h & ((1 << uint(bits)) - 1)
		h >>= uint(bits)
	}
	return folded
}

// btbLookup returns (found, target) for a given PC in the BTB.
func (bp *BranchPredictor) btbLookup(pc uint64) (bool, uint64) {
	setIdx := int((pc >> 2) % uint64(bp.btbSets))
	base := setIdx * bp.BTBWays
	for w := 0; w < bp.BTBWays; w++ {
		e := &bp.btb[base+w]
		if e.valid && e.tag == pc {
			return true, e.target
		}
	}
	return false, 0
}

// btbUpdate stores a target for the given PC in the BTB.
func (bp *BranchPredictor) btbUpdate(pc, target uint64) {
	setIdx := int((pc >> 2) % uint64(bp.btbSets))
	base := setIdx * bp.BTBWays

	// Look for existing entry or invalid slot.
	oldestIdx := base
	for w := 0; w < bp.BTBWays; w++ {
		e := &bp.btb[base+w]
		if !e.valid {
			e.valid = true
			e.tag = pc
			e.target = target
			return
		}
		if e.tag == pc {
			e.target = target
			return
		}
	}

	// Evict first way (simple replacement).
	bp.btb[oldestIdx] = btbEntry{valid: true, tag: pc, target: target}
}

// rasPush pushes a return address onto the RAS.
func (bp *BranchPredictor) rasPush(addr uint64) {
	bp.ras[bp.rasTOS] = addr
	bp.rasTOS = (bp.rasTOS + 1) % bp.RASSize
}

// rasPop pops and returns the top of the RAS.
func (bp *BranchPredictor) rasPop() uint64 {
	bp.rasTOS = (bp.rasTOS - 1 + bp.RASSize) % bp.RASSize
	return bp.ras[bp.rasTOS]
}

// Predict predicts whether a branch at pc is taken and its target.
func (bp *BranchPredictor) Predict(pc uint64, isCall, isReturn bool) (taken bool, target uint64) {
	bp.Predictions++

	// Check tagged table first.
	tIdx := bp.taggedIndex(pc)
	tEntry := &bp.tagged[tIdx]
	if tEntry.valid && tEntry.tag == bp.taggedTag(pc) {
		taken = tEntry.counter >= 4
	} else {
		// Fall back to bimodal.
		bIdx := bp.bimodalIndex(pc)
		taken = bp.bimodal[bIdx] >= 2
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
func (bp *BranchPredictor) Update(
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
		if bp.bimodal[bIdx] < 3 {
			bp.bimodal[bIdx]++
		}
	} else {
		if bp.bimodal[bIdx] > 0 {
			bp.bimodal[bIdx]--
		}
	}

	// Update tagged table.
	tIdx := bp.taggedIndex(pc)
	tEntry := &bp.tagged[tIdx]
	tagVal := bp.taggedTag(pc)

	if tEntry.valid && tEntry.tag == tagVal {
		// Tag matched: update counter.
		if actualTaken {
			if tEntry.counter < 7 {
				tEntry.counter++
			}
		} else {
			if tEntry.counter > 0 {
				tEntry.counter--
			}
		}
	} else {
		// Tag didn't match: allocate if mispredicted.
		mispredicted := wasPredTaken != actualTaken
		if mispredicted {
			*tEntry = taggedEntry{
				tag:     tagVal,
				counter: 4,
				valid:   true,
			}
		}
	}

	// Update BTB if branch was taken.
	if actualTaken {
		bp.btbUpdate(pc, actualTarget)
	}

	// Shift global history register.
	bp.ghr = (bp.ghr << 1)
	if actualTaken {
		bp.ghr |= 1
	}
}
