package elfsim

import (
	"bytes"
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

// DefaultTimingProfile is the revision-bound single-core ELF OoO calibration.
// Generic components and in-order/multicore execution retain their own defaults.
func DefaultTimingProfile(freq timing.Freq) TimingProfile {
	c := ooo.DefaultSpec()
	c.Freq = freq
	c.NumStores = 2
	m := mem.DefaultCacheConfig()
	m.L2Latency = 20
	return TimingProfile{Configuration: c, CacheConfig: m}
}
func ReadTimingProfile(r io.Reader) (*TimingProfile, error) {
	var raw json.RawMessage
	d := json.NewDecoder(r)
	if err := d.Decode(&raw); err != nil {
		return nil, fmt.Errorf("timing profile: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("timing profile must contain exactly one JSON object")
	}
	// Derive required keys from the canonical public representation rather than
	// maintaining a second list of CPU/cache fields. Presence is distinct from
	// value validation: omitted booleans and optional-resource zeros are not a
	// complete profile, even when their Go zero values would pass Validate.
	canonical, err := json.Marshal(DefaultTimingProfile(timing.GHz))
	if err != nil {
		return nil, fmt.Errorf("canonical timing profile: %w", err)
	}
	if err := requireProfileShape(raw, canonical, "profile"); err != nil {
		return nil, err
	}
	var p TimingProfile
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return nil, fmt.Errorf("timing profile: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// requireProfileShape checks complete canonical object keys and non-null values.
// Typed decoding below still owns scalar types and Validate owns numeric bounds.
func requireProfileShape(raw, canonical json.RawMessage, path string) error {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		if path == "profile.configuration.func_hook_cycles" {
			return nil
		}
		return fmt.Errorf("timing profile: %s must not be null", path)
	}
	canonical = bytes.TrimSpace(canonical)
	if len(canonical) == 0 || canonical[0] != '{' {
		return nil
	}
	var actual, expected map[string]json.RawMessage
	if err := json.Unmarshal(raw, &actual); err != nil {
		return fmt.Errorf("timing profile: %s must be an object: %w", path, err)
	}
	if err := json.Unmarshal(canonical, &expected); err != nil {
		return err
	}
	for key, shape := range expected {
		value, ok := actual[key]
		if !ok {
			return fmt.Errorf("timing profile: missing %s.%s", path, key)
		}
		if err := requireProfileShape(value, shape, path+"."+key); err != nil {
			return err
		}
	}
	for key := range actual {
		if _, ok := expected[key]; !ok {
			return fmt.Errorf("timing profile: unknown field %s.%s", path, key)
		}
	}
	return nil
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
