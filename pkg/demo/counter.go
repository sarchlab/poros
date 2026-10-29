// Package demo contains a minimal Akita v5 ticking component.
package demo

import (
	"fmt"

	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/modeling/ticking"
	"github.com/sarchlab/akita/v5/sim/timing"
)

type Spec struct {
	Freq   timing.Freq `json:"freq"`
	Target int         `json:"target"`
}

func DefaultSpec() Spec { return Spec{Freq: timing.GHz, Target: 10} }

type State struct{ Current int }
type Ports struct{}
type Middlewares struct{ Counter *counter }
type Comp = ticking.Component[Spec, State, modeling.None, Ports, Middlewares]

func (m *Middlewares) Initialize(c *Comp) { m.Counter = &counter{comp: c} }

type counter struct{ comp *Comp }

func (c *counter) Handle(_ timing.Event) bool {
	if c.comp.State.Current >= c.comp.Spec.Target {
		return false
	}
	c.comp.State.Current++
	if c.comp.State.Current == c.comp.Spec.Target {
		fmt.Printf("%s: reached target %d\n", c.comp.Name(), c.comp.Spec.Target)
		return false
	}
	return true
}
