package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func mapMessage(t *testing.T) (*kernelState, *usermem.AddressSpace, emu.Registers, []byte) {
	t.Helper()
	m := &usermem.AddressSpace{}
	request := make([]byte, 100)
	binary.LittleEndian.PutUint32(request[:4], 0x80001513)
	binary.LittleEndian.PutUint32(request[4:8], 100)
	binary.LittleEndian.PutUint32(request[8:12], 0x103)
	binary.LittleEndian.PutUint32(request[12:16], 0x10f)
	binary.LittleEndian.PutUint32(request[20:24], 4811)
	binary.LittleEndian.PutUint32(request[24:28], 1)
	request[38] = 19
	copy(request[40:48], nativeNDR[:])
	binary.LittleEndian.PutUint64(request[48:56], 0x200000000)
	binary.LittleEndian.PutUint64(request[56:64], 0x16c000)
	binary.LittleEndian.PutUint32(request[96:100], 1)
	if e := m.Map(0x1000, 128, usermem.Read|usermem.Write, request); e != nil {
		t.Fatal(e)
	}
	k := &kernelState{taskSendRefs: 1}
	reply := k.allocateReceivePort(receivePort{typedReply: true, queueLimit: 5})
	r := emu.Registers{C: true}
	r.GPR[16] = 0xffffffd1
	r.GPR[0] = 0x1000
	r.GPR[1] = 0x200000003
	r.GPR[2] = uint64(100)<<32 | 0x80001513
	r.GPR[3] = uint64(reply)<<32 | 0x103
	r.GPR[4] = uint64(4811) << 32
	r.GPR[5] = uint64(reply)<<32 | 1
	r.GPR[6] = 52
	return k, m, r, request
}

func TestMessageUnsupportedIsBeforeMutation(t *testing.T) {
	for _, tc := range []string{"options", "id", "descriptor", "object", "ndr", "offset", "copy", "inherit", "target", "reply", "flags"} {
		k, m, r, b := mapMessage(t)
		switch tc {
		case "options":
			r.GPR[1] |= 0x10
		case "id":
			r.GPR[4] += 1 << 32
		case "descriptor":
			r.GPR[5]++
		case "object":
			b[28] = 1
		case "ndr":
			b[44] = 0
		case "offset":
			b[76] = 1
		case "copy":
			b[84] = 1
		case "inherit":
			b[96] = 0
		case "target":
			r.GPR[3]++
		case "reply":
			r.GPR[3] += 4 << 32
			r.GPR[5] += 4 << 32
		case "flags":
			b[72] = 0x10
		}
		m.Write(0x1000, b)
		before := r
		if k.trap(&r, m) || r != before || k.anonymousBytes != 0 || len(m.Mappings()) != 1 {
			t.Fatal("unsupported mutated state", tc)
		}
		got := make([]byte, 100)
		m.Read(0x1000, got)
		for i := range b {
			if got[i] != b[i] {
				t.Fatal("buffer changed", tc)
			}
		}
	}
}

func TestMessageCopiesOnlyTailAndPreservesEffectsOnBadOutput(t *testing.T) {
	k, m, r, b := mapMessage(t)
	m.Unmap(0x1000, 128)
	m.Map(0x1000+28, 72, usermem.Read, b[28:])
	old := r
	if !k.trap(&r, m) || r.GPR[0] != 0x10004008 || !r.C || r.GPR[1] != old.GPR[1] || k.anonymousBytes != 0x16c000 {
		t.Fatal("copyin/header or copyout order", r, k)
	}
	found := false
	for _, v := range m.Mappings() {
		if v.Start == 0x200000000 {
			found = true
			if v.Flags != 0 || m.Protect(v.Start, v.End-v.Start, usermem.Read, false) == nil {
				t.Fatal(v)
			}
		}
	}
	if !found {
		t.Fatal("failed receive undid VM map")
	}
}

func TestMessageVMMapNativeContract(t *testing.T) {
	var ob struct {
		Cases []struct {
			Case           int
			Return         uint64 `json:"trap_return"`
			Kind           string `json:"response_kind"`
			ReceiveSize    uint64 `json:"receive_size"`
			Suffix         bool   `json:"suffix_preserved"`
			Mapped         bool   `json:"hint_mapped"`
			Protection     int    `json:"hint_protection"`
			MaxProtection  int    `json:"hint_max_protection"`
			ReplyBits      uint32 `json:"reply_bits"`
			ReplySize      uint32 `json:"reply_size"`
			RemoteNull     bool   `json:"reply_remote_null"`
			LocalMatches   bool   `json:"reply_local_matches"`
			Voucher        uint32 `json:"reply_voucher"`
			ID             uint32 `json:"reply_id"`
			NDR            string `json:"reply_ndr_hex"`
			Kernel         uint32 `json:"kernel_return"`
			AddressMatches bool   `json:"address_matches_hint"`
			Trailer        string `json:"trailer_hex"`
			Raw            string `json:"raw_bytes_24_48_hex"`
		}
	}
	kernelNativeResult(t, "mach-msg-map-probe.c", &ob)
	if len(ob.Cases) != 31 {
		t.Fatal(len(ob.Cases))
	}
	for _, tc := range ob.Cases {
		t.Run(fmt.Sprint(tc.Case), func(t *testing.T) {
			k, m, r, b := mapMessage(t)
			const hint uint64 = 0x200000000
			switch tc.Case {
			case 1:
				m.MapProtected(hint, 0x16c000, usermem.Read|usermem.Write, 7, nil)
			case 2:
				clear(b[56:64])
			case 3:
				b[88] = 1
			case 4:
				b[72] = 1
				b[88] = 3
				b[92] = 3
			case 5:
				b[92] = 8
			case 10:
				clear(b[:24])
			case 11:
				clear(b[24:28])
			case 18:
				r.GPR[0] = 1
			}
			if tc.Case >= 19 {
				for i := 0; i < 28; i++ {
					b[i] = 0xa5
				}
			}
			m.Write(0x1000, b)
			var before [128]byte
			m.Read(0x1000, before[:])
			r.GPR[6] = tc.ReceiveSize
			if tc.Case == 8 {
				m.Protect(0x1000, 128, usermem.Read, false)
			}
			old := r
			if !k.trap(&r, m) || r.GPR[0] != tc.Return || r.GPR[1] != old.GPR[1] || !r.C {
				t.Fatal(tc, r)
			}
			var got [128]byte
			m.Read(0x1000, got[:])
			u := binary.LittleEndian
			if tc.Kind != "none" {
				if u.Uint32(got[:4]) != tc.ReplyBits || u.Uint32(got[4:8]) != tc.ReplySize || (u.Uint32(got[8:12]) == 0) != tc.RemoteNull || (u.Uint32(got[12:16]) == 0x10f) != tc.LocalMatches || u.Uint32(got[16:20]) != tc.Voucher || u.Uint32(got[20:24]) != tc.ID {
					t.Fatal("reply header", tc, got[:24])
				}
				if hex.EncodeToString(got[tc.ReplySize:tc.ReplySize+8]) != tc.Trailer {
					t.Fatal("trailer", tc, got)
				}
			}
			written := uint32(0)
			if tc.Kind == "mig_reply" {
				written = tc.ReplySize + 8
				if hex.EncodeToString(got[24:32]) != tc.NDR || u.Uint32(got[32:36]) != tc.Kernel {
					t.Fatal("MIG result", tc, got)
				}
				if tc.Kernel == 0 && (u.Uint64(got[36:44]) == hint) != tc.AddressMatches {
					t.Fatal("reply address", tc, got)
				}
			} else {
				if hex.EncodeToString(got[24:48]) != tc.Raw {
					t.Fatal("partial copyout", tc, hex.EncodeToString(got[24:48]))
				}
				if tc.Kind == "header_only" {
					written = uint32(min(tc.ReceiveSize, 32))
				}
			}
			if !tc.Suffix || !bytes.Equal(got[written:], before[written:]) {
				t.Fatal("bytes outside reply changed", tc)
			}
			mapped := false
			for _, v := range m.Mappings() {
				if v.Start <= hint && hint < v.End {
					mapped = true
					if v.Flags != machPermissions(uint32(tc.Protection)) {
						t.Fatal("current protection", tc, v)
					}
					for bit := usermem.Permission(1); bit <= 4; bit <<= 1 {
						allowed := m.Protect(v.Start, v.End-v.Start, bit, false) == nil
						if allowed != (machPermissions(uint32(tc.MaxProtection))&bit != 0) {
							t.Fatal("maximum protection", tc, bit)
						}
					}
				}
			}
			if mapped != tc.Mapped || len(k.receivePorts) != 1 || k.taskSendRefs != 1 {
				t.Fatal("kernel effects", tc, k)
			}
		})
	}
}

func TestMessageMixedPermissionOutputFailsBeforeVMEffect(t *testing.T) {
	k, m, r, b := mapMessage(t)
	m.Unmap(0x1000, 128)
	m.Map(0x1000, 40, usermem.Read|usermem.Write, b[:40])
	m.Map(0x1028, 60, usermem.Read, b[40:])
	old := r
	if k.trap(&r, m) || r != old || k.anonymousBytes != 0 || len(m.Mappings()) != 2 {
		t.Fatal("partial-copyout contract was guessed", r, k)
	}
}
