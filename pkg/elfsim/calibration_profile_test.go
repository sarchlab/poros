package elfsim

import (
	"encoding/json"
	"github.com/sarchlab/akita/v5/sim/timing"
	"strings"
	"testing"
)

func TestTimingProfileStrictValidation(t *testing.T) {
	good := DefaultTimingProfile(3490 * timing.MHz)
	for name, change := range map[string]func(*TimingProfile){
		"negative stores":   func(p *TimingProfile) { p.Configuration.NumStores = -1 },
		"zero MSHR":         func(p *TimingProfile) { p.Configuration.NumMSHREntries = 0 },
		"huge ROB":          func(p *TimingProfile) { p.Configuration.ROBCapacity = 1 << 30 },
		"bad geometry":      func(p *TimingProfile) { p.CacheConfig.L1LineSize = 0 },
		"negative latency":  func(p *TimingProfile) { p.CacheConfig.DRAMLatency = -1 },
		"invalid predictor": func(p *TimingProfile) { p.Configuration.Predictor.BTBWays = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p := good
			change(&p)
			b, _ := json.Marshal(p)
			if _, err := ReadTimingProfile(strings.NewReader(string(b))); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
	b, _ := json.Marshal(good)
	if _, err := ReadTimingProfile(strings.NewReader(string(b))); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`null`, `{}`, string(b) + ` {}`, strings.Replace(string(b), `"num_stores":0`, `"num_storse":0`, 1)} {
		if _, err := ReadTimingProfile(strings.NewReader(bad)); err == nil {
			t.Fatal("malformed profile accepted", bad)
		}
	}
}
