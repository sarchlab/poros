package elfsim

import (
	"context"
	"encoding/json"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"os"
	"reflect"
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

func TestTimingProfileAPIContract(t *testing.T) {
	image, err := os.ReadFile("../../tests/elf/hello.elf")
	if err != nil {
		t.Fatal(err)
	}
	fresh := func() *linuxuser.Process {
		p, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"program"}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	profile := DefaultTimingProfile(timing.GHz)
	profile.Configuration.NumStores = 2
	profile.CacheConfig.L1Latency = 7
	got, err := Run(context.Background(), fresh(), Options{Model: "ooo", FrequencyHz: 1e9, TimingProfile: &profile})
	if err != nil {
		t.Fatal(err)
	}
	if got.Termination != "exit" || string(got.Stdout) != "hello, ELF\n" || got.Instructions != 51 || !reflect.DeepEqual(got.Timing.Configuration, profile.Configuration) || got.Timing.CacheConfig != profile.CacheConfig {
		t.Fatalf("profile API did not use requested values: %+v", got)
	}
	for _, tc := range []struct {
		name       string
		options    Options
		diagnostic string
	}{
		{"inorder", Options{Model: "inorder", FrequencyHz: 1e9, TimingProfile: &profile}, "require the OoO model"},
		{"frequency", Options{Model: "ooo", FrequencyHz: 2e9, TimingProfile: &profile}, "frequency differs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fresh()
			before := p.Registers
			result, err := Run(context.Background(), p, tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.diagnostic) || result.Instructions != 0 || p.Registers != before {
				t.Fatalf("invalid profile executed or wrong error: result%+v err%v", result, err)
			}
		})
	}
	p := fresh()
	before := p.Registers
	result, err := RunMultiCore(context.Background(), []*linuxuser.Process{p}, Options{Model: "ooo", FrequencyHz: 1e9, TimingProfile: &profile})
	if err == nil || !strings.Contains(err.Error(), "custom timing profiles") || len(result.Cores) != 0 || p.Registers != before {
		t.Fatalf("multicore silently accepted profile: result%+v err%v", result, err)
	}
}
