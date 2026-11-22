package mem

import "testing"

func TestMixedL1GroupSharesReadWriteAndEviction(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L1MixedServiceBanks = 32
	cfg.L1MixedReadCycles = 3
	cfg.L1MixedWriteCycles = 7
	cfg.L1MixedEvictCycles = 9
	cache := NewCache(cfg)

	line := uint64(cfg.L1LineSize)
	written := cache.transferL1Mixed(0, 10, 10, uint64(cfg.L1MixedWriteCycles), false)
	// The same address group must wait for the write, even when the bus
	// has capacity. A different group can still use the earlier gap.
	read := cache.transferL1Mixed(line*32, 10, 10, uint64(cfg.L1MixedReadCycles), true)
	other := cache.transferL1Mixed(line, 10, 10, uint64(cfg.L1MixedReadCycles), true)
	evict := cache.transferL1Mixed(line*64, 10, 10, uint64(cfg.L1MixedEvictCycles), false)
	if written != 12 || read != 19 || other != 14 || evict != 22 {
		t.Fatalf("transfer ends written=%d read=%d other=%d evict=%d", written, read, other, evict)
	}
}
