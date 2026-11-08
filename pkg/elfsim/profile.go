package elfsim

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/mem"
)

// TimingProfile is a complete, auditable candidate for the lightweight OoO model.
// A supplied profile is not a patch: missing required configuration is rejected.
type TimingProfile struct {
	Configuration ooo.Spec        `json:"configuration"`
	CacheConfig   mem.CacheConfig `json:"cache_config"`
}

func DefaultTimingProfile(freq timing.Freq) TimingProfile {
	c := ooo.DefaultSpec()
	c.Freq = freq
	return TimingProfile{Configuration: c, CacheConfig: mem.DefaultCacheConfig()}
}
func ReadTimingProfile(r io.Reader) (*TimingProfile, error) {
	var p TimingProfile
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return nil, fmt.Errorf("timing profile: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("timing profile must contain exactly one JSON object")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}
func (p TimingProfile) Validate() error {
	c := p.Configuration
	if c.Freq < timing.MHz || c.Freq > 1000*timing.GHz || c.FetchWidth < 1 || c.FetchWidth > 64 || c.RetireWidth < 1 || c.RetireWidth > 64 || c.ROBCapacity < 1 || c.ROBCapacity > 4096 || c.NumINT < 1 || c.NumINT > 64 || c.NumFP < 1 || c.NumFP > 64 || c.NumLDST < 1 || c.NumLDST > 64 || c.NumStores < 0 || c.NumStores > 64 || c.LoadLatency < 1 || c.LoadLatency > 100000 || c.FpPhysLimit < 1 || c.FpPhysLimit > 4096 || c.StoreBufferSize < 1 || c.StoreBufferSize > 4096 || c.NumMSHREntries < 1 || c.NumMSHREntries > 4096 || c.BranchPenalty < 0 || c.BranchPenalty > 10000 {
		return fmt.Errorf("invalid timing profile CPU configuration")
	}
	if len(c.FuncHookCycles) > 0 {
		return fmt.Errorf("ELF timing profiles cannot install functional hooks")
	}
	v := c.Predictor
	if c.BranchPrediction && (v.BimodalSize < 1 || v.BimodalSize > 1<<20 || v.TaggedTableSize < 1 || v.TaggedTableSize > 1<<20 || v.BTBSize < 1 || v.BTBSize > 1<<20 || v.BTBWays < 1 || v.BTBWays > 64 || v.BTBSize%v.BTBWays != 0 || v.RASSize < 1 || v.RASSize > 4096 || v.TaggedHistLen < 1 || v.TaggedHistLen > 63) {
		return fmt.Errorf("invalid timing profile predictor")
	}
	m := p.CacheConfig
	for _, g := range [][3]int{{m.L1Size, m.L1Ways, m.L1LineSize}, {m.L2Size, m.L2Ways, m.L2LineSize}} {
		size, ways, line := g[0], g[1], g[2]
		if size < 1 || size > 512<<20 || ways < 1 || ways > 64 || line < 16 || line > 4096 || line&(line-1) != 0 || size%(ways*line) != 0 {
			return fmt.Errorf("invalid timing profile cache geometry")
		}
	}
	if m.L1Latency < 1 || m.L1Latency > 100000 || m.L2Latency < 1 || m.L2Latency > 100000 || m.DRAMLatency < 1 || m.DRAMLatency > 100000 || m.L2LineSize < m.L1LineSize {
		return fmt.Errorf("invalid timing profile cache latency/line size")
	}
	return nil
}
