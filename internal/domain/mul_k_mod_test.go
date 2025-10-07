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
		aHex       string
		wantHex    string
	}{
		{
			name:       "32-bit *4",
			bits:       32,
			graphGrade: 4,
			aHex:       "71c8502c",
			wantHex:    "c72140b0",
		},
		{
			name:       "8-bit *3 overflow",
			bits:       8,
			graphGrade: 3,
			aHex:       "ff",
			wantHex:    "fd",
		},
		{
			name:       "16-bit *2 with overflow",
			bits:       16,
			graphGrade: 2,
			aHex:       "ffff",
			wantHex:    "fffe",
		},
		{
			name:       "12-bit (not byte aligned) *2",
			bits:       12,
			graphGrade: 2,
			aHex:       "0fff",
			wantHex:    "0ffe",
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
