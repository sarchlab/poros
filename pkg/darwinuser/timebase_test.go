package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

func TestTimebaseTrapNativeCopyoutContract(t *testing.T) {
	var native struct {
		Numer, Denom uint32
		Cases        []struct {
			Case   int
			Return uint64
			Bytes  string
		}
	}
	kernelNativeResult(t, "timebase-trap-probe.c", &native)
	if len(native.Cases) != 10 {
		t.Fatal(len(native.Cases))
	}
	for _, tc := range native.Cases {
		t.Run(fmt.Sprint(tc.Case), func(t *testing.T) {
			m := &usermem.AddressSpace{}
			if e := m.Map(0x10000, 3*vmPage, usermem.Read|usermem.Write, bytes.Repeat([]byte{0xa5}, int(3*vmPage))); e != nil {
				t.Fatal(e)
			}
			address := uint64(0x14000)
			switch tc.Case {
			case 1:
				address++
			case 2:
				address = 0
			case 3:
				address = 1
			case 4:
				m.Protect(address, vmPage, usermem.Read, false)
			case 5:
				m.Protect(address, vmPage, 0, false)
			case 6:
				address -= 4
				m.Protect(0x14000, vmPage, usermem.Read, false)
			case 7:
				address -= 4
				m.Unmap(0x14000, vmPage)
			case 8:
				address -= 8
				m.Protect(0x14000, vmPage, 0, false)
			case 9:
				address -= 7
				m.Protect(0x14000, vmPage, usermem.Read, false)
			}
			r := emu.Registers{N: true, C: true, V: true}
			r.GPR[0] = address
			r.GPR[1] = 0x123456789
			r.GPR[16] = uint64(0xffffffa7)
			before := r
			k := kernelState{timebase: [2]uint32{native.Numer, native.Denom}}
			handled := k.trap(&r, m)
			partial := tc.Case == 6 || tc.Case == 7 || tc.Case == 9
			if partial {
				if handled || r != before {
					t.Fatal("partial copyout accepted")
				}
			} else {
				before.GPR[0] = tc.Return
				if !handled || r != before {
					t.Fatal("trap return/state", r, handled)
				}
			}
			got := bytes.Repeat([]byte{0xa5}, 8)
			for i := range got {
				var b [1]byte
				if m.Read(address+uint64(i), b[:]) == nil {
					got[i] = b[0]
				}
			}
			want, e := hex.DecodeString(tc.Bytes)
			if e != nil {
				t.Fatal(e)
			}
			if partial {
				want = bytes.Repeat([]byte{0xa5}, 8)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("copyout", got, want)
			}
		})
	}
}

func TestTimebaseTrapUsesOwnedRuntimeTuple(t *testing.T) {
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 100, TimebaseDenom: 3}
	p := process(t, Options{RuntimeProfile: &profile})
	profile.TimebaseNumer = 125
	r := emu.Registers{C: true}
	r.GPR[0] = p.Registers.SP - 16
	r.GPR[16] = uint64(0xffffffa7)
	if !p.kernel.trap(&r, p.Memory) || r.GPR[0] != 0 || !r.C {
		t.Fatal(r)
	}
	var data [8]byte
	if e := p.Memory.Read(p.Registers.SP-16, data[:]); e != nil || binary.LittleEndian.Uint32(data[:]) != 100 || binary.LittleEndian.Uint32(data[4:]) != 3 {
		t.Fatal(data, e)
	}
	for _, tuple := range [][2]uint32{{}, {1, 0}, {0, 1}} {
		k := kernelState{timebase: tuple}
		r.GPR[0] = p.Registers.SP - 16
		before := r
		if k.trap(&r, p.Memory) || r != before {
			t.Fatal("uncaptured timebase accepted")
		}
	}
}
