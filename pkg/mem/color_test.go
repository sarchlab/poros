package mem

import "testing"

func TestL2ColorKeepsPageOffsetsAndGuestTags(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L2ColorPageBytes = 16 << 10
	cfg.L2RandomVictim = true
	level := newL2(cfg)
	lines := int(cfg.L2ColorPageBytes / cfg.L2LineSize)
	changed := false
	for page := 0; page < 128; page++ {
		base := uint64(page * cfg.L2ColorPageBytes)
		set := level.setIndex(base)
		changed = changed || set != page*lines%level.numSets
		for i := 0; i < lines; i++ {
			addr := base + uint64(i*cfg.L2LineSize)
			if level.setIndex(addr) != (set+i)%level.numSets || level.tag(addr) != addr/uint64(cfg.L2LineSize) {
				t.Fatalf("page %d, line %d: incorrect color or tag", page, i)
			}
		}
	}
	if !changed {
		t.Fatal("page coloring left every index unchanged")
	}
	shared := NewSharedL2(cfg)
	cache := NewPrivateL1WithSharedL2(cfg, shared)
	if !cache.l2.randomVictim || cache.l2.colorPageBytes != uint64(cfg.L2ColorPageBytes) || cache.l1.randomVictim || cache.l1.colorPageBytes != 0 {
		t.Fatal("colored replacement must apply only at shared L2")
	}
}
