package mem

// HitLevel indicates where a memory access was satisfied.
type HitLevel int

const (
	HitL1   HitLevel = iota
	HitL2
	HitDRAM
)

// CacheConfig holds parameters for the two-level cache hierarchy.
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
		L1Latency:   3,
		L2Latency:   18,
		DRAMLatency: 335,
	}
}

// CacheStats tracks hit/miss counters.
type CacheStats struct {
	L1Hits   uint64
	L1Misses uint64
	L2Hits   uint64
	L2Misses uint64
}

type cacheLine struct {
	valid    bool
	tag      uint64
	lastUsed uint64
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

// lookup checks if addr is in this cache level. Returns true on hit.
func (cl *cacheLevel) lookup(addr uint64) bool {
	set := cl.setIndex(addr)
	t := cl.tag(addr)
	cl.counter++
	for i := range cl.sets[set] {
		if cl.sets[set][i].valid && cl.sets[set][i].tag == t {
			cl.sets[set][i].lastUsed = cl.counter
			return true
		}
	}
	return false
}

// install puts addr into this cache level, evicting the LRU line if needed.
func (cl *cacheLevel) install(addr uint64) {
	set := cl.setIndex(addr)
	t := cl.tag(addr)
	cl.counter++

	// Find an invalid slot first.
	for i := range cl.sets[set] {
		if !cl.sets[set][i].valid {
			cl.sets[set][i] = cacheLine{valid: true, tag: t, lastUsed: cl.counter}
			return
		}
	}

	// Evict LRU.
	lru := 0
	for i := 1; i < cl.ways; i++ {
		if cl.sets[set][i].lastUsed < cl.sets[set][lru].lastUsed {
			lru = i
		}
	}
	cl.sets[set][lru] = cacheLine{valid: true, tag: t, lastUsed: cl.counter}
}

// Cache is a synchronous two-level cache model (L1D + L2).
type Cache struct {
	l1    *cacheLevel
	l2    *cacheLevel
	cfg   CacheConfig
	stats CacheStats
}

// NewCache creates a cache hierarchy from the given configuration.
func NewCache(cfg CacheConfig) *Cache {
	return &Cache{
		l1:  newCacheLevel(cfg.L1Size, cfg.L1Ways, cfg.L1LineSize),
		l2:  newCacheLevel(cfg.L2Size, cfg.L2Ways, cfg.L2LineSize),
		cfg: cfg,
	}
}

// Lookup simulates a memory access and returns the level where it was satisfied.
func (c *Cache) Lookup(addr uint64, size int, isWrite bool) HitLevel {
	if c.l1.lookup(addr) {
		c.stats.L1Hits++
		return HitL1
	}
	c.stats.L1Misses++

	if c.l2.lookup(addr) {
		c.stats.L2Hits++
		c.l1.install(addr)
		return HitL2
	}
	c.stats.L2Misses++

	c.l2.install(addr)
	c.l1.install(addr)
	return HitDRAM
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
