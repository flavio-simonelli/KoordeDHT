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
		aHex       string // input ID in hex
		wantDigit  uint64
		wantRest   string // expected rest in hex
	}{
		{
			name:       "8-bit, base 2 (r=1)",
			bits:       8,
			graphGrade: 2,
			aHex:       "aa", // 10101010
			wantDigit:  1,    // MSB = 1
			wantRest:   "54", // shift left 1 bit = 01010100
		},
		{
			name:       "8-bit, base 4 (r=2)",
			bits:       8,
			graphGrade: 4,
			aHex:       "c3", // 11000011
			wantDigit:  3,    // primi 2 bit = 11 (bin) = 3
			wantRest:   "0c", // shift left 2 → 00001100
		},
		{
			name:       "16-bit, base 4 (r=2)",
			bits:       16,
			graphGrade: 4,
			aHex:       "abcd", // 1010 1011 1100 1101
			wantDigit:  2,      // primi 2 bit = 10 (bin) = 2
			wantRest:   "af34", // risultato dello shift
		},
		{
			name:       "12-bit, base 4 (r=2, non byte-aligned)",
			bits:       12,
			graphGrade: 4,
			aHex:       "00f0", // 0000 1111 0000 (solo 12 bit validi)
			wantDigit:  0,      // primi 2 bit = 00
			wantRest:   "03c0", // shift left 2 → 0011 1100 0000
		},
		{
			name:       "12-bit, base 8 (r=3, non byte-aligned)",
			bits:       32,
			graphGrade: 4,
			aHex:       "c72140b0", // 0000 1111 0000 (solo 12 bit validi)
			wantDigit:  3,          // primi 3 bit = 000
			wantRest:   "1c8502c0", // shift left 3 → 0001 1110 0000
		},
		{
			name:       "20-bit, base 16 (r=4, non byte-aligned)",
			bits:       32,
			graphGrade: 4,
			aHex:       "1c8502c0", // 1010 1011 1100 1101 1110 (solo 20 bit validi)
			wantDigit:  0,          // primi 4 bit = 1010 (bin) = 10
			wantRest:   "72140b00", // shift left 4 → 1011 1100 1101 1110 0000
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
