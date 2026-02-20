// Package demo contains a trivial Akita TickingComponent for verification.
package demo

import (
	"fmt"

	"github.com/sarchlab/akita/v4/sim"
)

// Counter is a simple TickingComponent that increments a counter each tick
// until it reaches a target value, then stops.
type Counter struct {
	*sim.TickingComponent

	Target  int
	Current int
}

// Tick increments the counter. Returns true if progress was made.
func (c *Counter) Tick() bool {
	if c.Current >= c.Target {
		return false
	}

	c.Current++

	if c.Current == c.Target {
		fmt.Printf("%s: reached target %d\n", c.Name(), c.Target)
		return false
	}

	return true
}

// NewCounter creates a Counter that ticks until it reaches target.
func NewCounter(
	name string,
	engine sim.Engine,
	freq sim.Freq,
	target int,
) *Counter {
	c := &Counter{
		Target: target,
	}

	c.TickingComponent = sim.NewTickingComponent(name, engine, freq, c)

	return c
}
