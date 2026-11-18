package mem

import "testing"

func TestBankedReadFillCalendars(t *testing.T) {
	setup := func() *Cache {
		cfg := DefaultCacheConfig()
		cfg.L2Latency = 12
		cfg.L1ReadFillBanks, cfg.L1ReadFillBankCycles = 8, 3
		return NewCache(cfg)
	}
	const a = 0x1000
	for _, tc := range []struct {
		name string
		addr uint64
		want uint64
	}{
		{"same group", a + 512, 25},
		{"distinct group", a + 64, 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := setup()
			if found := c.fillL1Banked(a, 0, 10); found != 22 {
				t.Fatalf("first fill completes at %d, want 22", found)
			}
			if found := c.fillL1Banked(tc.addr, 0, 10); found != tc.want {
				t.Fatalf("second fill completes at %d, want %d", found, tc.want)
			}
		})
	}

	c := setup()
	if c.fillL1Banked(a, 0, 100) != 112 ||
		c.fillL1Banked(a+512, 0, 0) != 12 ||
		c.fillL1Banked(a+1024, 0, 0) != 15 ||
		c.fillL1Banked(a+64, 0, 1) != 17 {
		t.Fatalf("unexpected transfer schedule: shared %+v, group %+v", c.channel.busy, c.readFillBanks[0].busy)
	}
	c.Advance(16)
	if len(c.readFillBanks[0].busy) != 1 || c.readFillBanks[0].busy[0].Start != 110 {
		t.Fatalf("expired group reservations were not pruned: %+v", c.readFillBanks[0].busy)
	}
}
