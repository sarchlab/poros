package darwinuser

import (
	"fmt"
	"github.com/sarchlab/poros/pkg/usermem"
)

// QuiescencePolicy describes virtual scheduler state, not a captured native
// counter value. NoScheduling has one running guest thread and no scheduler
// events; other CPUs are inactive. Its epoch starts at zero and never advances.
// XNU smr.c advances this counter after scheduler/SMR quiescence, not on every
// instruction or syscall. A future scheduler must supply those real events.
type QuiescencePolicy string

const QuiescenceNoScheduling QuiescencePolicy = "single-thread-no-scheduling"

func (p QuiescencePolicy) mapCounter(m *usermem.AddressSpace) error {
	switch p {
	case "":
		return nil // Missing state remains unavailable.
	case QuiescenceNoScheduling:
		return m.Map(0xfffffc180, 8, usermem.Read, make([]byte, 8))
	default:
		return fmt.Errorf("unsupported virtual quiescence policy %q", p)
	}
}
