package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

func reclaimRequest(t *testing.T, receive uint32) (*kernelState, *usermem.AddressSpace, emu.Registers, []byte) {
	t.Helper()
	b := make([]byte, 128)
	copy(b[24:32], nativeNDR[:])
	binary.LittleEndian.PutUint32(b[32:], 1016)
	binary.LittleEndian.PutUint32(b[36:], 4195320)
	k, m, r := clockRequest(t, b, 0x11f, 40, receive)
	r.GPR[3] = uint64(0x11f)<<32 | 0x103
	r.GPR[4] = 4822 << 32
	r.Counter.Enabled = true
	r.Counter.Frequency = 24000000
	r.Counter.Value = 12345
	k.reclaim = &reclaimState{policy: ReclaimPolicy{"deadline-128-v1", 240000000}, clock: &virtualClock{anchor: ClockAnchor{AbsoluteOffset: 37}}}
	return k, m, r, b
}

func TestReclaimNativeFirstAllocationAndCopyout(t *testing.T) {
	var first struct {
		Cases []struct {
			Initial    string `json:"initial_hex"`
			Size       uint64 `json:"region_size"`
			Protection int    `json:"protection"`
			Maximum    int    `json:"max_protection"`
			Deadline   uint64 `json:"second_output"`
			Before     uint64 `json:"before_absolute"`
			After      uint64 `json:"after_absolute"`
			Consumes   bool   `json:"diagnostic_consumes_first_allocation"`
		}
	}
	kernelNativeResult(t, "reclaim-first-message-probe.c", &first)
	if len(first.Cases) != 3 {
		t.Fatal(len(first.Cases))
	}
	for _, c := range first.Cases {
		zero, e := hex.DecodeString(c.Initial)
		if e != nil || len(zero) != 128 || !bytes.Equal(zero, make([]byte, 128)) || c.Size != 67125248 || c.Protection != 3 || c.Maximum != 3 || !c.Consumes || c.Deadline < 240000000 || c.Deadline-240000000 < c.Before || c.Deadline-240000000 > c.After {
			t.Fatal(c, e)
		}
	}
	var copyout struct {
		Cases []struct {
			Receive    uint32 `json:"receive_size"`
			Readonly   bool
			Return     uint64 `json:"trap_return"`
			ReplySize  uint32 `json:"reply_size"`
			Original   uint32 `json:"original_wrapper_result"`
			Deallocate int    `json:"deallocate_result"`
			Reply      string `json:"reply_hex"`
		}
	}
	kernelNativeResult(t, "reclaim-copyout-probe.c", &copyout)
	if len(copyout.Cases) != 5 {
		t.Fatal(len(copyout.Cases))
	}
	for i, c := range copyout.Cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			k, m, r, b := reclaimRequest(t, c.Receive)
			if c.Readonly {
				if e := m.Protect(0x1000, 128, usermem.Read, false); e != nil {
					t.Fatal(e)
				}
			}
			before := r
			if !k.trap(&r, m) || r.GPR[0] != c.Return {
				t.Fatal(r, c)
			}
			before.GPR[0] = c.Return
			if r != before || k.reclaim.address == 0 || k.reclaim.size != 67125248 || k.ownedBytes != 67125248 || c.Original != 0x20004006 {
				t.Fatal("allocation/ABI state", r, k.reclaim)
			}
			var zero [128]byte
			if e := m.Read(k.reclaim.address, zero[:]); e != nil || zero != [128]byte{} {
				t.Fatal("kernel must leave ring initialization to guest", e, zero)
			}
			if m.CheckProtection(k.reclaim.address, k.reclaim.size, usermem.Read|usermem.Write) != nil || m.CheckProtection(k.reclaim.address, k.reclaim.size, usermem.Execute) == nil {
				t.Fatal("mapping maximum")
			}
			got := make([]byte, 128)
			m.Read(0x1000, got)
			if c.Readonly {
				if !bytes.Equal(got, b) {
					t.Fatal("readonly reply changed")
				}
			} else {
				if binary.LittleEndian.Uint32(got[4:]) != c.ReplySize || binary.LittleEndian.Uint32(got[20:]) != 4922 {
					t.Fatal("reply header", got[:24])
				}
				if c.Return == 0 {
					if binary.LittleEndian.Uint64(got[36:]) != k.reclaim.address || binary.LittleEndian.Uint64(got[44:]) != 12345+37+240000000 {
						t.Fatal("virtual address/deadline", got[:60])
					}
				}
			}

			if !c.Readonly {
				want, err := hex.DecodeString(c.Reply)
				if err != nil || len(want) != 60 {
					t.Fatal(err, want)
				}
				binary.LittleEndian.PutUint32(want[12:], 0x11f)
				if c.Return == 0 {
					binary.LittleEndian.PutUint64(want[36:], k.reclaim.address)
					binary.LittleEndian.PutUint64(want[44:], 12345+37+240000000)
				}
				if !bytes.Equal(got[:60], want) {
					t.Fatalf("native reply bytes differ: got %x want %x", got[:60], want)
				}
			}
			// Native registration survives every failed delivery. A subsequent valid
			// request must return the recorded existing-ring code, without allocating.
			m.Protect(0x1000, 128, usermem.Read|usermem.Write, false)
			m.Write(0x1000, b)
			r = before
			r.GPR[0] = 0x1000
			r.GPR[6] = 60
			if !k.trap(&r, m) || r.GPR[0] != 0 {
				t.Fatal(r)
			}
			m.Read(0x1000, got)
			if binary.LittleEndian.Uint32(got[32:]) != c.Original || k.ownedBytes != 67125248 {
				t.Fatal("duplicate changed mapping", got[:44])
			}
		})
	}
}

func TestReclaimNativeGuestHeaderInitialization(t *testing.T) {
	var native struct {
		Cases []struct {
			Length  uint32
			Maximum uint32 `json:"max_length"`
			Initial string `json:"initial_hex"`
			Before  uint64 `json:"before_absolute"`
			After   uint64 `json:"after_absolute"`
			Matches bool   `json:"output_matches_uninstrumented"`
		}
	}
	kernelNativeResult(t, "reclaim-ring-startup-probe.c", &native)
	if len(native.Cases) != 3 {
		t.Fatal(len(native.Cases))
	}
	for _, c := range native.Cases {
		b, e := hex.DecodeString(c.Initial)
		if e != nil || len(b) != 128 || !c.Matches {
			t.Fatal(c, e)
		}
		start, deadline := binary.LittleEndian.Uint64(b[56:]), binary.LittleEndian.Uint64(b[64:])
		if binary.LittleEndian.Uint32(b[16:]) != c.Length || binary.LittleEndian.Uint32(b[20:]) != c.Maximum || start < c.Before || start > c.After || deadline-240000000 < c.Before || deadline-240000000 > c.After {
			t.Fatal("installed wrapper ABI", c)
		}
		clear(b[16:24])
		clear(b[56:72])
		if !bytes.Equal(b, make([]byte, 128)) {
			t.Fatal("unknown initialized fields")
		}
	}
}

func TestReclaimUnsupportedBeforeMutation(t *testing.T) {
	for _, name := range []string{"missing", "clock", "frequency", "abi", "deadline-overflow", "partial-reply", "ndr", "future-id"} {
		t.Run(name, func(t *testing.T) {
			k, m, r, b := reclaimRequest(t, 60)
			switch name {
			case "missing":
				k.reclaim = nil
			case "clock":
				k.reclaim.clock = nil
			case "frequency":
				r.Counter.Frequency++
			case "abi":
				k.reclaim.policy.ABI = "unknown"
			case "deadline-overflow":
				r.Counter.Value = ^uint64(0) - 100
			case "partial-reply":
				m.Protect(0x1028, 88, usermem.Read, false)
			case "ndr":
				b[28] = 0
				m.Write(0x1000, b)
			case "future-id":
				r.GPR[4] = 4823 << 32
			}
			before := r
			if k.trap(&r, m) || r != before || k.ownedBytes != 0 || len(m.Mappings()) > 2 {
				t.Fatal("unsupported mutation", r)
			}
			got := make([]byte, 128)
			m.Read(0x1000, got)
			if !bytes.Equal(got, b) {
				t.Fatal("unsupported copyout")
			}
		})
	}
}

func TestReclaimPermanentOperationsRemainUnsupported(t *testing.T) {
	for _, name := range []string{"mach-protect", "bsd-protect", "unmap", "overwrite"} {
		t.Run(name, func(t *testing.T) {
			k, m, r, _ := reclaimRequest(t, 60)
			if !k.trap(&r, m) || r.GPR[0] != 0 {
				t.Fatal(r)
			}
			a := k.reclaim.address
			if e := m.Write(a, []byte{0x5a}); e != nil {
				t.Fatal(e)
			}
			r = emu.Registers{C: true}
			r.GPR[0] = 0x103
			r.GPR[1] = a
			r.GPR[2] = vmPage
			r.GPR[4] = 1
			before := r
			handled := false
			switch name {
			case "mach-protect":
				handled = k.machProtect(&r, m)
			case "bsd-protect":
				r.GPR[0] = a
				r.GPR[1] = vmPage
				r.GPR[2] = 1
				before = r
				handled = k.mprotect(&r, m)
			case "unmap":
				handled = k.unmapRange(m, a, a+vmPage)
			case "overwrite":
				_, _, handled = k.allocateAnonymous(m, a, vmPage, 0, 0x4000, 3, 7)
			}
			var b [1]byte
			m.Read(a, b[:])
			if handled || r != before || b[0] != 0x5a || m.Check(a, 1, usermem.Write, "test") != nil {
				t.Fatal("permanent mapping altered", name, r, b)
			}
		})
	}
}

func TestReclaimPolicyOwnedAndBoundToVirtualClock(t *testing.T) {
	p := startupFixture(t)
	p.AMFI = &AMFIPolicy{OutputFlags: 0x1df}
	p.Reclaim = &ReclaimPolicy{"deadline-128-v1", 20}
	anchor := clockFixture()
	options := Options{StartupProfile: p, RuntimeProfile: clockProfile(), ClockAnchor: &anchor}
	process := process(t, options)
	p.Reclaim.IntervalTicks = 30
	if process.kernel.reclaim.policy.IntervalTicks != 20 || process.kernel.reclaim.clock == nil {
		t.Fatal("borrowed or missing private state")
	}
	for _, name := range []string{"missing-clock", "missing-signing", "wrong-frequency", "wrong-ABI", "wrong-binary"} {
		t.Run(name, func(t *testing.T) {
			p := startupFixture(t)
			p.AMFI = &AMFIPolicy{OutputFlags: 0x1df}
			p.Reclaim = &ReclaimPolicy{"deadline-128-v1", 20}
			o := Options{StartupProfile: p, RuntimeProfile: clockProfile(), ClockAnchor: &anchor}
			switch name {
			case "missing-clock":
				o.ClockAnchor = nil
			case "missing-signing":
				p.AMFI = nil
			case "wrong-frequency":
				p.Reclaim.IntervalTicks = 21
			case "wrong-ABI":
				p.Reclaim.ABI = "unknown"
			case "wrong-binary":
				p.BinarySHA256 = "wrong"
			}
			if _, e := Parse(machotest.Image(2), machotest.Image(7), o); e == nil {
				t.Fatal("uncaptured policy accepted")
			}
		})
	}
}
