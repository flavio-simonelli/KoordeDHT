package domain

import (
	"encoding/hex"
	"testing"
)

func mustHexID(s string, byteLen int) ID {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	// pad to ByteLen if necessario
	if len(b) < byteLen {
		padded := make([]byte, byteLen)
		copy(padded[byteLen-len(b):], b)
		return ID(padded)
	}
	return ID(b)
}

func TestNextDigitBaseK(t *testing.T) {
	tests := []struct {
		name       string
		bits       int
		graphGrade int
		aHex       string
		wantDigit  uint64
		wantRest   string
	}{
		{
			name:       "8-bit, base 2 (r=1)",
			bits:       8,
			graphGrade: 2,
			aHex:       "aa",
			wantDigit:  1,
			wantRest:   "54",
		},
		{
			name:       "8-bit, base 4 (r=2)",
			bits:       8,
			graphGrade: 4,
			aHex:       "c3",
			wantDigit:  3,
			wantRest:   "0c",
		},
		{
			name:       "16-bit, base 4 (r=2)",
			bits:       16,
			graphGrade: 4,
			aHex:       "abcd",
			wantDigit:  2,
			wantRest:   "af34",
		},
		{
			name:       "12-bit, base 4 (r=2, non byte-aligned)",
			bits:       12,
			graphGrade: 4,
			aHex:       "00f0",
			wantDigit:  0,
			wantRest:   "03c0",
		},
		{
			name:       "12-bit, base 8 (r=3, non byte-aligned)",
			bits:       12,
			graphGrade: 8,
			aHex:       "00f0",
			wantDigit:  0,
			wantRest:   "0780",
		},
		{
			name:       "12-bit, base 8 (r=3, non byte-aligned)",
			bits:       12,
			graphGrade: 8,
			aHex:       "0780",
			wantDigit:  3,
			wantRest:   "0c00",
		},
		{
			name:       "20-bit, base 16 (r=4, non byte-aligned)",
			bits:       32,
			graphGrade: 4,
			aHex:       "1c8502c0",
			wantDigit:  0,
			wantRest:   "72140b00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := Space{
				Bits:       tt.bits,
				ByteLen:    (tt.bits + 7) / 8,
				GraphGrade: tt.graphGrade,
			}
			a := mustHexID(tt.aHex, sp.ByteLen)

			gotDigit, gotRest, err := sp.NextDigitBaseK(a)
			if err != nil {
				t.Fatalf("NextDigitBaseK error: %v", err)
			}
			if gotDigit != tt.wantDigit {
				t.Errorf("digit = %d, want %d", gotDigit, tt.wantDigit)
			}
			gotRestHex := hex.EncodeToString(gotRest)
			if gotRestHex != tt.wantRest {
				t.Errorf("rest = %s, want %s", gotRestHex, tt.wantRest)
			}
		})
	}
}
