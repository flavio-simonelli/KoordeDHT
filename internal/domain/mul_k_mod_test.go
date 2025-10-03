package domain

import (
	"encoding/hex"
	"testing"
)

// helper per convertire da hex string a ID
func mustHex(s string) ID {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return ID(b)
}

func TestMulKMod(t *testing.T) {
	tests := []struct {
		name       string
		bits       int
		graphGrade int
		aHex       string // input ID in hex
		wantHex    string // expected ID in hex
	}{
		{
			name:       "32-bit *4",
			bits:       32,
			graphGrade: 4,
			aHex:       "71c8502c",
			wantHex:    "c72140b0", // (0x71c8502c * 4) mod 2^32
		},
		{
			name:       "8-bit *3 overflow",
			bits:       8,
			graphGrade: 3,
			aHex:       "ff", // 255
			wantHex:    "fd", // (255*3) mod 256 = 765 mod 256 = 253
		},
		{
			name:       "16-bit *2 with overflow",
			bits:       16,
			graphGrade: 2,
			aHex:       "ffff", // 65535
			wantHex:    "fffe", // (65535*2) mod 2^16 = 131070 mod 65536 = 65534
		},
		{
			name:       "12-bit (not byte aligned) *2",
			bits:       12,
			graphGrade: 2,
			aHex:       "0fff", // 4095 (12 bits all 1)
			wantHex:    "0ffe", // (4095*2) mod 2^12 = 8190 mod 4096 = 4094
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := Space{
				Bits:       tt.bits,
				ByteLen:    (tt.bits + 7) / 8,
				GraphGrade: tt.graphGrade,
			}
			a := mustHex(tt.aHex)
			got, err := sp.MulKMod(a)
			if err != nil {
				t.Fatalf("MulKMod returned error: %v", err)
			}
			gotHex := hex.EncodeToString(got)
			if gotHex != tt.wantHex {
				t.Errorf("MulKMod(%s) = %s, want %s", tt.aHex, gotHex, tt.wantHex)
			}
		})
	}
}
