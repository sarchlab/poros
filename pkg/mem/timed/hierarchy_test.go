package timed

import (
	"math"
	"testing"

	"github.com/sarchlab/akita/v5/sim/timing"
)

func TestConfigurationValidationAndIndependence(t *testing.T) {
	for _, f := range []timing.Freq{timing.MHz, 3490 * timing.MHz, 1000 * timing.GHz} {
		if err := DefaultConfig(f).Validate(f); err != nil {
			t.Fatal(f, err)
		}
	}
	a, b := DefaultConfig(3490*timing.MHz), DefaultConfig(3490*timing.MHz)
	a.L1.Bottom.Destinations[0] = "other.Top"
	if b.L1.Bottom.Destinations[0] != "L2.Top" {
		t.Fatal("default configs alias routing")
	}
	tests := map[string]func(*Config){
		"bank width":          func(c *Config) { c.L1.BankWidth = 1 },
		"negative stage":      func(c *Config) { c.L2.BankLatency = -1 },
		"zero memory stage":   func(c *Config) { c.Memory.StageLatency = 0 },
		"oversized cache":     func(c *Config) { c.L1.NumSets = 1 << 30 },
		"invalid frequency":   func(c *Config) { c.Memory.Freq = timing.Freq(math.NaN()) },
		"sub-picosecond tick": func(c *Config) { c.L1.Freq = 2e12; c.L2.Freq = 2e12 },
		"routing":             func(c *Config) { c.L1.Bottom.Destinations[0] = "nowhere" },
	}
	for name, f := range tests {
		t.Run(name, func(t *testing.T) {
			c := DefaultConfig(3490 * timing.MHz)
			f(&c)
			if c.Validate(3490*timing.MHz) == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	c := DefaultConfig(3490 * timing.MHz)
	c.L1.DirLatency = 0
	c.L1.BankLatency = 0
	if err := c.Validate(3490 * timing.MHz); err != nil {
		t.Fatal("supported cache bypass rejected", err)
	}
}
