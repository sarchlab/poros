package mem

import "sort"

// HitLevel indicates where a memory access was satisfied.
type HitLevel int

const (
	HitL1 HitLevel = iota
	HitL2
	// HitSLC is an L2 miss supplied by the memory-side cache. Only the timed
	// path (Access, Store, Probe) reports it.
	HitSLC
	HitDRAM
)

// CacheConfig holds parameters for the two-level cache hierarchy.
//
// The first nine fields describe the legacy synchronous model. The remaining
// fields configure the timed access path (Access): a finite fill transport, a
// trained, optionally page-bounded sequential stream prefetcher, and a
// memory-side cache behind L2. Zero values retain legacy adjacent-line
// prefetch and disable the other timed mechanisms; every timed fill still
// carries an arrival time.
type CacheConfig struct {
	L1Size      int // bytes
	L1Ways      int // associativity
	L1LineSize  int // bytes
	L2Size      int // bytes
	L2Ways      int // associativity
	L2LineSize  int // bytes
	L1Latency   int // cycles
	L2Latency   int // cycles
	DRAMLatency int // cycles

	// FillBytesPerCycle bounds the L2-to-L1 fill transport used by every timed
	// L1 line fill (demand or adjacent-line prefetch). 0 means unlimited.
	FillBytesPerCycle int
	// DRAMBytesPerCycle bounds the DRAM-to-L2 transport used by every timed L2
	// line fill (demand or stream prefetch). 0 means unlimited. The two
	// transports are separate model resources; their rates are effective
	// assumptions, not measured physical bus widths.
	DRAMBytesPerCycle int
	// WriteBufferEntries bounds pending write-allocate lines and each dirty
	// eviction stage (private L1, shared L2). Zero disables
	// combining and dirty eviction traffic for legacy configurations.
	WriteBufferEntries int
	// DisableAdjacentL1Prefetch suppresses automatic next-line fills on timed
	// L1 misses (Access/Store). False preserves legacy configurations; the M2
	// default is true. This does not affect trained L2 stream prefetch or the
	// untimed Lookup/Prefetch warming APIs.
	DisableAdjacentL1Prefetch bool
	// PrefetchDistance is how many L2 lines ahead of the triggering demand line
	// a trained stream may run. 0 disables stream prefetch.
	PrefetchDistance int
	// PrefetchDegree is the maximum number of L2 lines issued per trigger.
	PrefetchDegree int
	// PrefetchTrainMisses is the number of consecutive same-direction demand
	// L1 misses needed before a stream issues prefetches (at least 1).
	PrefetchTrainMisses int
	// PrefetchStreams is the number of concurrently tracked streams (LRU).
	PrefetchStreams int
	// PrefetchPageBytes stops the prefetch frontier at this boundary. 0 means
	// no boundary.
	PrefetchPageBytes int
	// SLCSize is the capacity in bytes of a memory-side cache that keeps the
	// lines L2 evicts: the most recent SLCSize/L2LineSize victims, in the
	// order they left, with no set structure. A timed L2 miss on a line it
	// still holds arrives after SLCLatency cycles instead of DRAMLatency and
	// moves the line back to L2. A clean victim's move out of L2 occupies a
	// transport of its own, as wide as the fill transport (DRAMBytesPerCycle).
	// SLCSize 0 disables all of it. The untimed Lookup/Prefetch paths neither
	// fill nor consult it.
	SLCSize    int
	SLCLatency int
	// TLBPageBytes is the page size of data-address translation timing
	// (Translate). 0 disables it: every translation is available at once.
	TLBPageBytes int
	// TLBSets and TLBWays give the reach of the modelled translation level:
	// TLBSets*TLBWays pages, indexed by page number modulo TLBSets, LRU.
	TLBSets int
	TLBWays int
	// TLBWalkLatency is the cycles one page walk takes and TLBWalkers the
	// number of walks that can be in progress at once.
	TLBWalkLatency int
	TLBWalkers     int
}

// DefaultCacheConfig returns a reasonable default configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		L1Size:      128 * 1024,
		L1Ways:      8,
		L1LineSize:  64,
		L2Size:      16 * 1024 * 1024,
		L2Ways:      16,
		L2LineSize:  128,
		L1Latency:   5,
		L2Latency:   14,
		DRAMLatency: 335,

		// Effective model assumptions; see evaluation/model-profile.md for
		// the diagnostic evidence, assumed-frequency conversion and limits.
		// These values do not identify physical Apple M2 transport widths or
		// prefetch structures.
		FillBytesPerCycle:         32,
		DRAMBytesPerCycle:         16,
		WriteBufferEntries:        56,
		DisableAdjacentL1Prefetch: true,
		PrefetchDistance:          48,
		PrefetchDegree:            4,
		PrefetchTrainMisses:       3,
		PrefetchStreams:           16,
		// No extra page-boundary penalty in this model. This does not assert
		// that a physical prefetcher crosses virtual or physical boundaries.
		PrefetchPageBytes: 0,

		// Effective values; evaluation/mvt-outlier-investigation.md has the
		// captures and their limits. 138 cycles is what a hop of a dependent
		// chase costs on an M2 performance core when the chased lines
		// conflict in L2. 8 MiB is the published figure for the M1's
		// system-level cache, not something measured here: the captures only
		// require more than about 0.3 MiB.
		SLCSize:    8 * 1024 * 1024,
		SLCLatency: 138,
		// 16 KiB pages and a 3,072-page translation reach (256 sets of 12).
		// The same probes give about 30 cycles per walk for a dependent
		// load and four walks' worth of throughput for independent loads.
		TLBPageBytes:   16384,
		TLBSets:        256,
		TLBWays:        12,
		TLBWalkLatency: 30,
		TLBWalkers:     4,
	}
}

// CacheStats tracks hit/miss counters per L1 line segment, so a cross-line
// Access or Store can count more than once. The prefetch and in-flight counters are
// only advanced by the timed Access/Store paths.
type CacheStats struct {
	WriteAllocations, WriteMerges              uint64
	L1Writebacks, L2Writebacks, WritebackBytes uint64
	L1Hits                                     uint64
	L1Misses                                   uint64
	L2Hits                                     uint64
	L2Misses                                   uint64
	// L1HitsInFlight counts accesses to L1 lines whose fill is still pending.
	L1HitsInFlight uint64
	// L2HitsInFlight counts L2 hits on a line whose DRAM fill was pending.
	L2HitsInFlight uint64
	// Prefetches counts L2 lines requested by the stream prefetcher.
	Prefetches uint64
	// PrefetchedL2Hits counts demand L2 hits on stream-prefetched lines.
	PrefetchedL2Hits uint64
	// StreamsTrained counts stream entries that reached the training threshold.
	StreamsTrained uint64
	// StreamEvictions counts LRU replacements of a trained stream entry.
	StreamEvictions uint64
	// FillWaitCycles sums readiness beyond nominal latency for demand loads
	// and store allocations. It is not a pipeline stall-cycle counter: stores
	// may forward their data while their allocation remains outstanding.
	FillWaitCycles uint64
	// SLCHits counts L2 line fetches supplied by the memory-side cache
	// (demand misses, prefetches and writeback allocations alike).
	SLCHits uint64
	// TLBWalks counts page walks started by Translate. TLBWaitCycles sums how
	// long translations were not yet available to the accesses that asked.
	TLBWalks      uint64
	TLBWaitCycles uint64
}

type cacheLine struct {
	generation uint64
	dirty      bool
	dirtyAt    uint64
	valid      bool
	tag        uint64
	lastUsed   uint64
	// readyAt is the cycle at which the line's data has arrived. Lines installed
	// by the legacy untimed path have readyAt 0 (always ready).
	readyAt uint64
	// prefetched marks a line installed by the stream prefetcher and not yet
	// touched by demand.
	prefetched bool
}

type cacheLevel struct {
	sets     [][]cacheLine
	numSets  int
	ways     int
	lineSize int
	counter  uint64
}

func newCacheLevel(totalSize, ways, lineSize int) *cacheLevel {
	numSets := totalSize / (ways * lineSize)
	sets := make([][]cacheLine, numSets)
	for i := range sets {
		sets[i] = make([]cacheLine, ways)
	}
	return &cacheLevel{
		sets:     sets,
		numSets:  numSets,
		ways:     ways,
		lineSize: lineSize,
	}
}

func (cl *cacheLevel) setIndex(addr uint64) int {
	return int((addr / uint64(cl.lineSize)) % uint64(cl.numSets))
}

func (cl *cacheLevel) tag(addr uint64) uint64 {
	return addr / uint64(cl.lineSize)
}

// find returns the line holding addr and marks it most recently used.
func (cl *cacheLevel) find(addr uint64) *cacheLine {
	set := cl.setIndex(addr)
	t := cl.tag(addr)
	cl.counter++
	for i := range cl.sets[set] {
		if cl.sets[set][i].valid && cl.sets[set][i].tag == t {
			cl.sets[set][i].lastUsed = cl.counter
			return &cl.sets[set][i]
		}
	}
	return nil
}

// peek returns the line holding addr without touching replacement state.
func (cl *cacheLevel) peek(addr uint64) *cacheLine {
	set := cl.setIndex(addr)
	t := cl.tag(addr)
	for i := range cl.sets[set] {
		if cl.sets[set][i].valid && cl.sets[set][i].tag == t {
			return &cl.sets[set][i]
		}
	}
	return nil
}

// lookup checks if addr is in this cache level. Returns true on hit.
func (cl *cacheLevel) lookup(addr uint64) bool {
	return cl.find(addr) != nil
}

// victim selects the first free slot, or the LRU occupied slot with first-way
// tie breaking. Timed writeback and installation use this same selector.
func (cl *cacheLevel) victim(addr uint64) *cacheLine {
	set := cl.sets[cl.setIndex(addr)]
	var victim *cacheLine
	for i := range set {
		if !set[i].valid {
			return &set[i]
		}
		if victim == nil || set[i].lastUsed < victim.lastUsed {
			victim = &set[i]
		}
	}
	return victim
}

// installAt puts addr into this cache level with the given arrival cycle,
// evicting the LRU line if needed, and returns the installed line.
func (cl *cacheLevel) installAt(addr uint64, readyAt uint64, prefetched bool) *cacheLine {
	slot := cl.victim(addr)
	cl.counter++
	*slot = cacheLine{valid: true, tag: cl.tag(addr), lastUsed: cl.counter, generation: cl.counter, readyAt: readyAt, prefetched: prefetched}
	return slot
}

// install puts addr into this cache level as immediately ready (legacy path).
func (cl *cacheLevel) install(addr uint64) {
	cl.installAt(addr, 0, false)
}

// fillInterval is one occupied interval [Start, End) on the fill transport.
type fillInterval struct {
	Start uint64
	End   uint64
}

// fillChannel is a serial transport shared by every timed fill. It retains
// non-overlapping occupied intervals so out-of-order request times can still
// use earlier gaps, exactly like the execution-unit calendar.
type fillChannel struct {
	bytesPerCycle uint64
	busy          []fillInterval
}

func (ch *fillChannel) occupancy(bytes int) uint64 {
	if ch == nil || ch.bytesPerCycle == 0 {
		return 0
	}
	occ := (uint64(bytes) + ch.bytesPerCycle - 1) / ch.bytesPerCycle
	if occ == 0 {
		occ = 1
	}
	return occ
}

func (ch *fillChannel) prune(now uint64) {
	if ch == nil {
		return
	}
	n := sort.Search(len(ch.busy), func(i int) bool { return ch.busy[i].End > now })
	// Drop the expired prefix without copying every surviving reservation.
	ch.busy = ch.busy[n:]
}

// deliver books a transfer and coalesces adjacent occupied intervals. A packed
// stream is one interval, not a growing list scanned for every cache line.
// Gaps remain available to requests visited out of issue-time order.
func (ch *fillChannel) deliver(earliestStart uint64, bytes int) uint64 {
	occ := ch.occupancy(bytes)
	if occ == 0 {
		return earliestStart
	}
	start := earliestStart
	pos := sort.Search(len(ch.busy), func(i int) bool { return ch.busy[i].End > start })
	for pos < len(ch.busy) && start+occ > ch.busy[pos].Start {
		start = ch.busy[pos].End
		pos++
	}
	end := start + occ
	if pos > 0 && ch.busy[pos-1].End == start {
		ch.busy[pos-1].End = end
		if pos < len(ch.busy) && ch.busy[pos].Start == end {
			ch.busy[pos-1].End = ch.busy[pos].End
			copy(ch.busy[pos:], ch.busy[pos+1:])
			ch.busy = ch.busy[:len(ch.busy)-1]
		}
	} else if pos < len(ch.busy) && ch.busy[pos].Start == end {
		ch.busy[pos].Start = start
	} else {
		ch.busy = append(ch.busy, fillInterval{})
		copy(ch.busy[pos+1:], ch.busy[pos:])
		ch.busy[pos] = fillInterval{Start: start, End: end}
	}
	return end
}

// stream is one tracked sequential stream in units of L2 lines.
type stream struct {
	valid      bool
	trained    bool
	lastLine   uint64 // last demand L2 line observed
	frontier   uint64 // next L2 line to prefetch
	dir        int64  // +1 ascending, -1 descending, 0 unknown
	confidence int
	lastUse    uint64
}

// Cache is an analytic two-level cache model (L1D + L2). Lookup is untimed;
// Access tracks fill arrival and transport occupancy, and can be backed by a
// memory-side cache of L2 victims and preceded by translation timing. Tags,
// replacement and stream training evolve in caller/dispatch order, not
// chronological issue order, retaining the legacy cache-residency
// approximation.
type Cache struct {
	writes                     []writeAllocation
	writebackUntil             uint64
	l1Writebacks, l2Writebacks []uint64
	l1                         *cacheLevel
	l2                         *cacheLevel
	slc                        *victimStore // lines evicted from L2; nil when disabled
	cfg                        CacheConfig
	stats                      CacheStats
	channel                    *fillChannel // L2 -> L1 fills
	dram                       *fillChannel // memory side (SLC or DRAM) -> L2 fills
	spill                      *fillChannel // clean L2 victims -> SLC
	streams                    []stream
	tlb                        *tlb // private to this cache, like L1
	useTick                    uint64
	watermark                  uint64
	timingDone                 bool
	shared                     *SharedL2
}

// NewCache creates a cache hierarchy from the given configuration.
// Each call creates an independent L1+L2 pair (not shared).
func NewCache(cfg CacheConfig) *Cache {
	c := &Cache{
		l1:  newCacheLevel(cfg.L1Size, cfg.L1Ways, cfg.L1LineSize),
		l2:  newCacheLevel(cfg.L2Size, cfg.L2Ways, cfg.L2LineSize),
		cfg: cfg,
	}
	c.slc, c.spill = newMemorySideCache(cfg)
	c.initTimed()
	c.l2Writebacks = make([]uint64, cfg.WriteBufferEntries)
	return c
}

// newMemorySideCache returns the store behind L2 and the transport that
// carries clean victims to it, or nil for both when disabled. The transport
// is nil (unlimited) when the fill transport is.
func newMemorySideCache(cfg CacheConfig) (*victimStore, *fillChannel) {
	if cfg.SLCSize == 0 {
		return nil, nil
	}
	if cfg.SLCLatency < 1 || cfg.SLCSize < cfg.L2LineSize {
		panic("mem: memory-side cache needs a positive latency and room for one line")
	}
	var spill *fillChannel
	if cfg.DRAMBytesPerCycle > 0 {
		spill = &fillChannel{bytesPerCycle: uint64(cfg.DRAMBytesPerCycle)}
	}
	return newVictimStore(cfg.SLCSize / cfg.L2LineSize), spill
}

func (c *Cache) initTimed() {
	if c.cfg.WriteBufferEntries < 0 {
		panic("mem: negative write-buffer capacity")
	}
	c.writes = make([]writeAllocation, c.cfg.WriteBufferEntries)
	c.l1Writebacks = make([]uint64, c.cfg.WriteBufferEntries)
	if c.cfg.FillBytesPerCycle > 0 {
		c.channel = &fillChannel{bytesPerCycle: uint64(c.cfg.FillBytesPerCycle)}
	}
	if c.cfg.DRAMBytesPerCycle > 0 {
		c.dram = &fillChannel{bytesPerCycle: uint64(c.cfg.DRAMBytesPerCycle)}
	}
	if c.cfg.PrefetchDistance > 0 && c.cfg.PrefetchStreams > 0 {
		c.streams = make([]stream, c.cfg.PrefetchStreams)
	}
	c.tlb = newTLB(c.cfg)
}

// SharedL2 is an L2 cache that can be shared across multiple cores, together
// with the memory-side cache behind it.
type SharedL2 struct {
	writebacks []uint64
	level      *cacheLevel
	slc        *victimStore
	cfg        CacheConfig
	dram       *fillChannel
	spill      *fillChannel
	clients    map[*Cache]uint64
	started    bool
}

// NewSharedL2 creates a shared L2 cache from the given config.
// The L2 parameters (L2Size, L2Ways, L2LineSize) from cfg are used.
func NewSharedL2(cfg CacheConfig) *SharedL2 {
	if cfg.WriteBufferEntries < 0 {
		panic("mem: negative write-buffer capacity")
	}
	s := &SharedL2{level: newCacheLevel(cfg.L2Size, cfg.L2Ways, cfg.L2LineSize), cfg: cfg, clients: make(map[*Cache]uint64), writebacks: make([]uint64, cfg.WriteBufferEntries)}
	s.slc, s.spill = newMemorySideCache(cfg)
	if cfg.DRAMBytesPerCycle > 0 {
		s.dram = &fillChannel{bytesPerCycle: uint64(cfg.DRAMBytesPerCycle)}
	}
	return s
}

// NewPrivateL1WithSharedL2 creates a per-core cache with a private L1 and a
// shared L2. Multiple caches created with the same SharedL2 will share L2
// state, so a miss in one core's L1 that populates the shared L2 will be
// visible as an L2 hit from another core. DRAM transport is shared; L2-to-L1
// transport and stream training are private. Attach all clients before Advance.
// Clients must use the same cycle domain and call FinishTiming when stopped.
func NewPrivateL1WithSharedL2(cfg CacheConfig, shared *SharedL2) *Cache {
	if shared.started {
		panic("mem: cannot attach a cache after shared timing starts")
	}
	if cfg.L2Size != shared.cfg.L2Size || cfg.L2Ways != shared.cfg.L2Ways || cfg.L2LineSize != shared.cfg.L2LineSize || cfg.L2Latency != shared.cfg.L2Latency || cfg.DRAMLatency != shared.cfg.DRAMLatency || cfg.DRAMBytesPerCycle != shared.cfg.DRAMBytesPerCycle || cfg.WriteBufferEntries != shared.cfg.WriteBufferEntries ||
		cfg.SLCSize != shared.cfg.SLCSize || cfg.SLCLatency != shared.cfg.SLCLatency {
		panic("mem: inconsistent shared L2 timing configuration")
	}
	c := &Cache{
		l1: newCacheLevel(cfg.L1Size, cfg.L1Ways, cfg.L1LineSize),
		l2: shared.level, slc: shared.slc, spill: shared.spill, cfg: cfg, shared: shared,
	}
	c.initTimed()
	c.dram = shared.dram
	c.l2Writebacks = shared.writebacks
	shared.clients[c] = 0
	return c
}

// Advance retires transport history only at an actual simulation-cycle
// watermark. Access issue times may be in the future and must not retire it.
// No later Access on this cache may have an issue time below this watermark.
func (c *Cache) Advance(cycle uint64) {
	if c.timingDone || cycle < c.watermark {
		panic("mem: invalid cache timing watermark")
	}
	c.watermark = cycle
	c.channel.prune(cycle)
	if c.tlb != nil {
		c.tlb.prune(cycle)
	}
	if c.shared == nil {
		c.dram.prune(cycle)
		c.spill.prune(cycle)
		return
	}
	c.shared.started = true
	c.shared.clients[c] = cycle
	c.shared.retireTransport()
}

// FinishTiming removes a stopped client from the shared retirement watermark.
// Its outstanding transfers remain booked for any still-running clients.
// The caller must not issue any further timed access from this cache.
func (c *Cache) FinishTiming() {
	if c.timingDone {
		return
	}
	c.timingDone = true
	if c.shared != nil {
		delete(c.shared.clients, c)
		c.shared.retireTransport()
	}
}

func (s *SharedL2) retireTransport() {
	if len(s.clients) == 0 {
		return
	}
	minimum := ^uint64(0)
	for _, cycle := range s.clients {
		if cycle < minimum {
			minimum = cycle
		}
	}
	s.dram.prune(minimum)
	s.spill.prune(minimum)
}

// Probe classifies current residency without changing tags, replacement,
// statistics, streams, or transport reservations. A pending L1 fill is an L1
// hit; Access still enforces its arrival time.
func (c *Cache) Probe(addr uint64) HitLevel {
	if c.l1.peek(addr) != nil {
		return HitL1
	}
	if c.l2.peek(addr) != nil {
		return HitL2
	}
	if c.slc != nil && c.slc.holds(c.l2Line(addr)) {
		return HitSLC
	}
	return HitDRAM
}

// Prefetch silently brings addr into L1 (legacy instant next-line prefetch).
// It does not account for dirty evictions: use only for pre-run warming when
// combined with timed stores, never interleave it with the timed write path.
// An L1 line copied from an L2 line whose timed fill is still pending inherits
// that arrival cycle, so the untimed path can never expose data earlier than
// the timed path would.
func (c *Cache) Prefetch(addr uint64) {
	if c.l1.lookup(addr) {
		return // already in L1
	}
	if line := c.l2.find(addr); line != nil {
		c.l1.installAt(addr, line.readyAt, false)
	} else {
		c.l2.install(addr)
		c.l1.install(addr)
	}
}

// Lookup simulates an untimed memory access and returns the level where it
// was satisfied. This is the legacy synchronous path: fills are instantaneous
// and the adjacent L1 line is prefetched for free on a miss. Lines it installs
// are immediately ready for the timed path as well, except that a copy of a
// pending L2 line inherits the pending arrival. Like Prefetch, this path does
// not account for dirty evictions; mixing with timed stores is limited to
// pre-run warming before any timed writes.
func (c *Cache) Lookup(addr uint64, size int, isWrite bool) HitLevel {
	if c.l1.lookup(addr) {
		c.stats.L1Hits++
		return HitL1
	}
	c.stats.L1Misses++

	nextLine := ((addr / uint64(c.cfg.L1LineSize)) + 1) * uint64(c.cfg.L1LineSize)

	if line := c.l2.find(addr); line != nil {
		c.stats.L2Hits++
		c.l1.installAt(addr, line.readyAt, false)
		c.Prefetch(nextLine)
		return HitL2
	}
	c.stats.L2Misses++

	c.l2.install(addr)
	c.l1.install(addr)
	c.Prefetch(nextLine)
	return HitDRAM
}

// Access simulates a timed access issued at cycle now. It returns the level
// that satisfies the access and the cycle at which its data is available, which
// is never earlier than now plus that level's latency and never earlier than
// the arrival of the fill that supplies the line. Fills share the finite
// transports. A demand load miss trains and triggers the stream prefetcher; a
// store miss allocates its line through the same transports but does not train
// streams. Automatic adjacent L1 fills are controlled separately by
// DisableAdjacentL1Prefetch for both loads and stores. This charges allocation
// traffic and, when write buffering is enabled, dirty eviction traffic. Coherence
// is not modeled. Callers decide whether the
// returned readiness delays the instruction or holds a store allocation slot.
// Access records an already-admitted transaction; it does not acquire a write
// queue slot. CPU stores that need bounded admission use Store instead.
// Residency/replacement/training remain in dispatch order, even when issue
// times are not monotonic; this does not implement a chronological cache.
func (c *Cache) Access(addr uint64, size int, isWrite bool, now uint64) (HitLevel, uint64) {
	if size <= 0 {
		panic("mem: nonpositive access size")
	}
	worst, ready := HitL1, uint64(0)
	for size > 0 {
		n := c.cfg.L1LineSize - int(addr%uint64(c.cfg.L1LineSize))
		if n > size {
			n = size
		}
		level, arrival := c.accessLine(addr, n, isWrite, now)
		if level > worst {
			worst = level
		}
		ready = max(ready, arrival)
		addr += uint64(n)
		size -= n
	}
	return worst, ready
}

func (c *Cache) accessLine(addr uint64, size int, isWrite bool, now uint64) (HitLevel, uint64) {
	if c.timingDone || now < c.watermark {
		panic("mem: access before watermark or after timing completion")
	}
	if c.shared != nil {
		c.shared.started = true
	}
	c.useTick++

	if line := c.l1.find(addr); line != nil {
		if isWrite && c.cfg.WriteBufferEntries > 0 {
			line.dirty = true
			line.dirtyAt = max(line.dirtyAt, now)
		}
		c.stats.L1Hits++
		ready := now + uint64(c.cfg.L1Latency)
		if line.readyAt > ready {
			c.stats.L1HitsInFlight++
			c.stats.FillWaitCycles += line.readyAt - ready
			ready = line.readyAt
		}
		return HitL1, ready
	}
	c.stats.L1Misses++

	level := HitL2
	var l2Ready uint64
	if line := c.l2.find(addr); line != nil {
		c.stats.L2Hits++
		if line.prefetched {
			c.stats.PrefetchedL2Hits++
			line.prefetched = false
		}
		l2Ready = line.readyAt
		if l2Ready > now {
			c.stats.L2HitsInFlight++
		}
	} else {
		c.stats.L2Misses++
		l2Ready, level = c.fetchL2(addr, now, false)
	}

	var ready uint64
	if level != HitL2 {
		// The demand fill delivers the critical line to L1 as the data arrives
		// from the memory side; its L1 transfer still occupies the L2-to-L1
		// transport.
		ready = c.transferL1(l2Ready, now)
	} else {
		ready = c.fillL1(l2Ready, now)
	}
	line := c.installL1(addr, ready, false, now)
	ready = line.readyAt
	if isWrite && c.cfg.WriteBufferEntries > 0 {
		line.dirty = true
		line.dirtyAt = now
	}
	nominal := now + uint64(c.cfg.L2Latency)
	switch level {
	case HitSLC:
		nominal = now + uint64(c.cfg.SLCLatency)
	case HitDRAM:
		nominal = now + uint64(c.cfg.DRAMLatency)
	}
	if ready > nominal {
		c.stats.FillWaitCycles += ready - nominal
	}

	if !c.cfg.DisableAdjacentL1Prefetch {
		c.prefetchAdjacent(addr, now, level != HitL2, l2Ready)
	}

	// Train the stream table on the demand load miss and run the frontier ahead.
	if !isWrite {
		c.trainAndPrefetch(addr, now)
	}
	return level, ready
}

// fillL1 books a 64 B transfer from L2 into L1 for a line whose data is in L2
// at l2Ready and returns the L1 arrival cycle (an L2 access that starts when
// the line is present, plus the transfer).
func (c *Cache) fillL1(l2Ready, now uint64) uint64 {
	start := now
	if l2Ready > start {
		start = l2Ready
	}
	return c.transferL1(start+uint64(c.cfg.L2Latency), start)
}

// transferL1 books the L2-to-L1 transfer of one L1 line whose data can finish
// arriving at earliest and returns when it has arrived.
func (c *Cache) transferL1(earliest, notBefore uint64) uint64 {
	occ := c.channel.occupancy(c.cfg.L1LineSize)
	if occ == 0 {
		return earliest
	}
	start := notBefore
	if earliest >= occ && earliest-occ > start {
		start = earliest - occ
	}
	return c.channel.deliver(start, c.cfg.L1LineSize)
}

// fetchL2 books a memory-side fill of the L2 line holding addr, installs it
// with its arrival cycle and returns that cycle and the supplier. A line the
// memory-side cache still holds arrives after SLCLatency, counted from when
// the line's own data reached that cache, and leaves it; any other line
// arrives from DRAM. Both use the same finite memory-side transport.
// prefetched marks stream prefetches.
func (c *Cache) fetchL2(addr uint64, now uint64, prefetched bool) (uint64, HitLevel) {
	level, arrival := HitDRAM, now+uint64(c.cfg.DRAMLatency)
	if c.slc != nil {
		if held, ok := c.slc.take(c.l2Line(addr)); ok {
			level, arrival = HitSLC, max(now, held)+uint64(c.cfg.SLCLatency)
			c.stats.SLCHits++
		}
	}
	if occ := c.dram.occupancy(c.cfg.L2LineSize); occ > 0 {
		start := now
		if arrival >= occ && arrival-occ > start {
			start = arrival - occ
		}
		arrival = c.dram.deliver(start, c.cfg.L2LineSize)
	}
	line := c.installL2(addr, arrival, prefetched, now)
	return line.readyAt, level
}

// keepInSLC records an L2 victim that left L2 at cycle left in the
// memory-side cache. The line cannot be supplied from there before it left
// L2, nor before its data had arrived in, or been written to, L2, nor before
// it has been moved: a clean victim's 128 bytes travel on the spill
// transport, one line after another. (A dirty victim's transfer was already
// booked on the fill transport by the caller, and left is its end.) Tags are
// updated in dispatch order, so a request visited later but issued earlier
// than left still finds the line here and waits until then: it can be later
// than the hardware would be, never earlier.
func (c *Cache) keepInSLC(victim cacheLine, left uint64) {
	if c.slc == nil {
		return
	}
	ready := max(left, victim.readyAt, victim.dirtyAt)
	if !victim.dirty {
		ready = c.spill.deliver(ready, c.cfg.L2LineSize)
	}
	c.slc.keep(victim.tag, ready)
}

// prefetchAdjacent brings the next 64 B line into L1 with a real arrival time.
// The other half of a 128 B line fetched from DRAM by this demand miss arrives
// with that fill and only pays its own L2-to-L1 transfer. Otherwise the adjacent
// line is an ordinary L2-to-L1 fill, and if its L2 line is absent it is fetched
// from DRAM through the DRAM transport.
func (c *Cache) prefetchAdjacent(addr uint64, now uint64, demandFill bool, demandReady uint64) {
	next := ((addr / uint64(c.cfg.L1LineSize)) + 1) * uint64(c.cfg.L1LineSize)
	if c.l1.find(next) != nil {
		return // present (replacement state touched exactly as the legacy path does)
	}
	if demandFill && c.l2Line(next) == c.l2Line(addr) {
		c.installL1(next, c.transferL1(demandReady, now), false, now)
		return
	}
	var l2Ready uint64
	if line := c.l2.find(next); line != nil {
		l2Ready = line.readyAt
	} else {
		l2Ready, _ = c.fetchL2(next, now, false)
	}
	c.installL1(next, c.fillL1(l2Ready, now), false, now)
}

func (c *Cache) l2Line(addr uint64) uint64 { return addr / uint64(c.cfg.L2LineSize) }

// trainAndPrefetch implements a trained, direction-aware, optionally page-bounded stream
// prefetcher in units of L2 lines. A demand L1 miss that continues a tracked
// stream advances its frontier by at most PrefetchDegree lines, never more than
// PrefetchDistance lines ahead of demand. A nonzero PrefetchPageBytes also
// restricts the frontier to that page; the default has no page bound.
func (c *Cache) trainAndPrefetch(addr uint64, now uint64) {
	if len(c.streams) == 0 {
		return
	}
	m := c.l2Line(addr)
	const window = 4 // lines a continuation may skip
	best := -1
	for i := range c.streams {
		s := &c.streams[i]
		if !s.valid {
			continue
		}
		delta := int64(m) - int64(s.lastLine)
		if delta == 0 {
			// Another L1 miss inside the same L2 line (e.g. its second half);
			// the stream already accounts for this line.
			s.lastUse = c.useTick
			return
		}
		if s.dir == 0 {
			if delta <= window && delta >= -window {
				best = i
				break
			}
			continue
		}
		if delta*s.dir >= 1 && delta*s.dir <= window {
			best = i
			break
		}
	}
	if best < 0 {
		// Allocate the least recently used entry.
		victim := 0
		for i := range c.streams {
			if !c.streams[i].valid {
				victim = i
				break
			}
			if c.streams[i].lastUse < c.streams[victim].lastUse {
				victim = i
			}
		}
		if c.streams[victim].valid && c.streams[victim].trained {
			c.stats.StreamEvictions++
		}
		c.streams[victim] = stream{valid: true, lastLine: m, confidence: 1, lastUse: c.useTick}
		return
	}
	s := &c.streams[best]
	s.lastUse = c.useTick
	if s.dir == 0 {
		if m > s.lastLine {
			s.dir = 1
		} else {
			s.dir = -1
		}
		s.frontier = uint64(int64(m) + s.dir)
	}
	s.lastLine = m
	s.confidence++
	// The frontier never falls behind the demand line.
	if (s.dir > 0 && s.frontier <= m) || (s.dir < 0 && s.frontier >= m) {
		s.frontier = uint64(int64(m) + s.dir)
	}
	threshold := c.cfg.PrefetchTrainMisses
	if threshold < 1 {
		threshold = 1
	}
	if s.confidence < threshold {
		return
	}
	if !s.trained {
		s.trained = true
		c.stats.StreamsTrained++
	}
	limit := int64(m) + s.dir*int64(c.cfg.PrefetchDistance)
	degree := c.cfg.PrefetchDegree
	if degree < 1 {
		degree = 1
	}
	for issued := 0; issued < degree; issued++ {
		f := int64(s.frontier)
		if (s.dir > 0 && f > limit) || (s.dir < 0 && (f < limit || f < 0)) {
			return
		}
		lineAddr := uint64(f) * uint64(c.cfg.L2LineSize)
		if c.cfg.PrefetchPageBytes > 0 && lineAddr/uint64(c.cfg.PrefetchPageBytes) != addr/uint64(c.cfg.PrefetchPageBytes) {
			return // page bounded: the next page re-trains on its own misses
		}
		if c.l2.peek(lineAddr) == nil {
			c.fetchL2(lineAddr, now, true)
			c.stats.Prefetches++
		}
		s.frontier = uint64(f + s.dir)
	}
}

// Config returns the cache configuration.
func (c *Cache) Config() CacheConfig {
	return c.cfg
}

// Stats returns the current hit/miss counters.
func (c *Cache) Stats() CacheStats {
	return c.stats
}

// ResetStats zeroes the hit/miss counters.
func (c *Cache) ResetStats() {
	c.stats = CacheStats{}
}
